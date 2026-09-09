package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// A directory used to be offered and then fail when it was read, with the
// far side already waiting. It is refused before anything is offered.
func TestSendRefusesADirectoryBeforeOfferingIt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "poster"), 0o755); err != nil {
		t.Fatal(err)
	}
	st := newStyles(true)
	c := newConversation(st, 100, 20, testLine("alice", true), "alice", "")

	if cmd := c.sendFile(st, filepath.Join(dir, "poster")); cmd != nil {
		t.Error("a directory was offered")
	}
	pane := stripANSI(c.render())
	if !strings.Contains(pane, "is a directory") || !strings.Contains(pane, "lists it") {
		t.Errorf("pane:\n%s", pane)
	}
	if strings.Contains(pane, "offering") {
		t.Errorf("something was offered:\n%s", pane)
	}
}

// A progress line says how fast and how much longer, once there has been
// enough of the transfer to say either.
func TestAProgressLineGrowsARateAndAnEstimate(t *testing.T) {
	t.Parallel()

	m := sized(newModel(t.Context(), testDeps(t), newStyles(true)))
	m.screen = screenConversation
	m.conv = newConversation(m.st, 100, 24, testLine("alice", true), "alice", "")

	// The first report starts the clock, so it carries neither.
	next, _ := m.Update(fileProgress{name: "big.bin", received: 1 << 20, total: 10 << 20})
	m = next.(model)
	first := stripANSI(m.conv.render())
	if !strings.Contains(first, "receiving big.bin") || !strings.Contains(first, "10%") {
		t.Errorf("pane:\n%s", first)
	}
	if strings.Contains(first, "/s") || strings.Contains(first, "left") {
		t.Errorf("a rate before there was one:\n%s", first)
	}

	// Wind the clock back and report again: now it can say both.
	m.conv.getting.started = time.Now().Add(-4 * time.Second)
	next, _ = m.Update(fileProgress{name: "big.bin", received: 4 << 20, total: 10 << 20})
	m = next.(model)
	second := stripANSI(m.conv.render())
	if !strings.Contains(second, "MB/s") || !strings.Contains(second, "left") {
		t.Errorf("pane:\n%s", second)
	}

	// A different file starts its own clock.
	next, _ = m.Update(fileProgress{name: "other.bin", received: 1 << 20, total: 10 << 20})
	m = next.(model)
	if m.conv.getting.name != "other.bin" {
		t.Errorf("the clock did not follow the file: %+v", m.conv.getting)
	}

	// The same for what goes the other way.
	next, _ = m.Update(sending{name: "mine.bin", received: 1 << 20, total: 4 << 20})
	m = next.(model)
	if !strings.Contains(stripANSI(m.conv.render()), "sending mine.bin") {
		t.Errorf("pane:\n%s", stripANSI(m.conv.render()))
	}
}

// /cancel with nothing moving says so, and does not pretend.
func TestCancelWithNothingMoving(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 100, 24, testLine("alice", true), "alice", "")
	typeInto(c, st, "/cancel")
	_, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyEnter})
	// With no session there is nothing to ask, and nothing is claimed.
	if strings.Contains(stripANSI(c.render()), "stopped") {
		t.Errorf("pane:\n%s", stripANSI(c.render()))
	}
}

// A failure for a file this side stopped is not reported twice.
func TestAStoppedTransferIsReportedOnce(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 100, 24, testLine("alice", true), "alice", "")
	c.stopped = map[string]bool{"big.bin": true}

	if !c.wasStopped("big.bin") {
		t.Fatal("the file was not remembered")
	}
	if c.wasStopped("big.bin") {
		t.Error("it was remembered twice")
	}
	if c.wasStopped("other.bin") {
		t.Error("a file nobody stopped")
	}
}
