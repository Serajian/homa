package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	incoming chan *call
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
	known   bool // the name came from the address book, not from the caller
	session *session.Session
	handler *chatHandler

	// deadline is when the caller stops being made to wait, counted from
	// when the call arrived.
	deadline time.Time

	// claimed is how the person and the deadline avoid both taking the same
	// call. Whoever swaps it first owns it; the other finds it gone.
	claimed atomic.Bool
}

// claim takes ownership of a call, reporting whether it was still available.
//
// A call is passed by pointer from here on. Two goroutines race for it, so
// there is one of each call and not a copy per holder.
func (c *call) claim() bool { return c.claimed.CompareAndSwap(false, true) }

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
		incoming: make(chan *call, 1),
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
	a.ui.Welcome()
	a.ui.Blank()
	a.ui.Info("you are %s", a.nick())
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
	name, known := a.describe(conn)
	handler := newChatHandler(a.ui, a.downloadDir, name)

	s, err := session.Start(conn, a.nick(), handler)
	if err != nil {
		// The caller hung up, or is not speaking homa. Not worth
		// interrupting the person over.
		lg.Debug("could not greet a caller", "err", err)
		_ = conn.Close()
		return
	}

	// A caller the address book does not know is shown by the name they
	// announced, marked so it cannot pass for one of yours. The nick is
	// only available once the handshake is done, which is why this is not
	// settled in describe. Writing it here is safe: the handler is not
	// read from until Run starts, and that is later still.
	if !known {
		name = unknownMark + s.Peer().Nick
		handler.name = name
	}

	c := &call{
		conn:     conn,
		name:     name,
		known:    known,
		session:  s,
		handler:  handler,
		deadline: time.Now().Add(callAnswerTimeout),
	}

	select {
	case a.incoming <- c:
		a.ui.Blank()
		a.ui.Info("%s is calling (expires in %s).", a.ui.peer(name), callAnswerTimeout)
		go a.expire(ctx, c)

	case <-ctx.Done():
		_ = s.Close()

	default:
		a.ui.Warn("%s called while another call was waiting", name)
		a.turnAway(s)
	}
}

