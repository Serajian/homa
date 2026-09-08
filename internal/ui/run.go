// Package ui is homa's terminal front end: it reads what a person types and
// draws what they should see. Everything below it deals in values and
// errors; this is the only package that knows a human is involved.
//
// It is a bubbletea program: one model, every event a message, every
// screen drawn whole. The design is docs/design/2026-09-08-full-screen-interface.md.
package ui

import (
	"context"
	"errors"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"

	"github.com/Serajian/homa/internal/config"
	"github.com/Serajian/homa/internal/contacts"
	"github.com/Serajian/homa/internal/logx"
	"github.com/Serajian/homa/internal/peer"
)

var lg = logx.For("ui")

// unicodeLocale reports whether the locale says the terminal shows UTF-8,
// which block characters, rounded corners and the middle dot need. LC_ALL
// overrides LC_CTYPE, which overrides LANG, so the first of those that is
// set is the one that counts. Color needs no such check: bubbletea reads
// the terminal's own answer, and NO_COLOR, itself.
func unicodeLocale(getenv func(string) string) bool {
	for _, name := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := getenv(name); v != "" {
			v = strings.ToUpper(strings.ReplaceAll(v, "-", ""))
			return strings.Contains(v, "UTF8")
		}
	}
	return false
}

// Deps is everything the interface needs from below, loaded by cmd/homa
// before the screen is taken over. The interface reads these through the
// interfaces they already expose and adds nothing to them.
type Deps struct {
	Cfg      *config.Config
	Book     *contacts.Book
	ID       *peer.Identity
	Listener *peer.Listener
	NoColor  bool // the -no-color flag; NO_COLOR and TERM=dumb are read by the program itself

	// Reset deletes the identity, the address book and the settings,
	// attempting every one even if an earlier one fails, and returns what
	// could not be removed, by name. cmd/homa supplies the real one; tests
	// supply one that touches nothing.
	Reset func() (failed []string)
}

// CheckTerminal is the terminal check for cmd/homa to run before anything
// else: before an identity is created or a listener opened, so a pipe is
// refused without touching the network or the disk.
func CheckTerminal() error { return needsTerminal(os.Stdin, os.Stdout) }

// Run takes over the terminal and returns when the person quits, by q,
// Ctrl+C, or a signal from outside: all three are leaving, none is an
// error, and all three get the same goodbye. It assumes CheckTerminal has
// passed.
func Run(ctx context.Context, deps Deps) error {
	st := newStyles(unicodeLocale(os.Getenv))

	opts := []tea.ProgramOption{tea.WithContext(ctx)}
	if deps.NoColor {
		opts = append(opts, tea.WithColorProfile(colorprofile.Ascii))
	}

	// The model needs a way to hand messages to the program from other
	// goroutines, and the program does not exist until the model does: the
	// closure fills in once both are made, before Run starts anything.
	// See clearScreen for why this is not optional.
	_, _ = os.Stdout.WriteString(clearScreen)

	var p *tea.Program
	m := newModel(ctx, deps, st)
	m.send = func(msg tea.Msg) { p.Send(msg) }
	p = tea.NewProgram(m, opts...)

	_, err := p.Run()
	if errors.Is(err, tea.ErrInterrupted) || errors.Is(err, tea.ErrProgramKilled) ||
		ctx.Err() != nil {
		return nil //nolint:nilerr // leaving is not an error, whichever way it came
	}
	return err
}

// ErrNeedsTerminal is returned when homa is started with something other
// than a terminal on either end. A full-screen program has nowhere to draw
// in a pipe, and nobody chats through one. Version 1 printed plain text
// there; version 2 says so and stops, because keeping a line-mode interface
// alongside would mean every later feature twice. See
// docs/design/2026-09-08-full-screen-interface.md, decision 3.
var ErrNeedsTerminal = errors.New("needs a terminal")

// needsTerminal reports whether in and out are both terminals. Both are
// checked: a terminal on stdout with a file on stdin is a script feeding
// homa, and it would hang waiting for keys that never come.
func needsTerminal(in, out *os.File) error {
	if !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return ErrNeedsTerminal
	}
	return nil
}
