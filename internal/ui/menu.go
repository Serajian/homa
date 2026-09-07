package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/Serajian/homa/internal/config"
	"github.com/Serajian/homa/internal/contacts"
	"github.com/Serajian/homa/internal/peer"
	"github.com/Serajian/homa/internal/session"
)

// App is the running program: everything the menu needs in one place.
type App struct {
	ui       *UI
	book     *contacts.Book
	id       *peer.Identity
	listener *peer.Listener

	// mu guards cfg, which the accept goroutine reads while the person
	// edits it from the menu. The book guards itself.
	mu  sync.RWMutex
	cfg *config.Config

	// incoming holds a call that arrived while the person was at the
	// menu. It holds one, because a second caller while one is already
	// waiting has nowhere sensible to queue.
	incoming chan call
}

// call is a conversation that has been greeted but not yet joined.
//
// The handshake happens the moment a call arrives, not when the person
// answers it. A caller waits fifteen seconds for a greeting, and nobody
// reaches the keyboard that fast: parking an ungreeted connection meant
// every call timed out before it could be picked up.
type call struct {
	conn    net.Conn
	name    string
	session *session.Session
	handler *chatHandler
}

// NewApp assembles the program. It does not start anything.
func NewApp(
	u *UI,
	cfg *config.Config,
	book *contacts.Book,
	id *peer.Identity,
	l *peer.Listener,
) *App {
	return &App{
		ui:       u,
		cfg:      cfg,
		book:     book,
		id:       id,
		listener: l,
		incoming: make(chan call, 1),
	}
}

// nick is the display name, read under the lock because settings can change
// while a call is being answered on another goroutine.
func (a *App) nick() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.cfg.Nick
}

// downloadDir prepares the directory received files go into. It is passed
// to a chat as a function so that nothing outside this file has to know
// about the lock.
func (a *App) downloadDir() (string, error) {
	a.mu.RLock()
	cfg := *a.cfg
	a.mu.RUnlock()

	return cfg.EnsureDownloadDir()
}

// Run starts listening for callers and shows the menu until the person
// quits or the context is canceled.
func (a *App) Run(ctx context.Context) error {
	go a.acceptLoop(ctx)

	a.ui.Blank()
	a.ui.Printf("homa | %s", a.nick())
	a.ui.Info("your address starts with %s", preview(a.listener.Addr()))
	a.ui.Info("listening for callers")

	return a.menuLoop(ctx)
}

// acceptLoop takes calls as they arrive, greets them, and parks them.
//
// The menu picks a parked call up at once, because it waits on the keyboard
// and on this channel together. A call still waits when the person is
// already in a conversation or answering a prompt, and greeting it first
// means the caller is connected while it waits rather than timing out.
func (a *App) acceptLoop(ctx context.Context) {
	for {
		conn, err := a.listener.Accept(ctx)
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, peer.ErrClosed) {
				a.ui.Warn("could not accept a call: %v", err)
			}
			return
		}

		a.greet(ctx, conn)
	}
}

// greet completes the handshake on an incoming connection and parks it for
// the person to pick up.
func (a *App) greet(ctx context.Context, conn net.Conn) {
	name := a.describe(conn)
	handler := newChatHandler(a.ui, a.downloadDir, name)

	s, err := session.Start(conn, a.nick(), handler)
	if err != nil {
		// The caller hung up, or is not speaking homa. Not worth
		// interrupting the person over.
		lg.Debug("could not greet a caller", "err", err)
		_ = conn.Close()
		return
	}

	c := call{conn: conn, name: name, session: s, handler: handler}

	select {
	case a.incoming <- c:
		a.ui.Blank()
		a.ui.Info("%s is calling.", name)

	case <-ctx.Done():
		_ = s.Close()

	default:
		a.ui.Warn("%s called while another call was waiting", name)
		a.turnAway(s)
	}
}

// turnAway tells a caller why they are being hung up on, rather than
// dropping the connection and leaving them to guess.
func (a *App) turnAway(s *session.Session) {
	if err := s.SendText("busy: another call is already waiting"); err != nil {
		lg.Debug("could not tell a caller we are busy", "err", err)
	}
	_ = s.Close()
}

// menuLoop is the main screen.
//
// It waits on three things at once, which is the whole reason the input
// pump exists: a line the person typed, a call that has arrived, and the
// program being shut down. A call is answered the moment it lands, without
// waiting for a keypress, and Ctrl+C returns from here immediately rather
// than after the next Enter.
func (a *App) menuLoop(ctx context.Context) error {
	for {
		// A call parked while we were busy takes priority over showing
		// the menu again: the caller is waiting. Checking here as well
		// as in the select keeps the menu from being printed and then
		// replaced a moment later.
		if c, ok := a.takeIncoming(); ok {
			a.answer(ctx, c)
			continue
		}

		keys, labels, list := a.menuEntries()

		if err := a.ui.ShowMenu("What now?", keys, labels); err != nil {
			return err
		}

		select {
		case c := <-a.incoming:
			a.answer(ctx, c)

		case line, ok := <-a.ui.Lines():
			if !ok {
				return nil // the person pressed Ctrl+D
			}
			if quit := a.act(ctx, strings.ToLower(line), list); quit {
				return nil
			}

		case <-ctx.Done():
			return nil
		}
	}
}