// expire hangs up on a call nobody got to in time.
//
// It runs for every parked call, because the person may be in a conversation
// that outlasts the caller's patience and never see the question at all. The
// call value stays in the channel either way; whoever picks it up afterwards
// finds it already claimed and passes over it.
func (a *App) expire(ctx context.Context, c *call) {
	t := time.NewTimer(time.Until(c.deadline))
	defer t.Stop()

	select {
	case <-t.C:
		if !c.claim() {
			return // somebody was already dealing with it
		}
		a.decline(c, "no answer", "the call from %s ran out of time while you were busy.")

	case <-ctx.Done():
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
			a.offer(ctx, c)
			continue
		}

		groups, list := a.menuEntries()

		if err := a.ui.ShowMenu("What now?", groups...); err != nil {
			return err
		}

		select {
		case c := <-a.incoming:
			a.offer(ctx, c)

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

// menuEntries builds the menu in four groups: the people you can call,
// numbered, first, because calling somebody is what this screen is for;
// then the address book; then homa itself; and last, set apart and quiet,
// the two ways out. list maps a numeric key back to the contact it stands
// for.
func (a *App) menuEntries() (groups [][]menuItem, list []contacts.Contact) {
	list = a.book.All()

	var people []menuItem
	for i, c := range list {
		people = append(people, menuItem{key: strconv.Itoa(i + 1), text: "call %s", name: c.Name})
	}

	return [][]menuItem{
		people,
		{
			{key: "n", text: "add a contact"},
			{key: "b", text: "contacts: rename, forget, call"},
			{key: "a", text: "show my address"},
		},
		{
			{key: "s", text: "settings"},
			{key: "c", text: "clear the screen"},
			{key: "h", text: "help"},
		},
		{
			{key: "r", text: "start over: forget everything", quiet: true},
			{key: "q", text: wordQuit, quiet: true},
		},
	}, list
}

// act performs one menu choice, reporting whether the person is leaving.
func (a *App) act(ctx context.Context, choice string, list []contacts.Contact) (quit bool) {
	if choice == "" {
		// A bare Enter is somebody looking again. Redrawing the menu is
		// the whole response.
		return false
	}

	switch choice {
	case "b", "contacts":
		return a.contactsScreen(ctx)
	case "h", "help":
		a.showHelp()
	case "c", "clear":
		// The word as well as the letter: it is what somebody who has
		// used a shell will type, and it costs one case.
		//
		// The menu is drawn again by the loop this returns to, so this
		// only has to take away what was above it.
		a.ui.Clear()
	case "r":
		return a.reset(ctx)
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
			a.ui.Info("press one of the keys on the left, or h for help")
			return false
		}
		a.dial(ctx, list[n-1])
	}
	return false
}

// reset throws away everything homa has saved, reporting whether the person
// is leaving.
//
// It always leaves when it did anything. The identity is loaded once at
// startup and the listener is bound to it, so homa cannot go on with the key
// deleted: the address on the screen would be one nobody can reach. Starting
// homa again is the first run, which is the whole point of the command.
func (a *App) reset(ctx context.Context) (quit bool) {
	a.ui.Blank()
	a.ui.Warn("This deletes your identity, your address book and your settings.")
	a.ui.Warn("Your address changes, and everyone who saved the old one can no")
	a.ui.Warn("longer reach you.")

	// A word rather than a yes or no. There is no undo here, and a
	// single letter is answered by reflex; typing "reset" is not.
	// Anything else, Enter included, leaves everything alone.
	answer, err := a.ui.Ask(ctx, "type the word reset to confirm", "cancel")
	if err != nil {
		return false // shutting down, or the input ended
	}
	if answer != "reset" {
		a.ui.Info("nothing was deleted.")
		return false
	}

	// Every one is attempted even if an earlier one fails, so a reset that
	// goes wrong halfway leaves as little behind as it can. What could not
	// be removed is named: a person told "reset failed" does not know
	// whether their key is still on the disk.
	failed := false
	for _, f := range []struct {
		what   string
		remove func() error
	}{
		{"your identity", peer.RemoveIdentity},
		{"your address book", contacts.Remove},
		{"your settings", config.Remove},
	} {
		if err := f.remove(); err != nil {
			a.ui.Warn("could not delete %s: %v", f.what, err)
			failed = true
		}
	}

	a.ui.Blank()
	if failed {
		// Do not claim more than happened. What went is above, named.
		a.ui.Warn("some of it is still on the disk; see above.")
	} else {
		a.ui.Info("your identity, your address book and your settings are gone.")
	}
	a.ui.Info("start homa again and it will ask the first-run questions.")

	return true
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
	a.ui.Info("calling %s...", a.ui.peer(c.Name))

	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	conn, err := peer.Dial(dialCtx, a.id, c.Addr)
	if err != nil {
		a.ui.Warn("could not reach %s: %v", a.ui.peer(c.Name), err)
		a.ui.Info("they may not be running homa right now; their address has not changed")
		return
	}

	// Now that we have spoken to them, remember the key that answered, so
	// their next call can be shown under this name.
	a.rememberKey(c.Name, peer.RemoteKey(conn))

	// startChat owns conn from here and closes it.
	a.startChat(ctx, conn, c.Name)
}

// offer asks before putting a call through. Answering the telephone is the
// person's decision, not the program's: a stranger having your address is
// not the same as being welcome to talk.
//
// The greeting has already happened, so the caller is connected and not
// timing out while the question sits on screen. What they are not yet is
// listened to.
//
// Refusing is the default. A keypress left over from the menu should not be
// able to let somebody in, and a call refused by accident can be made again,
// while one accepted by accident cannot be taken back.
func (a *App) offer(ctx context.Context, c *call) {
	if !c.claim() {
		return // it ran out of time while it sat in the channel
	}

	// The question gets the deadline; the conversation must not. People
	// talk for longer than they take to answer a telephone, so this is a
	// context of its own rather than a narrowing of the one passed in.
	//
	// The deadline is the same one the caller is counting against, and what
	// it counts from is why the call carries it rather than starting here.
	askCtx, cancel := context.WithDeadline(ctx, c.deadline)
	defer cancel()

	a.ui.Blank()

	take, err := a.ui.ConfirmBy(askCtx,
		fmt.Sprintf("take the call from %s?", a.ui.peer(c.name)), false, c.deadline)
	if err != nil {
		// The deadline passed, or homa is shutting down, or the input
		// ended. None of them is an answer, so the caller is told rather
		// than left holding an open line.
		a.decline(c, "no answer", "the call from %s went unanswered.")
		return
	}

	if !take {
		a.decline(c, "they are not taking calls right now", "the call from %s was not taken.")
		return
	}

	if err := c.session.SendAccept(); err != nil {
		// They went while the question was on screen. Nothing to join.
		a.ui.Warn("%s went before the call could be connected", a.ui.peer(c.name))
		_ = c.session.Close()
		return
	}

	a.answer(ctx, c)
}

// decline hangs up on a caller and says why, in the same words the caller
// would hear if the line were busy. Guessing why a call died is worse than
// being told.
//
// It says so on this side too, with format taking the caller's name, so an
// announcement of a call does not sit on the screen outliving the call.
func (a *App) decline(c *call, reason, format string) {
	if err := c.session.SendText(reason); err != nil {
		lg.Debug("could not tell a caller they were turned down", "err", err)
	}
	_ = c.session.Close()

	a.ui.Info(format, a.ui.peer(c.name))
}

// answer joins a call that was greeted, parked, agreed to, and told so.
//
// ctx here is the one offer put a deadline on, and the conversation must not
// inherit it: people talk for longer than they take to answer the telephone.
func (a *App) answer(ctx context.Context, c *call) {
	a.ui.Info("connected to %s", a.ui.peer(c.name))
	a.runChat(ctx, c.conn, c.session, c.handler, c.name, c.known)
}

// takeIncoming returns a parked call if there is one, without waiting.
func (a *App) takeIncoming() (*call, bool) {
	select {
	case c := <-a.incoming:
		return c, true
	default:
		return nil, false
	}
}

// describe names whoever is on a connection, using the key rather than
// anything they claim. An unknown key is said plainly, because "someone" is
// honest and a made-up name would not be.
func (a *App) describe(conn net.Conn) (name string, known bool) {
	if key := peer.RemoteKey(conn); key != "" {
		if c, ok := a.book.ByPubKey(key); ok {
			return c.Name, true
		}
		return "", false
	}

	// An accepted call brings no key, only the tunnel address it came from,
	// which carries the start of one. Enough to choose a label; see
	// peer.RemoteKeyPrefix for what a prefix does and does not prove.
	if prefix := peer.RemoteKeyPrefix(conn); prefix != "" {
		if c, ok := a.book.ByPubKeyPrefix(prefix); ok {
			return c.Name, true
		}
	}

	return "", false
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
		a.ui.Info("it is the long line a) shows on their side; paste all of it")
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
