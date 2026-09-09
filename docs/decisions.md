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

**No second encryption layer over tailcat.** The tunnel is WireGuard end to end, with
per-session keys (so recorded traffic stays closed if a key is stolen later) and a
pre-shared key carried in the address (so even the relay operator, who sees both public
keys, cannot join). A relay or anyone on the path learns who talks to whom, when, and how
much — never what. A compromised machine reads everything, and no layer above the tunnel
changes that, because its keys would live on the same machine. Encrypting again inside the
tunnel would therefore protect nothing the tunnel does not, while adding code that can be
wrong and confidence that is not earned. Where a second layer *does* belong is the key at
rest — a passphrase held in a head rather than a file — and that is version 3 work.

**The look: one header, one footer, keycaps, a name column, and three boxes.** Every
screen has the mark and the name top-left, what the screen is about flush right, and a
faint rule under both; the keys that matter sit under a rule at the bottom, drawn as
keycaps — the key on a slate chip, its meaning in grey — so they are found by one glance
down a column. In a conversation the names are right-aligned to one column with a faint
bar after them, so every message's words start in the same place. Boxes are drawn in three
places only, each with a job: an incoming call, in the far side's green, because it is the
one moment that needs the whole screen; the input line, so it is clear where typing goes; a
form's field, so it is clear what is being asked. Nothing else is boxed and nothing is
underlined, because a frame that is everywhere is a frame nobody sees. The menu puts the
people on the left and homa's own keys on the right when the terminal is wide enough, and
one under the other when it is not.

**The full-screen interface is bubbletea, not raw mode by hand.** `golang.org/x/term`'s
`Terminal` was measured and would have fixed the input line — history, cursor movement,
a prompt redrawn under arriving text — but only the input line: no pane that scrolls, no
answer to a resize, no layout, and no place for commands offered as they are typed.
bubbletea v2 gives all of that for three pure-Go modules and one shape: one model, every
event a message, every screen drawn whole, every `Update` on one goroutine. The whole
line-owning machinery of version 1 — the pump, the prompts and their erasing, the
countdown, the hand-rolled palette — went with the line.

**The pane keeps what was said, not what was drawn.** A conversation used to append a
finished line — the padded name, the bar and the words already joined — and the frame cut
every line to the terminal's width, so a message wider than the window lost its tail with
no way to get it back: not by scrolling, which only moves up and down, and not by widening
the window, because the line had been built at the old width. Since a message may be four
thousand bytes, that is an ordinary paragraph made unreadable. The pane now holds the name,
the bar and the words apart and unjoined, and lays them out at the width it is being drawn
at, wrapping the words to the room left of the name column and putting every row after the
first under the words rather than under the name. A resize re-renders, so the same message
reads at any size. `lipgloss.Wrap` does the wrapping because it re-applies the style at each
new row; a wrapped green line is green on every row of it. The frame still cuts, and must:
a body taller than the room would scroll the terminal and take the footer with it.

**Commands are offered in the row that was already there, one line, cut to the width.**
The blank row between the pane and the input box becomes the hint row when a line starts
with `/`, and stays blank otherwise, so nothing on the screen moves when a command is begun.
It is one line because a row that grows takes the pane with it and a person reading what
was said would see it jump; when the row is wider than the terminal it starts where the
marked command is still in view, and the frame cuts the rest. Left and right walk the row
while a command word is being typed, because there is nothing to edit inside a word a few
letters long and the row is what the eye is on; up and down stay history. Tab takes the
marked command; so does Enter, running it when it wants nothing and putting it in the line
when it wants an argument — which keeps the version-1 habit that a lone `/` and Enter is
`/help`, by keeping `/help` first in the table. The marked command carries a mark, `▸`, and
not only a color, and an alias (`/ls`) is taken when typed but never offered, so the row
does not show one command twice. The commands are one table read by the hint, `/help` and
the "no such command" listing alike, so the three cannot drift apart.

**A sound through a tool the machine has, beside the bell.** The bell was chosen for
version 2 because it costs no dependency, and then it was tested on two machines and heard
on neither: most terminals today keep it silent or flash instead. A mechanism nobody hears
is not a mechanism. So homa also plays a short system sound through whatever player the
machine already has — `afplay` and a system sound on macOS, `paplay`, `pw-play` or
`canberra-gtk-play` with the freedesktop bell on Linux — the same shape as the clipboard:
one optional `exec`, nothing linked, nothing shipped, and the bell byte still sent for the
terminals that do ring. Never over ssh, because the sound has to come out of the machine the
person sits at and a server has no speaker; the bell still crosses ssh. One setting for
both, because a person who wants silence wants all of it.

