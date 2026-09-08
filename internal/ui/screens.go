package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Serajian/homa/internal/contacts"
)

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
			key := fmt.Sprintf("%-*s", width, it.key)
			text := it.text
			if it.name != "" {
				text = fmt.Sprintf(it.text, st.peer(it.name))
			}

			b.WriteString(markInfo)
			if it.quiet {
				b.WriteString(st.dim.Render(cur + key + "  " + text))
			} else {
				b.WriteString(cur + st.you.Render(key) + "  " + text)
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
	return "\n" + st.you.Render("contacts") + "\n\n" + renderGroups(st, cm.groups(), cm.cursor)
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
}

func (p *page) view(st *styles) string {
	return "\n" + st.you.Render(p.title) + "\n\n" + p.body
}

// helpText is the page a person reaches with h at the menu: what the menu
// cannot say for itself. Every line is a claim about what homa does, and a
// claim that has gone stale is worse than no help at all.
const helpText = `  homa connects two people directly. There is no account and nothing in the
  middle: an address is all it takes, in either direction.

  to reach somebody, they give you their address and you add it with n. a
  shows yours for them to do the same. Treat it like a password: whoever
  has it can call you. b is the address book: renaming, forgetting, calling.

  q and Ctrl+C quit homa. Inside a conversation /quit leaves only the
  conversation, and /help lists what else you can do in there.

  when somebody calls, homa asks before putting them through, and hangs up
  on them if nobody answers within a minute.

  a name in [brackets] is the one you gave them. A ~ in front means they
  chose it themselves and are not in your contacts.
`

// addressText is the page under a: the whole address, which is the only
// time it is shown in full. Everywhere else it appears shortened, because
// it is a secret.
func addressText(st *styles, addr string) string {
	return markInfo + st.dim.Render("Give this to someone who should be able to reach you.") + "\n" +
		markInfo + st.dim.Render("Treat it like a password: whoever has it can call you.") + "\n\n" +
		addr + "\n"
}
