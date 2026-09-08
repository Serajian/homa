package ui

import (
	"regexp"
	"strings"
	"testing"
)

var escapes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripColor takes the color out, leaving what somebody without it reads.
func stripColor(s string) string { return escapes.ReplaceAllString(s, "") }

// everyScreen drives the painted primitives through one of everything, so the
// colored and the plain output can be held to the same text.
func everyScreen(u *UI) {
	u.Info("talking to %s (they call themselves %q)", u.peer("alice"), "alice")
	u.Info("%s is calling", u.peer("~bob"))
	u.Warn("could not reach %s: no route", u.peer("alice"))
	u.Message("~bob", "salam: chetori")
	u.Message("alice", "khoobam")
	u.Prompt("%s", u.meLabel())
	u.ErasePrompt()
	u.Printf("%s%s %s: ", u.promptMark(), "Their address", u.dim("[none]"))
	_ = u.ShowMenu("What now?",
		[]menuItem{{key: "1", text: "call %s", name: "alice"}, {key: "10", text: "call %s", name: "~bob"}},
		[]menuItem{{key: "q", text: "quit homa", quiet: true}})
}

func TestColorChangesNothingButColor(t *testing.T) {
	t.Parallel()

	colored, _, out1 := newTerminalTest(t)
	colored.st.color = true
	everyScreen(colored)

	plain, _, out2 := newTerminalTest(t)
	everyScreen(plain)

	if strings.Count(out1.String(), "\033[") == 0 {
		t.Fatal("color was on and nothing was painted")
	}
	if got, want := stripColor(out1.String()), out2.String(); got != want {
		t.Errorf("with the color taken out:\n%q\nwithout color:\n%q", got, want)
	}
}

func TestAPeerIsGreenAndTheirOwnMarkIsGrey(t *testing.T) {
	t.Parallel()

	u, _, _ := newTerminalTest(t)
	u.st.color = true

	green, grey := u.st.code(roleThem), u.st.code(roleDim)
	if got := u.peer("alice"); got != green+"alice"+colorReset {
		t.Errorf("a contact's name = %q", got)
	}
	want := grey + unknownMark + colorReset + green + "bob" + colorReset
	if got := u.peer("~bob"); got != want {
		t.Errorf("a self-chosen name = %q, want %q", got, want)
	}
}

// A grey line with a green name in it has to stay grey after the name: the
// inner reset would otherwise end the outer color too.
func TestPaintPutsTheOuterColorBackAfterAnInnerReset(t *testing.T) {
	t.Parallel()

	st := style{color: true, truecolor: true}
	got := paint(st, roleDim, "talking to "+paint(st, roleThem, "alice")+" now")
	want := color24Muted + "talking to " + color24Green + "alice" + colorReset + color24Muted + " now" + colorReset
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// Anything that is not a terminal — a pipe, a file — gets no escape sequence
// at all, whatever is printed around a prompt. A line that would have been
// erased is ended instead.
func TestAPipeGetsNewlinesWhereATerminalGetsErased(t *testing.T) {
	t.Parallel()

	u, _, out := newTest(t)
	u.Prompt("[me] ")
	u.Message("bob", "salam")
	u.Prompt("[46s] ")
	u.Clear()
	u.ErasePrompt()

	got := out.String()
	if strings.Contains(got, "\033") {
		t.Fatalf("an escape sequence reached a pipe: %q", got)
	}
	want := "[me] \n[bob] salam\n[me] \n[46s] \n[46s] \n"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestATerminalStillGetsThePromptErased(t *testing.T) {
	t.Parallel()

	u, _, out := newTerminalTest(t)
	u.Prompt("[me] ")
	u.Message("bob", "salam")

	if got := out.String(); !strings.Contains(got, clearLine) {
		t.Errorf("no erase on a terminal: %q", got)
	}
}

// The logo's 24-bit palette is used only when the terminal says it can show
// it; everything else gets the sixteen ANSI colors, which every terminal has.
// "You" is bold in the terminal's own foreground in both, so it is visible on
// a light theme as well as a dark one.
func TestTheTerminalChoosesThePalette(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		env       map[string]string
		truecolor bool
	}{
		{"nothing said", map[string]string{}, false},
		{"COLORTERM=truecolor", map[string]string{"COLORTERM": "truecolor"}, true},
		{"COLORTERM=24bit", map[string]string{"COLORTERM": "24bit"}, true},
		{"TERM=xterm-direct", map[string]string{"TERM": "xterm-direct"}, true},
		{"COLORTERM=256color is not it", map[string]string{"COLORTERM": "256color"}, false},
		{"truecolor but NO_COLOR", map[string]string{"COLORTERM": "truecolor", "NO_COLOR": "1"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			st := styleFor(80, env(c.env))
			if st.truecolor != c.truecolor {
				t.Errorf("truecolor = %v, want %v", st.truecolor, c.truecolor)
			}
		})
	}

	basic := style{color: true}
	if got := paint(basic, roleThem, "x"); got != color16Green+"x"+colorReset {
		t.Errorf("them without truecolor = %q", got)
	}
	if got := paint(basic, roleDim, "x"); got != color16Muted+"x"+colorReset {
		t.Errorf("dim without truecolor = %q", got)
	}
	for _, st := range []style{basic, {color: true, truecolor: true}} {
		if got := paint(st, roleYou, "x"); got != colorYou+"x"+colorReset {
			t.Errorf("you = %q in %+v; want bold in the terminal's own color", got, st)
		}
		if got := paint(st, roleWarn, "x"); got != colorWarn+"x"+colorReset {
			t.Errorf("warn = %q in %+v; want the terminal's yellow", got, st)
		}
	}
}
