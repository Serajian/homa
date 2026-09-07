package ui

import (
	"time"

	"github.com/Serajian/homa/internal/session"
)

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

// unknownMark goes in front of a name a peer chose for themselves, so it
// cannot be read as a name you gave them.
//
// A nick is text the far side typed. Without a mark, somebody could call
// themselves "BB" and appear on your screen exactly as the contact you saved
// under that name. Contact names never carry this prefix, so an unmarked
// label is always one of yours. What actually identifies a peer is the key
// the tunnel proved; see peer.RemoteKeyPrefix.
const unknownMark = "~"

// clearScreen erases the screen and puts the cursor back at the top left: the
// ANSI "erase in display, everything" followed by "cursor home". Both are
// needed — erasing without moving leaves the cursor wherever it was, writing
// the next line into the middle of a blank screen.
//
// Like clearLine it is a constant homa writes to a terminal it owns, and
// nothing that arrived over the network is ever formatted into one.
const clearScreen = "\033[2J\033[H"

// maxInputLen bounds one typed line. Generous for a message, small enough
// that a stuck paste cannot exhaust memory.
const maxInputLen = 8 * 1024

// dialTimeout bounds how long a call attempt waits. Reaching a peer can
// take a while when a direct path has to be negotiated through a relay.
const dialTimeout = 60 * time.Second

// callAnswerTimeout is how long a call waits to be taken. It is
// session.AnswerWindow rather than a number of its own, because the caller is
// relying on the same figure: a deadline only this side knew about would hang
// up on somebody who was told they had longer.
//
// It runs from when the call arrived, not from when the question reaches the
// screen. The caller has been waiting the whole time either way, and a call
// parked behind a conversation has already spent some of it.
const callAnswerTimeout = session.AnswerWindow

// countdownStep is how often a line showing the time left is redrawn. Once a
// second is what a person expects of a countdown, and anything faster is a
// line that flickers for no information.
const countdownStep = time.Second

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
