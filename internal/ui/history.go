package ui

// history is the lines sent in this conversation, for the up and down
// arrows. The text input has no memory of its own; this is small enough
// to keep here.
type history struct {
	lines []string
	pos   int    // index into lines while walking; len(lines) when not
	draft string // what was being typed when the walk began
}

// push remembers a sent line and ends any walk. An empty line, or the same
// line again, is not worth a slot.
func (h *history) push(line string) {
	if line != "" && (len(h.lines) == 0 || h.lines[len(h.lines)-1] != line) {
		h.lines = append(h.lines, line)
	}
	h.pos = len(h.lines)
}

// up steps to an older line, keeping current as the draft on the first
// step so down can bring it back. It reports false at the oldest.
func (h *history) up(current string) (string, bool) {
	if h.pos == 0 {
		return "", false
	}
	if h.pos == len(h.lines) {
		h.draft = current
	}
	h.pos--
	return h.lines[h.pos], true
}

// down steps to a newer line, and past the newest to the draft. It reports
// false when there is nothing newer.
func (h *history) down() (string, bool) {
	if h.pos >= len(h.lines) {
		return "", false
	}
	h.pos++
	if h.pos == len(h.lines) {
		return h.draft, true
	}
	return h.lines[h.pos], true
}
