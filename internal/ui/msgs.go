package ui

import (
	"time"
)

// The messages that reach the model from outside a keypress: from below
// (calls, the peer), from timers, and from homa itself. Every one of them
// is a plain value; the goroutine that produced it hands it to Program.Send
// and touches nothing else, which is what lets the model run without a lock.

// noticeMsg is something homa wants to say once, in grey, under the body:
// a hint after a wrong key, why a call did not go through. The next key
// clears it.
type noticeMsg string

// warnMsg is a noticeMsg in the terminal's yellow: something went wrong.
type warnMsg string

// callArrived is a caller greeted and waiting to be taken or not.
type callArrived struct{ l *line }

// callGone is a parked call that ran out while nobody answered it; the bar
// drops it if it was showing.
type callGone struct{ l *line }

// callAnswered is a line both people agreed to: the conversation starts.
type callAnswered struct{ l *line }

// callRefused is a call that ended before a conversation, with what to
// say about it; format takes the far side's name.
type callRefused struct {
	// quiet is the person's own doing — they stopped calling — which the
	// bell has no reason to announce.
	quiet bool

	name   string
	format string
}

// callFailed is a call that could not be made or connected.
type callFailed struct {
	name string
	err  error
}

// peerSaid is a message from the far side, already sanitized.
type peerSaid struct{ text string }

// peerLeft is the far side gone, or the line broken; err is nil for a
// clean goodbye.
type peerLeft struct{ err error }

// sendFailed is a message of ours that did not go; the line is as good as
// broken.
type sendFailed struct{ err error }

// tickMsg is once a second while a countdown is on the screen.
type tickMsg time.Time

// fileOffered is the far side offering a file. The session's read goroutine
// waits on reply until the person answers with y or n; the answer carries
// where the file should go, decided at that moment from the settings.
type fileOffered struct {
	name  string
	size  int64
	reply chan<- fileAnswer
}

// fileAnswer is the person's decision on an offer.
type fileAnswer struct {
	accept bool
	dir    string
	reason string // shown to the sender when accept is false
}

// offerTimedOut is an offer nobody answered within offerAnswerTimeout.
type offerTimedOut struct{ name string }

// fileProgress is an incoming file, every progressStep percent.
type fileProgress struct {
	name string
	pct  int
}

// fileDone is a file that arrived and passed its checksum.
type fileDone struct{ name, path string }

// fileFailed is an incoming transfer that failed.
type fileFailed struct {
	name string
	err  error
}

// sending is our outgoing file, every progressStep percent; sent is it
// done; sendFileFailed is it not.
type sending struct {
	name string
	pct  int
}

type sent struct{ name string }

type sendFileFailed struct {
	name string
	err  error
}
