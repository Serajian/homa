package ui

import "time"

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

// dialTimeout bounds how long a call attempt waits. Reaching a peer can
// take a while when a direct path has to be negotiated through a relay.
const dialTimeout = 60 * time.Second

// addrPreviewLen is how much of an address to show in a header. The whole
// thing is a secret and two hundred characters long; a dozen is enough to
// tell two addresses apart.
const addrPreviewLen = 12

// offerAnswerTimeout is how long an incoming offer waits for /accept or
// /reject before giving up. Shorter than the sender's patience, so the
// receiver's side gives the clearer message.
const offerAnswerTimeout = 4 * time.Minute

// progressStep is how often progress is reported, in percent. Every chunk
// would be twenty thousand lines for a large file.
const progressStep = 10