**The bell rings for everything from the far side, and for nothing you did.** The
roadmap had it ring for a call only, on the worry that a conversation that beeps is one
people mute. The decision went the other way, because the reason for a bell — a window
nobody is looking at — holds for a message and a file as much as for a call, and a person
who is looking at the window is not startled by one byte; whoever finds it too much has
one setting, on the settings screen, and it turns every ring off at once rather than asking
six questions. A call left on the screen rings again every ten seconds until it is
answered or runs out, the way a phone does, because one ring at the moment of arrival is
the one most likely to be missed. The outcome of your own call — taken, refused, failed —
rings too, because you may have looked away in the minute it waited; stopping it yourself
does not. What never rings is your own doing: sending, progress, a command's answer — with
one exception, a file that finished sending, because a large one takes minutes and nobody
watches a progress bar for minutes.
The byte goes through `tea.Raw`, the program's own output path, so it never lands inside
a frame; it is a constant in `const.go`, never built from the network; and it is best
effort — a terminal may flash or ignore it, and nothing depends on it having been heard.
Default on, because a bell that has to be found before it works is a bell nobody hears
the first time it matters; the first run does not ask, because the default is right.
`Load` reads a settings file over the defaults, so a file from before the field existed
keeps the bell on rather than getting the zero value.

**The update check asks, never checks by itself, and never installs.** homa's promise is
that it talks to nothing but the relay; a request to GitHub on every start would break that
promise for the sake of a convenience, and would hand GitHub the address of every homa
each morning. So the check is a key, `u`, and the notice says where the request is going.
It does not install either: how homa was installed decides how it is upgraded — brew, apt,
`go install`, an archive — and a binary that replaces itself is one those managers cannot
account for. A self-update is also a download homa would run, and releases are not yet
signed; that belongs after signing, if at all. The page therefore says which release is
out and the one command that upgrades, guessed from where the binary lives.

**The page about you is `me`, on both screens.** `a` stood for "address", and the page
stopped being only the address when it grew a relay and a key; the key is `m` and the page
is `me`, so the menu and the conversation say the same word for the same thing — `m` there,
`/me` here. `a` still opens it, unlisted, the way `/ls` still runs `/files`: a rename should
not punish fingers that learned the old key. `/me` prints into the pane rather than opening
a page, because leaving a conversation to read your own address is exactly what the command
exists to avoid, and `/me copy` does what `c` does on the page, since `c` in a conversation
is an ordinary letter. A conversation draws no notice line, so what a copy says goes into
the pane there.

**What the screens say about the connection, and what they keep to themselves.** The `me`
page names the relay and shows the start of your key, because a person can read a
fingerprint over the phone and a relay name tells them where their traffic meets the world;
`/who` shows the far side's fingerprint and whether it matched the book, which is the
sentence `~` only hints at. The path — direct, or through which relay — is shown on the side
that called, because only that side can ping; the side that answered has no view of it in
the transport's status table (measured empty there, before and after traffic) and says so
rather than guess. The far side's IP is never printed, though the transport knows it: a
screen gets copied into screenshots, and "direct" says everything a person needs. A
duration rather than a clock time for "since", because homa shows no times yet and
`/store` in version 3 owns that question. `c` copies the address two ways at once — OSC 52
through the terminal, which reaches the clipboard of the machine the person sits at even
over ssh and tmux, and a local tool when one exists — because neither way reports
success and a terminal may ignore OSC 52 (Terminal.app does); the notice says what was
sent, not that it arrived.

**Output that is not a terminal is refused.** A full-screen program has nowhere to draw
in a pipe, and nobody chats through one. `homa: needs a terminal`, exit 1, before an
identity is created or a listener opened. Version 1 stays downloadable as v0.1.0; there is
no line-mode fallback, because two interfaces would mean every later feature twice.

**No alternate screen, and the screen is homed first.** When homa exits, what was on the
screen stays in the terminal's scrollback, the way version 1 left it; `vim`-style wiping
would take the conversation with it. Drawing in place has one condition the live tests
found the hard way: the frame must begin at the top of the screen, because the renderer
repaints only the lines that changed, from where it believes the frame began, and lines
scrolled away by a frame drawn lower stay wrong for good. So the screen is cleared and
the cursor homed once, before the program starts. The scrollback is not touched.
