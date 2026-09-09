// Package session runs one conversation over an established connection. It
// owns the handshake, the read loop, and the rules about what may be shown
// to a person. It knows nothing about how the connection was made, and
// nothing about terminals: the caller supplies a Handler.
package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/Serajian/homa/internal/logx"
	"github.com/Serajian/homa/internal/proto"
)

var lg = logx.For("session")

// Handler receives what happens during a conversation. Every method is
// called from the session's read goroutine, one at a time, so an
// implementation needs no locking of its own but must not block for long.
type Handler interface {
	// OnText is a chat message from the peer, already sanitized.
	OnText(text string)
}

// AddressHandler is the part of a Handler that takes an address the peer
// has decided to hand over. A Handler that does not implement it is a
// session that ignores the gift, the way one without a FileHandler refuses
// files. Nothing is saved here: this only says one arrived.
type AddressHandler interface {
	// OnAddress is the peer's own address, sanitized but not validated.
	// Whether it is really an address is for the caller to decide.
	OnAddress(addr string)
}

// Peer is what the far side told us about itself. None of it is proof:
// a name is announced, not authenticated. Identity comes from the key
// underneath the tunnel.
type Peer struct {
	Nick    string
	Version int
}

// Session is one live conversation.
type Session struct {
	conn    net.Conn
	c       *proto.Conn
	handler Handler
	peer    Peer
	files   fileState // zero value is ready to use
}

// Start performs the opening handshake and returns a ready session.
//
// Both sides announce themselves at once rather than taking turns, because
// homa peers are equals: neither is the client. The connection is left open
// on success and belongs to the caller either way.
func Start(conn net.Conn, nick string, h Handler) (*Session, error) {
	if h == nil {
		return nil, errors.New("session: a handler is required")
	}

	s := &Session{
		conn:    conn,
		c:       proto.NewConn(conn),
		handler: h,
	}

	// One deadline covers the whole handshake, then it is cleared so the
	// conversation itself can sit idle for as long as people are quiet.
	if err := conn.SetDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return nil, fmt.Errorf("session: setting handshake deadline: %w", err)
	}

	if err := s.handshake(nick); err != nil {
		return nil, err
	}

	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("session: clearing handshake deadline: %w", err)
	}

	lg.Info("session started", "peer", s.peer.Nick, "version", s.peer.Version)
	return s, nil
}

func (s *Session) handshake(nick string) error {
	out := proto.Hello{Nick: nick, Version: proto.Version}
	if err := s.c.WriteJSON(proto.TypeHello, out); err != nil {
		return fmt.Errorf("session: announcing ourselves: %w", err)
	}

	f, err := s.c.Read()
	if err != nil {
		return fmt.Errorf("session: waiting for the peer's greeting: %w", err)
	}
	if f.Type != proto.TypeHello {
		return fmt.Errorf("session: expected a greeting, got %s", f.Type)
	}

	var in proto.Hello
	if err = proto.DecodeJSON(f, &in); err != nil {
		return fmt.Errorf("session: reading the peer's greeting: %w", err)
	}

	s.peer = Peer{
		Nick:    sanitizeNick(in.Nick),
		Version: in.Version,
	}

	if in.Version != proto.Version {
		// Not fatal: the frame format is stable, so an older or newer
		// peer can still chat. The caller decides whether to warn.
		lg.Warn("protocol version differs",
			"ours", proto.Version, "theirs", in.Version)
	}
	return nil
}

// Peer reports what the far side announced about itself.
func (s *Session) Peer() Peer { return s.peer }

// AnswerWindow is how long the person being called may take to answer. It is
// re-exported so the interface can put the same deadline on its question that
// the caller puts on its waiting, without reaching past this package into the
// wire protocol.
const AnswerWindow = proto.AnswerTimeout

