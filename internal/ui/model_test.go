package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Serajian/homa/internal/config"
)

// testDeps is enough for the screens that touch neither the network nor
// the disk: settings and a book, no identity, no listener.
func testDeps(t *testing.T, names ...string) Deps {
	t.Helper()
	return Deps{Cfg: config.Default(), Book: bookWith(t, names...)}
}

func TestQuitFromTheMenu(t *testing.T) {
	t.Parallel()

	m := newModel(t.Context(), testDeps(t), newStyles(true))
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("q did nothing")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q did not quit")
	}
}

func TestViewIsTheTerminalsSizeAndStaysOnTheMainScreen(t *testing.T) {
	t.Parallel()

	m := newModel(t.Context(), testDeps(t, "alice"), newStyles(true))
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	v := next.(model).View()
	if n := strings.Count(v.Content, "\n") + 1; n != 12 {
		t.Errorf("%d lines, want 12:\n%s", n, v.Content)
	}
	if v.AltScreen {
		t.Error("the alternate screen must stay off: what was on screen stays in scrollback")
	}
}

func TestAWrongKeySaysSoUnderTheMenu(t *testing.T) {
	t.Parallel()

	m := newModel(t.Context(), testDeps(t), newStyles(true))
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	next, _ = next.(model).Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	got := stripANSI(next.(model).View().Content)
	if !strings.Contains(got, "that is not one of the choices") {
		t.Errorf("no hint after a wrong key:\n%s", got)
	}
	next, _ = next.(model).Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	next, _ = next.(model).Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if strings.Contains(stripANSI(next.(model).View().Content), "not one of the choices") {
		t.Error("the hint outlived the next key")
	}
}

// Color changes nothing but color: the screen drawn with the styles, with
// the escapes taken out, is the screen drawn with no styles at all.
func TestScreensChangeNothingButColor(t *testing.T) {
	t.Parallel()

	colored := newModel(t.Context(), testDeps(t, "alice", "~bob"), newStyles(true))
	plain := newModel(t.Context(), testDeps(t, "alice", "~bob"), plainStyles(true))
	size := tea.WindowSizeMsg{Width: 70, Height: 20}
	c, _ := colored.Update(size)
	p, _ := plain.Update(size)
	cv, pv := c.(model).View().Content, p.(model).View().Content
	if !strings.Contains(cv, "\x1b[") {
		t.Fatal("the styled screen has no escapes in it")
	}
	if stripANSI(cv) != pv {
		t.Errorf("styled, stripped:\n%s\nplain:\n%s", stripANSI(cv), pv)
	}
}
