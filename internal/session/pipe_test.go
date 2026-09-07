package session

import (
	"net"
	"testing"
)

// connPair is two ends of a connection on the loopback interface. Nothing
// leaves the machine: no listener anybody else can reach, no relay, no
// tailcat.
//
// net.Pipe would be the obvious choice and does not work, which is worth
// writing down. It is entirely unbuffered, so a write blocks until the other
// side reads — and homa's handshake has both sides write their greeting
// before either reads one. Over a real connection the kernel's buffers
// absorb that; over net.Pipe both sides block until the handshake deadline.
//
// So the tests use the same shape of connection production does, minus the
// tunnel underneath it.
func connPair(t *testing.T) (a, b net.Conn) {
	t.Helper()

	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening on loopback: %v", err)
	}
	defer func() { _ = l.Close() }()

	type accepted struct {
		c   net.Conn
		err error
	}
	ch := make(chan accepted, 1)
	go func() {
		c, aerr := l.Accept()
		ch <- accepted{c, aerr}
	}()

	var d net.Dialer
	a, err = d.DialContext(t.Context(), "tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("dialing loopback: %v", err)
	}

	got := <-ch
	if got.err != nil {
		t.Fatalf("accepting on loopback: %v", got.err)
	}
	b = got.c

	t.Cleanup(func() {
		_ = a.Close()
		_ = b.Close()
	})
	return a, b
}
