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

	if got := u.peer("alice"); got != colorGreen+"alice"+colorReset {
		t.Errorf("a contact's name = %q", got)
	}
	want := colorMuted + unknownMark + colorReset + colorGreen + "bob" + colorReset
	if got := u.peer("~bob"); got != want {
		t.Errorf("a self-chosen name = %q, want %q", got, want)
	}
}

// A grey line with a green name in it has to stay grey after the name: the
// inner reset would otherwise end the outer color too.
func TestPaintPutsTheOuterColorBackAfterAnInnerReset(t *testing.T) {
	t.Parallel()

	st := style{color: true}
	got := paint(st, colorMuted, "talking to "+paint(st, colorGreen, "alice")+" now")
	want := colorMuted + "talking to " + colorGreen + "alice" + colorReset + colorMuted + " now" + colorReset
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
