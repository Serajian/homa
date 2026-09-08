package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// frame lays one screen out: the status line, the body, the key line. It
// returns exactly height lines of at most width columns, whatever it is
// given, because the renderer draws the frame in place and a frame one row
// too tall scrolls the terminal and takes the key line with it.
//
// A body taller than the room is cut from the top: in a conversation the
// newest lines are the ones that matter. A line wider than the room is cut
// rather than wrapped, for the same reason — wrapping grows rows.
func frame(width, height int, status, body, keys string) string {
	if height < statusHeight+keysHeight+1 {
		height = statusHeight + keysHeight + 1
	}
	bodyHeight := height - statusHeight - keysHeight

	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if len(lines) > bodyHeight {
		lines = lines[len(lines)-bodyHeight:]
	}
	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}

	cut := lipgloss.NewStyle().MaxWidth(width)
	out := make([]string, 0, height)
	out = append(out, cut.Render(status))
	for _, l := range lines {
		out = append(out, cut.Render(l))
	}
	out = append(out, cut.Render(keys))
	return strings.Join(out, "\n")
}
