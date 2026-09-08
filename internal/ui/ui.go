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

	// prompt is what is on the screen with the cursor sitting at the end of
	// it, or empty when the cursor is at the start of a line. Printing has
	// to work around it: without this a message arriving mid-conversation
	// would read "[me] [bob] salam", and afterwards the label would be gone
	// from in front of what is being typed. Guarded by mu, because a
	// session's read goroutine prints while this one waits for input.
	prompt string

	// st is what the output can show, decided once in New. Guarded by mu
	// only because DisableColor writes it; after that it is read-only.
	st style
}

// New returns a UI reading from in and writing to out, and starts the
// goroutine that does the reading.
func New(in io.Reader, out io.Writer) *UI {
	u := &UI{
		out:   out,
		lines: make(chan string),
		st:    detect(out),
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
			u.lines <- strings.TrimSpace(stripKeys(line))
		}

		if err != nil {
			if !errors.Is(err, io.EOF) {
				lg.Warn("reading input stopped", "err", err)
			}
			return
		}
	}
}

// stripKeys removes what a key that does nothing leaves behind.
//
// A terminal handing over whole lines does not act on an arrow key: it drops
// the escape sequence into the line like any other typing, so pressing Up
// twice and then typing puts "\x1b[A\x1b[A" in front of the message. The
// receiving side strips the escape byte, because a terminal obeys what it is
// given, but the letters after it are ordinary text and survive: somebody
// reads "[A[A" and wonders what was meant.
//
// So the whole sequence goes, here, before anybody sees the line. It is done
// in the pump rather than in the conversation because the menu has the same
// problem, and because a line nobody typed on purpose should not exist as far
// as the rest of this package is concerned.
//
// A line that was only arrow keys comes out empty, and every caller already
// knows what an empty line means.
func stripKeys(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		if s[i] != esc {
			// Anything else in the control range is dropped too, with
			// tab spared because it is something a person can mean.
			if r := rune(s[i]); r < ' ' && r != '\t' {
				i++
				continue
			}
			b.WriteByte(s[i])
			i++
			continue
		}

		i++ // the escape itself
		if i >= len(s) {
			break
		}

		switch s[i] {
		case '[':
			// A CSI sequence: parameters, then one byte that ends it.
			// This is what every arrow key sends.
			i++
			for i < len(s) && (s[i] < '@' || s[i] > '~') {
				i++
			}
			if i < len(s) {
				i++
			}
		case ']':
			// An OSC sequence, ended by a bell or by an escape. No
			// keyboard sends one, and a paste can.
			i++
			for i < len(s) && s[i] != bel && s[i] != esc {
				i++
			}
			if i < len(s) {
				i++
			}
		default:
			i++ // a two-byte escape
		}
	}

	return b.String()
}

// Lines is the stream of typed lines, closed when the input ends.
//
// It is here for callers that have to wait on the keyboard and on something
// else at the same time, which a function call cannot do: the menu, which
// takes a call the moment it arrives rather than at the next keypress, and a
// caller waiting to be let in, who can give up on it. Everything else wants
// ReadLine.
//
// Two goroutines reading this would each get some of the lines, and neither
// would get all of them. That is not prevented here; it is avoided by the two
// callers never waiting at the same time, because homa is either at the menu
// or placing a call and never both. A third caller, or a menu that stays live
// during a call, would break that and would have to be given something else.
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

		// Pressing Enter ended the line the terminal was echoing, so
		// the cursor has already moved on and that prompt is spent. A
		// canceled context is the other case: nothing was submitted,
		// the prompt is still on the screen, and it stays recorded.
		u.mu.Lock()
		u.prompt = ""
		u.mu.Unlock()

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

	// Take the prompt off the screen, print, then put it back underneath,
	// so the label stays in front of whatever is being typed and no empty
	// "[me] " is left stranded above every arriving message.
	//
	// What was already typed is not redrawn. Those characters are still in
	// the terminal's own line buffer and will be sent, but they are no
	// longer on the screen. Nothing here can read them back; a full-screen
	// interface owns the input line and is what fixes it.
	if u.prompt != "" {
		_, _ = fmt.Fprint(u.out, clearLine) //nolint:errcheck // there is nowhere to report this
	}

	// Nothing useful can be done if the terminal cannot be written to,
	// and reporting it would need the same terminal.
	_, _ = fmt.Fprint(u.out, s) //nolint:errcheck // there is nowhere to report this

	if u.prompt != "" {
		_, _ = fmt.Fprint(u.out, u.prompt) //nolint:errcheck // there is nowhere to report this
	}
}

// Prompt writes without ending the line, so what the person types appears
// after it rather than underneath it.
//
// Printf and everything built on it know about this: the next line printed
// from anywhere closes the prompt first, so output never lands inside it.
func (u *UI) Prompt(format string, args ...any) {
	p := fmt.Sprintf(format, args...)

	u.mu.Lock()
	defer u.mu.Unlock()

	// A prompt replaces a prompt. That is what lets one redraw itself once
	// a second with a countdown in it, and it costs nothing when there was
	// none there to begin with.
	if u.prompt != "" {
		_, _ = fmt.Fprint(u.out, clearLine) //nolint:errcheck // there is nowhere to report this
	}

	_, _ = fmt.Fprint(u.out, p) //nolint:errcheck // there is nowhere to report this
	u.prompt = p
}

// ErasePrompt takes the prompt off the screen, as though it had never been put
// there. Whoever put one up with Prompt takes it down, so the menu is not
// printed with a chat prompt trailing it.
func (u *UI) ErasePrompt() {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.prompt == "" {
		return
	}

	_, _ = fmt.Fprint(u.out, clearLine) //nolint:errcheck // there is nowhere to report this
	u.prompt = ""
}

// EndPrompt leaves the prompt where it is and moves past it. It is for a
// question answered by time rather than by a person: the reader should still
// be able to see what was on the screen when it ran out.
func (u *UI) EndPrompt() {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.prompt == "" {
		return
	}

	_, _ = fmt.Fprint(u.out, "\n") //nolint:errcheck // there is nowhere to report this
	u.prompt = ""
}

// Clear wipes the screen and puts the cursor back at the top.
//
// It writes the escape itself rather than going through Printf, which would
// add a newline and undo the cursor being sent home, and would erase a prompt
// that is about to be wiped anyway.
//
// A prompt on the screen is drawn again afterwards. Otherwise this package
// would go on believing a line is there that a person can no longer see.
func (u *UI) Clear() {
	u.mu.Lock()
	defer u.mu.Unlock()

	_, _ = fmt.Fprint(u.out, clearScreen) //nolint:errcheck // there is nowhere to report this

	if u.prompt != "" {
		_, _ = fmt.Fprint(u.out, u.prompt) //nolint:errcheck // there is nowhere to report this
	}
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
