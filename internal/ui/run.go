package ui

import (
	"context"
	"errors"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"

	"github.com/Serajian/homa/internal/config"
	"github.com/Serajian/homa/internal/contacts"
	"github.com/Serajian/homa/internal/peer"
)

// Deps is everything the interface needs from below, loaded by cmd/homa
// before the screen is taken over. The interface reads these through the
// interfaces they already expose and adds nothing to them.
type Deps struct {
	Cfg      *config.Config
	Book     *contacts.Book
	ID       *peer.Identity
	Listener *peer.Listener
	NoColor  bool // the -no-color flag; NO_COLOR and TERM=dumb are read by the program itself
}

// CheckTerminal is the terminal check for cmd/homa to run before anything
// else: before an identity is created or a listener opened, so a pipe is
// refused without touching the network or the disk.
func CheckTerminal() error { return needsTerminal(os.Stdin, os.Stdout) }

// Run takes over the terminal and returns when the person quits. A
// canceled ctx, or Ctrl+C, is reported as context.Canceled, which cmd/homa
// treats as a quiet exit. It assumes CheckTerminal has passed.
func Run(ctx context.Context, deps Deps) error {
	st := newStyles(styleFor(0, os.Getenv).unicode)

	opts := []tea.ProgramOption{tea.WithContext(ctx)}
	if deps.NoColor {
		opts = append(opts, tea.WithColorProfile(colorprofile.Ascii))
	}

	// The model needs a way to hand messages to the program from other
	// goroutines, and the program does not exist until the model does: the
	// closure fills in once both are made, before Run starts anything.
	var p *tea.Program
	m := newModel(ctx, deps, st)
	m.send = func(msg tea.Msg) { p.Send(msg) }
	p = tea.NewProgram(m, opts...)

	_, err := p.Run()
	if errors.Is(err, tea.ErrInterrupted) || errors.Is(err, tea.ErrProgramKilled) || ctx.Err() != nil {
		return context.Canceled
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
