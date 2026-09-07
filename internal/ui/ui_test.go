package ui

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// ui.New takes any io.Reader, so a pipe stands in for a keyboard. That is
// what the input pump bought: before it, the only way to test reading was to
// have a terminal.
func newTest(t *testing.T) (*UI, *io.PipeWriter, *strings.Builder) {
	t.Helper()

	r, w := io.Pipe()
	t.Cleanup(func() { _ = w.Close() })

	var out strings.Builder
	return New(r, &out), w, &out
}

func typeLine(t *testing.T, w *io.PipeWriter, s string) {
	t.Helper()

	go func() {
		if _, err := io.WriteString(w, s); err != nil {
			t.Errorf("typing: %v", err)
		}
	}()
}

func TestReadLineDeliversALine(t *testing.T) {
	t.Parallel()

	u, w, _ := newTest(t)
	typeLine(t, w, "salam\n")

	got, err := u.ReadLine(t.Context())
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if got != "salam" {
		t.Errorf("got %q, want %q", got, "salam")
	}
}

// Canceling the context has to return at once, whatever the keyboard is
// doing. This is the whole reason the pump exists: before it, Ctrl+C printed
// a message and then waited for Enter.
func TestReadLineReturnsAsSoonAsTheContextIsCanceled(t *testing.T) {
	t.Parallel()

	u, _, _ := newTest(t)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := u.ReadLine(ctx)
		done <- err
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, ErrCanceled) {
			t.Errorf("err = %v, want ErrCanceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ReadLine did not return when its context was canceled")
	}
}

func TestReadLineReportsTheEndOfInput(t *testing.T) {
	t.Parallel()

	u, w, _ := newTest(t)
	_ = w.Close()

	if _, err := u.ReadLine(t.Context()); !errors.Is(err, ErrCanceled) {
		t.Errorf("err = %v, want ErrCanceled", err)
	}
}

// Input that ends without a final newline still counts. It is what arrives
// when input is piped from a file that does not end in one.
func TestALastLineWithNoNewlineStillArrives(t *testing.T) {
	t.Parallel()

	u, w, _ := newTest(t)
	go func() {
		_, _ = io.WriteString(w, "no newline here")
		_ = w.Close()
	}()

	got, err := u.ReadLine(t.Context())
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if got != "no newline here" {
		t.Errorf("got %q", got)
	}
}

// A stuck paste must not be able to make homa hold an unbounded line.
func TestALineLongerThanTheLimitIsCut(t *testing.T) {
	t.Parallel()

	u, w, _ := newTest(t)
	typeLine(t, w, strings.Repeat("a", maxInputLen*2)+"\n")

	got, err := u.ReadLine(t.Context())
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if len(got) > maxInputLen {
		t.Errorf("line is %d bytes, want no more than %d", len(got), maxInputLen)
	}
}

func TestLinesIsClosedWhenInputEnds(t *testing.T) {
	t.Parallel()

	u, w, _ := newTest(t)
	_ = w.Close()

	select {
	case _, ok := <-u.Lines():
		if ok {
			t.Error("Lines produced a line after input ended")
		}
	case <-time.After(time.Second):
		t.Fatal("Lines was not closed when input ended")
	}
}

