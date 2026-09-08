package ui

import (
	"strings"
	"testing"
	"time"
)

// An offer is one line: who, what, how big, what to type. The person answers
// from the input loop, which answerOffer stands in for here.
func TestAnOfferIsAnnouncedOnOneLineAndAnsweredFromTheChat(t *testing.T) {
	t.Parallel()

	u, _, out := newTerminalTest(t)
	h := newChatHandler(u, func() (string, error) { return "/tmp/dl", nil }, "~bob")

	type result struct {
		dir    string
		accept bool
	}
	done := make(chan result, 1)
	go func() {
		dir, ok, _ := h.OnFileOffer("notes.md", 14*1024)
		done <- result{dir, ok}
	}()

	// Wait for the offer to be parked, the way the input loop would find
	// it, rather than reading the screen while it is being written to.
	deadline := time.After(2 * time.Second)
	for !h.answerOffer(true) {
		select {
		case <-deadline:
			t.Fatal("the offer was never parked for an answer")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	r := <-done
	if !r.accept || r.dir != "/tmp/dl" {
		t.Errorf("answer = %+v, want accepted into /tmp/dl", r)
	}
	got := out.String()
	if !strings.Contains(got, "y to accept") {
		t.Errorf("the offer was announced without saying how to answer: %q", got)
	}
	if !strings.Contains(got, "~bob offers notes.md (14.0 KB)") {
		t.Errorf("the offer line reads %q", got)
	}
}

func TestTheSeparatorNeedsAUTF8Terminal(t *testing.T) {
	t.Parallel()

	u, _, _ := newTest(t)
	if got := u.sep(); got != sepASCII {
		t.Errorf("without UTF-8: %q", got)
	}
	u.st.unicode = true
	if got := u.sep(); got != sepUnicode {
		t.Errorf("with UTF-8: %q", got)
	}
}
