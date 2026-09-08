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
	cfg, err := loadSettings()
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
	return ui.Deps{Cfg: cfg, Book: book, ID: id, Listener: listener, NoColor: noColor}, cleanup, nil
}

// loadSettings reads the saved settings. The first-run questions move to a
// screen of their own in a later step of the full-screen work; until then a
// machine with no settings is told so rather than asked on an interface
// that no longer exists.
func loadSettings() (*config.Config, error) {
	cfg, err := config.Load()
	if errors.Is(err, config.ErrNotFound) {
		return nil, errors.New("first run: the settings screen is not built yet; " +
			"run v0.1.0 once to answer the two questions, or wait for the next step")
	}
	return cfg, err
}
