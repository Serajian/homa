package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Serajian/homa/internal/contacts"
	"github.com/Serajian/homa/internal/peer"
	"github.com/Serajian/homa/internal/update"
)

// menuItem is one line of a menu: the key to press and what it does.
type menuItem struct {
	key  string
	text string // what the key does; holds one %s when name is set

	// name is a peer's name to be painted into text as one, so the
	// people on a menu look like people everywhere else.
	name string

	// quiet makes the line recede: leaving, and anything that cannot be
	// undone, should not weigh the same as calling somebody.
	quiet bool
}

// preview shortens an address for a header.
func preview(addr string) string {
	if len(addr) <= addrPreviewLen {
		return addr
	}
	return fmt.Sprintf("%s...", addr[:addrPreviewLen])
}

// renderGroups draws keyed groups separated by a blank line, keys padded
// to the widest, a cursor on one row of the first group when cursor is not
// negative, quiet rows grey throughout. The menu, the contacts screen and
// the contact screen are all this.
func renderGroups(st *styles, groups [][]menuItem, cursor int) string {
	width := 0
	for _, g := range groups {
		for _, it := range g {
			width = max(width, len(it.key))
		}
	}

	mark, none := "▸ ", "  "
	if !st.unicode {
		mark = "> "
	}

	var b strings.Builder
	first := true
	for gi, g := range groups {
		if len(g) == 0 {
			continue
		}
		if !first {
			b.WriteString("\n")
		}
		first = false

		for i, it := range g {
			cur := none
			if gi == 0 && i == cursor {
				cur = mark
			}
			key := st.chip(fmt.Sprintf("%-*s", width, it.key))
			text := it.text
			if it.name != "" {
				text = fmt.Sprintf(it.text, st.peer(it.name))
			}

			b.WriteString(markInfo + cur + key + " ")
			if it.quiet {
				b.WriteString(st.dim.Render(text))
			} else {
				b.WriteString(text)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// contactsModel is the address book screen: every contact numbered, with a
// cursor, and the two ways out.
type contactsModel struct {
	list   []contacts.Contact
	cursor int
}

func newContacts(book *contacts.Book) contactsModel {
	return contactsModel{list: book.All()}
}

func (cm *contactsModel) groups() [][]menuItem {
	var people []menuItem
	for i, c := range cm.list {
		people = append(people, menuItem{
			key:  strconv.Itoa(i + 1),
			text: "%s  " + preview(c.Addr),
			name: fmt.Sprintf("%-16s", c.Name),
		})
	}
	return [][]menuItem{
		people,
		{{key: "b", text: wordBack, quiet: true}, {key: "q", text: wordQuit, quiet: true}},
	}
}

func (cm *contactsModel) view(st *styles) string {
	return "\n  " + st.label.Render("CONTACTS") + "\n\n" + renderGroups(st, cm.groups(), cm.cursor)
}

// contactAction is what a key on the contacts screens asks for.
type contactAction int

const (
	contactNone contactAction = iota
	contactOpen               // the one under the cursor, or numbered
	contactBack
	contactQuit
	contactCall
	contactRename
	contactAddress
	contactForget
)

func (cm *contactsModel) key(k string) (contactAction, bool) {
	switch k {
	case keyUp:
		if cm.cursor > 0 {
			cm.cursor--
		}
		return contactNone, true
	case keyDown:
		if cm.cursor < len(cm.list)-1 {
			cm.cursor++
		}
		return contactNone, true
	case keyEnter:
		if len(cm.list) == 0 {
			return contactNone, false
		}
		return contactOpen, true
	case "b":
		return contactBack, true
	case "q":
		return contactQuit, true
	}
	if n, err := strconv.Atoi(k); err == nil && n >= 1 && n <= len(cm.list) {
		cm.cursor = n - 1
		return contactOpen, true
	}
	return contactNone, false
}

func (cm *contactsModel) chosen() contacts.Contact { return cm.list[cm.cursor] }

// contactGroups is the screen for one contact: what can be done with them.
func contactGroups(c contacts.Contact) [][]menuItem {
	return [][]menuItem{
		{
			{key: "c", text: wordCall, name: c.Name},
			{key: "r", text: "rename"},
			{key: "a", text: "show their address"},
			{key: "f", text: wordForget, quiet: true},
		},
		{
			{key: "b", text: wordBack, quiet: true},
			{key: "q", text: wordQuit, quiet: true},
		},
	}
}

func contactKey(k string) (contactAction, bool) {
	switch k {
	case "c":
		return contactCall, true
	case "r":
		return contactRename, true
	case "a":
		return contactAddress, true
	case "f":
		return contactForget, true
	case "b":
		return contactBack, true
	case "q":
		return contactQuit, true
	}
	return contactNone, false
}

// page is a screen of text and a way back: help, an address.
type page struct {
	title string
	body  string
	back  screen

	// copyText, when set, is what c copies to the clipboard; the footer
	// offers the key. Every other key goes back.
	copyText string
}

// view wraps the body to the width: an address is two hundred characters
// with no space in it, and a page that cut it at the edge would be showing
// a secret nobody can copy.
func (p *page) view(st *styles, width int) string {
	body := lipgloss.NewStyle().Width(max(width-4, 20)).Render(p.body)
	return "\n  " + st.you.Render(p.title) + "\n\n  " + strings.ReplaceAll(body, "\n", "\n  ")
}

// helpText is the page a person reaches with h at the menu: what the menu
// cannot say for itself. Every line is a claim about what homa does, and a
// claim that has gone stale is worse than no help at all.
const helpText = `homa connects two people directly. There is no account and nothing in the
middle: an address is all it takes, in either direction.

to reach somebody, they give you their address and you add it with n. m
shows yours for them to do the same. Treat it like a password: whoever
has it can call you. b is the address book: renaming, forgetting, calling.

u asks GitHub whether a newer release is out, and says how to get it.
It is the only time homa reaches anything but the relay, and only
because you asked.

q and Ctrl+C quit homa. Inside a conversation /quit leaves only the
conversation, and /help lists what else you can do in there.

when somebody calls, homa asks before putting them through, and hangs up
on them if nobody answers within a minute.

a name in [brackets] is the one you gave them. A ~ in front means they
chose it themselves and are not in your contacts.
`

// meText is the page under a: who you are on the network. Three groups in
// the menu's own style — the address, whole, which is the only time it is
// shown in full because it is a secret; the relay you sit behind; and the
// start of your key, which is what a contact's book records about you.
func meText(
	st *styles,
	addr string,
	relay peer.Relay,
	key string,
	width int,
	copyHint string,
) string {
	var b strings.Builder
	b.WriteString(st.label.Render("ADDRESS") + "\n")
	b.WriteString(st.dim.Render("give this to someone who should be able to reach you") + "\n")
	b.WriteString(st.dim.Render("treat it like a password: whoever has it can call you") + "\n")
	b.WriteString(
		st.dim.Render("it wraps; copy every row, line breaks and all, "+copyHint) + "\n\n",
	)
	b.WriteString(blockRows(addr, width) + "\n\n")

	b.WriteString(st.label.Render("RELAY") + "\n")
	if relay.Code == "" {
		b.WriteString(st.dim.Render("not connected to a relay yet") + "\n\n")
	} else {
		line := st.you.Render(relay.Code)
		if relay.Name != "" {
			line += st.dim.Render(" · " + relay.Name)
		}
		if relay.Connected {
			line += "   " + st.them.Render("●") + st.dim.Render(" connected")
		}
		b.WriteString(line + "\n\n")
	}

	b.WriteString(st.label.Render("KEY") + "\n")
	b.WriteString(
		st.you.Render(key) + st.dim.Render("   what your contacts record about you") + "\n",
	)
	return b.String()
}

// blockRows cuts s into rows of width runes, the last one shorter.
func blockRows(s string, width int) string {
	width = max(width, 8)
	r := []rune(s)
	var b strings.Builder
	for len(r) > width {
		b.WriteString(string(r[:width]))
		b.WriteString("\n")
		r = r[width:]
	}
	b.WriteString(string(r))
	return b.String()
}

// checkUpdate asks GitHub, off the update loop, and reports back.
func checkUpdate(ctx context.Context, deps Deps) tea.Cmd {
	return func() tea.Msg {
		res, err := deps.Update.Check(ctx, deps.Version)
		return updateChecked{res: res, err: err}
	}
}

// updateText is the page under u: which release is out, which one this
// is, and the command that upgrades, guessed from where the binary lives.
// A build from source is told so rather than compared.
func updateText(st *styles, r update.Result) string {
	return updateTextFor(st, r, installedAt())
}

// installedAt is where the running binary really lives: Homebrew starts
// homa through a symlink in its bin directory, and only the target names
// the Caskroom that says "brew".
func installedAt() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved
	}
	return exe
}

func updateTextFor(st *styles, r update.Result, exe string) string {
	var b strings.Builder
	switch {
	case !r.Known:
		fmt.Fprintf(&b, "  this is a build from source (%s); the latest release is %s\n",
			r.Current, r.Latest)
	case r.Newer:
		fmt.Fprintf(&b, "  %s is out; you have %s\n\n", st.you.Render(r.Latest), r.Current)
		fmt.Fprintf(&b, "  to upgrade:  %s\n", st.you.Render(update.Advice(exe)))
	default:
		fmt.Fprintf(&b, "  you have %s, and it is the latest release\n", r.Current)
	}
	b.WriteString("\n  " + st.dim.Render(r.URL) + "\n")
	return b.String()
}
