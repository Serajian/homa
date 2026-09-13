package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fakeOpener puts a script named open on the PATH that writes down what it
// was given, and points folderTool at it.
func fakeOpener(t *testing.T) (dir, got string) {
	t.Helper()

	dir = t.TempDir()
	got = filepath.Join(dir, "argv")
	script := "#!/bin/sh\n/bin/echo \"$@\" > " + got + "\n"
	if err := os.WriteFile(filepath.Join(dir, "fakeopen"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)
	was := folderTool
	folderTool = func() []string { return []string{"fakeopen"} }
	t.Cleanup(func() { folderTool = was })

	return dir, got
}

func TestOpeningAFolderHandsItToTheMachine(t *testing.T) {
	dir, got := fakeOpener(t)
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("SSH_TTY", "")

	msg, ok := openFolder(dir)().(folderOpened)
	if !ok {
		t.Fatalf("answered with %T", msg)
	}
	if msg.err != nil || msg.path != dir {
		t.Fatalf("%+v", msg)
	}

	argv, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(argv) != dir+"\n" {
		t.Errorf("the opener was given %q, want %q", argv, dir)
	}
}

// Over ssh the window would open on the machine homa runs on, where nobody
// is looking, so it is refused and the path is handed back instead.
func TestAFolderIsNotOpenedOverSSH(t *testing.T) {
	dir, got := fakeOpener(t)
	t.Setenv("SSH_CONNECTION", "10.0.0.2 51234 10.0.0.1 22")

	msg := openFolder(dir)().(folderOpened)
	if !errors.Is(msg.err, errOverSSH) || msg.path != dir {
		t.Errorf("%+v", msg)
	}
	if _, err := os.Stat(got); err == nil {
		t.Error("the opener ran anyway")
	}
}

func TestAMachineWithNoOpenerSaysSo(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("SSH_TTY", "")
	was := folderTool
	folderTool = func() []string { return nil }
	t.Cleanup(func() { folderTool = was })

	msg := openFolder("/tmp/x")().(folderOpened)
	if !errors.Is(msg.err, errNoOpener) || msg.path != "/tmp/x" {
		t.Errorf("%+v", msg)
	}
}
