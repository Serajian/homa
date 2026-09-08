package ui

import "testing"

// The far side is always green, and the mark on a name they chose for
// themselves is grey, so "this name is theirs" reads at a glance while the
// mark itself stays for anyone reading without color.
func TestPeerPaintsTheMarkGreyAndTheNameGreen(t *testing.T) {
	t.Parallel()

	s := newStyles(true)
	if got, want := s.peer("~bob"), s.dim.Render(unknownMark)+s.them.Render("bob"); got != want {
		t.Errorf("a self-chosen name: got %q, want %q", got, want)
	}
	if got, want := s.peer("alice"), s.them.Render("alice"); got != want {
		t.Errorf("a saved name: got %q, want %q", got, want)
	}
}

func TestStylesSeparatorNeedsAUTF8Terminal(t *testing.T) {
	t.Parallel()

	if got := newStyles(false).sep(); got != sepASCII {
		t.Errorf("without UTF-8: %q", got)
	}
	if got := newStyles(true).sep(); got != sepUnicode {
		t.Errorf("with UTF-8: %q", got)
	}
}
