package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// A fake pbcopy on the PATH, writing what it is given to a file, stands
// in for the real one.
func TestCopyGoesThroughTheTerminalAndALocalTool(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "clip.txt")
	script := "#!/bin/sh\n/bin/cat > " + out + "\n"
	if err := os.WriteFile(filepath.Join(dir, "pbcopy"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	msgs := copyToClipboard("tcpABC")()
	batch, ok := msgs.(tea.BatchMsg)
	if !ok {
		t.Fatalf("not a batch: %T", msgs)
	}
	var sawTerminal, sawTool bool
	for _, cmd := range batch {
		switch m := cmd().(type) {
		case copied:
			sawTool = m.tool == "pbcopy"
		default:
			sawTerminal = true // tea's own clipboard message, whatever its type
		}
	}
	if !sawTerminal || !sawTool {
		t.Errorf("terminal %v, tool %v", sawTerminal, sawTool)
	}
	if got, _ := os.ReadFile(out); string(got) != "tcpABC" {
		t.Errorf("the tool received %q", got)
	}
}

func TestNoToolIsNotAFailure(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if tool := localClipboardTool(); tool != nil {
		t.Errorf("found %v on an empty PATH", tool)
	}
	if got := copyWithTool("x", nil); got != "" {
		t.Errorf("no tool reported as %q", got)
	}
	if s := copiedNotice(""); s == "" || s == copiedNotice("pbcopy") {
		t.Errorf("notice %q", s)
	}
}
