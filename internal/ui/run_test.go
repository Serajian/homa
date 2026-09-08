package ui

import (
	"errors"
	"os"
	"testing"
)

// A pipe is not a terminal, and homa says so rather than drawing into it:
// a full-screen program has nowhere to put a cursor in a pipe, and nobody
// chats through one.
func TestAPipeIsRefused(t *testing.T) {
	t.Parallel()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	if err := needsTerminal(r, w); !errors.Is(err, ErrNeedsTerminal) {
		t.Errorf("a pipe was accepted: err = %v", err)
	}
}