// menuEntries builds the menu: saved contacts first, numbered, then the
// fixed actions. list maps a numeric key back to the contact it stands for.
func (a *App) menuEntries() (keys, labels []string, list []contacts.Contact) {
	list = a.book.All()

	for i, c := range list {
		keys = append(keys, strconv.Itoa(i+1))
		labels = append(labels, "call "+c.Name)
	}

	keys = append(keys, "n", "a", "s", "q")
	labels = append(labels,
		"add a contact",
		"show my address",
		"settings",
		"quit",
	)

	return keys, labels, list
}

// act performs one menu choice, reporting whether the person is leaving.
func (a *App) act(ctx context.Context, choice string, list []contacts.Contact) (quit bool) {
	if choice == "" {
		// A bare Enter is somebody looking again. Redrawing the menu is
		// the whole response.
		return false
	}

	switch choice {
	case "n":
		a.addContact(ctx)
	case "a":
		a.showAddress()
	case "s":
		a.editSettings(ctx)
	case "q":
		return true
	default:
		// Atoi returns 0 for anything that is not a number, and 0 is
		// already outside the valid range, so the range check alone
		// covers both a bad number and no number at all.
		n, _ := strconv.Atoi(choice)
		if n < 1 || n > len(list) {
			a.ui.Warn("that is not one of the choices")
			return false
		}
		a.dial(ctx, list[n-1])
	}
	return false
}

// editSettings walks the settings questions and swaps in the result.
// Backing out is a decision rather than a failure, so it is silent.
func (a *App) editSettings(ctx context.Context) {
	a.mu.RLock()
	current := a.cfg
	a.mu.RUnlock()

	// The questions are asked without the lock held: someone thinking
	// about their answer must not block an incoming call.
	updated, err := EditSettings(ctx, a.ui, current)
	if err != nil {
		if !errors.Is(err, ErrCanceled) {
			a.ui.Warn("%v", err)
		}
		return
	}

	a.mu.Lock()
	a.cfg = updated
	a.mu.Unlock()
}

// ------------------------------------------------------------------ calls

// dial calls a saved contact and hands the connection to the chat.
func (a *App) dial(ctx context.Context, c contacts.Contact) {
	a.ui.Info("calling %s...", c.Name)

	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	conn, err := peer.Dial(dialCtx, a.id, c.Addr)
	if err != nil {
		a.ui.Warn("could not reach %s: %v", c.Name, err)
		return
	}

	// Now that we have spoken to them, remember the key that answered, so
	// their next call can be shown under this name.
	a.rememberKey(c.Name, peer.RemoteKey(conn))

	// startChat owns conn from here and closes it.
	a.startChat(ctx, conn, c.Name)
}

// answer joins a call that was greeted and parked by the accept goroutine.
func (a *App) answer(ctx context.Context, c call) {
	a.ui.Info("connected to %s", c.name)
	a.runChat(ctx, c.conn, c.session, c.handler, c.name)
}

// takeIncoming returns a parked call if there is one, without waiting.
func (a *App) takeIncoming() (call, bool) {
	select {
	case c := <-a.incoming:
		return c, true
	default:
		return call{}, false
	}
}

// describe names whoever is on a connection, using the key rather than
// anything they claim. An unknown key is said plainly, because "someone" is
// honest and a made-up name would not be.
func (a *App) describe(conn net.Conn) string {
	key := peer.RemoteKey(conn)
	if key == "" {
		return "someone unrecognized"
	}
	if c, ok := a.book.ByPubKey(key); ok {
		return c.Name
	}
	return "someone not in your contacts"
}

// rememberKey records a contact's key the first time we see it.
func (a *App) rememberKey(name, key string) {
	if key == "" {
		return
	}

	changed, err := a.book.SetPubKey(name, key)
	if err != nil || !changed {
		return
	}
	if err := a.book.Save(); err != nil {
		a.ui.Warn("could not save the address book: %v", err)
	}
}

// --------------------------------------------------------------- contacts

// addContact asks for a name and an address and saves them.
func (a *App) addContact(ctx context.Context) {
	name, err := a.ui.Ask(ctx, "A name for them", "")
	if err != nil {
		return
	}

	addr, err := a.ui.Ask(ctx, "Their address", "")
	if err != nil {
		return
	}
	if !peer.ValidAddr(addr) {
		a.ui.Warn("that does not look like a homa address")
		return
	}

	if err := a.book.Add(contacts.Contact{Name: name, Addr: addr}); err != nil {
		a.ui.Warn("%v", err)
		return
	}
	if err := a.book.Save(); err != nil {
		a.ui.Warn("could not save the address book: %v", err)
		return
	}

	a.ui.Info("%s added.", name)
}

// showAddress prints the whole address, which is the only time it is shown
// in full. Everywhere else it appears shortened, because it is a secret.
func (a *App) showAddress() {
	a.ui.Blank()
	a.ui.Info("Give this to someone who should be able to reach you.")
	a.ui.Info("Treat it like a password: whoever has it can call you.")
	a.ui.Blank()
	a.ui.Printf("%s", a.listener.Addr())
	a.ui.Blank()
}

// preview shortens an address for a header.
func preview(addr string) string {
	if len(addr) <= addrPreviewLen {
		return addr
	}
	return fmt.Sprintf("%s...", addr[:addrPreviewLen])
}
