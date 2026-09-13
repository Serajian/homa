package ui

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// folderCandidates are the local ways of opening a folder, one per system.
// Like the clipboard and the sound, homa ships none of this: it asks the
// machine to do what the machine already does.
var folderCandidates = map[string][]string{
	"darwin": {"open"},
	"linux":  {"xdg-open"},
}

// folderTool is the opener this machine has, looked up once.
var folderTool = sync.OnceValue(func() []string {
	tool := folderCandidates[runtime.GOOS]
	if len(tool) == 0 {
		return nil
	}
	if _, err := exec.LookPath(tool[0]); err != nil {
		return nil
	}
	return tool
})

var (
	// errOverSSH is homa running somewhere else: the window would open on
	// the machine homa is on, where nobody is looking.
	errOverSSH = errors.New("ui: homa is running over ssh, so it would open on the far machine")

	// errNoOpener is a system with nothing that opens a folder.
	errNoOpener = errors.New("ui: nothing here knows how to open a folder")
)

// openFolder hands a directory to whatever the machine uses to look at one.
//
// A folder, and never a file. What arrives over homa was chosen by somebody
// else, and handing one of those to whichever application the system picks
// for it would make homa the thing that opened it. Opening the folder puts
// the choice back where it belongs: the person sees what came and decides
// what to open.
func openFolder(dir string) tea.Cmd {
	return func() tea.Msg {
		switch {
		case overSSH():
			return folderOpened{path: dir, err: errOverSSH}
		case len(folderTool()) == 0:
			return folderOpened{path: dir, err: errNoOpener}
		}

		ctx, cancel := context.WithTimeout(context.Background(), folderTimeout)
		defer cancel()

		tool := folderTool()
		args := append(append([]string{}, tool[1:]...), dir)
		cmd := exec.CommandContext(ctx, tool[0], args...) //nolint:gosec // one of folderCandidates
		if err := cmd.Run(); err != nil {
			return folderOpened{path: dir, err: err}
		}
		return folderOpened{path: dir}
	}
}
