package ui

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/Serajian/homa/internal/session"
)

// connPair is two ends of a loopback TCP connection. Not net.Pipe: it has
// no buffer, and homa's handshake writes on both sides before reading.
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
		c, acceptErr := l.Accept()
		ch <- accepted{c, acceptErr}
	}()

	var d net.Dialer
	a, err = d.DialContext(t.Context(), "tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("dialing loopback: %v", err)
	}
	got := <-ch
	if got.err != nil {
		t.Fatalf("accepting loopback: %v", got.err)
	}
	t.Cleanup(func() { _ = a.Close(); _ = got.c.Close() })
	return a, got.c
}

// recorder is the far side's handler: it keeps what it was told.
type recorder struct{ texts chan string }

func (r *recorder) OnText(text string) { r.texts <- text }

// The whole path through the interface with a real session on each end:
// a call arrives, y takes it, a message goes each way, /quit returns to
// the menu.
func TestACallIsTakenAndAMessageGoesEachWay(t *testing.T) {
	connA, connB := connPair(t)

	var tm *teatest.TestModel
	m := newModel(t.Context(), testDeps(t), newStyles(true))
	m.send = func(msg tea.Msg) { tm.Send(msg) }

	// Both sides greet at once, so the two Starts run together.
	far := &recorder{texts: make(chan string, 8)}
	type started struct {
		s   *session.Session
		err error
	}
	bDone := make(chan started, 1)
	go func() {
		s, err := session.Start(connB, "bob", far)
		bDone <- started{s, err}
	}()
	sessA, err := session.Start(connA, "alice", &adapter{send: func(msg tea.Msg) { tm.Send(msg) }})
	if err != nil {
		t.Fatalf("starting A: %v", err)
	}
	b := <-bDone
	if b.err != nil {
		t.Fatalf("starting B: %v", b.err)
	}

	tm = teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	waitFor := func(want string) {
		t.Helper()
		teatest.WaitFor(
			t,
			tm.Output(),
			func(out []byte) bool { return bytes.Contains(out, []byte(want)) },
			teatest.WithDuration(5*time.Second),
		)
	}

	waitFor("PEOPLE")
	tm.Send(
		callArrived{
			&line{conn: connA, s: sessA, name: "~bob", deadline: time.Now().Add(time.Minute)},
		},
	)
	waitFor("is calling") // the name before it is painted

	// B waits to be let in, then reads.
	bIn := make(chan error, 1)
	go func() {
		if err := b.s.WaitAccepted(context.Background()); err != nil {
			bIn <- err
			return
		}
		bIn <- nil
		_ = b.s.Run(context.Background())
	}()

	tm.Type("y")
	waitFor("talking to")
	if err := <-bIn; err != nil {
		t.Fatalf("B was not let in: %v", err)
	}

	tm.Type("salam")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	select {
	case got := <-far.texts:
		if got != "salam" {
			t.Errorf("B received %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("B never received the line")
	}

	if err := b.s.SendText("khoobam"); err != nil {
		t.Fatalf("B sending: %v", err)
	}
	waitFor("│ khoobam") // the name before it is painted, so an escape sits between them

	tm.Type("/quit")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitFor("PEOPLE")

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
