package peer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/tailscale/tailcat"
)

// ErrClosed is returned by Accept once the listener has been closed.
var ErrClosed = errors.New("peer: listener closed")

// Listener accepts incoming homa connections under our identity.
//
// tailcat has no Accept: it calls a handler per connection and closes the
// connection when that handler returns. Listener bridges the two models by
// parking the handler until the caller is finished with the connection.
type Listener struct {
	srv   *tailcat.Server
	addr  string
	conns chan *heldConn

	closeOnce sync.Once
	closed    chan struct{}
}

// heldConn keeps tailcat's handler parked for as long as the caller uses the
// connection. Closing it releases the handler, which lets tailcat tear the
// stream down.
type heldConn struct {
	net.Conn
	release     chan struct{}
	releaseOnce sync.Once
}

func (h *heldConn) Close() error {
	err := h.Conn.Close()
	h.releaseOnce.Do(func() { close(h.release) })
	return err
}

// Listen starts accepting connections addressed to id. The returned Listener's
// Addr is the string a peer needs in order to reach this machine.
func Listen(id *Identity) (*Listener, error) {
	l := &Listener{
		conns:  make(chan *heldConn),
		closed: make(chan struct{}),
	}

	l.srv = &tailcat.Server{
		Key:          id.pk.Private,
		PresharedKey: id.pk.Public.PresharedKey,
		RegionID:     id.pk.Public.RegionID,
		Logf:         tailcatLogf,

		// Only our own port is served. Anything else gets an RST, and the
		// filter never even admits the packets.
		ServedTCPPorts: servedPorts(),
		OnTCP: func(port uint16) func(net.Conn) {
			if port != Port {
				logger.Debug("rejecting connection to unexpected port", "port", port)
				return nil
			}
			return l.handle
		},
	}

	if err := l.srv.Start(); err != nil {
		return nil, fmt.Errorf("peer: starting listener: %w", err)
	}
	l.addr = string(l.srv.TailcatAddr())

	logger.Info("listening", "region", id.pk.Public.RegionID)
	return l, nil
}

// handle runs on tailcat's goroutine for one incoming connection. It hands the
// connection to Accept and then waits, because returning would close it.
func (l *Listener) handle(c net.Conn) {
	h := &heldConn{Conn: c, release: make(chan struct{})}

	select {
	case l.conns <- h:
		logger.Debug("incoming connection accepted")
		<-h.release
		logger.Debug("incoming connection finished")
	case <-l.closed:
		// Shutting down, and nobody is left to take it.
		err := c.Close()
		if err != nil {
			logger.Error("close connection failed", "err", err)
			return
		}
	}
}

// Accept blocks until a peer connects, ctx is canceled, or the listener is
// closed. The caller owns the returned connection and must Close it.
func (l *Listener) Accept(ctx context.Context) (net.Conn, error) {
	select {
	case h := <-l.conns:
		return h, nil
	case <-l.closed:
		return nil, ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Addr is this machine's address: the string a peer needs to reach us. It is
// a secret, because it carries the pre-shared key that guards the tunnel.
func (l *Listener) Addr() string { return l.addr }

// Close stops accepting and shuts the tunnel down. Connections already handed
// out by Accept are not closed; the caller closes those.
func (l *Listener) Close() error {
	l.closeOnce.Do(func() {
		close(l.closed)
		logger.Info("listener closing")
	})
	return l.srv.Close()
}
