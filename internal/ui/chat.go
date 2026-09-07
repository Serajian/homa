package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Serajian/homa/internal/paths"
	"github.com/Serajian/homa/internal/session"
)

// startChat greets a peer we dialed and then runs the conversation.
//
// Only the dialing side greets here. An incoming call was already greeted
// when it arrived, so that its caller did not sit waiting for a handshake
// that would not happen until somebody touched the keyboard.
func (a *App) startChat(ctx context.Context, conn net.Conn, name string) {
	h := newChatHandler(a.ui, a.downloadDir, name)

	s, err := session.Start(conn, a.nick(), h)
	if err != nil {
		_ = conn.Close()
		a.ui.Warn("could not start the conversation: %v", err)
		return
	}

	// A finished handshake means two programs are talking. It does not mean
	// a person agreed to, and saying "talking to ..." here is what used to
	// announce a conversation moments before it was refused.
	//
	// An older peer never signals, so there is nothing to wait through and
	// nothing to give up on: its handshake is all the agreement there is.
	if s.SignalsAcceptance() && !a.awaitAccept(ctx, s, name) {
		return
	}

	// The name is one this machine gave: a contact was picked from the
	// menu to get here.
	a.runChat(ctx, conn, s, h, name, true)
}

// awaitAccept waits for the other person to take the call, and lets this one
// give up on it. It reports whether the conversation may start; when it may
// not, it has already said why and closed the session.
//
// Waiting on the keyboard as well as on the connection is the whole point.
// Before this, the only key that did anything while a call rang was Ctrl+C,
// which closes homa: changing your mind about one call cost the program.
//
// The read runs in a goroutine because it cannot be selected on directly, and
// its channel is buffered so that giving up leaves nothing blocked on a send
// that nobody will receive.
func (a *App) awaitAccept(ctx context.Context, s *session.Session, name string) bool {
	deadline := time.Now().Add(callAnswerTimeout)

	stop := a.ui.countdown(deadline, func(left time.Duration) string {
		return fmt.Sprintf("%swaiting for %s to answer... %s  (Enter to give up)",
			markInfo, name, left)
	})

	waited := make(chan error, 1)
	go func() { waited <- s.WaitAccepted(ctx) }()

	var (
		err    error
		gaveUp bool
	)

	// Shutting down needs no case of its own: WaitAccepted watches the same
	// context and answers on the channel when it is canceled.
	select {
	case err = <-waited:
	case <-a.ui.Lines():
		// Any line, including an empty one. The person is being asked to
		// press something, not to spell anything.
		gaveUp = true
	}

	stop()

	// Left on screen rather than erased: how long it waited is worth seeing
	// next to whatever happened.
	a.ui.EndPrompt()

	if gaveUp {
		// Close rather than drop: the goodbye is a frame the far side
		// already knows how to read.
		_ = s.Close()
		a.ui.Info("you stopped calling %s.", name)
		return false
	}

	if err != nil {
		_ = s.Close()
		a.ui.Warn("%s", trimSessionPrefix(err))
		return false
	}

	return true
}

// runChat runs one conversation until either side leaves.
//
// It owns conn: every path through this function closes it exactly once.
// Callers hand it over and do not close it themselves.
//
// Two goroutines share the terminal: this one reads what the person types,
// while the session's reads what the peer sends. They never both read the
// keyboard, which is why a file offer is answered by typing a command
// rather than by a prompt appearing mid-conversation.
func (a *App) runChat(
	ctx context.Context,
	conn net.Conn,
	s *session.Session,
	h *chatHandler,
	name string,
	known bool,
) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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
	if known {
		a.ui.Info("talking to %s (they call themselves %q)", name, s.Peer().Nick)
	} else {
		// The label already is their nick, so repeating it would say
		// nothing. What is worth saying is where it came from.
		a.ui.Info("talking to %s, which is what they call themselves.", name)
		a.ui.Info("they are not in your contacts, so that name is theirs, not yours.")
	}
	a.ui.Info("/help for commands, /quit to leave")
	a.ui.Blank()

	go func() {
		err := s.Run(ctx)

		// Either flag means we are the ones going: leaving is set when
		// the person typed /quit, and a canceled context is Ctrl+C. Both
		// close the connection from this side, and neither is the peer
		// walking out.
		if leaving.Load() || ctx.Err() != nil {
			ended.Store(true)
			return
		}

		// The conversation is over, so take the input prompt down before
		// saying so. Otherwise the notice is followed by a "[me] " that
		// nothing will ever read a line into.
		a.ui.ErasePrompt()

		if err != nil {
			a.ui.Warn("the conversation ended: %v", trimSessionPrefix(err))
		} else {
			a.ui.Info("%s left the conversation.", name)
		}
		ended.Store(true)

		// Wake the goroutine reading the keyboard so the menu comes
		// back on its own. Before the input pump, this was a printed
		// "press Enter to go back to the menu", because nothing could
		// interrupt that read.
		cancel()
	}()

	a.chatInput(ctx, s, h, &ended)
}

