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
// vanish. lipgloss and bubbletea downsample the fixed colors to what the
// terminal declares it can show, and honor NO_COLOR.
type styles struct {
	you, them, dim, warn lipgloss.Style

	// key is a key to press, drawn as a keycap: your color on a slate
	// chip, so every key on a screen is found by one glance down a column.
	key lipgloss.Style

	// label is a section heading, in grey small capitals by convention:
	// PEOPLE, HOMA.
	label lipgloss.Style

	// rule is the thin line under a header and above a footer, kept faint
	// so it separates without shouting.
	rule lipgloss.Style

	unicode bool // block characters, rounded corners and the middle dot are safe
}

func newStyles(unicode bool) *styles {
	grey := lipgloss.Color("#9AA3AD")
	return &styles{
		you:     lipgloss.NewStyle().Bold(true),
		them:    lipgloss.NewStyle().Foreground(lipgloss.Color("#22E6A7")),
		dim:     lipgloss.NewStyle().Foreground(grey),
		warn:    lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		key:     lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#2B2F36")),
		label:   lipgloss.NewStyle().Foreground(grey),
		rule:    lipgloss.NewStyle().Foreground(grey).Faint(true),
		unicode: unicode,
	}
}

// plainStyles is styles with no styling at all, for tests that hold the
// colored screen, stripped of its escapes, equal to the plain one.
func plainStyles(unicode bool) *styles { return &styles{unicode: unicode} }

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

// chip draws one key as a keycap.
func (s *styles) chip(k string) string { return s.key.Render(" " + k + " ") }

// keys draws pairs of key and meaning for a footer: chip, a space, the
// meaning in grey, three spaces between pairs.
func (s *styles) keys(pairs ...string) string {
	parts := make([]string, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, s.chip(pairs[i])+" "+s.dim.Render(pairs[i+1]))
	}
	return strings.Join(parts, "   ")
}

// border is the box-drawing set the terminal can show.
func (s *styles) border() lipgloss.Border {
	if s.unicode {
		return lipgloss.RoundedBorder()
	}
	return lipgloss.ASCIIBorder()
}

// hrule is a horizontal line of n cells.
func (s *styles) hrule(n int) string {
	return s.rule.Render(strings.Repeat(s.border().Top, max(n, 0)))
}

// box draws lines inside a bordered box of the given outer width, with an
// optional title on the top edge, the frame in the given style. Lines are
// styled strings; each is padded to the inside width. It is built by hand
// rather than through lipgloss's Border, because a title on the edge is
// not something that can be spliced into a rendered box afterwards.
func (s *styles) box(frame *lipgloss.Style, width int, title string, lines ...string) string {
	b := s.border()
	inner := max(width-2, 1)

	var out strings.Builder
	top := b.Top + " " + title + " "
	if title == "" {
		top = ""
	}
	out.WriteString(frame.Render(b.TopLeft) + frame.Render(top) +
		frame.Render(strings.Repeat(b.Top, max(inner-lipgloss.Width(top), 0))) + frame.Render(b.TopRight) + "\n")
	cut := lipgloss.NewStyle().MaxWidth(inner - 1)
	for _, l := range lines {
		// Cut, never wrapped: a line longer than the box would break the
		// box open on the next row, which the live tests saw with an
		// address in a form.
		l = cut.Render(l)
		pad := max(inner-1-lipgloss.Width(l), 0)
		out.WriteString(frame.Render(b.Left) + " " + l + strings.Repeat(" ", pad) + frame.Render(b.Right) + "\n")
	}
	out.WriteString(frame.Render(b.BottomLeft) + frame.Render(strings.Repeat(b.Bottom, inner)) + frame.Render(b.BottomRight))
	return out.String()
}

// boxThem is the frame for the one box that is the far side's: an incoming call.
func (s *styles) boxThem() *lipgloss.Style { return &s.them }

// boxDim is the frame for everything else boxed: the input line, a form's field.
func (s *styles) boxDim() *lipgloss.Style { return &s.dim }
