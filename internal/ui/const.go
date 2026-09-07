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

// selfNick labels your own side of a conversation, so it reads as two people
// rather than one. It is the literal "me" rather than the configured nick:
// shorter to read, impossible to confuse with the peer's name, and it needs
// no lookup on the path a message takes out.
const selfNick = "me"

// clearLine puts the cursor back at the start of the line and erases what is
// on it: a carriage return, then the ANSI "erase to end of line". It takes a
// prompt off the screen so something else can be printed where it was,
// instead of leaving an empty "[me] " above every arriving message.
//
// It is a constant homa writes to a terminal it owns. Nothing that arrives
// over the network is ever formatted into an escape sequence; see
// session/sanitize.go for that boundary.
//
// A prompt long enough to have wrapped leaves its earlier rows behind. There
// is no portable way to know how many rows a line took, and guessing wrong
// erases somebody's conversation.
const clearLine = "\r\033[K"

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
