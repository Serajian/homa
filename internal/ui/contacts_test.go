package ui

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Serajian/homa/internal/config"
	"github.com/Serajian/homa/internal/contacts"
)

// The contacts screen can be driven in process: it touches the address book
// and the terminal and nothing else. The identity and the listener are nil
// because nothing here reaches them — calling is the one action that would,
// and it is not exercised.
//
// None of these run in parallel. They point HOME at a temporary directory
// because renaming and forgetting save the book, and t.Setenv and t.Parallel
// cannot be used together.

func testApp(t *testing.T, names ...string) (*App, *io.PipeWriter, *strings.Builder) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))

	book := &contacts.Book{}
	for i, n := range names {
		if err := book.Add(contacts.Contact{
			Name: n,
			Addr: "tcpGFwWC" + strings.Repeat("x", 60) + string(rune('a'+i)),
		}); err != nil {
			t.Fatalf("adding %s: %v", n, err)
		}
	}

	r, w := io.Pipe()
	t.Cleanup(func() { _ = w.Close() })

	var out strings.Builder
	u := New(r, &out)

	cfg := &config.Config{Nick: "alice", DownloadDir: dir}
	return NewApp(u, cfg, book, nil, nil), w, &out
}

// drive types a script into the screen and waits for it to return, so a test
// never hangs on a screen expecting one more line than it was given.
func drive(t *testing.T, a *App, w *io.PipeWriter, out *strings.Builder, lines ...string) bool {
	t.Helper()

	go func() {
		for _, l := range lines {
			if _, err := io.WriteString(w, l+"\n"); err != nil {
				return
			}
		}
	}()

	done := make(chan bool, 1)
	go func() { done <- a.contactsScreen(t.Context()) }()

	select {
	case quit := <-done:
		return quit
	case <-time.After(5 * time.Second):
		t.Fatalf("the screen did not return. It had shown:\n%s", out)
		return false
	}
}

func TestTheScreenListsTheAddressBook(t *testing.T) {
	a, w, out := testApp(t, "BB", "zara")
	drive(t, a, w, out, "b")

	got := out.String()
	for _, want := range []string{"contacts", "1)", "BB", "2)", "zara", "back"} {
		if !strings.Contains(got, want) {
			t.Errorf("the screen never showed %q:\n%s", want, got)
		}
	}
}

// An empty book says so and comes straight back, rather than showing a list
// of nothing and asking for a number.
func TestAnEmptyBookSaysSo(t *testing.T) {
	a, w, out := testApp(t)
	if quit := drive(t, a, w, out); quit {
		t.Error("an empty address book quit homa")
	}

	if !strings.Contains(out.String(), "no contacts yet") {
		t.Errorf("screen was:\n%s", out.String())
	}
}

func TestRenamingThroughTheScreenKeepsTheKey(t *testing.T) {
	a, w, out := testApp(t, "BB")
	_ = out
	if _, err := a.book.SetPubKey("BB", "nodekey:abcdef"); err != nil {
		t.Fatalf("setting a key: %v", err)
	}

	drive(t, a, w, out, "1", "r", "babak", "b", "b")

	c, err := a.book.ByName("babak")
	if err != nil {
		t.Fatalf("the rename did not take: %v", err)
	}
	if c.PubKey != "nodekey:abcdef" {
		t.Errorf("pubkey = %q, want it kept", c.PubKey)
	}
	if _, err := a.book.ByName("BB"); err == nil {
		t.Error("the old name still resolves")
	}
}

func TestRenamingOntoATakenNameIsRefused(t *testing.T) {
	a, w, out := testApp(t, "BB", "zara")
	drive(t, a, w, out, "1", "r", "zara", "b", "b")

	if !strings.Contains(out.String(), "already exists") {
		t.Errorf("the screen did not say why:\n%s", out.String())
	}
	if a.book.Len() != 2 {
		t.Errorf("the book holds %d contacts, want both still there", a.book.Len())
	}
}

// Forgetting takes a word rather than a letter, because it takes the key with
// it and there is no undo.
func TestForgettingNeedsTheWord(t *testing.T) {
	a, w, out := testApp(t, "BB")
	drive(t, a, w, out, "1", "f", "yes please", "b", "b")

	if a.book.Len() != 1 {
		t.Error("a contact was forgotten without the word being typed")
	}
	if !strings.Contains(out.String(), "still there") {
		t.Errorf("the screen did not say it was kept:\n%s", out.String())
	}
}

func TestForgettingWithTheWordRemovesTheContact(t *testing.T) {
	a, w, out := testApp(t, "BB", "zara")
	drive(t, a, w, out, "1", "f", "forget", "b")

	if _, err := a.book.ByName("BB"); err == nil {
		t.Error("BB is still in the book")
	}
	if a.book.Len() != 1 {
		t.Errorf("the book holds %d contacts, want the other one kept", a.book.Len())
	}
	if !strings.Contains(out.String(), "takes their address and their key") {
		t.Errorf("the warning did not say what goes with it:\n%s", out.String())
	}
}

func TestQuittingFromInsideTheScreenLeavesHoma(t *testing.T) {
	a, w, out := testApp(t, "BB")
	if quit := drive(t, a, w, out, "q"); !quit {
		t.Error("q inside the address book did not leave homa")
	}

	// And from one level deeper, where it would be easiest to strand
	// somebody.
	a2, w2, out2 := testApp(t, "BB")
	if quit := drive(t, a2, w2, out2, "1", "q"); !quit {
		t.Error("q inside a contact did not leave homa")
	}
}

func TestANumberThatIsNotThereIsRefused(t *testing.T) {
	a, w, out := testApp(t, "BB")
	drive(t, a, w, out, "9", "b")

	if !strings.Contains(out.String(), "not one of the choices") {
		t.Errorf("screen was:\n%s", out.String())
	}
}

func TestShowingAnAddressPrintsTheWholeThing(t *testing.T) {
	a, w, out := testApp(t, "BB")
	c, err := a.book.ByName("BB")
	if err != nil {
		t.Fatal(err)
	}

	drive(t, a, w, out, "1", "a", "b", "b")

	if !strings.Contains(out.String(), c.Addr) {
		t.Error("the address was shown shortened, and it is the one place it should not be")
	}
}

// preview is what keeps an address off the screen everywhere else: it is a
// secret, and a list of them would put several on one screen at once.
func TestPreviewShortens(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", 200)
	got := preview(long)

	if len(got) >= len(long) {
		t.Errorf("preview kept %d characters of %d", len(got), len(long))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("preview = %q, want it to show it was cut", got)
	}
	if got := preview("short"); got != "short" {
		t.Errorf("preview of something already short = %q", got)
	}
}
