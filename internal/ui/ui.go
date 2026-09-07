// Package ui is homa's terminal front end: it reads what a person types and
// writes what they should see. Everything below it deals in values and
// errors; this is the only package that knows a human is involved.
package ui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/Serajian/homa/internal/logx"
)

var lg = logx.For("ui")

// ErrCanceled means the person is done answering: they ended the input with
// Ctrl+D, or the program is shutting down. It is not a failure.
var ErrCanceled = errors.New("ui: canceled")

// UI reads from one stream and writes to another. It is safe to write to
// from several goroutines: incoming chat arrives on the session's read
// goroutine while the person is typing on another.
//
// Reading is done by one goroutine of its own, started by New, which hands
// finished lines over on a channel. Nothing else touches the input stream.
type UI struct {
	out io.Writer

	// lines carries one typed line at a time and is closed when the input
	// ends. It is unbuffered on purpose: a line waits in the pump until
	// somebody asks for it, which is what makes a keystroke typed at the
	// menu still count once a prompt appears.
	lines chan string

	mu sync.Mutex // serializes writes so two lines never interleave
}

// New returns a UI reading from in and writing to out, and starts the
// goroutine that does the reading.
func New(in io.Reader, out io.Writer) *UI {
	u := &UI{
		out:   out,
		lines: make(chan string),
	}

	go u.pump(bufio.NewReader(in))

	return u
}

// pump reads lines for the life of the program and hands them over one at a
// time.
//
// This goroutine is deliberately never stopped. A read on a terminal blocks
// inside a system call that nothing in Go can interrupt: not a context, not
// a signal, and not closing the descriptor, which on macOS is not
// guaranteed to make a read already in progress return. So the read is left
// blocked on input the process is about to abandon, at a cost of one
// goroutine. Do not "fix" this by closing standard input; that was the
// previous design and it is why Ctrl+C used to hang.
func (u *UI) pump(in *bufio.Reader) {
	defer close(u.lines)

	for {
		line, err := in.ReadString('\n')

		// A final line without a newline still counts: it is what
		// arrives when input is piped from a file that does not end in
		// one. So deliver what was read before looking at the error.
		if line != "" {
			if len(line) > maxInputLen {
				line = line[:maxInputLen]
			}
			u.lines <- strings.TrimSpace(line)
		}

		if err != nil {
			if !errors.Is(err, io.EOF) {
				lg.Warn("reading input stopped", "err", err)
			}
			return
		}
	}
}

// Lines is the stream of typed lines, closed when the input ends.
//
// It is here for the one caller that has to wait on the keyboard and on
// something else at the same time: the menu, which answers a call the
// moment it arrives rather than at the next keypress. Everything else wants
// ReadLine.
func (u *UI) Lines() <-chan string { return u.lines }

// ReadLine waits for one line the person typed, with the surrounding
// whitespace removed.
//
// It returns ErrCanceled when the input ends or when ctx is canceled. Both
// mean the same thing to every caller: stop asking. Unlike the blocking
// read this replaces, it returns the moment ctx is canceled, whatever the
// keyboard is doing.
func (u *UI) ReadLine(ctx context.Context) (string, error) {
	select {
	case line, ok := <-u.lines:
		if !ok {
			return "", ErrCanceled
		}
		return line, nil

	case <-ctx.Done():
		return "", ErrCanceled
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
