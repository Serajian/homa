# Decisions, and why

These were argued through once. Reopen them only with a reason.

**The relay region is frozen at first run.** tailcat can pick the nearest relay
at every startup, but the region number is encoded in the address. Letting it
drift would change the address on every launch and break every contact who saved
it. The address must be stable, so latency is measured once and the answer is
written into `key.json`.

**Both peers are equal.** homa listens from the moment it starts. There is no
"host" and no coordination about who waits.

**A call is greeted the moment it arrives, and put through only when the person
says so.** The handshake happens in the accept goroutine, so a caller is
connected while they wait rather than timing out after fifteen seconds. The menu
waits on the keyboard and on the channel of arrived calls together, in one
select, so the question appears as the call lands rather than at the next
keypress.

Answering the telephone is the person's decision. Having somebody's address is
not the same as being welcome to talk to them, and an address is a string that
can be forwarded to anyone. Refusing is the default: a keypress left over from
the menu must not be able to let a stranger in, and a call refused by accident
can be made again while one accepted by accident cannot be taken back.

A refused caller is told, in the same way a caller finding the line busy is
told, rather than having the connection dropped on them to puzzle over.

**A finished handshake means two programs are talking, not that a person
agreed.** They used to be the same event, and reading one as the other is what
told a caller they were in a conversation moments before being turned away from
it. So the side that was called sends a frame of its own, `TypeAccept`, at the
moment the person says yes, and the caller waits for it before claiming
anything. Refusal needs no frame: the reason and the close already say it in
words a person can read.

Version 2 of the protocol is that frame. A peer announcing version 1 never
sends it, and a caller seeing version 1 does not wait — their handshake is all
the agreement there is, which is exactly how homa behaved before. A version 1
peer skips the frame as an unknown type, which the framing has always allowed.

**A call is answered inside a minute or it is hung up on.** A person who has
walked away from the keyboard should not hold somebody's line open, and a
caller should not be left guessing. The window is counted from when the call
arrived rather than from when the question reaches the screen, because a call
parked behind a conversation has already spent some of it and the caller has
been waiting the whole time.

Both sides use the same figure: `proto.AnswerTimeout` is what the receiver
promises and what the caller relies on, with a grace period on top of the
caller's wait so the receiver's own message normally arrives first. It is an
agreement between peers, so it lives with the protocol rather than in settings.

Both sides also watch it run down. A deadline nobody can see is a deadline that
arrives as a surprise, so the question and the caller's waiting line each redraw
themselves once a second with the time left. The receiver pays for this: the
question is a prompt being rewritten, so a `y` already typed leaves the screen
while staying in the terminal's buffer. Those characters cannot be read back to
redraw them, which is the same limitation the full-screen interface removes by
owning the input line.

**A second caller is told why they are turned away.** The greeting completes, a
message says "busy: another call is already waiting", then the connection
closes. Guessing why a connection died is worse than being told.

**Peers are identified by key, never by the name they announce.** The nick in a
handshake is text they typed. `peer.RemoteKey` returns what the WireGuard
handshake proved. The interface behind it is unexported so no other package can
fabricate a connection that claims to know its remote key.

A peer the address book does not know is still shown by the name they announced,
because "someone not in your contacts" on every line tells the reader nothing
they cannot see from its absence elsewhere. It is prefixed with `~`, and contact
names never carry that prefix, so a caller who names themselves `BB` appears as
`~BB` and cannot be mistaken for the `BB` you saved. The mark is the whole
defence: without it the two are the same string.

**An incoming caller is named from the start of their key, carried in the
tunnel address.** A dialed connection knows the peer's key, because the address
that was dialed contains it. An accepted one does not: tailcat's status table is
empty on the accepting side, measured on a server that had just taken a call, so
there is nothing to look the connection up in. What does arrive is the tunnel
address, and its last ten bytes are the first ten of the caller's key. That is
enough to pick one contact out of an address book.

It is a label and nothing else. A prefix match says a contact is consistent with
the caller, not that it is them, and a prefix matching two contacts names
neither. Nothing about who may connect is decided here: the tunnel already
completed a WireGuard handshake with whoever holds the private half of that key,
and an address book adds nothing to that.

**Tests live in the package they test, not beside it.** The rule about testing
only a public API is for libraries with users outside the repository, and homa
has none: every package is under `internal/`. What it does have is a deliberately
small exported surface, which puts the things most worth testing out of reach of
an external test package — `sanitizeText`, `sanitizeNick` and `safeFileName` are
the security boundary and none of them is exported.

The cost is that a test can pin an implementation rather than a promise, and
then a harmless refactor breaks it. That is answered by discipline rather than
by structure: test what a function promises, which is what its comment says, and
not how it currently does it.

**Everything from the network is sanitized before it is printed.** A terminal
obeys what it is given: an escape sequence could clear the screen, a carriage
return could repaint earlier lines and forge messages. `session/sanitize.go` is
the single boundary; do not print network content that has not been through it.

