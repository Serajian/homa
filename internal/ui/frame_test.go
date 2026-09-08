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

	got := frame(40, 6, "status\nrule", "a\nb", "rule\nkeys")
	lines := strings.Split(got, "\n")
	if len(lines) != 6 {
		t.Fatalf("%d lines, want 6:\n%s", len(lines), got)
	}
	if lines[0] != "status" || lines[1] != "rule" || lines[2] != "a" || lines[3] != "b" ||
		lines[5] != "keys" {
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

// The header puts the status flush right and a rule under both; a
// terminal too narrow for both keeps the name and drops the status.
func TestHeaderRightAlignsTheStatus(t *testing.T) {
	t.Parallel()

	st := plainStyles(true)
	got := header(st, 40, ">_ homa", "mohsen")
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("%d lines, want 2: %q", len(lines), got)
	}
	// two columns of margin on each side: the line is width-2 wide
	if !strings.HasPrefix(lines[0], "  >_ homa") || !strings.HasSuffix(lines[0], "mohsen") ||
		lipgloss.Width(lines[0]) != 38 {
		t.Errorf("header line %q", lines[0])
	}
	if lipgloss.Width(lines[1]) != 39 {
		t.Errorf("rule is %d wide", lipgloss.Width(lines[1]))
	}
	narrow := strings.Split(header(st, 14, ">_ homa", "mohsen"), "\n")[0]
	if strings.Contains(narrow, "mohsen") {
		t.Errorf("a narrow header kept the status: %q", narrow)
	}
}

func TestBoxHasATitleOnItsEdgeAndPadsItsLines(t *testing.T) {
	t.Parallel()

	st := plainStyles(true)
	got := st.box(st.boxThem(), 30, "incoming call", "a", "bb")
	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("%d lines: %q", len(lines), got)
	}
	if lines[0] != "╭─ incoming call ────────────╮" {
		t.Errorf("top = %q", lines[0])
	}
	for _, l := range lines {
		if lipgloss.Width(l) != 30 {
			t.Errorf("line %q is %d wide, want 30", l, lipgloss.Width(l))
		}
	}
	if !strings.HasPrefix(lines[1], "│ a") || !strings.HasPrefix(lines[2], "│ bb") {
		t.Errorf("lines %q", lines[1:3])
	}
}