// ErrNotTaken means the call was never picked up: refused, hung up on, or left
// unanswered. It is not a failure of the connection, so callers report it to
// the person rather than as an error.
var ErrNotTaken = errors.New("session: the call was not taken")

// SignalsAcceptance reports whether the peer is new enough to say when their
// side actually took the call. An older peer never will, so there is nothing
// to wait for and nothing to tell the person about waiting.
func (s *Session) SignalsAcceptance() bool {
	return s.peer.Version >= proto.VersionAccept
}

// CanTakeAddress reports whether the peer is new enough to understand an
// address being handed over. An older one drops what it does not know,
// silently, so the person has to be told before they try rather than left
// believing it arrived.
func (s *Session) CanTakeAddress() bool {
	return s.peer.Version >= proto.VersionAddress
}

// SendAddress hands the peer this machine's address, so they can call back.
// It is a secret, and it goes only because somebody asked for it to go.
func (s *Session) SendAddress(addr string) error {
	if strings.TrimSpace(addr) == "" {
		return errors.New("session: there is no address to send")
	}
	if len(addr) > MaxAddrLen {
		return fmt.Errorf("session: an address of %d bytes is not one", len(addr))
	}
	return s.c.WriteJSON(proto.TypeAddress, proto.Address{Addr: addr})
}

// SendAccept tells the caller that the person took the call. The side that
// was called sends it once, when they agree.
func (s *Session) SendAccept() error {
	if err := s.c.WriteAccept(); err != nil {
		return fmt.Errorf("session: accepting the call: %w", err)
	}
	return nil
}

// WaitAccepted blocks until the far side says a person took the call, and is
// called by the side that dialed, once, before Run.
//
// It exists because a completed handshake only means two programs are
// talking. Treating it as agreement is what used to tell a caller they were
// in a conversation moments before being turned away from it.
//
// A peer older than proto.VersionAccept never sends an acceptance, so for
// them this returns at once: their handshake is all the agreement there is,
// which is exactly how homa behaved before this frame existed.
//
// It returns ErrNotTaken when the call was refused, hung up on, or not
// answered in time, wrapping whatever the far side said so the person can be
// told which it was.
func (s *Session) WaitAccepted(ctx context.Context) error {
	if !s.SignalsAcceptance() {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, proto.AnswerTimeout+proto.AnswerGrace)
	defer cancel()

	// The read is what has to be interrupted, and closing the connection is
	// the only thing that interrupts it. Run does the same.
	stop := context.AfterFunc(ctx, func() {
		lg.Debug("giving up on waiting for the call to be taken")
		_ = s.conn.Close()
	})
	defer stop()

	// Anything the far side says before hanging up is the reason they gave,
	// and it is worth more to the person than "the call was not taken".
	var said string

	for {
		f, err := s.c.Read()
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("%w: no answer", ErrNotTaken)
			}
			if said != "" {
				return fmt.Errorf("%w: %s", ErrNotTaken, said)
			}
			return fmt.Errorf("%w: they hung up", ErrNotTaken)
		}

		switch f.Type {
		case proto.TypeAccept:
			return nil

		case proto.TypeText:
			said = sanitizeText(string(f.Payload))

		case proto.TypeBye:
			if said != "" {
				return fmt.Errorf("%w: %s", ErrNotTaken, said)
			}
			return fmt.Errorf("%w: they hung up", ErrNotTaken)

		default:
			// Nothing else is meaningful before a call is taken, and a
			// peer sending it is not a reason to drop the call.
			lg.Debug("ignoring a frame sent before the call was taken", "type", f.Type)
		}
	}
}

// SendText sends a chat message. Empty messages are dropped rather than
// sent, so a stray Enter does not put a blank line on the peer's screen.
func (s *Session) SendText(text string) error {
	text = strings.TrimRight(text, "\r\n")
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if len(text) > MaxTextLen {
		return fmt.Errorf("session: message is longer than %d bytes", MaxTextLen)
	}
	return s.c.WriteText(text)
}

