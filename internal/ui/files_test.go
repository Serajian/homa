package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree builds a directory to list: a subdirectory, a hidden file, a name with
// a space in it, and a file called "2" — the one that cannot be sent by name
// because a number means a line.
func tree(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "archive"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, size := range map[string]int{
		"notes.md":            5,
		"gozaresh nahayi.pdf": 4300,
		"2":                   3,
		".hidden":             1,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, size), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func names(l *listing) []string {
	out := make([]string, 0, len(l.entries))
	for _, e := range l.entries {
		out = append(out, e.name)
	}
	return out
}

// Directories first and then names in order, because walking into somewhere
// is what the listing is for as often as sending out of it. ".." leads,
// because a way back should never be somewhere in the middle.
func TestReadDirOrdersAndHides(t *testing.T) {
	t.Parallel()

	l, hidden, err := readDir(tree(t))
	if err != nil {
		t.Fatalf("readDir: %v", err)
	}
	if hidden != 0 {
		t.Errorf("hid %d entries from a small directory", hidden)
	}

	want := []string{"..", "archive", "2", "gozaresh nahayi.pdf", "notes.md"}
	got := names(l)
	if len(got) != len(want) {
		t.Fatalf("listed %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestReadDirSkipsHiddenFiles(t *testing.T) {
	t.Parallel()

	l, _, err := readDir(tree(t))
	if err != nil {
		t.Fatalf("readDir: %v", err)
	}
	for _, n := range names(l) {
		if n != ".." && strings.HasPrefix(n, ".") {
			t.Errorf("listed %q, which is hidden", n)
		}
	}
}

// A home directory can hold thousands of files, and a listing longer than the
// screen is one nobody can pick a number out of.
func TestReadDirCapsTheListingAndSaysHowMuchIsMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	const total = maxListing + 7
	for i := range total {
		if err := os.WriteFile(filepath.Join(dir, string(rune('a'+i%26))+string(rune('a'+i/26))), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	l, hidden, err := readDir(dir)
	if err != nil {
		t.Fatalf("readDir: %v", err)
	}
	if hidden != 7 {
		t.Errorf("hidden = %d, want 7", hidden)
	}

	// ".." is prepended after the cap, so a way back is never what got cut.
	if got := len(l.entries); got != maxListing+1 {
		t.Errorf("listed %d entries, want %d plus the parent", got, maxListing)
	}
	if l.entries[0].name != ".." {
		t.Error("the way back was lost to the cap")
	}
}

func TestReadDirGivesTheRootNoParent(t *testing.T) {
	t.Parallel()

	l, _, err := readDir("/")
	if err != nil {
		t.Fatalf("readDir: %v", err)
	}
	for _, n := range names(l) {
		if n == ".." {
			t.Error("the root was given a parent to walk into")
		}
	}
}

func TestListingPathIsCountedFromOne(t *testing.T) {
	t.Parallel()

	dir := tree(t)
	l, _, err := readDir(dir)
	if err != nil {
		t.Fatalf("readDir: %v", err)
	}

	if _, _, ok := l.path(0); ok {
		t.Error("0 resolved to something; the listing is numbered from one")
	}
	if _, _, ok := l.path(len(l.entries) + 1); ok {
		t.Error("a number past the end resolved to something")
	}

	p, e, ok := l.path(2)
	if !ok {
		t.Fatal("2 resolved to nothing")
	}
	if e.name != "archive" || !e.isDir {
		t.Errorf("entry 2 = %+v, want the directory", e)
	}
	if p != filepath.Join(dir, "archive") {
		t.Errorf("path = %q, want it under the listed directory", p)
	}
}

func TestListingPathOnNothingListed(t *testing.T) {
	t.Parallel()

	var l *listing
	if _, _, ok := l.path(1); ok {
		t.Error("a number resolved against a listing that was never made")
	}
}

// A name is resolved where the listing is, not where homa was started.
// Resolving against a working directory nobody can see from inside a
// conversation would make the same command mean two things.
func TestUnderListingResolvesAgainstTheListing(t *testing.T) {
	t.Parallel()

	got, err := underListing("/tmp/fsend", "archive")
	if err != nil {
		t.Fatalf("underListing: %v", err)
	}
	if got != "/tmp/fsend/archive" {
		t.Errorf("got %q, want it under the listing", got)
	}

	// An absolute path is left alone.
	if got, _ := underListing("/tmp/fsend", "/etc/hosts"); got != "/etc/hosts" {
		t.Errorf("an absolute path became %q", got)
	}

	// With nothing listed there is nothing to resolve against, and the
	// working directory is the only sensible answer left.
	if got, _ := underListing("", "notes.md"); got != "notes.md" {
		t.Errorf("with no listing, got %q", got)
	}

	// Walking out is a path like any other.
	if got, _ := underListing("/tmp/fsend/archive", ".."); got != "/tmp/fsend" {
		t.Errorf("going up gave %q", got)
	}
}
