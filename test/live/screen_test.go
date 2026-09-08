//go:build live

package live

import (
	"strings"
	"testing"
)

// The grid shows what a terminal would: cursor moves land text where it
// belongs, erasing takes it away, and a line drawn over is the new line.
func TestScreenAppliesCursorMovesAndErasing(t *testing.T) {
	t.Parallel()

	s := newScreen(4, 20)
	s.write([]byte("\x1b[2J\x1b[H\x1b[1mfirst\x1b[0m line\r\nsecond\x1b[?25l"))
	s.write([]byte("\x1b[1;1Hfresh\x1b[K"))          // over the first line, rest erased
	s.write([]byte("\x1b[3;5Hdeep\x1b[2;1H\x1b[2K")) // third row at column 5; second row wiped

	want := "fresh\n\n    deep\n"
	if got := s.text(); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestScreenScrollsAtTheBottom(t *testing.T) {
	t.Parallel()

	s := newScreen(2, 10)
	s.write([]byte("a\r\nb\r\nc"))
	if got := s.text(); got != "b\nc" {
		t.Errorf("got %q", got)
	}
}

func TestScreenIgnoresQueriesAndOSC(t *testing.T) {
	t.Parallel()

	s := newScreen(1, 10)
	s.write([]byte("\x1b[?2026$p\x1b]0;title\x07\x1b[>4;2mhi\x1b[<1u"))
	if got := strings.TrimSpace(s.text()); got != "hi" {
		t.Errorf("got %q", got)
	}
}

// The renderer moves by tab stops as well as by columns: a backward tab
// lands on the previous eighth column, a forward one on the next.
func TestScreenMovesByTabStops(t *testing.T) {
	t.Parallel()

	s := newScreen(1, 40)
	s.write([]byte("abcdefghijklmnopqrstu\x1b[2Zx\tY"))
	// 21 cells written, cursor at 21; two tabs back land on 8; x at 8; a tab
	// forward lands on 16.
	if got := s.text(); got != "abcdefghxjklmnopYrstu" {
		t.Errorf("got %q", got)
	}
}

// A read boundary can fall inside an escape sequence; the grid waits for
// the rest rather than drawing the color code as text.
func TestScreenKeepsAnEscapeCutByARead(t *testing.T) {
	t.Parallel()

	s := newScreen(1, 20)
	s.write([]byte("a\x1b[38;2;1"))
	s.write([]byte("54;163;173mb\x1b["))
	s.write([]byte("mc"))
	if got := s.text(); got != "abc" {
		t.Errorf("got %q", got)
	}
}

// The renderer climbs with ESC M, the reverse index: a line drawn after it
// lands one row up, not on the row below.
func TestScreenClimbsWithReverseIndex(t *testing.T) {
	t.Parallel()

	s := newScreen(3, 10)
	s.write([]byte("one\r\ntwo\r\nthree\x1bM\rTWO\x1b[K"))
	if got := s.text(); got != "one\nTWO\nthree" {
		t.Errorf("got %q", got)
	}
}
