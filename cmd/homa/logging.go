package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/Serajian/homa/internal/logx"
)

// setupLogging points homa's diagnostics somewhere, or nowhere.
//
// Nowhere is the default on purpose: this program draws a terminal
// interface, and a log line landing in the middle of a conversation would
// scramble it. A file is the useful choice; stderr is there for when
// something fails before the interface even starts.
func setupLogging(opts options) error {
	level := slog.LevelInfo
	if opts.debug {
		level = slog.LevelDebug
	}

	if opts.logFile != "" {
		f, err := os.OpenFile(opts.logFile,
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("opening the log file: %w", err)
		}
		// Deliberately not closed: it must stay open for the life of the
		// program, and the operating system closes it at exit.
		logx.ToWriter(f, level)
		return nil
	}

	if opts.debug {
		logx.ToWriter(os.Stderr, level)
	}
	return nil
}
