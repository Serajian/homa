//go:build live

package live

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"

	"github.com/Serajian/homa/internal/peer"
)

// instance is one homa process with a config directory of its own, driven
// through a pseudo-terminal and watched on a screen grid.
//
// A pseudo-terminal, because the full-screen interface refuses anything
// else: it draws in place with cursor movement, so what a person sees is
// not the stream of bytes but what the stream leaves on the grid, and the
// waits here look at the grid. Keys are single bytes; a line ends in \r,
// which is what Enter sends. Ctrl+C is a byte too, so the program sees it
// as a key and says goodbye to a peer on the way out.
type instance struct {
	t    *testing.T
	name string
	cmd  *exec.Cmd
	tty  *os.File
	home string
	scr  *screen

	mu  sync.Mutex
	raw []byte // everything the process wrote, for a failure to keep
}

// The terminal every instance gets: wide enough for the two-column menu
// and the header's status, tall enough for a conversation.
const ttyRows, ttyCols = 30, 100

func start(t *testing.T, name string) *instance {
	t.Helper()

	home := t.TempDir()
	if os.Getenv("HOMA_FRAMES") != "" {
		// Screens for the README: a home whose paths read as a person's
		// would, not a test's.
		home = filepath.Join("/tmp", name) // short, so the header keeps its right-hand side
		_ = os.RemoveAll(home)
		if err := os.MkdirAll(home, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(home) })
	}
	cmd := exec.CommandContext(t.Context(), homaBinary(t),
		"-log", filepath.Join(home, "homa.log"), "-debug")
	cmd.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"LANG=en_US.UTF-8", "TERM=xterm-256color", "COLORTERM=truecolor")

	tty, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: ttyRows, Cols: ttyCols})
	if err != nil {
		t.Fatalf("starting homa on a pty: %v", err)
	}

	in := &instance{
		t:    t,
		name: name,
		cmd:  cmd,
		tty:  tty,
		home: home,
		scr:  newScreen(ttyRows, ttyCols),
	}

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := tty.Read(buf)
			if n > 0 {
				in.scr.write(buf[:n])
				in.mu.Lock()
				in.raw = append(in.raw, buf[:n]...)
				in.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()

	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = tty.Close()
	})

	// The two first-run questions, on the setup screen.
	in.await("first run")
	in.line(name)
	in.line("")
	in.await("listening")

	return in
}

func homaBinary(t *testing.T) string {
	t.Helper()

	// Built by `make test-live` before the tests run, so a stale binary
	// cannot make a passing run mean nothing.
	p, err := filepath.Abs(filepath.Join("..", "..", "build", "homa"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("no binary at %s; run make test-live rather than go test", p)
	}
	return p
}

// key presses one key: what a menu, a bar or a page takes.
func (i *instance) key(k string) {
	i.t.Helper()

	if _, err := i.tty.WriteString(k); err != nil {
		i.t.Fatalf("%s: pressing %q: %v", i.name, k, err)
	}
}

// line types a line and Enter: what a form field or the conversation takes.
// A long line goes in pieces with a breath between them, the way a
// terminal delivers a paste, rather than as one burst; see typeSlowly.
func (i *instance) line(s string) {
	i.t.Helper()
	i.typeSlowly(s)
	i.key("\r")
}

// typeSlowly writes text in small pieces. A single write of a few hundred
// bytes reached the program with its beginning missing under load — the
// address field held the address from somewhere in its middle — and a
// person never types that way anyway.
func (i *instance) typeSlowly(s string) {
	i.t.Helper()
	const piece = 32
	for len(s) > 0 {
		n := min(piece, len(s))
		i.key(s[:n])
		s = s[n:]
		time.Sleep(20 * time.Millisecond)
	}
}

func (i *instance) screen() string { return i.scr.text() }

// snapshot writes the screen to HOMA_FRAMES/<name>.txt when that variable
// names a directory. It is how the README's screens are taken: from the
// real program on a real pseudo-terminal, never retyped.
func (i *instance) snapshot(name string) {
	i.t.Helper()
	dir := os.Getenv("HOMA_FRAMES")
	if dir == "" {
		return
	}
	time.Sleep(400 * time.Millisecond) // let the frame settle
	if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(i.screen()), 0o600); err != nil {
		i.t.Fatalf("snapshot %s: %v", name, err)
	}
}

// await waits for something to be on the screen, and says what was on it
// when it gives up. A failure that only says "timed out" is a failure
// nobody can act on.
func (i *instance) await(what string) {
	i.t.Helper()

	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(i.screen(), what) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	i.mu.Lock()
	dump := filepath.Join(
		os.TempDir(),
		"homa-live-"+i.name+"-"+strings.ReplaceAll(i.t.Name(), "/", "_")+".raw",
	)
	_ = os.WriteFile(dump, i.raw, 0o600)
	i.mu.Unlock()
	i.t.Fatalf("%s: waited for %q. Sequences seen: %s. Raw output kept at %s. Screen was:\n%s",
		i.name, what, i.scr.seen(), dump, i.screen())
}

