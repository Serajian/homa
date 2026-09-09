package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// adapter is what a session reports to. It turns each callback into a
// message and hands it to the program; it decides nothing, so the
// session's read goroutine never touches the model. Its one piece of
// state, the progress steps already reported, is only ever touched from
// that goroutine.
type adapter struct {
	send     func(tea.Msg)
	lastStep map[string]int
}

func newAdapter(send func(tea.Msg)) *adapter {
	return &adapter{send: send, lastStep: make(map[string]int)}
}

// OnText is a chat message from the peer, already sanitized.
// OnAddress is the peer handing over their address. It only reaches the
// screen; keeping it is a decision made there.
func (a *adapter) OnAddress(addr string) { a.send(addressGiven{addr: addr}) }

func (a *adapter) OnText(text string) { a.send(peerSaid{text}) }

// OnFileOffer announces the offer and waits for the person to answer it in
// the chat. It blocks the read goroutine on purpose: the sender is waiting
// anyway, and nothing else from them should slip in front of the question.
func (a *adapter) OnFileOffer(name string, size int64) (dir string, accept bool, reason string) {
	reply := make(chan fileAnswer, 1)
	a.send(fileOffered{name: name, size: size, reply: reply})

	select {
	case ans := <-reply:
		if !ans.accept {
			return "", false, ans.reason
		}
		return ans.dir, true, ""
	case <-time.After(offerAnswerTimeout):
		a.send(offerTimedOut{name})
		return "", false, "no answer"
	}
}

// OnFileProgress reports an incoming transfer in steps rather than on every
// chunk: a 700 MB file is twenty thousand chunks, and a line each would
// bury the conversation.
func (a *adapter) OnFileProgress(name string, received, total int64) {
	step := percent(received, total) / progressStep
	if last, seen := a.lastStep[name]; seen && step <= last {
		return
	}
	a.lastStep[name] = step
	a.send(fileProgress{name: name, pct: step * progressStep})
}

// OnFileDone reports a file that arrived and passed its checksum.
func (a *adapter) OnFileDone(name, path string) {
	delete(a.lastStep, name)
	a.send(fileDone{name: name, path: path})
}

// OnFileError reports a transfer that failed.
func (a *adapter) OnFileError(name string, err error) {
	delete(a.lastStep, name)
	a.send(fileFailed{name: name, err: err})
}
