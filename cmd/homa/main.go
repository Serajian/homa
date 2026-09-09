// Command homa is a peer-to-peer terminal chat: two machines exchange an
// address once and then talk directly, with no account and no server.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Serajian/homa/internal/ui"
)

func main() {
	if err := run(); err != nil {
		// Ctrl+C is how people stop a program, not a failure worth an
		// error message or a non-zero exit.
		if errors.Is(err, context.Canceled) {
			return
		}

		_, _ = fmt.Fprintf(os.Stderr, "homa: %v\n", err)
		os.Exit(1)
	}
}

// run holds the real work, so main is only about exit codes. Deferred
// cleanup does not run after os.Exit, and this is what keeps the two apart.
func run() error {
	opts := parseFlags()
	if opts.showVersion {
		fmt.Println("homa", version)
		return nil
	}

	if err := setupLogging(opts); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Refuse a pipe before anything is created for it.
	if err := ui.CheckTerminal(); err != nil {
		return err
	}

	deps, cleanup, err := bootstrap(ctx, opts.noColor)
	if err != nil {
		return err
	}
	defer cleanup()
	deps.Version = version

	// Run owns the terminal until the person quits. Ctrl+C reaches it as a
	// key, not as a signal, and comes back as context.Canceled like any
	// other quiet exit.
	if err := ui.Run(ctx, deps); err != nil {
		return err
	}

	fmt.Println("bye.")
	return nil
}
