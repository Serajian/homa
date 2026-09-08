package ui

import "testing"

// Up walks back through what was sent, keeping whatever was being typed so
// Down past the newest line brings it back; a repeat of the last line is
// not stored twice.
func TestHistoryWalksBackAndKeepsTheDraft(t *testing.T) {
	t.Parallel()

	var h history
	h.push("one")
	h.push("two")
	h.push("two")
	h.push("")

	got, ok := h.up("draft")
	if !ok || got != "two" {
		t.Fatalf("up = %q, %v; want two", got, ok)
	}
	if got, _ = h.up(got); got != "one" {
		t.Fatalf("up up = %q; want one", got)
	}
	if _, past := h.up(got); past {
		t.Error("walked past the oldest line")
	}

	if got, _ = h.down(); got != "two" {
		t.Errorf("down = %q; want two", got)
	}
	got, ok = h.down()
	if !ok || got != "draft" {
		t.Errorf("down past the newest = %q, %v; want the draft back", got, ok)
	}
	if _, more := h.down(); more {
		t.Error("down with nothing newer did something")
	}
}