func (i *instance) refute(what string) {
	i.t.Helper()

	if strings.Contains(i.screen(), what) {
		i.t.Errorf("%s: screen holds %q and should not:\n%s", i.name, what, i.screen())
	}
}

// addrPattern matches a row of the address page: the page shows the address
// in rows of one width, the last of which can be short.
var addrPattern = regexp.MustCompile(`[A-Za-z0-9+/=_.:-]{4,}`)

// address opens the address page, reads the address off it, and comes back.
func (i *instance) address() string {
	i.t.Helper()

	i.key("a")
	i.await("Give this to someone")

	// The page wraps the address across rows; the rows that are nothing
	// but address characters, joined, are it. The page is read twice and
	// has to say the same thing both times: under load a frame arrives in
	// pieces, and the first rows of an address are not the address.
	deadline := time.Now().Add(30 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		var parts []string
		for _, row := range strings.Split(i.screen(), "\n") {
			row = strings.TrimSpace(row)
			if addrPattern.MatchString(row) && addrPattern.FindString(row) == row {
				parts = append(parts, row)
			}
		}
		// A homa address is well over two hundred characters; fewer is a
		// page still being drawn.
		addr := strings.Join(parts, "")
		if len(addr) >= 200 && addr == last {
			if !peer.ValidAddr(addr) {
				i.t.Fatalf(
					"%s: read an address off the page that does not parse (%d chars): %q\nrows: %q\nscreen:\n%s",
					i.name,
					len(addr),
					addr,
					parts,
					i.screen(),
				)
			}
			i.key("x") // any key leaves the page
			i.await("PEOPLE")
			return addr
		}
		last = addr
		time.Sleep(500 * time.Millisecond)
	}
	i.t.Fatalf("%s: never showed an address. Screen was:\n%s", i.name, i.screen())
	return ""
}

func (i *instance) addContact(name, addr string) {
	i.t.Helper()

	i.key("n")
	i.await("A name for them")
	i.line(name)
	i.line(addr)
	i.t.Logf("%s typed the address %q", i.name, addr)
	i.await(name + " added")
}

// interrupt is Ctrl+C typed at the terminal, which is how a person sends it.
func (i *instance) interrupt() {
	i.t.Helper()
	i.key("\x03")
}

// waitForExit reports how long the process took to go, so a test can say
// "at once" and mean it.
func (i *instance) waitForExit() time.Duration {
	i.t.Helper()

	started := time.Now()
	done := make(chan error, 1)
	go func() { done <- i.cmd.Wait() }()

	select {
	case err := <-done:
		var exit *exec.ExitError
		if err != nil && !errors.As(err, &exit) {
			i.t.Fatalf("%s: waiting for it to exit: %v", i.name, err)
		}
		return time.Since(started)
	case <-time.After(30 * time.Second):
		i.t.Fatalf("%s: did not exit. Screen was:\n%s", i.name, i.screen())
		return 0
	}
}

func TestACallIsAskedAboutAndPutThrough(t *testing.T) {
	t.Parallel()

	alice, bob := start(t, "alice"), start(t, "bob")
	bob.addContact("alice", alice.address())

	bob.snapshot("menu")
	bob.key("1")
	bob.await("waiting for them to answer")
	bob.snapshot("calling")

	// Nothing has been agreed to, so nothing may claim otherwise.
	bob.refute("talking to alice")

	// A tilde, because alice has never saved bob: it is the name he chose
	// for himself, marked so it cannot pass for one she gave. bob has
	// alice in his address book, so he sees the name he gave her.
	alice.await("~bob is calling")
	alice.snapshot("incoming")
	alice.key("y")

	alice.await("talking to ~bob")
	bob.await("talking to alice")

	bob.line("salam from bob")
	alice.await("salam from bob")

	alice.line("salam from alice")
	bob.await("salam from alice")

	// A half-typed line on bob's side while alice speaks: the fault that
	// started version 2, and the screen that shows it gone.
	bob.typeSlowly("man dar")
	alice.line("chetori?")
	bob.await("chetori?")
	bob.snapshot("conversation")
	alice.snapshot("conversation-answering")
}

func TestARefusedCallIsNeverAConversation(t *testing.T) {
	t.Parallel()

	alice, bob := start(t, "alice"), start(t, "bob")
	bob.addContact("alice", alice.address())

	bob.key("1")
	alice.await("incoming call")
	alice.key("n")

	bob.await("not taking calls right now")
	bob.refute("talking to alice")

	alice.await("was not taken")
}

