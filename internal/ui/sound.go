package ui

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// soundCandidates are the local ways of playing a short sound, per OS,
// tried in order until one is on the PATH. Each is a tool the machine
// already has: homa ships no sound and links no audio library.
var soundCandidates = map[string][][]string{
	"darwin": {
		{"afplay", "/System/Library/Sounds/Glass.aiff"},
	},
	"linux": {
		{"paplay", "/usr/share/sounds/freedesktop/stereo/bell.oga"},
		{"pw-play", "/usr/share/sounds/freedesktop/stereo/bell.oga"},
		{"canberra-gtk-play", "-i", "bell"},
	},
}

// soundTool is the player found on this machine, looked up once: nil when
// there is none, or when homa runs over ssh, where a sound would come out
// of the wrong machine.
var soundTool = sync.OnceValue(func() []string {
	if overSSH() {
		return nil
	}
	return pickSoundTool(soundCandidates[runtime.GOOS])
})

func overSSH() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != ""
}

// pickSoundTool is the first candidate whose tool is on the PATH and whose
// sound file, when it names one, exists.
func pickSoundTool(candidates [][]string) []string {
	for _, c := range candidates {
		if _, err := exec.LookPath(c[0]); err != nil {
			continue
		}
		if len(c) > 1 && c[1][0] == '/' {
			if _, err := os.Stat(c[1]); err != nil {
				continue
			}
		}
		return c
	}
	return nil
}

// playSound plays the machine's sound, off the update loop, and reports
// nothing: a sound that could not be played is not worth a line on the
// screen, and the bell byte went out regardless.
func playSound(tool []string) tea.Cmd {
	if len(tool) == 0 {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), soundTimeout)
		defer cancel()
		cmd := exec.CommandContext(
			ctx,
			tool[0],
			tool[1:]...) //nolint:gosec // one of soundCandidates
		if err := cmd.Run(); err != nil {
			lg.Debug("sound tool failed", "tool", tool[0], "err", err)
		}
		return nil
	}
}
