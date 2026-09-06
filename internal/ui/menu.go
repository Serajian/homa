package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
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

// call is a connection together with the name to show for it, worked out
// once when it arrives rather than again when it is answered.
type call struct {
	conn net.Conn
	name string
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

// acceptLoop takes calls as they arrive. It runs for the life of the
// program, in parallel with whatever the person is doing.
//
// A caller cannot interrupt a blocked read on the terminal, so an arriving
// call is announced and parked. The menu picks it up the next time the
// person presses a key. That is the price of a line-based interface; a
// full-screen one would answer without waiting.
func (a *App) acceptLoop(ctx context.Context) {
	for {
		conn, err := a.listener.Accept(ctx)
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, peer.ErrClosed) {
				a.ui.Warn("could not accept a call: %v", err)
			}
			return
		}

		incoming := call{conn: conn, name: a.describe(conn)}

		select {
		case a.incoming <- incoming:
			a.ui.Blank()
			a.ui.Info("%s is calling. Press Enter to answer.", incoming.name)
		default:
			a.ui.Warn("%s called while another call was waiting", incoming.name)
			go a.turnAway(conn)
		}
	}
}

// menuLoop is the main screen.
func (a *App) menuLoop(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		// A call parked while we were busy takes priority over showing
		// the menu again: the caller is waiting.
		if c, ok := a.takeIncoming(); ok {
			a.answer(ctx, c)
			continue
		}

		keys, labels, list := a.menuEntries()

		choice, err := a.ui.ChooseKeyed("What now?", keys, labels)
		if errors.Is(err, ErrCanceled) {
			return nil
		}
		if err != nil {
			return err
		}

		// Pressing a key may have been the person answering a call that
		// arrived while they were reading the menu.
		if c, ok := a.takeIncoming(); ok {
			a.answer(ctx, c)
			continue
		}

		if quit := a.act(ctx, choice, list); quit {
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
	switch choice {
	case "n":
		a.addContact()
	case "a":
		a.showAddress()
	case "s":
		a.editSettings()
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
func (a *App) editSettings() {
	a.mu.RLock()
	current := a.cfg
	a.mu.RUnlock()

	// The questions are asked without the lock held: someone thinking
	// about their answer must not block an incoming call.
	updated, err := EditSettings(a.ui, current)
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

	// chat owns conn from here and closes it.
	a.chat(ctx, conn, c.Name)
}

// answer takes a call that was parked by acceptLoop.
func (a *App) answer(ctx context.Context, c call) {
	a.ui.Info("connected to %s", c.name)
	a.chat(ctx, c.conn, c.name)
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
func (a *App) addContact() {
	name, err := a.ui.Ask("A name for them", "")
	if err != nil {
		return
	}

	addr, err := a.ui.Ask("Their address", "")
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

// turnAway tells a caller why they are being hung up on, rather than
// dropping the connection and leaving them to guess.
//
// It runs in its own goroutine because the greeting has a deadline of its
// own, and the accept loop must stay free for the next caller.
func (a *App) turnAway(conn net.Conn) {
	s, err := session.Start(conn, a.nick(), silentHandler{})
	if err != nil {
		_ = conn.Close()
		return // they hung up, or never greeted us
	}

	if err := s.SendText("busy: another call is already waiting"); err != nil {
		lg.Debug("could not tell a caller we are busy", "err", err)
	}
	_ = s.Close()
}

// silentHandler ignores everything. It is for a session that exists only
// long enough to say one thing.
type silentHandler struct{}

func (silentHandler) OnText(string) {}
