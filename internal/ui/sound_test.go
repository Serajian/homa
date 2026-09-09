package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func fakePlayer(t *testing.T) (dir, out string) {
	t.Helper()
	dir = t.TempDir()
	out = filepath.Join(dir, "played")
	script := "#!/bin/sh\n/usr/bin/touch " + out + "\n"
	if err := os.WriteFile(filepath.Join(dir, "fakeplay"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir, out
}

func TestTheFirstPlayerOnThePathIsPickedAndPlays(t *testing.T) {
	dir, out := fakePlayer(t)
	t.Setenv("PATH", dir)

	tool := pickSoundTool([][]string{{"nosuchplayer"}, {"fakeplay", "-q"}})
	if len(tool) == 0 || tool[0] != "fakeplay" {
		t.Fatalf("picked %v", tool)
	}
	if got := playSound(tool)(); got != nil {
		t.Errorf("playing reported %v", got)
	}
	if _, err := os.Stat(out); err != nil {
		t.Error("the player never ran")
	}
}

func TestAPlayerNeedsItsSoundFile(t *testing.T) {
	dir, _ := fakePlayer(t)
	t.Setenv("PATH", dir)
	if tool := pickSoundTool([][]string{{"fakeplay", "/no/such/sound.oga"}}); tool != nil {
		t.Errorf("picked a player whose file is missing: %v", tool)
	}
	if playSound(nil) != nil {
		t.Error("no tool is a command")
	}
}

func TestNoSoundOverSSH(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "10.0.0.2 51234 10.0.0.1 22")
	if !overSSH() {
		t.Error("ssh not noticed")
	}
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("SSH_TTY", "")
	if overSSH() {
		t.Error("ssh seen where there is none")
	}
}

// The package's tests never play a sound: the lookup is replaced before
// any test runs, and the one test that wants a player installs a fake.
func init() { soundTool = func() []string { return nil } }

func TestRingSendsTheBellAndPlaysTheSound(t *testing.T) {
	dir, out := fakePlayer(t)
	t.Setenv("PATH", dir)
	soundTool = func() []string { return []string{"fakeplay"} }
	t.Cleanup(func() { soundTool = func() []string { return nil } })

	m := sized(newModel(t.Context(), testDeps(t), newStyles(true)))
	cmd := m.ring()
	if !ringing(cmd) {
		t.Fatal("the bell byte is gone")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("ring is not a batch: %T", cmd())
	}
	for _, c := range batch {
		_ = c() // the bell and the player, both run
	}
	if _, err := os.Stat(out); err != nil {
		t.Error("the sound was not played")
	}
}
