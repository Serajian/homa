package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// frame lays one screen out: the header, the body, the footer. It returns
// exactly height lines of at most width columns, whatever it is given,
// because the renderer draws the frame in place and a frame one row too
// tall scrolls the terminal and takes the footer with it.
//
// A body taller than the room is cut from the top: in a conversation the
// newest lines are the ones that matter. A line wider than the room is cut
// rather than wrapped, for the same reason — wrapping grows rows.
func frame(width, height int, header, body, footer string) string {
	head := strings.Split(strings.TrimRight(header, "\n"), "\n")
	foot := strings.Split(strings.TrimRight(footer, "\n"), "\n")
	if height < len(head)+len(foot)+1 {
		height = len(head) + len(foot) + 1
	}
	bodyHeight := height - len(head) - len(foot)

	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if len(lines) > bodyHeight {
		lines = lines[len(lines)-bodyHeight:]
	}
	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}

	cut := lipgloss.NewStyle().MaxWidth(width)
	out := make([]string, 0, height)
	for _, l := range head {
		out = append(out, cut.Render(l))
	}
	for _, l := range lines {
		out = append(out, cut.Render(l))
	}
	for _, l := range foot {
		out = append(out, cut.Render(l))
	}
	return strings.Join(out, "\n")
}

// header is the top of every screen: the mark and the name on the left,
// whatever the screen has to say about itself on the right, and a rule
// under both. Two lines. Right-aligning the status is what makes the line
// read as designed rather than typed.
func header(st *styles, width int, left, right string) string {
	gap := width - 2 - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		// Not enough room for both: the status goes, the name stays.
		right, gap = "", 0
	}
	return "  " + left + strings.Repeat(" ", gap) + right + "\n " + st.hrule(width-2) + "\n"
}

// brand is the mark and the name, the left of every header.
func brand(st *styles) string { return st.you.Render(bannerPlainMark) + " " + st.you.Render("homa") }

// footer is the bottom of every screen: a rule, then the keys that matter
// here. Two lines.
func footer(st *styles, width int, keys string) string {
	return " " + st.hrule(width-2) + "\n  " + keys
}
