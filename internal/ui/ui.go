// Package ui is homa's terminal front end: it reads what a person types and
// writes what they should see. Everything below it deals in values and
// errors; this is the only package that knows a human is involved.
package ui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/Serajian/homa/internal/logx"
)

var lg = logx.For("ui")

// ErrCanceled means the person ended the input, usually with Ctrl+D. It is
// not a failure: it is how someone says "stop asking".
var ErrCanceled = errors.New("ui: canceled")

// UI reads from one stream and writes to another. It is safe to write to
// from several goroutines: incoming chat arrives on the session's read
// goroutine while the person is typing on another.
//
// It is not safe to read from more than one goroutine, and nothing in homa
// tries to.
type UI struct {
	in  *bufio.Reader
	out io.Writer

	mu sync.Mutex // serializes writes so two lines never interleave
}

// New returns a UI reading from in and writing to out.
func New(in io.Reader, out io.Writer) *UI {
	return &UI{
		in:  bufio.NewReader(in),
		out: out,
	}
}

// Printf writes a line. A newline is added if the format does not end in
// one, because a line that does not end is a line the next writer ruins.
func (u *UI) Printf(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	// Nothing useful can be done if the terminal cannot be written to,
	// and reporting it would need the same terminal.
	_, _ = fmt.Fprint(u.out, s) //nolint:errcheck // there is nowhere to report this
}

// Info states something that happened.
func (u *UI) Info(format string, args ...any) {
	u.Printf(markInfo+format, args...)
}

// Warn states something that went wrong but did not stop anything.
func (u *UI) Warn(format string, args ...any) {
	u.Printf(markWarn+format, args...)
}

// Blank writes an empty line, for spacing between sections.
func (u *UI) Blank() { u.Printf("") }

// Message shows a chat message from the peer.
//
// The nick is drawn in brackets rather than followed by a colon, so a
// message whose own text contains a colon cannot be mistaken for a second
// speaker.
func (u *UI) Message(nick, text string) {
	u.Printf("[%s] %s", nick, text)
}

// ReadLine reads one line the person typed, with the surrounding whitespace
// removed. It returns ErrCanceled when the input ends.
func (u *UI) ReadLine() (string, error) {
	line, err := u.in.ReadString('\n')

	// A final line without a newline still counts: it is what arrives when
	// input is piped from a file that does not end in one.
	if errors.Is(err, io.EOF) && line == "" {
		return "", ErrCanceled
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("ui: reading input: %w", err)
	}

	if len(line) > maxInputLen {
		line = line[:maxInputLen]
	}
	return strings.TrimSpace(line), nil
}
