package main

import (
	"context"
	"os"

	"github.com/Serajian/homa/internal/ui"
)

// watchForShutdown closes standard input when the program is interrupted.
//
// Ctrl+C cancels the context, but homa is usually sitting in a blocking
// read on the keyboard, and no context can wake that: the read is a system
// call the runtime cannot interrupt. Closing the input makes it return, and
// every loop in homa already treats the end of input as "we are done".
//
// The returned channel must be closed when the program is finishing
// normally, so this goroutine does not announce a shutdown that is already
// over.
func watchForShutdown(ctx context.Context, out *ui.UI) chan struct{} {
	finished := make(chan struct{})

	go func() {
		select {
		case <-ctx.Done():
			out.Blank()
			out.Info("shutting down...")

			// From here every ReadLine returns ErrCanceled, which
			// unwinds the chat and then the menu.
			_ = os.Stdin.Close()

		case <-finished:
		}
	}()

	return finished
}