**File names from a peer go through `filepath.Base`.** Without it, a name like
`../../.ssh/authorized_keys` escapes the download directory. This was the
sharpest edge in the program.

**Files land in `.part` and are renamed only after the digest matches.** An
interrupted or corrupted transfer never leaves a file that looks finished. A
mismatch discards the file: TCP already catches damage in transit, so a mismatch
means the two sides disagree about the content.

**Chunks are 32 KiB of raw bytes, not JSON.** Base64 inside JSON would add a
third to every transfer, and small chunks let chat messages interleave with a
file so a conversation keeps flowing.

**Frames are capped at 1 MiB.** A peer cannot announce a huge length and make
homa allocate it. The largest frame homa itself sends is a chunk.

**A version mismatch is logged, not fatal.** The frame format is stable, so
peers on different versions can still chat. Cutting them off would fragment
every future release. Unknown frame types are skipped for the same reason, and
that is also what let the build tolerate file frames before `files.go` existed.

**Protocol values are not configurable.** `peer.Port`, `proto.Version`,
`proto.ChunkSize` and `proto.MaxPayload` are agreements between two peers, not
settings. Letting two sides disagree would break the connection with no useful
error. Settings live in `config.json`; debug knobs are flags.

**One goroutine reads the keyboard, for the life of the program.** A read on a
terminal blocks inside a system call that nothing in Go can interrupt: not a
context, not a signal, and not closing the descriptor, which on macOS is not
guaranteed to make a read already in progress return. So `ui.New` starts a pump
that reads forever and hands finished lines over an unbuffered channel, and
every consumer selects between that channel and whatever else it is waiting on.
Ctrl+C, the peer leaving, and a call arriving all become one more case in a
select.

The pump is never stopped, deliberately. It ends up blocked on input the process
is about to abandon, which costs one goroutine for the life of the program. The
alternative was closing standard input to force the read to return, and that is
exactly the unreliable trick this replaced.

**Locks live with the thing they guard.** `contacts.Book` takes its own RWMutex,
because the accept goroutine reads it while the person edits it. `EditSettings`
returns a new config rather than writing through the old pointer, so the lock is
held for a pointer swap rather than for as long as somebody takes to answer a
question.

**The logo carries its own letters.** The wordmark and tagline in
`docs/assets/logo/` are outlines, not `<text>`: an SVG that names a font renders
differently on every machine that lacks it, and GitHub lacks all of them. The
face is JetBrains Mono, chosen because its licence (SIL OFL 1.1) allows exactly
this and the system fonts on the design machine do not.

**Color means one thing each, and the meaning comes from the logo.** The prompt in the
logo is cream and the bubble is green, so in the interface cream is you (the prompt, your
label, the keys), green is them (names, their messages' label, files arriving), grey is
homa talking (information, hints, the marks around a name), and the terminal's own yellow
is a warning. Nothing else is colored: the words people type are theirs. Four colors with
one meaning each is the whole vocabulary; adding a fifth needs a fifth meaning.

The meanings are fixed; the palette is the terminal's to choose. The logo's 24-bit
green and grey are sent only when the terminal says it can show them (`COLORTERM`
truecolor or 24bit, or a `-direct` TERM); everything else gets the sixteen ANSI colors,
which every terminal has shown for forty years. "You" is bold in the terminal's own
foreground in both — cream on a dark theme, ink on a light one — because a fixed cream
disappears on white, and the warning is the terminal's own yellow for the same reason.

**Color is never the only signal.** The marks in `ui/const.go` — `>` for a prompt, two
spaces for information, `!` for a warning, `[name]` and `~` — carry the meaning on their
own, and color only reinforces them. The colored output, with its escapes removed, is
byte for byte the plain output, and `ui/theme_test.go` holds it there. So a terminal
without color, a person who cannot see the difference, and a log file read the same thing.

**What the output can show is decided once, from the writer.** `ui/style.go` looks at
where output is going when the UI is made: a terminal or not, its width, `NO_COLOR`,
`TERM=dumb`, a UTF-8 locale, and `-no-color`. Everything that would erase or color goes
through it, so a pipe or a redirected file never receives an escape sequence — a line
that would have been erased is ended instead. Asking on every line would be asking a
pipe; asking once is enough because none of it changes while homa runs.

**Network text is wrapped in a color, never formatted into one.** A nick, a file name and
a message have been through `session/sanitize.go`; painting one is `color + text + reset`,
so the text is content and never a parameter of the escape. `fmt.Sprintf("\033[%sm", x)`
with anything from the far side in `x` is the mistake this rules out.

**Menus are grouped by space, not by lines, with the people first.** Calling somebody is
what the menu is for, so the contacts come first; what belongs together sits together with
a blank line between groups; and leaving and anything that cannot be undone sit last and
recede in grey. Boxes, rules and bold are for the full-screen interface, if at all.
