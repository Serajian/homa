package ui

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Serajian/homa/internal/contacts"
	"github.com/Serajian/homa/internal/peer"
	"github.com/Serajian/homa/internal/session"
)

// line is a connection that has been greeted: the session, what we call
// the far side, and — for a caller waiting to be taken — when the wait
// runs out. It is passed by pointer, because a parked call is raced for
// by the person and its deadline, and there must be one of it.
type line struct {
	conn  net.Conn
	s     *session.Session
	name  string
	known bool // the name came from the address book, not from the caller

	// deadline is when a caller stops being made to wait, counted from
	// when the call arrived, because that is what the caller is counting.
	deadline time.Time

	// claimed is how the person and the deadline avoid both taking the
	// same call. Whoever swaps it first owns it; the other finds it gone.
	claimed atomic.Bool
}

// claim takes ownership of a parked call, reporting whether it was still
// there to take.
func (l *line) claim() bool { return l.claimed.CompareAndSwap(false, true) }

// acceptLoop greets callers for the life of the program. It is one
// long-running command: it sends a message for every call and returns only
// when homa is shutting down.
func acceptLoop(ctx context.Context, deps Deps, send func(tea.Msg)) tea.Cmd {
	return func() tea.Msg {
		for {
			conn, err := deps.Listener.Accept(ctx)
			if err != nil {
				if ctx.Err() == nil && !errors.Is(err, peer.ErrClosed) {
					send(warnMsg("could not accept a call: " + err.Error()))
				}
				return nil
			}
			greet(ctx, deps, conn, send)
		}
	}
}

// greet completes the handshake on an incoming connection the moment it
// arrives, not when the person answers: a caller waits fifteen seconds for
// a greeting, and nobody reaches the keyboard that fast. The greeted call
// is then handed to the model to park, and a timer hangs up on it if
// nobody gets to it in time.
func greet(ctx context.Context, deps Deps, conn net.Conn, send func(tea.Msg)) {
	name, known := describe(deps.Book, conn)

	s, err := session.Start(conn, deps.Cfg.Nick, newAdapter(send))
	if err != nil {
		// The caller hung up, or is not speaking homa. Not worth
		// interrupting the person over.
		lg.Debug("could not greet a caller", "err", err)
		_ = conn.Close()
		return
	}

	// A caller the address book does not know is shown by the name they
	// announced, marked so it cannot pass for one of yours. The nick is
	// only known once the handshake is done, which is why describe cannot
	// settle it.
	if !known {
		name = unknownMark + s.Peer().Nick
	}

	l := &line{
		conn:     conn,
		s:        s,
		name:     name,
		known:    known,
		deadline: time.Now().Add(callAnswerTimeout),
	}
	send(callArrived{l})
	go expire(ctx, l, send)
}

// expire hangs up on a call nobody got to in time. It runs for every parked
// call, because the person may be in a conversation that outlasts the
// caller's patience and never see the bar at all.
func expire(ctx context.Context, l *line, send func(tea.Msg)) {
	t := time.NewTimer(time.Until(l.deadline))
	defer t.Stop()

	select {
	case <-t.C:
		if !l.claim() {
			return // somebody was already dealing with it
		}
		hangUp(l, "no answer")
		send(callGone{l})
	case <-ctx.Done():
	}
}

// describe names whoever is on a connection, using the key rather than
// anything they claim. An unknown key is said plainly, because "someone"
// is honest and a made-up name would not be.
func describe(book *contacts.Book, conn net.Conn) (name string, known bool) {
	if key := peer.RemoteKey(conn); key != "" {
		if c, ok := book.ByPubKey(key); ok {
			return c.Name, true
		}
		return "", false
	}

	// An accepted call brings no key, only the tunnel address it came
	// from, which carries the start of one. Enough to choose a label; see
	// peer.RemoteKeyPrefix for what a prefix does and does not prove.
	if prefix := peer.RemoteKeyPrefix(conn); prefix != "" {
		if c, ok := book.ByPubKeyPrefix(prefix); ok {
			return c.Name, true
		}
	}
	return "", false
}

