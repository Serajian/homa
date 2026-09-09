package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Serajian/homa/internal/contacts"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI leaves what somebody reading without color sees.
func stripANSI(s string) string { return ansi.ReplaceAllString(s, "") }

// bookWith is an address book with these names, in this order.
func bookWith(t *testing.T, names ...string) *contacts.Book {
	t.Helper()
	book := &contacts.Book{}
	for i, n := range names {
		if err := book.Add(contacts.Contact{
			Name: n,
			Addr: "tcpGFwWC" + strings.Repeat("x", 60) + string(rune('a'+i)),
		}); err != nil {
			t.Fatalf("adding %s: %v", n, err)
		}
	}
	return book
}

func TestMenuKeysMapToActionsAndEnterCallsTheCursor(t *testing.T) {
	t.Parallel()

	mm := newMenu(bookWith(t, "alice", "bob"))
	cases := map[string]menuAction{
		"1": actCall, "2": actCall, "enter": actCall,
		"n": actAdd, "b": actContacts, "a": actAddress,
		"s": actSettings, "c": actClear, "h": actHelp, "r": actReset, "q": actQuit,
	}
	for k, want := range cases {
		if got, ok := mm.key(k); !ok || got != want {
			t.Errorf("%q → %v, %v; want %v", k, got, ok, want)
		}
	}
	if _, ok := mm.key("x"); ok {
		t.Error("x is not a key")
	}
	if _, ok := mm.key("3"); ok {
		t.Error("there is no third contact")
	}
}

func TestMenuCursorStaysWithinTheContacts(t *testing.T) {
	t.Parallel()

	mm := newMenu(bookWith(t, "alice", "bob"))
	mm.key("down")
	if mm.cursor != 1 {
		t.Fatalf("cursor = %d after one down", mm.cursor)
	}
	mm.key("down")
	if mm.cursor != 1 {
		t.Error("the cursor ran off the end")
	}
	mm.key("up")
	mm.key("up")
	if mm.cursor != 0 {
		t.Error("the cursor ran off the top")
	}
	if got := mm.chosen(); got.Name != "alice" {
		t.Errorf("chosen = %q", got.Name)
	}

	empty := newMenu(bookWith(t))
	if _, ok := empty.key("enter"); ok {
		t.Error("Enter with nobody to call did something")
	}
}

func TestMenuViewIsGroupedAndMarksTheCursor(t *testing.T) {
	t.Parallel()

	mm := newMenu(bookWith(t, "alice", "~bob"))
	got := stripANSI(mm.view(newStyles(true)))
	want := "  ▸  1  call alice\n" +
		"     2  call ~bob\n" +
		"\n" +
		"     n  add a contact\n" +
		"     b  contacts: rename, forget, call\n" +
		"     a  show my address\n" +
		"\n" +
		"     s  settings\n" +
		"     c  clear the screen\n" +
		"     h  help\n" +
		"     u  check for updates\n" +
		"\n" +
		"     r  start over: forget everything\n" +
		"     q  quit homa\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
