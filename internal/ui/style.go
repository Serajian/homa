package ui

import (
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// style is what the output can show: color, block characters, and how many
// columns there are. It is decided once, from the stream homa writes to,
// because the answer does not change while the program runs and because a
// pipe has no answer to give.
//
// Color is never the only signal; the marks in const.go are what a reader
// goes by. style decides whether color may reinforce them, and whether the
// welcome banner can be drawn in blocks or has to be one plain line.
type style struct {
	tty       bool // output is a terminal: a prompt can be erased and redrawn
	color     bool // color escapes will be understood, and are wanted
	truecolor bool // 24-bit color will be shown as such; otherwise the 16 ANSI colors
	unicode   bool // block characters will come out as blocks, not as "?"
	width     int  // columns, or 0 when nobody knows
}

// role is what a color means. Painting goes through a role rather than an
// escape, so the same meaning comes out in whichever palette the terminal
// can show.
type role int

const (
	roleYou  role = iota // the prompt, your label, a key to press
	roleThem             // a name, their message label, a file arriving
	roleDim              // homa's own voice: information, hints, metadata
	roleWarn             // a warning
)

// code is the escape for a role in this style's palette.
func (st style) code(r role) string {
	switch r {
	case roleThem:
		if st.truecolor {
			return color24Green
		}
		return color16Green
	case roleDim:
		if st.truecolor {
			return color24Muted
		}
		return color16Muted
	case roleWarn:
		return colorWarn
	default:
		return colorYou
	}
}

// detect looks at where output is going. Anything that is not a terminal —
// a pipe, a file, a test — gets plain text and no escape sequences, whatever
// the environment says.
func detect(out io.Writer) style {
	f, ok := out.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return style{}
	}

	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil {
		width = 0
	}

	st := styleFor(width, os.Getenv)
	st.tty = true
	return st
}

// styleFor is the decision for something that is a terminal, kept apart from
// detect so the rules can be tested without one.
//
// NO_COLOR is honored as https://no-color.org says: set to anything but the
// empty string, it turns color off. TERM=dumb is the other way a terminal
// says it would rather not.
//
// 24-bit color is used only when the terminal says so: COLORTERM set to
// truecolor or 24bit, or a TERM ending in -direct. Everything else gets the
// sixteen ANSI colors, which every terminal shows; a 24-bit escape on a
// terminal without it is at best approximated and at worst ignored, and a
// prompt in the default color is worse than one in plain green.
//
// Block characters need a UTF-8 locale. LC_ALL overrides LC_CTYPE, which
// overrides LANG, so the first of those that is set is the one that counts.
func styleFor(width int, getenv func(string) string) style {
	st := style{width: width}

	st.color = getenv("NO_COLOR") == "" && getenv("TERM") != "dumb"

	switch ct := getenv("COLORTERM"); {
	case ct == "truecolor", ct == "24bit", strings.HasSuffix(getenv("TERM"), "-direct"):
		st.truecolor = st.color
	}

	for _, name := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := getenv(name); v != "" {
			v = strings.ToUpper(strings.ReplaceAll(v, "-", ""))
			st.unicode = strings.Contains(v, "UTF8")
			break
		}
	}

	return st
}

// paint wraps s in the color a role has in this palette, when style allows
// color at all, and leaves it alone when it does not. s is always text homa
// wrote itself; see the note on the color constants.
//
// s may itself contain painted text, which ends with a reset that would end
// this color too; the color is put back after each one, so a grey line with
// a green name in it stays grey to the end.
func paint(st style, r role, s string) string {
	if !st.color || s == "" {
		return s
	}
	color := st.code(r)
	return color + strings.ReplaceAll(s, colorReset, colorReset+color) + colorReset
}