// Run reads from the peer until the conversation ends, dispatching to the
// handler as frames arrive. It returns nil on a clean ending: either the
// peer said goodbye or the connection closed normally.
//
// Canceling ctx closes the connection, which is what unblocks the read.
func (s *Session) Run(ctx context.Context) error {
	stop := context.AfterFunc(ctx, func() {
		lg.Debug("context canceled, closing connection")
		_ = s.conn.Close()
	})
	defer stop()

	// A conversation that ends mid-transfer leaves half-written files and
	// senders waiting on a reply that will never come. Release both.
	defer s.abortTransfers()

	for {
		f, err := s.c.Read()
		if err != nil {
			return s.readErr(ctx, err)
		}

		switch f.Type {
		case proto.TypeText:
			s.handler.OnText(sanitizeText(string(f.Payload)))

		case proto.TypeBye:
			lg.Info("peer said goodbye")
			return nil

		case proto.TypeHello:
			lg.Debug("ignoring a repeated greeting")

		case proto.TypeAccept:
			// WaitAccepted consumed the one that mattered. A second is
			// a peer being careless, not a reason to end a conversation
			// that is already running.
			lg.Debug("ignoring a repeated acceptance")

		case proto.TypeAddress:
			s.onAddress(f)

		default:
			// File frames land here. A failure inside one transfer is
			// reported to the person and the conversation carries on:
			// a rejected file is no reason to hang up.
			if s.handleFileFrame(f) {
				continue
			}
			lg.Debug("ignoring an unhandled frame", "type", f.Type)
		}
	}
}

// readErr turns the end of a read loop into either a clean stop or a real
// error, so callers do not have to know which sentinel means what.
func (s *Session) readErr(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, io.EOF):
		lg.Info("peer disconnected")
		return nil

	case ctx.Err() != nil:
		// We closed the connection ourselves; whatever the read reported
		// is a consequence, not the cause.
		return ctx.Err()

	case errors.Is(err, net.ErrClosed):
		lg.Info("connection closed")
		return nil

	case errors.Is(err, io.ErrUnexpectedEOF):
		return errors.New("session: the peer vanished mid-message")

	default:
		return fmt.Errorf("session: reading from the peer: %w", err)
	}
}

// Close ends the conversation politely: it tells the peer we are leaving,
// waits for them to close first — bounded by byeLinger — and then closes
// the connection. The goodbye is best-effort, since the reason for closing
// is often that the connection already broke; when it could not be sent
// there is nothing to wait for.
func (s *Session) Close() error {
	if err := s.c.WriteBye(); err != nil {
		lg.Debug("could not send goodbye", "err", err)
		return s.conn.Close()
	}

	// Read until the far side closes or the bound passes. Run may be
	// reading too; a net.Conn takes readers from several goroutines, and
	// whichever of them the peer's close reaches first, this one returns
	// as soon as the connection is done.
	_ = s.conn.SetReadDeadline(time.Now().Add(byeLinger))
	buf := make([]byte, 64)
	for {
		if _, err := s.conn.Read(buf); err != nil {
			break
		}
	}
	return s.conn.Close()
}

// onAddress takes the address a peer has handed over and passes it up. A
// handler that cannot take one is not an error: the address is simply not
// kept, which is what a session with no interface would want anyway.
func (s *Session) onAddress(f proto.Frame) {
	ah, ok := s.handler.(AddressHandler)
	if !ok {
		lg.Debug("ignoring an address: this peer has nowhere to put one")
		return
	}

	var msg proto.Address
	if err := proto.DecodeJSON(f, &msg); err != nil {
		lg.Warn("unreadable address", "err", err)
		return
	}

	addr := sanitizeAddr(msg.Addr)
	if addr == "" {
		lg.Warn("an address arrived with nothing left in it after sanitizing")
		return
	}
	lg.Info("the peer handed over an address")
	ah.OnAddress(addr)
}
