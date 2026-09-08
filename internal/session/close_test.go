package session

import (
	"testing"
	"time"

	"github.com/Serajian/homa/internal/proto"
)

// Close sends the goodbye and waits for the far side to close first, so a
// goodbye is never dropped by closing the connection under it; a peer that
// never closes is waited for no longer than byeLinger.
func TestCloseWaitsForThePeerToCloseFirst(t *testing.T) {
	t.Parallel()

	a, b := connPair(t)
	s := &Session{conn: a, c: proto.NewConn(a)}

	closed := make(chan time.Duration, 1)
	started := time.Now()
	go func() {
		_ = s.Close()
		closed <- time.Since(started)
	}()

	// The far side reads the goodbye, then closes, as a peer does.
	f, err := proto.NewConn(b).Read()
	if err != nil || f.Type != proto.TypeBye {
		t.Fatalf("the far side got %v, %v; want a goodbye", f.Type, err)
	}
	select {
	case took := <-closed:
		t.Fatalf("Close returned after %s, before the far side closed", took)
	case <-time.After(50 * time.Millisecond):
	}
	_ = b.Close()

	select {
	case took := <-closed:
		if took > byeLinger {
			t.Errorf("Close took %s after the far side closed", took)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close never returned")
	}
}

func TestCloseGivesUpOnAPeerThatNeverCloses(t *testing.T) {
	t.Parallel()

	a, b := connPair(t)
	defer func() { _ = b.Close() }()
	s := &Session{conn: a, c: proto.NewConn(a)}

	started := time.Now()
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if took := time.Since(started); took < byeLinger/2 || took > byeLinger*3 {
		t.Errorf("Close took %s; want about %s", took, byeLinger)
	}
	if _, err := a.Write([]byte{0}); err == nil {
		t.Error("the connection is still open after Close")
	}
}
