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
