package ui

import tea "charm.land/bubbletea/v2"

// adapter is what a session reports to. It turns each callback into a
// message and hands it to the program; it holds nothing and decides
// nothing, so the session's read goroutine never touches the model.
type adapter struct {
	send func(tea.Msg)
}

// OnText is a chat message from the peer, already sanitized.
func (a *adapter) OnText(text string) { a.send(peerSaid{text}) }
