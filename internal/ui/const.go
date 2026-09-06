package ui

// The marks that begin a line, so a reader can tell at a glance where a
// line came from without any color.
const (
	markPrompt = "> " // homa is waiting for you
	markInfo   = "  " // homa telling you something
	markWarn   = "! " // something did not go as planned
	markPeer   = ""   // a message from the other person, shown as [nick] text
)

// maxInputLen bounds one typed line. Generous for a message, small enough
// that a stuck paste cannot exhaust memory.
const maxInputLen = 8 * 1024
