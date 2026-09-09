package ui

import (
	"strings"
	"testing"
)

func commandNames(cs []command) string {
	var out []string
	for _, c := range cs {
		out = append(out, c.name)
	}
	return strings.Join(out, " ")
}

func TestASlashOffersEveryCommandAndLettersNarrowIt(t *testing.T) {
	t.Parallel()

	if got := commandNames(matches("/")); got != "/help /files /send /accept /reject /who /me /add /clear /quit" {
		t.Errorf("/: %s", got)
	}
	if got := commandNames(matches("/s")); got != "/send" {
		t.Errorf("/s: %s", got)
	}
	if got := commandNames(matches("/ls")); got != "/files" {
		t.Errorf("/ls: %s", got)
	}
	if got := commandNames(matches("/l")); got != "" {
		t.Errorf("an alias was offered: %s", got)
	}
	if got := commandNames(matches("/x")); got != "" {
		t.Errorf("/x: %s", got)
	}
}

func TestCompleteTakesTheOneLeftOrThePick(t *testing.T) {
	t.Parallel()

	cases := []struct {
		typed string
		pick  int
		want  string
	}{
		{"/s", 0, "/send "},
		{"/qu", 0, "/quit"},
		{"/", 0, "/help"},
		{"/", 1, "/files "},
		{"/", 99, "/quit"},
		{"/x", 0, "/x"},
		{"/send fo", 0, "/send fo"},
		{"hello", 0, "hello"},
	}
	for _, c := range cases {
		if got := complete(c.typed, c.pick); got != c.want {
			t.Errorf("complete(%q, %d) = %q, want %q", c.typed, c.pick, got, c.want)
		}
	}
}

func TestTheHintSaysWhatCanFollow(t *testing.T) {
	t.Parallel()

	st := plainStyles(true)
	cases := []struct {
		typed string
		pick  int
		want  string
	}{
		{"hello", 0, ""},
		{"", 0, ""},
		{
			"/",
			0,
			"▸ /help  ·  /files [dir]  ·  /send <path>  ·  /accept  ·  /reject  ·  /who  ·  /me  ·  /add [name]  ·  /clear  ·  /quit",
		},
		{
			"/",
			2,
			"/help  ·  /files [dir]  ·  ▸ /send <path>  ·  /accept  ·  /reject  ·  /who  ·  /me  ·  /add [name]  ·  /clear  ·  /quit",
		},
		{"/s", 0, "/send <path>   offer a file  ·  Tab completes"},
		{"/send", 0, "/send <path>   offer a file"},
		{"/send ~/x", 0, "/send <path>   offer a file"},
		{"/ls .", 0, "/files [dir]   list a directory, numbered"},
		{"/x", 0, `no such command: "/x"`},
		{"/x y", 0, `no such command: "/x"`},
	}
	for _, c := range cases {
		if got := hint(st, c.typed, c.pick, 200); got != c.want {
			t.Errorf("hint(%q, %d):\n got %q\nwant %q", c.typed, c.pick, got, c.want)
		}
	}
}

func TestAPickPastTheEdgeScrollsTheRow(t *testing.T) {
	t.Parallel()

	st := plainStyles(false)
	got := hint(st, "/", 9, 40)
	if !strings.HasPrefix(got, "...") || !strings.Contains(got, "> /quit") {
		t.Errorf("the pick is out of view: %q", got)
	}
	if strings.Contains(got, "/help") {
		t.Errorf("the front was kept: %q", got)
	}
	if got := hint(st, "/", 0, 40); strings.HasPrefix(got, "...") {
		t.Errorf("nothing was dropped and yet: %q", got)
	}
}

func TestTheHintReadsTheSameWithoutColor(t *testing.T) {
	t.Parallel()

	for _, typed := range []string{"/", "/s", "/send x", "/x"} {
		colored := stripANSI(hint(newStyles(true), typed, 1, 60))
		plain := hint(plainStyles(true), typed, 1, 60)
		if colored != plain {
			t.Errorf("%q: colored %q, plain %q", typed, colored, plain)
		}
	}
}

func TestHelpLinesReadFromTheTable(t *testing.T) {
	t.Parallel()

	lines := helpLines()
	if lines[0] != "/help         this list, or just /" {
		t.Errorf("first line: %q", lines[0])
	}
	if lines[1] != "/files [dir]  list a directory, numbered" {
		t.Errorf("second line: %q", lines[1])
	}
	if len(lines) != 14 {
		t.Errorf("%d lines", len(lines))
	}
}

// A row wider than the room ends in the same mark the front uses, rather
// than stopping mid-separator where the frame would cut it.
func TestARowWiderThanTheRoomSaysSoAtTheEnd(t *testing.T) {
	t.Parallel()

	st := plainStyles(true)
	got := hint(st, "/", 0, 40)
	if len([]rune(got)) > 40 {
		t.Errorf("%d wide in 40: %q", len([]rune(got)), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("no mark at the end: %q", got)
	}
	if strings.HasPrefix(got, "…") {
		t.Errorf("the front was dropped for a pick that is in view: %q", got)
	}
	if wide := hint(st, "/", 0, 400); strings.HasSuffix(wide, "…") {
		t.Errorf("a row with room to spare was marked: %q", wide)
	}
}