// chatInput reads what the person types until they leave, the peer does, or
// the program is shut down. All three arrive the same way: ReadLine returns
// ErrCanceled, because the context this loop was given has been canceled.
func (a *App) chatInput(
	ctx context.Context,
	s *session.Session,
	h *chatHandler,
	ended *atomic.Bool,
) {
	// The prompt belongs to this loop and goes when it does, so the menu is
	// not printed with a "[me] " hanging off it.
	defer a.ui.ErasePrompt()

	for {
		// The label goes up before the read, not after the send, so what
		// is typed lands after it and the terminal's own echo is the
		// only copy on the screen.
		a.ui.Prompt("[%s] ", selfNick)

		line, err := a.ui.ReadLine(ctx)
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

		// A bare Enter is not a message. It used to be sent as an empty
		// one, which is how a keypress left over from the menu arrived
		// at the peer as nothing at all.
		if line == "" {
			continue
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

// showCommands lists what can be typed in a conversation.
//
// One Printf rather than a line each, so a message arriving cannot land in
// the middle of the list.
func (a *App) showCommands() {
	const commands = `  /files [dir]  list a directory, numbered
  /send <path>  offer a file
  /send <n>     offer one from the last listing
  /accept       take the file being offered
  /reject       refuse it
  /who          who you are talking to
  /clear        wipe the screen
  /quit         leave the conversation, not homa`

	a.ui.Printf("%s", commands)
}

// resolveSend turns what was typed after /send into a path.
//
// A number means a line from the last listing; anything else is a path. So a
// file actually named "2" cannot be sent as "/send 2" — "/send ./2" is how,
// and that is the price of not needing a flag to tell the two apart.
//
// It returns an empty path with no error when it has already said what was
// wrong, which is the case for a number that names nothing or names a
// directory.
func (a *App) resolveSend(h *chatHandler, arg string) (string, error) {
	n, err := strconv.Atoi(arg)
	if err != nil {
		return paths.ExpandHome(arg)
	}

	path, e, ok := h.listed(n)
	if !ok {
		a.ui.Warn("there is no %d in the last listing; /files to make one", n)
		return "", nil
	}

	if e.isDir {
		// The same answer the layer below gives for a path, said here
		// because a number reaches it before anything is opened.
		a.ui.Warn("%s is a directory; send an archive instead", e.name)
		return "", nil
	}

	return path, nil
}

// command runs a slash command, reporting whether the person is leaving.
func (a *App) command(ctx context.Context, s *session.Session, h *chatHandler, line string) bool {
	cmd, arg, _ := strings.Cut(strings.TrimSpace(line), " ")
	arg = strings.TrimSpace(arg)

	switch cmd {
	case "/quit", "/q":
		return true

	// A lone slash is somebody reaching for the list. It cannot be
	// answered as they type it — the terminal hands over a whole line and
	// not a keystroke, which is the same wall tab completion is behind —
	// so the next best thing is to answer it on Enter rather than tell
	// them it is not a command.
	case "/help", "/h", "/":
		a.showCommands()

	case "/clear":
		a.ui.Clear()

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

	case "/files", "/ls":
		a.showFiles(h, arg)

	case "/send":
		a.sendFile(ctx, s, h, arg)

	default:
		// Show them rather than send them somewhere: they have already
		// guessed once and being told to guess again is not help.
		a.ui.Warn("no such command: %q", cmd)
		a.showCommands()
	}

	return false
}

// sendFile offers a file in the background, so the conversation carries on
// while it transfers.
func (a *App) sendFile(ctx context.Context, s *session.Session, h *chatHandler, arg string) {
	if arg == "" {
		a.ui.Warn("which file? /send <path>, or /send <number> after /files")
		return
	}

	full, err := a.resolveSend(h, arg)
	if err != nil {
		a.ui.Warn("%v", trimUIPrefix(err))
		return
	}
	if full == "" {
		return // already explained
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
