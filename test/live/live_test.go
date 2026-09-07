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
	"syscall"
	"testing"
	"time"
)

// instance is one homa process with a config directory of its own, driven
// through its standard input and watched through its standard output.
//
// Standard input is a pipe that is never closed, so nothing here can pass
// because the program saw the end of its input. Ctrl+C has to be a signal,
// and a call has to be answered by something typed.
type instance struct {
	t    *testing.T
	name string
	cmd  *exec.Cmd
	in   *os.File
	home string

	mu  sync.Mutex
	out strings.Builder
}

func start(t *testing.T, name string) *instance {
	t.Helper()

	home := t.TempDir()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("making a pipe: %v", err)
	}

	cmd := exec.CommandContext(t.Context(), homaBinary(t),
		"-log", filepath.Join(home, "homa.log"), "-debug")
	cmd.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"))
	cmd.Stdin = r

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("taking stdout: %v", err)
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		t.Fatalf("starting homa: %v", err)
	}
	_ = r.Close()

	in := &instance{t: t, name: name, cmd: cmd, in: w, home: home}

	// Read raw bytes rather than lines. homa writes its prompts and its
	// countdowns without a newline — that is what Prompt is for — so a
	// line scanner would hold them until something else flushed, and every
	// wait here would be deciding on a screen that lags behind the real
	// one. This cost an afternoon to find.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				in.mu.Lock()
				in.out.Write(buf[:n])
				in.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()

	t.Cleanup(func() {
		_ = w.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	// The two first-run questions.
	in.send(name)
	in.send("")
	in.await("listening for callers")

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

func (i *instance) send(line string) {
	i.t.Helper()

	if _, err := i.in.WriteString(line + "\n"); err != nil {
		i.t.Fatalf("%s: typing: %v", i.name, err)
	}
}

func (i *instance) screen() string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.out.String()
}

// await waits for something to appear on screen, and says what was on it when
// it gives up. A failure that only says "timed out" is a failure nobody can
// act on.
func (i *instance) await(what string) {
	i.t.Helper()

	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(i.screen(), what) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	i.t.Fatalf("%s: waited for %q. Screen was:\n%s", i.name, what, i.screen())
}

func (i *instance) refute(what string) {
	i.t.Helper()

	if strings.Contains(i.screen(), what) {
		i.t.Errorf("%s: screen holds %q and should not:\n%s", i.name, what, i.screen())
	}
}

var addrPattern = regexp.MustCompile(`[A-Za-z0-9+/=_.:-]{60,}`)

func (i *instance) address() string {
	i.t.Helper()

	before := len(i.screen())
	i.send("a")

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if m := addrPattern.FindAllString(i.screen()[before:], -1); len(m) > 0 {
			return m[len(m)-1]
		}
		time.Sleep(100 * time.Millisecond)
	}
	i.t.Fatalf("%s: never showed an address", i.name)
	return ""
}

func (i *instance) addContact(name, addr string) {
	i.t.Helper()

	i.send("n")
	i.send(name)
	i.send(addr)
	i.await(name + " added")
}

func (i *instance) interrupt() {
	i.t.Helper()

	if err := i.cmd.Process.Signal(syscall.SIGINT); err != nil {
		i.t.Fatalf("%s: interrupting: %v", i.name, err)
	}
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

	bob.send("1")
	bob.await("waiting for alice to answer")

	// Nothing has been agreed to, so nothing may claim otherwise.
	bob.refute("talking to alice")

	// A tilde, because alice has never saved bob: it is the name he chose
	// for himself, marked so it cannot pass for one she gave. bob has
	// alice in his address book, so he sees the name he gave her.
	alice.await("~bob is calling")
	alice.send("y")

	alice.await("talking to ~bob")
	bob.await("talking to alice")

	bob.send("salam from bob")
	alice.await("salam from bob")

	alice.send("salam from alice")
	bob.await("salam from alice")
}

func TestARefusedCallIsNeverAConversation(t *testing.T) {
	t.Parallel()

	alice, bob := start(t, "alice"), start(t, "bob")
	bob.addContact("alice", alice.address())

	bob.send("1")
	alice.await("take the call from")
	alice.send("n")

	bob.await("not taking calls right now")
	bob.refute("talking to alice")

	alice.await("was not taken")
}

func TestGivingUpOnACallLeavesHomaRunning(t *testing.T) {
	t.Parallel()

	alice, bob := start(t, "alice"), start(t, "bob")
	bob.addContact("alice", alice.address())

	bob.send("1")
	bob.await("Enter to give up")
	bob.send("")

	bob.await("you stopped calling alice")
	bob.await("What now?")

	// Still alive: the menu it came back to answers.
	bob.send("h")
	bob.await("homa connects two people directly")
}

func TestTheAnsweringSideComesBackWhenTheCallerLeaves(t *testing.T) {
	t.Parallel()

	alice, bob := start(t, "alice"), start(t, "bob")
	bob.addContact("alice", alice.address())

	bob.send("1")
	alice.await("take the call from")
	alice.send("y")
	alice.await("talking to ~bob")

	bob.send("/quit")

	// On its own: nothing is typed at alice after this point.
	alice.await("left the conversation")
	alice.await("What now?")
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

	bob.send("1")
	alice.await("take the call from")
	alice.send("y")
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

	bob.send("1")
	alice.await("~bob is calling")
	alice.send("y")
	alice.await("talking to ~bob")

	// alice is busy now, so carol's call parks rather than being asked
	// about, and a third would be turned away outright.
	carol.send("1")
	alice.await("~carol is calling")

	dave := start(t, "dave")
	dave.addContact("alice", addr)
	dave.send("1")

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

	bob.send("1")
	alice.await("take the call from")
	alice.send("y")
	bob.await("talking to alice")

	bob.send("/send " + src)
	alice.await("wants to send")
	alice.send("y")

	alice.await("saved")
	bob.await("sent.")

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

	bob.send("1")
	alice.await("take the call from")
	alice.send("y")
	bob.await("talking to alice")

	bob.send("/files " + dir)
	bob.await("notes.md")

	// ".." is line 1, so the file is line 2.
	bob.send("/send 2")
	alice.await("wants to send")
	alice.send("y")
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
