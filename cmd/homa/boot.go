package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/Serajian/homa/internal/config"
	"github.com/Serajian/homa/internal/contacts"
	"github.com/Serajian/homa/internal/peer"
	"github.com/Serajian/homa/internal/ui"
)

// bootstrap loads everything homa needs and returns a ready app along with
// the cleanup its resources need.
//
// The order matters: settings first, because the first run asks questions
// and nothing else should have started when it does; then the address book;
// then the identity, which may reach the network; then the listener.
func bootstrap(ctx context.Context, noColor bool) (deps ui.Deps, cleanup func(), err error) {
	cfg, err := loadSettings(ctx, noColor)
	if err != nil {
		return ui.Deps{}, nil, err
	}

	book, err := contacts.Load()
	if err != nil {
		return ui.Deps{}, nil, err
	}

	// Creating an identity measures relay latency, so a first run takes a
	// moment. Say what the wait is for, and that it is once; a later run
	// only reads a file and needs no more than a word. This is printed
	// before the screen is taken over, so it scrolls away under the frame.
	if peer.HasIdentity() {
		fmt.Println("  starting up...")
	} else {
		fmt.Println("  first run: measuring the relays to pick the nearest, once. a few seconds.")
	}

	id, err := peer.LoadOrCreateIdentity(ctx)
	if err != nil {
		return ui.Deps{}, nil, err
	}

	listener, err := peer.Listen(id)
	if err != nil {
		return ui.Deps{}, nil, err
	}

	cleanup = func() { _ = listener.Close() }
	return ui.Deps{Cfg: cfg, Book: book, ID: id, Listener: listener, NoColor: noColor, Reset: resetAll}, cleanup, nil
}

// loadSettings reads the saved settings, asking the first-run questions on
// a screen of their own if there are none yet.
func loadSettings(ctx context.Context, noColor bool) (*config.Config, error) {
	cfg, err := config.Load()
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, config.ErrNotFound) {
		return nil, err
	}
	return ui.RunSetup(ctx, noColor)
}

// resetAll deletes the identity, the address book and the settings. Every
// one is attempted even if an earlier one fails, so a reset that goes wrong
// halfway leaves as little behind as it can; what could not be removed is
// returned by name.
func resetAll() (failed []string) {
	for _, f := range []struct {
		what   string
		remove func() error
	}{
		{"your identity", peer.RemoveIdentity},
		{"your address book", contacts.Remove},
		{"your settings", config.Remove},
	} {
		if err := f.remove(); err != nil {
			failed = append(failed, f.what)
		}
	}
	return failed
}
