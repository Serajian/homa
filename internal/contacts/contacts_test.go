package contacts

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Serajian/homa/internal/paths"
)

// sandbox points the config directory at a temporary one, so a test never
// reads or writes the address book of whoever is running it.
//
// Both variables are set because the two platforms disagree about which one
// decides: os.UserConfigDir reads XDG_CONFIG_HOME on Linux and HOME on macOS.
func sandbox(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))

	return dir
}

func addr(n string) string {
	// Only its emptiness is checked, so anything non-empty stands in for a
	// real one.
	return "tcpGFwWC" + n
}

func TestSaveAndLoad(t *testing.T) {
	sandbox(t)

	b := &Book{}
	if err := b.Add(Contact{Name: "BB", Addr: addr("1")}); err != nil {
		t.Fatalf("adding: %v", err)
	}
	if err := b.Add(Contact{Name: "zara", Addr: addr("2")}); err != nil {
		t.Fatalf("adding: %v", err)
	}
	if err := b.Save(); err != nil {
		t.Fatalf("saving: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if got.Len() != 2 {
		t.Fatalf("loaded %d contacts, want 2", got.Len())
	}

	c, err := got.ByName("BB")
	if err != nil {
		t.Fatalf("looking up a saved contact: %v", err)
	}
	if c.Addr != addr("1") {
		t.Errorf("addr = %q, want %q", c.Addr, addr("1"))
	}
}

// A first run has no file, and that is a normal state rather than a failure:
// homa simply knows nobody yet.
func TestLoadingWithNoFileGivesAnEmptyBook(t *testing.T) {
	sandbox(t)

	b, err := Load()
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if b.Len() != 0 {
		t.Errorf("loaded %d contacts from nothing", b.Len())
	}
}

// A file somebody edited by hand and broke must say so, not be silently
// replaced: it holds addresses that cannot be recovered from anywhere else.
func TestCorruptFileIsReportedRatherThanDiscarded(t *testing.T) {
	sandbox(t)

	p := writeBook(t, "{ this is not json")

	_, err := Load()
	if err == nil {
		t.Fatal("loaded a book from broken JSON")
	}
	if !strings.Contains(err.Error(), p) {
		t.Errorf("error does not name the file: %v", err)
	}
	if !strings.Contains(err.Error(), "delete it") {
		t.Errorf("error does not say what to do about it: %v", err)
	}
}

// Valid JSON holding an invalid contact is the other half of the same
// problem, and is caught by validating rather than by parsing.
func TestFileWithABadEntryIsRejected(t *testing.T) {
	sandbox(t)
	writeBook(t, `[{"name":"","addr":"x"}]`)

	if _, err := Load(); err == nil {
		t.Fatal("loaded a book with a nameless contact")
	}
}

func TestFileWithDuplicateNamesIsRejected(t *testing.T) {
	sandbox(t)
	writeBook(t, `[{"name":"BB","addr":"x"},{"name":"bb","addr":"y"}]`)

	if _, err := Load(); err == nil {
		t.Fatal("loaded a book with two contacts under one name")
	}
}

func writeBook(t *testing.T, content string) string {
	t.Helper()

	p, err := paths.File(contactsFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAddRefusesADuplicateName(t *testing.T) {
	sandbox(t)

	b := &Book{}
	if err := b.Add(Contact{Name: "BB", Addr: addr("1")}); err != nil {
		t.Fatalf("adding: %v", err)
	}

	// Matching ignores case, so this is the same person as far as a lookup
	// is concerned.
	err := b.Add(Contact{Name: "bb", Addr: addr("2")})
	if !errors.Is(err, ErrExists) {
		t.Errorf("err = %v, want ErrExists", err)
	}
	if b.Len() != 1 {
		t.Errorf("book holds %d contacts, want 1", b.Len())
	}
}

func TestAddRefusesAContactWithNoAddress(t *testing.T) {
	sandbox(t)

	if err := (&Book{}).Add(Contact{Name: "BB"}); err == nil {
		t.Fatal("added a contact with no address")
	}
}

// A rename keeps the address and the key, because those are what a contact
// is. Dropping the key would make their next call arrive as a stranger.
func TestRenameKeepsTheAddressAndTheKey(t *testing.T) {
	sandbox(t)

	b := &Book{}
	if err := b.Add(Contact{Name: "BB", Addr: addr("1")}); err != nil {
		t.Fatalf("adding: %v", err)
	}
	if _, err := b.SetPubKey("BB", "nodekey:abc"); err != nil {
		t.Fatalf("setting a key: %v", err)
	}

	if err := b.Rename("BB", "babak"); err != nil {
		t.Fatalf("renaming: %v", err)
	}

	c, err := b.ByName("babak")
	if err != nil {
		t.Fatalf("looking up the new name: %v", err)
	}
	if c.Addr != addr("1") {
		t.Errorf("addr = %q, want it unchanged", c.Addr)
	}
	if c.PubKey != "nodekey:abc" {
		t.Errorf("pubkey = %q, want it unchanged", c.PubKey)
	}

	if _, err := b.ByName("BB"); !errors.Is(err, ErrNotFound) {
		t.Error("the old name still resolves")
	}
}

func TestRenameRefusesANameAlreadyTaken(t *testing.T) {
	sandbox(t)

	b := &Book{}
	for _, n := range []string{"BB", "zara"} {
		if err := b.Add(Contact{Name: n, Addr: addr(n)}); err != nil {
			t.Fatalf("adding: %v", err)
		}
	}

	if err := b.Rename("BB", "zara"); !errors.Is(err, ErrExists) {
		t.Errorf("err = %v, want ErrExists", err)
	}

	// Both are still there, and still themselves.
	if b.Len() != 2 {
		t.Errorf("book holds %d contacts, want 2", b.Len())
	}
	if _, err := b.ByName("BB"); err != nil {
		t.Errorf("the contact being renamed was lost: %v", err)
	}
}

// Fixing "bb" to "BB" is the reason renaming to the same name in a different
// case has to be allowed: a lookup already ignores case.
func TestRenameAllowsAChangeOfCase(t *testing.T) {
	sandbox(t)

	b := &Book{}
	if err := b.Add(Contact{Name: "bb", Addr: addr("1")}); err != nil {
		t.Fatalf("adding: %v", err)
	}
	if err := b.Rename("bb", "BB"); err != nil {
		t.Fatalf("renaming: %v", err)
	}

	c, err := b.ByName("BB")
	if err != nil {
		t.Fatalf("looking up: %v", err)
	}
	if c.Name != "BB" {
		t.Errorf("name = %q, want %q", c.Name, "BB")
	}
}

func TestRemoveAndLookups(t *testing.T) {
	sandbox(t)

	b := &Book{}
	if err := b.Add(Contact{Name: "BB", Addr: addr("1")}); err != nil {
		t.Fatalf("adding: %v", err)
	}

	if err := b.Remove("nobody"); !errors.Is(err, ErrNotFound) {
		t.Errorf("removing a stranger: err = %v, want ErrNotFound", err)
	}
	if err := b.Remove("bb"); err != nil {
		t.Errorf("removing by a differently cased name: %v", err)
	}
	if b.Len() != 0 {
		t.Errorf("book holds %d contacts, want 0", b.Len())
	}
}

func TestByPubKeyAndItsPrefix(t *testing.T) {
	sandbox(t)

	b := &Book{}
	for _, n := range []string{"BB", "zara"} {
		if err := b.Add(Contact{Name: n, Addr: addr(n)}); err != nil {
			t.Fatalf("adding: %v", err)
		}
	}
	if _, err := b.SetPubKey("BB", "nodekey:aaaa1111"); err != nil {
		t.Fatalf("setting a key: %v", err)
	}
	if _, err := b.SetPubKey("zara", "nodekey:aaaa2222"); err != nil {
		t.Fatalf("setting a key: %v", err)
	}

	if c, ok := b.ByPubKey("nodekey:aaaa1111"); !ok || c.Name != "BB" {
		t.Errorf("ByPubKey found %v, %v", c.Name, ok)
	}
	if c, ok := b.ByPubKeyPrefix("nodekey:aaaa1"); !ok || c.Name != "BB" {
		t.Errorf("ByPubKeyPrefix found %v, %v", c.Name, ok)
	}

	// A prefix that fits two contacts names neither: showing the wrong
	// name is worse than admitting to not knowing.
	if c, ok := b.ByPubKeyPrefix("nodekey:aaaa"); ok {
		t.Errorf("an ambiguous prefix resolved to %q", c.Name)
	}
	if _, ok := b.ByPubKeyPrefix(""); ok {
		t.Error("an empty prefix matched something")
	}
}

// The accept goroutine reads the book while the person edits it from the
// menu. This is the test -race exists for.
func TestBookSurvivesConcurrentUse(t *testing.T) {
	sandbox(t)

	b := &Book{}
	for _, n := range []string{"a", "b", "c"} {
		if err := b.Add(Contact{Name: n, Addr: addr(n)}); err != nil {
			t.Fatalf("adding: %v", err)
		}
	}

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				switch i % 4 {
				case 0:
					b.All()
				case 1:
					_, _ = b.ByName("a")
				case 2:
					_, _ = b.ByPubKey("nodekey:none")
				case 3:
					_, _ = b.SetPubKey("a", "nodekey:1234")
				}
			}
		}()
	}
	wg.Wait()
}

// All returns a copy. A caller ranging over it while somebody edits the book
// must not be looking at the book's own slice.
func TestAllReturnsACopy(t *testing.T) {
	sandbox(t)

	b := &Book{}
	if err := b.Add(Contact{Name: "BB", Addr: addr("1")}); err != nil {
		t.Fatalf("adding: %v", err)
	}

	list := b.All()
	list[0].Name = "changed"

	if c, _ := b.ByName("BB"); c.Name != "BB" {
		t.Error("editing what All returned changed the book")
	}
}
