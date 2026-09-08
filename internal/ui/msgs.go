package ui

// The messages that reach the model from outside a keypress: from below
// (calls, the peer, files), from timers, and from homa itself. Every one
// of them is a plain value; the goroutine that produced it hands it to
// Program.Send and touches nothing else, which is what lets the model run
// without a lock.

// noticeMsg is something homa wants to say once, in grey, under the body:
// a hint after a wrong key, why a call did not go through. The next key
// clears it.
type noticeMsg string
