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

// sepUnicode and sepASCII join the parts of one status line — who you are
// talking to, what they call themselves, where files go. A middle dot needs
// a UTF-8 terminal; everything else gets a dash.
const (
	sepUnicode = "  ·  "
	sepASCII   = "  -  "
)

// The frame every screen is drawn in: a two-line header at the top, a
// two-line footer at the bottom, the body between. Below frameMinWidth the
// frame keeps its text and drops its decorations, because a 40-column
// terminal is still a terminal.
const (
	headerHeight  = 2 // the header line and its rule
	footerHeight  = 2 // the rule and the key line
	frameMinWidth = 50

	// menuTwoColumns is the width from which the menu shows the people and
	// homa's own keys side by side; narrower, one under the other.
	// menuLeftColumn is the people's column then.
	menuTwoColumns = 72
	menuLeftColumn = 44

	// inputBoxHeight is the boxed input line of a conversation: a border,
	// the line, a border.
	inputBoxHeight = 3

	// formFieldWidth is the box a form's answer is typed in.
	formFieldWidth = 44

	// nameColumn is where message text starts in a conversation: names are
	// right-aligned to it, so every line of text begins at the same place.
	nameColumn = 9
)

// The keys the screens match on, as bubbletea names them
// (tea.KeyPressMsg.String). Named once so a typo is a compile error rather
// than a key that does nothing.
const (
	keyEnter  = "enter"
	keyUp     = "up"
	keyDown   = "down"
	keyPgUp   = "pgup"
	keyPgDown = "pgdown"
	keyLeft   = "left"
	keyRight  = "right"
	keyTab    = "tab"
	keyQuit   = "ctrl+c"
)

// bell is the terminal's bell, rung when something arrives from the far
// side and the setting says so. A control character, so it lives here with
// the others and is never built from anything that came over the network.
const bell = "\a"

// pickUnicode and pickASCII mark the command picked in the hint row, so
// the pick is a character and not only a color.
const (
	pickUnicode = "▸ "
	pickASCII   = "> "
)

// clearScreen erases the screen and puts the cursor at the top left: the
// ANSI "erase in display, everything" followed by "cursor home". Run writes
// it once, before the program starts, and nothing else writes an escape
// by hand.
//
// It has to be written. The program draws in place, without the alternate
// screen, from wherever the cursor is, and its renderer then repaints only
// the lines that changed, counting rows from where it believes the frame
// began. bootstrap has printed a line or two above, so a frame as tall as
// the terminal scrolls them off — and every later partial repaint lands
// two rows from where it was meant, leaving stale rows behind. The live
// tests' screen grid showed the doubled rows; a real terminal shows the
// same. Homing the cursor first is what makes the frame and the renderer
// agree. The terminal's scrollback is not touched.
const clearScreen = "\033[2J\033[H"

// The words on the menus. They are typed as well as shown, so a menu and
// the switch that reads it have to agree on them.
const (
	wordBack   = "back"
	wordQuit   = "quit homa"
	wordHelp   = "help"
	wordForget = "forget"
	wordCall   = "call %s" // takes the name
)

// unknownMark goes in front of a name a peer chose for themselves, so it
// cannot be read as a name you gave them.
//
// A nick is text the far side typed. Without a mark, somebody could call
// themselves "BB" and appear on your screen exactly as the contact you saved
// under that name. Contact names never carry this prefix, so an unmarked
// label is always one of yours. What actually identifies a peer is the key
// the tunnel proved; see peer.RemoteKeyPrefix.
const unknownMark = "~"

// maxListing bounds how many entries /files shows at once. A home directory
// can hold thousands, and a listing longer than the screen is one nobody can
// pick a number out of.
const maxListing = 50

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

// callRingEvery is how often the bell rings again while a call waits on
// the screen unanswered: a phone that rang once and fell silent is a phone
// nobody heard. Ten seconds is a few rings over the minute a call waits.
const callRingEvery = 10 * time.Second

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

// The welcome banner: homa-icon.svg reduced to half-block cells, 48 columns
// wide, with every horizontal edge — the bubble's top and bottom, the dots,
// the underscore — moved onto whole rows. Drawn by bannerBlock through the
// styles. A feature that ends halfway through
// a cell is drawn with ▀ or ▄, and where those meet a terminal shows a seam;
// only the diagonals still need them. Stroke widths are matched to the cell,
// which is about twice as tall as it is wide: the bubble's walls are two
// columns and its edges one row, so it weighs the same all round; the
// underscore is two rows, to match the chevron. See docs/assets/logo/README.md. It is drawn once, on the welcome screen, and only when style
// says the terminal will show block characters; otherwise bannerPlain is used.
//
// Columns before bannerSplit are the prompt, the rest the bubble, which is
// how the two are given different colors without marking up the rows.
const bannerArt = `   ▄▄▄                     ████████████████
   ████▄▄                ██                ██
     ▀████▄▄             ██                ██
        ▀████▄           ██    ██ ██ ██    ██
         ▄████           ██                ██
      ▄████▀             ██                ██
   ▄████▀                ██████████████████
   ██▀    ██████████████ █████▀
          ██████████████ ██▀`

// bannerPlain is the banner for a terminal that cannot show the blocks, a
// terminal too narrow for them, and output that is not a terminal at all.
// bannerPlainMark is its first half, the prompt, which is the mark beside
// the name in every header.
const (
	bannerPlain     = ">_ [...] HOMA"
	bannerPlainMark = ">_"
)

const (
	bannerWordmark = "H O M A"
	bannerTagline  = "Peer-to-peer terminal chat"
	bannerIndent   = "  "
	bannerCols     = 48 // columns bannerArt needs, after the indent
	bannerSplit    = 24 // where the prompt ends and the bubble begins
)
