package ui

import (
	"strconv"

	"github.com/Serajian/homa/internal/contacts"
)

// menuAction is what a menu key asks for. The model acts on it; the menu
// itself only knows keys and a cursor.
type menuAction int

const (
	actNone menuAction = iota // the cursor moved, nothing else
	actCall
	actAdd
	actContacts
	actAddress
	actSettings
	actClear
	actHelp
	actUpdate
	actReset
	actQuit
)

// menuModel is the main menu: the same four groups version 1 drew, with a
// cursor on the people, because calling somebody is what the screen is for.
type menuModel struct {
	groups   [][]menuItem
	contacts []contacts.Contact
	cursor   int // into contacts; meaningless when there are none
}

func newMenu(book *contacts.Book) menuModel {
	list := book.All()

	var people []menuItem
	for i, c := range list {
		people = append(people, menuItem{key: strconv.Itoa(i + 1), text: wordCall, name: c.Name})
	}

	return menuModel{
		contacts: list,
		groups: [][]menuItem{
			people,
			{
				{key: "n", text: "add a contact"},
				{key: "b", text: "contacts: rename, forget, call"},
				{key: "m", text: "me: address, relay, key"},
			},
			{
				{key: "s", text: "settings"},
				{key: "c", text: "clear the screen"},
				{key: "h", text: wordHelp},
				{key: "u", text: "check for updates"},
			},
			{
				{key: "r", text: "start over: forget everything", quiet: true},
				{key: "q", text: wordQuit, quiet: true},
			},
		},
	}
}

// chosen is the contact under the cursor. Only meaningful when key
// reported actCall.
func (mm *menuModel) chosen() contacts.Contact { return mm.contacts[mm.cursor] }

// key turns a keypress into an action. A number picks that contact and
// moves the cursor there, so what was called is what is highlighted; Enter
// calls whoever the cursor is on; up and down only move it.
func (mm *menuModel) key(k string) (menuAction, bool) {
	switch k {
	case keyUp:
		if mm.cursor > 0 {
			mm.cursor--
		}
		return actNone, true
	case keyDown:
		if mm.cursor < len(mm.contacts)-1 {
			mm.cursor++
		}
		return actNone, true
	case keyEnter:
		if len(mm.contacts) == 0 {
			return actNone, false
		}
		return actCall, true
	case "n":
		return actAdd, true
	case "b":
		return actContacts, true
	case "m", "a":
		// a is what this was until the page grew past the address; it
		// still works, unlisted, the way /ls still works for /files.
		return actAddress, true
	case "s":
		return actSettings, true
	case "c":
		return actClear, true
	case "h":
		return actHelp, true
	case "u":
		return actUpdate, true
	case "r":
		return actReset, true
	case "q":
		return actQuit, true
	}

	if n, err := strconv.Atoi(k); err == nil && n >= 1 && n <= len(mm.contacts) {
		mm.cursor = n - 1
		return actCall, true
	}
	return actNone, false
}

// view draws the groups, the cursor on the contacts.
func (mm *menuModel) view(st *styles) string { return renderGroups(st, mm.groups, mm.cursor) }
