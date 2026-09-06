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

var logger = logx.For("session")

// Handler receives what happens during a conversation. Every method is
// called from the session's read goroutine, one at a time, so an
// implementation needs no locking of its own but must not block for long.
type Handler interface {
	// OnText is a chat message from the peer, already sanitized.
	OnText(text string)
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

	logger.Info("session started", "peer", s.peer.Nick, "version", s.peer.Version)
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
		logger.Warn("protocol version differs",
			"ours", proto.Version, "theirs", in.Version)
	}
	return nil
}

// Peer reports what the far side announced about itself.
func (s *Session) Peer() Peer { return s.peer }

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
		logger.Debug("context canceled, closing connection")
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
			logger.Info("peer said goodbye")
			return nil

		case proto.TypeHello:
			logger.Debug("ignoring a repeated greeting")

		default:
			// File frames land here. A failure inside one transfer is
			// reported to the person and the conversation carries on:
			// a rejected file is no reason to hang up.
			if s.handleFileFrame(f) {
				continue
			}
			logger.Debug("ignoring an unhandled frame", "type", f.Type)
		}
	}
}

// readErr turns the end of a read loop into either a clean stop or a real
// error, so callers do not have to know which sentinel means what.
func (s *Session) readErr(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, io.EOF):
		logger.Info("peer disconnected")
		return nil

	case ctx.Err() != nil:
		// We closed the connection ourselves; whatever the read reported
		// is a consequence, not the cause.
		return ctx.Err()

	case errors.Is(err, net.ErrClosed):
		logger.Info("connection closed")
		return nil

	case errors.Is(err, io.ErrUnexpectedEOF):
		return errors.New("session: the peer vanished mid-message")

	default:
		return fmt.Errorf("session: reading from the peer: %w", err)
	}
}

// Close ends the conversation politely: it tells the peer we are leaving,
// then closes the connection. The goodbye is best-effort, since the reason
// for closing is often that the connection already broke.
func (s *Session) Close() error {
	if err := s.c.WriteBye(); err != nil {
		logger.Debug("could not send goodbye", "err", err)
	}
	return s.conn.Close()
}