func TestGivingUpOnACallLeavesHomaRunning(t *testing.T) {
	t.Parallel()

	alice, bob := start(t, "alice"), start(t, "bob")
	bob.addContact("alice", alice.address())

	bob.key("1")
	bob.await("give up")
	bob.key("\r")

	bob.await("you stopped calling alice")
	bob.await("PEOPLE")

	// Still alive: the menu it came back to answers.
	bob.key("h")
	bob.await("homa connects two people directly")
}

func TestTheAnsweringSideComesBackWhenTheCallerLeaves(t *testing.T) {
	t.Parallel()

	alice, bob := start(t, "alice"), start(t, "bob")
	bob.addContact("alice", alice.address())

	bob.key("1")
	alice.await("incoming call")
	alice.key("y")
	alice.await("talking to ~bob")

	bob.line("/quit")

	// On its own: nothing is typed at alice after this point. The pane
	// says they left; the menu is one Enter away.
	alice.await("left the conversation")
	alice.line("")
	alice.await("PEOPLE")
}

func TestInterruptingAtTheMenuExitsAtOnce(t *testing.T) {
	t.Parallel()

	alice := start(t, "alice")

	alice.interrupt()

	if took := alice.waitForExit(); took > 5*time.Second {
		t.Errorf("took %s to exit, want it at once", took)
	}
	if !strings.Contains(alice.screen(), "bye.") {
		t.Errorf("no goodbye on the way out:\n%s", alice.screen())
	}
}

func TestInterruptingInAConversationTellsThePeer(t *testing.T) {
	t.Parallel()

	alice, bob := start(t, "alice"), start(t, "bob")
	bob.addContact("alice", alice.address())

	bob.key("1")
	alice.await("incoming call")
	alice.key("y")
	bob.await("talking to alice")

	bob.interrupt()
	if took := bob.waitForExit(); took > 5*time.Second {
		t.Errorf("took %s to exit, want it at once", took)
	}

	alice.await("left the conversation")
}

func TestASecondCallerIsToldTheLineIsBusy(t *testing.T) {
	t.Parallel()

	alice := start(t, "alice")
	bob, carol := start(t, "bob"), start(t, "carol")

	addr := alice.address()
	bob.addContact("alice", addr)
	carol.addContact("alice", addr)

	bob.key("1")
	alice.await("~bob is calling")
	alice.key("y")
	alice.await("talking to ~bob")

	// alice is busy now, so carol's call parks, unseen, until the
	// conversation ends, and a third is turned away outright.
	carol.key("1")
	carol.await("waiting for them to answer")

	dave := start(t, "dave")
	dave.addContact("alice", addr)
	dave.key("1")

	dave.await("busy")
}

func TestAFileCrossesAndKeepsItsContents(t *testing.T) {
	t.Parallel()

	alice, bob := start(t, "alice"), start(t, "bob")
	bob.addContact("alice", alice.address())

	// Several chunks, so this is a transfer rather than one frame.
	content := strings.Repeat("homa, the bird that never lands. ", 4000)
	src := filepath.Join(t.TempDir(), "poster.txt")
	if err := os.WriteFile(src, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	bob.key("1")
	alice.await("incoming call")
	alice.key("y")
	bob.await("talking to alice")

	bob.line("/send " + src)
	alice.await("offers")
	alice.snapshot("offer")
	alice.line("y")

	alice.await("saved")
	bob.await("sent.")
	alice.snapshot("received")

	got := findFile(t, alice.home, "poster.txt")
	if string(got) != content {
		t.Errorf("the file that arrived is not the file that was sent (%d bytes vs %d)",
			len(got), len(content))
	}
}

func TestAFileIsPickedFromAListing(t *testing.T) {
	t.Parallel()

	alice, bob := start(t, "alice"), start(t, "bob")
	bob.addContact("alice", alice.address())

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("salam"), 0o600); err != nil {
		t.Fatal(err)
	}

	bob.key("1")
	alice.await("incoming call")
	alice.key("y")
	bob.await("talking to alice")

	bob.line("/files " + dir)
	bob.await("notes.md")

	// ".." is line 1, so the file is line 2.
	bob.line("/send 2")
	alice.await("offers")
	alice.line("y")
	alice.await("saved")

	if got := findFile(t, alice.home, "notes.md"); string(got) != "salam" {
		t.Errorf("arrived as %q", got)
	}
}

func findFile(t *testing.T, home, name string) []byte {
	t.Helper()

	var found string
	err := filepath.Walk(home, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && info.Name() == name {
			found = p
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == "" {
		t.Fatalf("no %s anywhere under %s", name, home)
	}

	b, err := os.ReadFile(found)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
