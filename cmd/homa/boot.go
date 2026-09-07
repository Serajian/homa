package main

import (
	"context"
	"errors"

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
func bootstrap(ctx context.Context, out *ui.UI) (app *ui.App, cleanup func(), err error) {
	cfg, err := loadSettings(out)
	if err != nil {
		return nil, nil, err
	}

	book, err := contacts.Load()
	if err != nil {
		return nil, nil, err
	}

	// Creating an identity measures relay latency, so a first run takes a
	// moment. Say so rather than looking frozen.
	out.Info("starting up...")

	id, err := peer.LoadOrCreateIdentity(ctx)
	if err != nil {
		return nil, nil, err
	}

	listener, err := peer.Listen(id)
	if err != nil {
		return nil, nil, err
	}

	cleanup = func() { _ = listener.Close() }
	return ui.NewApp(out, cfg, book, id, listener), cleanup, nil
}

// loadSettings reads the saved settings, asking the first-run questions if
// there are none yet.
func loadSettings(out *ui.UI) (*config.Config, error) {
	cfg, err := config.Load()
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, config.ErrNotFound) {
		return nil, err
	}

	cfg, err = ui.Setup(out)
	if err != nil {
		if errors.Is(err, ui.ErrCanceled) {
			return nil, errors.New("setup was not finished")
		}
		return nil, err
	}
	return cfg, nil
}
