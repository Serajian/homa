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
	// The exact rate depends on the clock, so what is asserted is that
	// there is one; perSecond and leftText are pinned in format_test.go.
	if !strings.Contains(second, "B/s") || !strings.Contains(second, "left") {
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

// dirWith makes a directory holding the given names; a name ending in a
// separator is made as a directory.
func dirWith(t *testing.T, names ...string) string {
	t.Helper()

	dir := t.TempDir()
	for _, n := range names {
		if strings.HasSuffix(n, "/") {
			if err := os.Mkdir(filepath.Join(dir, strings.TrimSuffix(n, "/")), 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestMatchPathPrefersDirectoriesAndHidesDotfiles(t *testing.T) {
	t.Parallel()

	all := []pathMatch{
		{name: "poster.png"},
		{name: "posters", isDir: true},
		{name: ".pos", isDir: true},
		{name: "other"},
	}
	got := matchPath(all, "pos")
	if len(got) != 2 || got[0].name != "posters" || !got[0].isDir || got[1].name != "poster.png" {
		t.Errorf("matched %+v", got)
	}
	if hidden := matchPath(all, ".p"); len(hidden) != 1 || hidden[0].name != ".pos" {
		t.Errorf("a typed dot did not ask for hidden names: %+v", hidden)
	}
	if none := matchPath(all, "zzz"); len(none) != 0 {
		t.Errorf("matched %+v", none)
	}
}

func TestSplitPathAndCommonPrefix(t *testing.T) {
	t.Parallel()

	for arg, want := range map[string][2]string{
		"poster":     {"", "poster"},
		"~/Down":     {"~/", "Down"},
		"/tmp/po":    {"/tmp/", "po"},
		"Downloads/": {"Downloads/", ""},
		"":           {"", ""},
	} {
		settled, prefix := splitPath(arg)
		if settled != want[0] || prefix != want[1] {
			t.Errorf("splitPath(%q) = %q, %q; want %q, %q", arg, settled, prefix, want[0], want[1])
		}
	}

	if got := commonPrefix([]pathMatch{{name: "poster.png"}, {name: "posters"}}); got != "poster" {
		t.Errorf("commonPrefix = %q", got)
	}
	if got := commonPrefix([]pathMatch{{name: "a"}, {name: "b"}}); got != "" {
		t.Errorf("commonPrefix of nothing shared = %q", got)
	}
	if got := commonPrefix(nil); got != "" {
		t.Errorf("commonPrefix of none = %q", got)
	}
}

// Tab after /send completes the path: one candidate outright, a directory
// with its separator, several as far as they agree.
func TestTabCompletesAPathAfterSend(t *testing.T) {
	t.Parallel()

	dir := dirWith(t, "poster.png", "posters/", "notes.md")
	st := newStyles(true)
	c := newConversation(st, 100, 24, testLine("alice", true), "alice", "")

	// Several candidates: it grows as far as they agree.
	typeInto(c, st, "/send "+filepath.Join(dir, "pos"))
	_, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyTab})
	if got, want := c.in.Value(), "/send "+filepath.Join(dir, "poster"); got != want {
		t.Fatalf("value %q, want %q", got, want)
	}
	// And the row says what they are, directories first.
	if _, body, _ := c.view(st, 100); !strings.Contains(stripANSI(body), "posters/") ||
		!strings.Contains(stripANSI(body), "poster.png") {
		t.Errorf("row:\n%s", stripANSI(body))
	}

	// One candidate: it is taken whole, with the separator when it is a
	// directory, so the next Tab looks inside it.
	c.in.SetValue("/send " + filepath.Join(dir, "posters"))
	_, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyTab})
	if got, want := c.in.Value(), "/send "+filepath.Join(dir, "posters")+"/"; got != want {
		t.Errorf("value %q, want %q", got, want)
	}

	// Nothing matches: the line is left alone.
	c.in.SetValue("/send " + filepath.Join(dir, "zzz"))
	before := c.in.Value()
	_, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyTab})
	if c.in.Value() != before {
		t.Errorf("value %q", c.in.Value())
	}
}

// The same after /files, and a command that takes no path is untouched.
func TestTabCompletesAPathAfterFilesAndNowhereElse(t *testing.T) {
	t.Parallel()

	dir := dirWith(t, "archive/", "art.txt")
	st := newStyles(true)
	c := newConversation(st, 100, 24, testLine("alice", true), "alice", "")

	// "arc" can only be the directory, so it is taken whole.
	typeInto(c, st, "/files "+filepath.Join(dir, "arc"))
	_, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyTab})
	if got, want := c.in.Value(), "/files "+filepath.Join(dir, "archive")+"/"; got != want {
		t.Errorf("value %q, want %q", got, want)
	}

	// "ar" is both, and they agree on no more than what is typed.
	c.in.SetValue("/files " + filepath.Join(dir, "ar"))
	before := c.in.Value()
	_, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyTab})
	if c.in.Value() != before {
		t.Errorf("it chose between two candidates: %q", c.in.Value())
	}

	c.in.SetValue("/who " + filepath.Join(dir, "ar"))
	before = c.in.Value()
	_, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyTab})
	if c.in.Value() != before {
		t.Errorf("a command with no path was completed: %q", c.in.Value())
	}
}
