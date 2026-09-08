package ui

import (
	"strings"
	"testing"
)

// env builds a getenv for styleFor from a map, so the rules can be tried
// without touching the process environment.
func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestStyleForHonorsNoColorAndADumbTerminal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"nothing set", map[string]string{}, true},
		{"NO_COLOR set", map[string]string{"NO_COLOR": "1"}, false},
		{"NO_COLOR set to anything", map[string]string{"NO_COLOR": "no"}, false},
		{"NO_COLOR present but empty does not count", map[string]string{"NO_COLOR": ""}, true},
		{"TERM=dumb", map[string]string{"TERM": "dumb"}, false},
		{"TERM=xterm-256color", map[string]string{"TERM": "xterm-256color"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := styleFor(80, env(c.env)).color; got != c.want {
				t.Errorf("color = %v, want %v", got, c.want)
			}
		})
	}
}

func TestStyleForNeedsAUTF8Locale(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"no locale at all", map[string]string{}, false},
		{"LANG UTF-8", map[string]string{"LANG": "en_US.UTF-8"}, true},
		{"LANG utf8, lower and without the dash", map[string]string{"LANG": "fa_IR.utf8"}, true},
		{"LANG=C", map[string]string{"LANG": "C"}, false},
		{
			"LC_ALL=C overrides a UTF-8 LANG",
			map[string]string{"LC_ALL": "C", "LANG": "en_US.UTF-8"},
			false,
		},
		{
			"LC_CTYPE UTF-8 overrides LANG=C",
			map[string]string{"LC_CTYPE": "en_US.UTF-8", "LANG": "C"},
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := styleFor(80, env(c.env)).unicode; got != c.want {
				t.Errorf("unicode = %v, want %v", got, c.want)
			}
		})
	}
}

func TestBannerIsOnePlainLineWhenThereIsNoTerminal(t *testing.T) {
	t.Parallel()

	got := banner(style{})
	if strings.Contains(got, "\033") {
		t.Errorf("banner for a pipe contains an escape sequence: %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("banner for a pipe is more than one line: %q", got)
	}
	if !strings.Contains(got, bannerPlain) {
		t.Errorf("banner for a pipe does not say %q: %q", bannerPlain, got)
	}
}

func TestBannerDrawsBlocksOnAWideUTF8Terminal(t *testing.T) {
	t.Parallel()

	got := banner(style{unicode: true, width: 80})
	if !strings.Contains(got, "█") {
		t.Fatalf("no block characters in %q", got)
	}
	if strings.Contains(got, "\033") {
		t.Errorf("color was off, but there is an escape in %q", got)
	}
	if !strings.Contains(got, bannerWordmark) || !strings.Contains(got, bannerTagline) {
		t.Errorf("the wordmark or tagline is missing from %q", got)
	}

	// Nothing may be wider than the terminal it was drawn for.
	for _, line := range strings.Split(got, "\n") {
		if n := len([]rune(line)); n > len(bannerIndent)+bannerCols {
			t.Errorf("a row is %d columns wide: %q", n, line)
		}
	}
}

func TestBannerColorIsBalancedAndOptional(t *testing.T) {
	t.Parallel()

	st := style{color: true, truecolor: true, unicode: true, width: 80}
	got := banner(st)
	you, green, grey := st.code(roleYou), st.code(roleThem), st.code(roleDim)
	starts := strings.Count(got, you) + strings.Count(got, green) + strings.Count(got, grey)
	if starts == 0 {
		t.Fatal("color was on, but nothing was painted")
	}
	if resets := strings.Count(got, colorReset); resets != starts {
		t.Errorf("%d colors started, %d ended: a color is leaking past the banner", starts, resets)
	}

	// The same banner without color is the same text with the escapes
	// taken out, so a reader with color off is told exactly as much.
	plain := banner(style{unicode: true, width: 80})
	stripped := strings.NewReplacer(you, "", green, "", grey, "", colorReset, "").Replace(got)
	if stripped != plain {
		t.Errorf("color changed the text:\n%q\n%q", stripped, plain)
	}
}

func TestBannerFallsBackWhenTheTerminalIsNarrow(t *testing.T) {
	t.Parallel()

	got := banner(style{unicode: true, width: len(bannerIndent) + bannerCols - 1})
	if strings.Contains(got, "█") {
		t.Errorf("a terminal too narrow for the blocks got them: %q", got)
	}
	if !strings.Contains(got, bannerPlain) {
		t.Errorf("the narrow fallback does not say %q: %q", bannerPlain, got)
	}
}

// A test's output is a strings.Builder, which is not a terminal, so this is
// what a pipe or a redirected file gets: the plain line, and no escapes.
func TestWelcomeWritesNoEscapesToAPipe(t *testing.T) {
	t.Parallel()

	u, _, out := newTest(t)
	u.Welcome()

	if strings.Contains(out.String(), "\033") {
		t.Errorf(
			"Welcome wrote an escape sequence to something that is not a terminal: %q",
			out.String(),
		)
	}
	if !strings.Contains(out.String(), bannerPlain) {
		t.Errorf("Welcome did not write %q: %q", bannerPlain, out.String())
	}
}

func TestDisableColorTurnsColorOffAndNothingElse(t *testing.T) {
	t.Parallel()

	u, _, out := newTest(t)
	u.st = style{color: true, unicode: true, width: 80}
	u.DisableColor()
	u.Welcome()

	if strings.Contains(out.String(), "\033") {
		t.Errorf("-no-color left an escape in %q", out.String())
	}
	if !strings.Contains(out.String(), "█") {
		t.Errorf("-no-color took the blocks away too: %q", out.String())
	}
}
