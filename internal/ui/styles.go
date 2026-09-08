package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// styles is the look, held as data. What each color means is decided in
// docs/decisions.md — cream is you, green is them, grey is homa talking,
// the terminal's yellow warns — and this is the only place those meanings
// become styles. Changing the look is changing this value, not a screen.
//
// "You" is bold in the terminal's own foreground rather than a fixed cream:
// cream on a dark theme, ink on a light one, where a fixed cream would
// vanish. lipgloss and bubbletea downsample the two fixed colors to what
// the terminal declares it can show, and honor NO_COLOR.
type styles struct {
	you, them, dim, warn lipgloss.Style

	// rule is the one line the interface draws, between the conversation
	// and the input, kept faint so it separates without shouting.
	rule lipgloss.Style

	unicode bool // block characters and the middle dot are safe to draw
}

// It is handed around by pointer: five lipgloss styles are a few kilobytes,
// and there is exactly one of these for the life of the program.
func newStyles(unicode bool) *styles {
	grey := lipgloss.Color("#9AA3AD")
	return &styles{
		you:     lipgloss.NewStyle().Bold(true),
		them:    lipgloss.NewStyle().Foreground(lipgloss.Color("#22E6A7")),
		dim:     lipgloss.NewStyle().Foreground(grey),
		warn:    lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		rule:    lipgloss.NewStyle().Foreground(grey).Faint(true),
		unicode: unicode,
	}
}

// sep joins the parts of a status line, with a middle dot where the
// terminal can show one.
func (s *styles) sep() string {
	if s.unicode {
		return sepUnicode
	}
	return sepASCII
}

// peer paints a name the way the far side is always shown: green, with the
// unknownMark in grey when the name is one they chose for themselves. The
// name is content passed to Render, never part of a style; that is the
// boundary session/sanitize.go and decisions.md hold, kept here too.
func (s *styles) peer(name string) string {
	if strings.HasPrefix(name, unknownMark) {
		return s.dim.Render(unknownMark) + s.them.Render(strings.TrimPrefix(name, unknownMark))
	}
	return s.them.Render(name)
}
