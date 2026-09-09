package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// An offer parks until y or n; the answer reaches the goroutine waiting on
// it, with the directory decided at that moment.
func TestAnOfferIsAnsweredWithYAndTheDirectory(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversationWith(
		t.Context(),
		st,
		80,
		20,
		testLine("~bob", false),
		"bob",
		"~/dl",
		func(tea.Msg) {},
		func() (string, error) { return "/tmp/dl", nil },
	)
	reply := make(chan fileAnswer, 1)
	c.offered(st, fileOffered{name: "notes.md", size: 14 * 1024, reply: reply})

	if got := stripANSI(c.render()); !strings.Contains(
		got,
		"~bob offers notes.md (14.0 KB)",
	) ||
		!strings.Contains(got, " y  accept") {
		t.Errorf("offer line:\n%s", got)
	}

	typeInto(c, st, "y")
	_, _ = c.update(st, key("enter"))
	select {
	case ans := <-reply:
		if !ans.accept || ans.dir != "/tmp/dl" {
			t.Errorf("answer = %+v", ans)
		}
	default:
		t.Fatal("y did not answer the offer")
	}
	if c.offer != nil {
		t.Error("the offer is still parked")
	}
}

func TestSlashFilesListsAndSlashSendPicksANumber(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "archive"), 0o700); err != nil {
		t.Fatal(err)
	}

	st := newStyles(true)
	c := newConversation(st, 80, 20, testLine("alice", true), "alice", "")
	c.showFiles(st, dir)
	got := stripANSI(c.render())
	if !strings.Contains(got, "1  ../") || !strings.Contains(got, "archive/") ||
		!strings.Contains(got, "notes.md") {
		t.Errorf("listing:\n%s", got)
	}

	path, err := fileFromArg(c.files, "3")
	if err != nil || filepath.Base(path) != "notes.md" {
		t.Errorf("/send 3 → %q, %v", path, err)
	}
	if _, err := fileFromArg(c.files, "2"); err == nil ||
		!strings.Contains(err.Error(), "directory") {
		t.Errorf("/send of a directory: %v", err)
	}
	if _, err := fileFromArg(c.files, "9"); err == nil || !strings.Contains(err.Error(), "no 9") {
		t.Errorf("/send of a missing number: %v", err)
	}
	if d, err := dirFromArg(c.files, "2"); err != nil || filepath.Base(d) != "archive" {
		t.Errorf("/files 2 → %q, %v", d, err)
	}
}
