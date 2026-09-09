package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// command is one slash command of a conversation: its word, the uses
// of what may follow it with what each does, and a second spelling that
// is taken when typed but never offered. /help, the "no such command"
// listing and the hint row all read this table; whoever adds a command
// adds a row here and nowhere else.
type command struct {
	name  string
	uses  []use
	alias string
}

// use is one way of using a command: the argument's shape, and what the
// command does with it. The first use is the one the hint row shows.
type use struct {
	arg, doc string
}

// arg is what the hint offers after the word: the first use's shape.
func (c command) arg() string { return c.uses[0].arg }

// takesArg reports whether anything may follow the word, which is what
// decides whether completing it leaves a space to type in.
func (c command) takesArg() bool { return c.arg() != "" }

// needsArg reports whether the command cannot run without an argument, in
// which case Enter on the word finishes it and waits rather than running
// it. The shape says which: <path> must be given, [dir] may be.
func (c command) needsArg() bool { return strings.HasPrefix(c.arg(), "<") }

// commands in the order the hint row offers them. /help is first so that
// a lone slash and Enter, the version-1 way of asking, still answers with
// the list.
var commands = []command{
	{name: "/help", uses: []use{{"", "this list, or just /"}}},
	{name: "/files", alias: "/ls", uses: []use{
		{"[dir]", "list a directory, numbered"},
		{"<n>", "list one from the last listing, .. included"},
	}},
	{name: "/send", uses: []use{
		{"<path>", "offer a file"},
		{"<n>", "offer one from the last listing"},
	}},
	{name: "/cancel", uses: []use{{"", "stop a file being sent or taken"}}},
	{name: "/accept", uses: []use{{"", "take the file being offered, or just y"}}},
	{name: "/reject", uses: []use{{"", "refuse it, or just n"}}},
	{name: "/who", uses: []use{{"", "who you are talking to"}}},
	{name: "/me", uses: []use{
		{"", "your address, relay and key"},
		{"copy", "put your address on the clipboard"},
		{"send", "give them your address, so they can call you back"},
	}},
	{name: "/add", uses: []use{
		{"[name]", "keep the address they sent you"},
	}},
	{name: "/clear", uses: []use{{"", "wipe the screen"}}},
	{name: "/quit", uses: []use{{"", "leave the conversation, not homa"}}},
}

// helpLines is the command table as /help prints it: one line per use,
// the word and the argument shape in a column, the doc after.
func helpLines() []string {
	var out []string
	for _, c := range commands {
		for _, f := range c.uses {
			out = append(out, fmt.Sprintf("%-13s %s", strings.TrimSpace(c.name+" "+f.arg), f.doc))
		}
	}
	return out
}

// exact is the command a whole word names, by its name or its alias.
func exact(word string) (command, bool) {
	for _, c := range commands {
		if word == c.name || (c.alias != "" && word == c.alias) {
			return c, true
		}
	}
	return command{}, false
}

// matches is the commands a word being typed can still become: every one
// whose name starts with it, or the one an alias names in full. An alias
// is never offered, only taken.
func matches(word string) []command {
	if c, ok := exact(word); ok {
		return []command{c}
	}
	var out []command
	for _, c := range commands {
		if strings.HasPrefix(c.name, word) {
			out = append(out, c)
		}
	}
	return out
}

// commandWord splits a typed line into the command word and whether an
// argument has begun. Only a line starting with a slash has a word.
func commandWord(typed string) (word string, argBegun bool) {
	if !strings.HasPrefix(typed, "/") {
		return "", false
	}
	word, _, argBegun = strings.Cut(typed, " ")
	return word, argBegun
}

// complete is the typed line with its command word completed: to the
// picked command when several are offered, to the one left when one is,
// with a space after it when it takes an argument. A line whose argument
// has begun, or whose word can become nothing, comes back as it was.
func complete(typed string, pick int) string {
	word, argBegun := commandWord(typed)
	if word == "" || argBegun {
		return typed
	}
	ms := matches(word)
	if len(ms) == 0 {
		return typed
	}
	c := ms[min(max(pick, 0), len(ms)-1)]
	if c.takesArg() {
		return c.name + " "
	}
	return c.name
}

// hint is the row between the pane and the input: what the typed line can
// become, or nothing when it is not a command. Several candidates are laid
// out in one row with the picked one marked; when the row is wider than
// the room, the row starts where the pick is still in view. One candidate
// left, or an argument begun after a known word, shows that command's
// shape and doc. A word that can become nothing warns at once, before
// Enter would.
func hint(st *styles, typed string, pick, width int) string {
	word, argBegun := commandWord(typed)
	if word == "" {
		return ""
	}
	nothing := st.warn.Render("no such command: " + quote(word))

	if argBegun {
		c, ok := exact(word)
		if !ok {
			return nothing
		}
		return st.dim.Render(usage(c))
	}

	ms := matches(word)
	switch len(ms) {
	case 0:
		return nothing
	case 1:
		line := usage(ms[0])
		if word != ms[0].name {
			line += st.sep() + "Tab completes"
		}
		return st.dim.Render(line)
	}

	pick = min(max(pick, 0), len(ms)-1)
	mark, more := pickASCII, "..."
	if st.unicode {
		mark, more = pickUnicode, "…"
	}
	items := make([]string, len(ms))
	for i, c := range ms {
		s := strings.TrimSpace(c.name + " " + c.arg())
		if i == pick {
			items[i] = mark + s
		} else {
			items[i] = st.dim.Render(s)
		}
	}
	sep := st.dim.Render(st.sep())

	// Drop candidates from the front until the pick fits, marking that
	// something was dropped; what follows the pick is cut by the frame.
	start := 0
	for start < pick && lipgloss.Width(more+strings.Join(items[start:pick+1], sep)) > width {
		start++
	}
	row := strings.Join(items[start:], sep)
	if start > 0 {
		row = st.dim.Render(more) + row
	}
	// The tail is cut by the frame anyway; cutting it here instead lets it
	// end in the same mark the front uses, so a row with more in it says
	// so at both ends rather than stopping mid-separator.
	if lipgloss.Width(row) > width {
		room := max(width-lipgloss.Width(more), 1)
		row = lipgloss.NewStyle().MaxWidth(room).Render(row) + st.dim.Render(more)
	}
	return row
}

// usage is one command's first use and doc, as the hint shows it.
func usage(c command) string {
	f := c.uses[0]
	return strings.TrimSpace(c.name+" "+f.arg) + "   " + f.doc
}