// hangUp tells a caller why they are being hung up on, in the same words
// the caller would see for a busy line, rather than dropping the
// connection and leaving them to guess. Then it closes.
func hangUp(l *line, reason string) {
	if err := l.s.SendText(reason); err != nil {
		lg.Debug("could not tell a caller they were turned down", "err", err)
	}
	_ = l.s.Close()
}

// turnAway is hangUp for a second caller while one is already waiting,
// as a command so the write happens off the model's goroutine.
func turnAway(l *line) tea.Cmd {
	return func() tea.Msg {
		hangUp(l, "busy: another call is already waiting")
		return warnMsg(l.name + " called while another call was waiting")
	}
}

// declineCall hangs up on a parked call the person did not take, and says
// so on this side with format, which takes the caller's name.
func declineCall(l *line, reason, format string) tea.Cmd {
	return func() tea.Msg {
		hangUp(l, reason)
		return callRefused{name: l.name, format: format}
	}
}

// takeCall tells a parked caller the person took the call. Refusing is the
// default everywhere else; this is the one key that lets somebody in.
func takeCall(l *line) tea.Cmd {
	return func() tea.Msg {
		if err := l.s.SendAccept(); err != nil {
			// They went while the bar was on screen. Nothing to join.
			_ = l.s.Close()
			return callRefused{name: l.name, format: "%s went before the call could be connected"}
		}
		return callAnswered{l}
	}
}

// dial calls a saved contact and waits for the person on the other side to
// take the call. ctx is the model's cancel for this one call: giving up
// cancels it, and that comes back as us having stopped calling.
func dial(ctx context.Context, deps Deps, c contacts.Contact, send func(tea.Msg)) tea.Cmd {
	return func() tea.Msg {
		dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
		defer cancel()

		conn, err := peer.Dial(dialCtx, deps.ID, c.Addr)
		if err != nil {
			if ctx.Err() != nil {
				return callRefused{name: c.Name, format: "you stopped calling %s."}
			}
			return callFailed{name: c.Name, err: err}
		}

		// Now that we have spoken to them, remember the key that answered,
		// so their next call can be shown under this name.
		rememberKey(deps.Book, c.Name, peer.RemoteKey(conn))

		s, err := session.Start(conn, deps.Cfg.Nick, newAdapter(send))
		if err != nil {
			_ = conn.Close()
			return callFailed{name: c.Name, err: err}
		}

		// A finished handshake means two programs are talking, not that a
		// person agreed. An older peer never signals, so there is nothing
		// to wait through: its handshake is all the agreement there is.
		if err := s.WaitAccepted(ctx); err != nil {
			_ = s.Close()
			if ctx.Err() != nil {
				return callRefused{name: c.Name, format: "you stopped calling %s."}
			}
			return callFailed{name: c.Name, err: err}
		}

		// The name is one this machine gave: a contact was picked to get here.
		return callAnswered{&line{conn: conn, s: s, name: c.Name, known: true}}
	}
}

// rememberKey records a contact's key the first time we see it.
func rememberKey(book *contacts.Book, name, key string) {
	if key == "" {
		return
	}
	changed, err := book.SetPubKey(name, key)
	if err != nil || !changed {
		return
	}
	if err := book.Save(); err != nil {
		lg.Warn("could not save the address book", "err", err)
	}
}

// runSession reads from the far side until the line ends, then says so.
func runSession(ctx context.Context, l *line, send func(tea.Msg)) tea.Cmd {
	return func() tea.Msg {
		err := l.s.Run(ctx)
		send(peerLeft{err})
		return nil
	}
}

// closeLine is /quit: a goodbye the far side knows how to read, then the
// connection goes.
func closeLine(l *line) tea.Cmd {
	return func() tea.Msg {
		_ = l.s.Close()
		return nil
	}
}

// hangUpAndQuit is Ctrl+C in a conversation: the same goodbye — which
// Close waits to see leave — then out.
func hangUpAndQuit(l *line) tea.Cmd {
	return func() tea.Msg {
		_ = l.s.Close()
		return tea.QuitMsg{}
	}
}
