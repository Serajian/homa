package ui

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"

	"github.com/Serajian/homa/internal/paths"
	"github.com/Serajian/homa/internal/session"
)

// chat runs one conversation until either side leaves.
//
// It owns conn: every path through this function closes it exactly once.
// The callers hand it over and do not close it themselves.
//
// Two goroutines share the terminal: this one reads what the person types,
// while the session's reads what the peer sends. They never both read the
// keyboard, which is why a file offer is answered by typing a command
// rather than by a prompt appearing mid-conversation.
func (a *App) chat(ctx context.Context, conn net.Conn, name string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	h := newChatHandler(a.ui, a.downloadDir, name)

	s, err := session.Start(conn, a.nick(), h)
	if err != nil {
		_ = conn.Close()
		a.ui.Warn("could not start the conversation: %v", err)
		return
	}

	var (
		ended   atomic.Bool // the peer went, or the connection broke
		leaving atomic.Bool // we are the ones going
	)

	// One close, chosen by who left first: a goodbye if we are the ones
	// leaving, a plain close if they already went.
	defer func() {
		if ended.Load() {
			_ = conn.Close()
			return
		}
		// Set before closing, so the reader goroutine does not report
		// our own departure as theirs. Run returns only after the close,
		// so the flag is always written before it is read.
		leaving.Store(true)
		_ = s.Close()
	}()

	a.ui.Blank()
	a.ui.Info("talking to %s (they call themselves %q)", name, s.Peer().Nick)
	a.ui.Info("/help for commands, /quit to leave")
	a.ui.Blank()

	go func() {
		err := s.Run(ctx)

		if leaving.Load() {
			ended.Store(true)
			return
		}

		if err != nil && ctx.Err() == nil {
			a.ui.Warn("the conversation ended: %v", trimSessionPrefix(err))
		} else {
			a.ui.Info("%s left the conversation.", name)
		}
		ended.Store(true)
		a.ui.Info("press Enter to go back to the menu")
	}()

	a.chatInput(ctx, s, h, &ended)
}

// chatInput reads what the person types until they leave or the peer does.
func (a *App) chatInput(
	ctx context.Context,
	s *session.Session,
	h *chatHandler,
	ended *atomic.Bool,
) {
	for {
		line, err := a.ui.ReadLine()
		if errors.Is(err, ErrCanceled) {
			return
		}
		if err != nil {
			a.ui.Warn("%v", err)
			return
		}

		// The peer may have left while this line was being typed. Check
		// after the read rather than before, so the notice is not shown
		// a whole line too late.
		if ended.Load() {
			return
		}

		if strings.HasPrefix(line, "/") {
			if quit := a.command(ctx, s, h, line); quit {
				return
			}
			continue
		}

		if err := s.SendText(line); err != nil {
			a.ui.Warn("could not send: %v", trimSessionPrefix(err))
			return
		}
	}
}

// command runs a slash command, reporting whether the person is leaving.
func (a *App) command(ctx context.Context, s *session.Session, h *chatHandler, line string) bool {
	cmd, arg, _ := strings.Cut(strings.TrimSpace(line), " ")
	arg = strings.TrimSpace(arg)

	switch cmd {
	case "/quit", "/q":
		return true

	case "/help", "/h":
		a.ui.Info("/send <path>  offer a file")
		a.ui.Info("/accept       take the file being offered")
		a.ui.Info("/reject       refuse it")
		a.ui.Info("/who          who you are talking to")
		a.ui.Info("/quit         leave the conversation")

	case "/who":
		a.ui.Info("%s, calling themselves %q", h.name, s.Peer().Nick)

	case "/accept":
		if !h.answerOffer(true) {
			a.ui.Warn("%v", errNoOffer)
		}

	case "/reject":
		if !h.answerOffer(false) {
			a.ui.Warn("%v", errNoOffer)
		}

	case "/send":
		a.sendFile(ctx, s, arg)

	default:
		a.ui.Warn("unknown command %q, try /help", cmd)
	}

	return false
}

// sendFile offers a file in the background, so the conversation carries on
// while it transfers.
func (a *App) sendFile(ctx context.Context, s *session.Session, path string) {
	if path == "" {
		a.ui.Warn("which file? /send <path>")
		return
	}

	full, err := paths.ExpandHome(path)
	if err != nil {
		a.ui.Warn("%v", err)
		return
	}

	a.ui.Info("offering %s, waiting for them to accept...", full)

	go func() {
		last := -1
		progress := func(sent, total int64) {
			step := percent(sent, total) / progressStep
			if step <= last {
				return
			}
			last = step
			a.ui.Info("sending: %d%%", step*progressStep)
		}

		if err := s.SendFile(ctx, full, progress); err != nil {
			a.ui.Warn("%v", trimSessionPrefix(err))
			return
		}
		a.ui.Info("sent.")
	}()
}
