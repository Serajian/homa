package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// The frame is always exactly as tall as the terminal, so the renderer
// never scrolls it, with the status line first and the keys last.
func TestFrameIsExactlyTheTerminalTall(t *testing.T) {
	t.Parallel()

	got := frame(40, 6, "status", "a\nb", "keys")
	lines := strings.Split(got, "\n")
	if len(lines) != 6 {
		t.Fatalf("%d lines, want 6:\n%s", len(lines), got)
	}
	if lines[0] != "status" || lines[1] != "a" || lines[2] != "b" || lines[5] != "keys" {
		t.Errorf("wrong placement:\n%q", lines)
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 40 {
			t.Errorf("line %d is %d columns wide", i, w)
		}
	}
}

// A body taller than the room is cut from the top: in a conversation the
// newest lines are the ones that matter.
func TestFrameCutsABodyFromTheTop(t *testing.T) {
	t.Parallel()

	got := frame(20, 4, "s", "1\n2\n3\n4\n5", "k")
	lines := strings.Split(got, "\n")
	if len(lines) != 4 || lines[1] != "4" || lines[2] != "5" {
		t.Errorf("got %q, want the last two body lines kept", lines)
	}
}

// A line wider than the terminal is cut rather than wrapped, or the frame
// would grow a row and push the keys off the bottom.
func TestFrameCutsALineWiderThanTheTerminal(t *testing.T) {
	t.Parallel()

	got := frame(10, 3, "s", strings.Repeat("x", 30), "k")
	lines := strings.Split(got, "\n")
	if len(lines) != 3 || lipgloss.Width(lines[1]) != 10 {
		t.Errorf("got %q", lines)
	}
}
