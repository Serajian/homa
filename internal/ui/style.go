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
	color   bool // color escapes will be understood, and are wanted
	unicode bool // block characters will come out as blocks, not as "?"
	width   int  // columns, or 0 when nobody knows
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

	return styleFor(width, os.Getenv)
}

// styleFor is the decision for something that is a terminal, kept apart from
// detect so the rules can be tested without one.
//
// NO_COLOR is honored as https://no-color.org says: set to anything but the
// empty string, it turns color off. TERM=dumb is the other way a terminal
// says it would rather not.
//
// Block characters need a UTF-8 locale. LC_ALL overrides LC_CTYPE, which
// overrides LANG, so the first of those that is set is the one that counts.
func styleFor(width int, getenv func(string) string) style {
	st := style{width: width}

	st.color = getenv("NO_COLOR") == "" && getenv("TERM") != "dumb"

	for _, name := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := getenv(name); v != "" {
			v = strings.ToUpper(strings.ReplaceAll(v, "-", ""))
			st.unicode = strings.Contains(v, "UTF8")
			break
		}
	}

	return st
}

// paint wraps s in a color when style allows it, and leaves it alone when
// it does not. s is always text homa wrote itself; see the note on the
// color constants.
func paint(st style, color, s string) string {
	if !st.color || s == "" {
		return s
	}
	return color + s + colorReset
}
