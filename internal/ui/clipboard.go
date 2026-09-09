package ui

import (
	"context"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// clipboardTools are the local ways of setting a clipboard, tried in order
// until one is on the PATH: macOS, Wayland, then X11 twice over.
var clipboardTools = [][]string{
	{"pbcopy"},
	{"wl-copy"},
	{"xclip", "-selection", "clipboard"},
	{"xsel", "--clipboard", "--input"},
}

// clipboardTimeout bounds the local tool: a clipboard that needs longer
// than this is one that is not going to take the text.
const clipboardTimeout = 3 * time.Second

// copied is the answer to c on the me page: which local tool took the
// text, or "" when none was there. The terminal is asked regardless.
type copied struct {
	tool string
}

// copyToClipboard sends text to the clipboard two ways at once. OSC 52
// asks the terminal to set the clipboard of the machine the person sits
// at, which is the right one across ssh and tmux, and which some terminals
// ignore (Terminal.app does). A local tool, when there is one, sets this
// machine's clipboard, which is the same machine when there is no ssh.
// Neither way reports success, so the notice says what was sent.
func copyToClipboard(text string) tea.Cmd {
	return tea.Batch(tea.SetClipboard(text), func() tea.Msg {
		return copied{tool: copyWithTool(text, localClipboardTool())}
	})
}

// localClipboardTool is the first of clipboardTools on the PATH, or nil.
func localClipboardTool() []string {
	for _, tool := range clipboardTools {
		if _, err := exec.LookPath(tool[0]); err == nil {
			return tool
		}
	}
	return nil
}

// copyWithTool pipes text into tool and names it, or "" when there was no
// tool or it failed; a failure is not worth a warning when the terminal
// was asked as well.
func copyWithTool(text string, tool []string) string {
	if len(tool) == 0 {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), clipboardTimeout)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		tool[0],
		tool[1:]...) //nolint:gosec // the tool is one of clipboardTools
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		lg.Debug("clipboard tool failed", "tool", tool[0], "err", err)
		return ""
	}
	return tool[0]
}

// copiedNotice is what the notice says once c has been pressed.
func copiedNotice(tool string) string {
	s := "sent to the clipboard through the terminal"
	if tool != "" {
		s += " and " + tool
	}
	return s + "; a terminal may not take it (Terminal.app does not)"
}