// An arrow key is not an action in a line-based terminal: it drops an escape
// sequence into the line. Removing only the escape byte is not enough, since
// the letters after it are ordinary text and would reach the peer.
func TestStripKeys(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"up up down, then typing", "\x1b[A\x1b[A\x1b[Bsalam", "salam"},
		{"nothing but arrows", "\x1b[A\x1b[B", ""},
		{"home and end around text", "\x1b[Hchetori\x1b[F", "chetori"},
		{"a colored paste", "\x1b[31mred\x1b[0m", "red"},
		{"an OSC sequence ended by a bell", "\x1b]0;title\x07after", "after"},
		{"a two-byte escape", "\x1bXafter", "after"},
		{"a stray control byte", "a\x00b", "ab"},
		{"a tab, which somebody can mean", "a\tb", "a\tb"},
		{"ordinary text", "salam donya", "salam donya"},
		{"non-latin text", "سلام دنیا", "سلام دنیا"},
		{"an escape at the very end", "text\x1b", "text"},
		{"a CSI with no final byte", "text\x1b[", "text"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := stripKeys(tc.in); got != tc.want {
				t.Errorf("stripKeys(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// Nothing typed reaches the rest of homa with an escape in it. This is the
// property; the cases above are how it is reached.
func TestNothingTypedArrivesWithAnEscapeInIt(t *testing.T) {
	t.Parallel()

	u, w, _ := newTest(t)
	typeLine(t, w, "\x1b[A\x1b[31msalam\x1b[0m\n")

	got, err := u.ReadLine(t.Context())
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if strings.ContainsRune(got, esc) {
		t.Errorf("line %q still holds an escape", got)
	}
}

// A session's read goroutine prints while this one waits for input, so
// output has to step around the prompt rather than into it.
func TestPrintingStepsAroundAPrompt(t *testing.T) {
	t.Parallel()

	u, _, out := newTest(t)

	u.Prompt("[me] ")
	u.Message("bob", "salam")

	got := out.String()
	if !strings.Contains(got, "[bob] salam") {
		t.Fatalf("output = %q, want the message in it", got)
	}
	// The prompt is put back underneath, so the label stays in front of
	// whatever is being typed.
	if !strings.HasSuffix(got, "[me] ") {
		t.Errorf("output = %q, want it to end with the prompt redrawn", got)
	}
	if !strings.Contains(got, clearLine) {
		t.Error("the prompt was written over rather than taken off the screen")
	}
}

// A prompt replaces a prompt: that is what lets a countdown redraw itself
// once a second instead of filling the screen.
func TestAPromptReplacesAPrompt(t *testing.T) {
	t.Parallel()

	u, _, out := newTest(t)

	u.Prompt("[47s] ")
	u.Prompt("[46s] ")

	got := out.String()
	if strings.Count(got, clearLine) != 1 {
		t.Errorf("output = %q, want the first prompt erased once", got)
	}
	if !strings.HasSuffix(got, "[46s] ") {
		t.Errorf("output = %q, want it to end with the newer prompt", got)
	}
}

func TestErasePromptAndEndPromptDifferAsTheyShould(t *testing.T) {
	t.Parallel()

	// ErasePrompt takes the line off, for a prompt nobody will type into.
	u, _, out := newTest(t)
	u.Prompt("[me] ")
	u.ErasePrompt()
	if got := out.String(); !strings.HasSuffix(got, clearLine) {
		t.Errorf("ErasePrompt left %q", got)
	}

	// EndPrompt leaves it, for a question that ran out of time: the reader
	// should still see what was on the screen.
	u2, _, out2 := newTest(t)
	u2.Prompt("take the call? ")
	u2.EndPrompt()
	if got := out2.String(); !strings.HasSuffix(got, "take the call? \n") {
		t.Errorf("EndPrompt left %q, want the question kept", got)
	}
}

// Clearing has to leave the prompt on the screen, or this package would go on
// believing a line is there that nobody can see.
func TestClearRedrawsAPromptItWipes(t *testing.T) {
	t.Parallel()

	u, _, out := newTest(t)

	u.Prompt("[me] ")
	u.Clear()

	got := out.String()
	if !strings.Contains(got, clearScreen) {
		t.Error("the screen was not cleared")
	}
	if !strings.HasSuffix(got, "[me] ") {
		t.Errorf("output = %q, want the prompt drawn again after the wipe", got)
	}
}

func TestPrintfAlwaysEndsTheLine(t *testing.T) {
	t.Parallel()

	u, _, out := newTest(t)

	u.Printf("no newline here")
	u.Printf("one already\n")

	if got := out.String(); got != "no newline here\none already\n" {
		t.Errorf("output = %q", got)
	}
}

func TestHumanBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{5 * 1024 * 1024 * 1024, "5.0 GB"},
	}

	for _, tc := range cases {
		if got := humanBytes(tc.in); got != tc.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPercentGuardsAgainstAnUnknownTotal(t *testing.T) {
	t.Parallel()

	// A peer can report a size of zero, and dividing by it would end the
	// transfer with a panic rather than an error.
	if got := percent(5, 0); got != 0 {
		t.Errorf("percent(5, 0) = %d, want 0", got)
	}
	if got := percent(1, 4); got != 25 {
		t.Errorf("percent(1, 4) = %d, want 25", got)
	}
}

// An error shown to a person should not name the package that noticed it.
func TestReasonDropsThePackagePrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   error
		want string
	}{
		{errors.New("session: the call was not taken"), "the call was not taken"},
		{errors.New("contacts: no such contact"), "no such contact"},
		{errors.New("ui: there is no 9 in the last listing"), "there is no 9 in the last listing"},
		{errors.New("no prefix at all"), "no prefix at all"},
		{errors.New("a message: with a colon in it"), "a message: with a colon in it"},
		{nil, ""},
	}

	for _, tc := range cases {
		if got := reason(tc.in); got != tc.want {
			t.Errorf("reason(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
