package ui

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// chatHandler is what a session reports to. It turns events into lines on
// the screen, and asks the person about incoming files.
//
// Its methods run on the session's read goroutine while the person types on
// another, so everything shared is guarded.
type chatHandler struct {
	ui   *UI
	name string // what we call the peer

	// downloadDir is asked at the moment a file is accepted rather than
	// captured up front, so a setting changed mid-conversation takes
	// effect. It is supplied by the caller, who owns any locking.
	downloadDir func() (string, error)

	mu       sync.Mutex
	offer    *pendingOffer
	lastStep map[string]int

	// files is the last directory shown with /files. It is under the same
	// lock as the rest, which costs nothing and saves having to reason
	// about which goroutine reaches it.
	files *listing
}

// setListing records what /files just showed.
func (h *chatHandler) setListing(l *listing) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.files = l
}

// listedDir is the directory the last listing came from, or empty if there
// has not been one.
func (h *chatHandler) listedDir() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.files == nil {
		return ""
	}
	return h.files.dir
}

// listed returns the nth entry of the last listing, or false if there was no
// listing or no such number.
func (h *chatHandler) listed(n int) (string, entry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.files.path(n)
}

// pendingOffer is a file offer waiting for an answer.
//
// The read goroutine cannot ask the question itself: the person's keyboard
// is being read by another goroutine, and two readers on one terminal fight
// over every keystroke. So the offer is announced, parked here, and the
// input loop delivers the answer when /accept or /reject is typed.
type pendingOffer struct {
	name  string
	reply chan bool
}

func newChatHandler(u *UI, downloadDir func() (string, error), name string) *chatHandler {
	return &chatHandler{
		ui:          u,
		name:        name,
		downloadDir: downloadDir,
		lastStep:    make(map[string]int),
	}
}

// OnText shows a message from the peer.
func (h *chatHandler) OnText(text string) {
	h.ui.Message(h.name, text)
}

// OnFileOffer announces the offer and waits for the person to answer it in
// the chat. It blocks the read goroutine on purpose: the sender is waiting
// anyway, and nothing else from them should slip in front of the question.
func (h *chatHandler) OnFileOffer(
	name string,
	size int64,
) (dir string, accept bool, reason string) {
	reply := make(chan bool, 1)

	h.mu.Lock()
	if h.offer != nil {
		h.mu.Unlock()
		return "", false, "another file is already waiting to be answered"
	}
	h.offer = &pendingOffer{name: name, reply: reply}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		h.offer = nil
		h.mu.Unlock()
	}()

	h.ui.Blank()
	h.ui.Info("%s wants to send %q (%s)", h.name, name, humanBytes(size))
	h.ui.Info("y to accept, n to reject")

	select {
	case ok := <-reply:
		if !ok {
			return "", false, "declined"
		}
	case <-time.After(offerAnswerTimeout):
		h.ui.Warn("the offer of %q timed out", name)
		return "", false, "no answer"
	}

	target, err := h.downloadDir()
	if err != nil {
		h.ui.Warn("%v", err)
		return "", false, "the receiver has nowhere to put it"
	}
	return target, true, ""
}

// answerOffer delivers the person's decision. It reports whether there was
// anything waiting for one.
// answerShorthand takes a bare yes or no as the answer to a file offer,
// reporting whether it was one and there was an offer waiting.
//
// A file offer used to be answerable only by /accept, because it arrives on
// the session's goroutine while another one is reading the keyboard, and the
// two could not both prompt. The input pump ended that: there is one reader
// now, and this is it deciding that the line in its hand is an answer rather
// than a message.
//
// It is only ever consulted while an offer is waiting, so a message that is
// nothing but "y" is lost only in the moment somebody is being asked a yes or
// no question. That is the cost, and it is why nothing shorter than a whole
// word counts as anything else.
func (h *chatHandler) answerShorthand(line string) bool {
	h.mu.Lock()
	waiting := h.offer != nil
	h.mu.Unlock()

	if !waiting {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return h.answerOffer(true)
	case "n", "no":
		return h.answerOffer(false)
	}
	return false
}

func (h *chatHandler) answerOffer(accept bool) bool {
	h.mu.Lock()
	offer := h.offer
	h.mu.Unlock()

	if offer == nil {
		return false
	}

	select {
	case offer.reply <- accept:
	default: // already answered
	}
	return true
}

// OnFileProgress reports an incoming transfer, in steps rather than on
// every chunk: a 700 MB file is twenty thousand chunks, and a line each
// would bury the conversation.
func (h *chatHandler) OnFileProgress(name string, received, total int64) {
	step := percent(received, total) / progressStep

	h.mu.Lock()
	last, seen := h.lastStep[name]
	if seen && step <= last {
		h.mu.Unlock()
		return
	}
	h.lastStep[name] = step
	h.mu.Unlock()

	h.ui.Info("receiving %q: %d%%", name, step*progressStep)
}

// OnFileDone reports a file that arrived and passed its checksum.
func (h *chatHandler) OnFileDone(name, path string) {
	h.mu.Lock()
	delete(h.lastStep, name)
	h.mu.Unlock()

	h.ui.Info("saved %q to %s", name, path)
}

// OnFileError reports a transfer that failed.
func (h *chatHandler) OnFileError(name string, err error) {
	h.mu.Lock()
	delete(h.lastStep, name)
	h.mu.Unlock()

	h.ui.Warn("%q: %v", name, trimSessionPrefix(err))
}

// trimSessionPrefix drops the package prefix before showing an error.
func trimSessionPrefix(err error) string {
	if err == nil {
		return ""
	}

	const prefix = "session: "
	s := err.Error()
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

// errNoOffer is returned when /accept or /reject is typed with nothing
// waiting to be answered.
var errNoOffer = errors.New("no file is waiting for an answer")
