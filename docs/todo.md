# homa: outstanding work

Work is grouped by version. **Version 1** is everything the first release needs,
and every numbered item below belongs to it. Versions 2 and 3 are at the bottom,
named but not detailed, so the boundary is visible rather than assumed.

Each item says what is wrong, why, what to build, which files to touch, and how
to know it is done.

Read [working-agreement.md](working-agreement.md), [architecture.md](architecture.md)
and [conventions.md](conventions.md) first: they apply to every item here. In
particular: propose before acting, one file at a time, comments explain why,
`make lint` must pass, and no tailcat or terminal type crosses its package
boundary.

**When an item is finished, delete it from this file.** This list holds only
outstanding work. Anything already shipped belongs in [status.md](status.md),
and the reasoning behind it in [decisions.md](decisions.md).

Work them in the order given.

The input pump that used to head this list is done: reading the keyboard now
happens in a goroutine of its own, so Ctrl+C, an arriving call and the peer
leaving are all just cases in a select. Several items below assumed it, and
their notes have been brought up to date.

---

# Version 1

The first release: two people, text, files, contacts, a line-based interface
that is worth looking at, tests under it, and a way to install it that is not
"clone the repository".

## 1. Make sending a file bearable

### The symptom

`/send` needs a full path, typed by hand, with no completion and no listing:

```
/send ./docs/architecture.md
```

Tab completion needs raw terminal mode, which is version 2 work. Something useful
can be built now without it.

### What to build

Two commands that work together.

**`/files [dir]`** lists what is in a directory, numbered, defaulting to the
working directory:

```
/files ~/Documents
  1) notes.md          4.2 KB
  2) poster.png        1.1 MB
  3) archive/          dir
```

**`/send <number>`** sends an entry from that listing, alongside the existing
`/send <path>`. A number is a number; anything else is a path.

Remember the listed directory on the handler, so `/files archive` then
`/send 2` reads naturally.

Rules worth keeping:

- Do not recurse. One directory at a time.
- Show directories, so `/files` can be used to walk into them, but refuse to
  send one: the existing "send an archive instead" message already covers that.
- Cap the listing (say 50 entries) and say how many were hidden. A `/files ~`
  on a full home directory should not flood the screen.
- Sort directories first, then files, both by name.
- Size in the same human form `humanBytes` already produces.

### Files

- `internal/ui/chat.go`: the `/files` command, and the number branch in `/send`
- `internal/ui/handler.go` or a new small file: the remembered directory and the
  last listing
- `internal/ui/format.go`: reuse `humanBytes`

### Done when

A file can be sent without typing a path, by listing a directory and picking a
number, and `/send <path>` still works as before.

---

## 2. Expire a parked call

**More pressing than it was.** A call is no longer put through on its own: it
waits for the person to answer `take the call from X?`. Nothing bounds that
wait, so a caller can hold an open line indefinitely against somebody who has
walked away from the keyboard.

### The symptom

If nobody answers, a call waits forever. The caller sits in what looks to them
like a conversation with somebody who is not there.

There are two waits now, and both need the deadline:

- the question is on screen and nobody types an answer
- the call is parked behind another conversation or another prompt, and never
  reaches the question at all

### What to build

Give a parked call a deadline of about a minute, counted from when it arrived
rather than from when it is put to the person, since the caller has been
waiting the whole time. When it passes:

- take it out of `a.incoming`, or abandon the question if that is what is on
  screen
- tell the caller: `no answer`
- close it
- tell this side too, so the announcement does not sit on screen as a lie

Say the limit in the announcement: `bob is calling (expires in 60s)`.

The awkward part is the question itself. `Confirm` blocks on `ReadLine`, and
what unblocks a read is its context being canceled, so the deadline wants to be
a context with a timeout around the `offer` call. A canceled `Confirm` already
returns `ErrCanceled` and `offer` already declines on it, so most of this is
choosing where the context is made rather than new machinery.

Put the timeout in `internal/ui/const.go` next to `dialTimeout`.

### Files

`internal/ui/menu.go`, `internal/ui/const.go`

### Done when

An unanswered call closes itself whether it was parked or sitting on the
question, the caller is told why rather than being left in an empty
conversation, and the notice on the answering side does not outlive the call it
describes.

---

## 3. Tell the caller when they have actually been let in

**A protocol change.** Read [architecture.md](architecture.md) and the framing
section of the README before starting: this adds a frame type and bumps
`proto.Version`, and both sides have to keep working with peers that have
neither.

### The symptom

The caller is told they are in a conversation before anyone has agreed to it:

```
  calling aa...

  talking to aa (they call themselves "alice")
  /help for commands, /quit to leave

[aa] they are not taking calls right now
  aa left the conversation.
```

The middle line is the refusal, shown as though the peer had said it in a
conversation that never happened.

### The cause

The handshake carries two meanings that used to be the same thing and are not
any more: *the machines are connected*, and *the person agreed to talk*. The
first is automatic. The second is now a question on the receiver's screen. The
caller cannot tell them apart, because only one signal arrives.

### What to build

**A frame that means "yes".** `proto.TypeAccept = 0x09`, no payload; `0x08` is
the last one taken. `proto.Version` goes to 2.

**The receiver sends it when the person says yes**, in `App.offer`, before
`answer`. Refusal needs no frame of its own: the text reason and the close
already say it, in words a person can read, and a type that exists only to
duplicate that is a type to keep working forever.

**The caller waits for it.** A new `session.WaitAccepted(ctx)`, called by the
dialing side between `Start` and `runChat`, and by nothing else. It reads until
one of:

- `TypeAccept` — return, and the conversation begins
- `TypeBye` — they hung up; return an error saying the call was not taken
- `TypeText` — the refusal reason. Keep it, and report it with the error that
  follows
- a read error, or the context ending

It goes in `session` because that is where framing lives, and it is a separate
call rather than part of `Start` because only one side of a call ever waits.

**The screen** says `waiting for them to answer...` while it waits, and prints
`talking to ...` only afterwards.

### Staying compatible

`Hello` already carries a version, and `Session.Peer().Version` already exposes
it, so this needs no guessing and no timeout to detect an old peer:

| caller | receiver | what happens |
| --- | --- | --- |
| new | new | the caller waits, the accept arrives, nothing is claimed early |
| new | old (version 1) | the caller sees `Version < 2` and skips the wait entirely, behaving exactly as today |
| old | new | the accept frame is an unknown type and is skipped, as `session.Run` already does with anything it does not know; the caller claims early, which is today's behaviour and no worse |

Do not make a version mismatch fatal. `decisions.md` says why, and this is the
first change that puts it to use.

### The wait needs a bound

The caller now waits on a person rather than on a machine, so `WaitAccepted`
takes a context and the dialing side gives it a deadline. Item 2 puts the same
deadline on the receiver's side of that wait; the two are one wait seen from
both ends, and they should agree on how long it is. Put the value in
`internal/proto/const.go` if both sides must agree on it, or in
`internal/ui/const.go` if it is only a local patience.

### Files

- `internal/proto/frame.go`, `internal/proto/const.go`: the type, its `String`, the version
- `internal/session/session.go`: `WaitAccepted`, and whatever sends the frame
- `internal/ui/menu.go`: `offer` sends the accept when the person agrees
- `internal/ui/chat.go`: `startChat` waits, and says so on screen
- `README.md`: the frame table, and "How a call is answered"
- `docs/decisions.md`: what the two signals mean and why they were separated

### Done when

A refused caller sees `waiting for them to answer...` and then that they were
not taken, never `talking to`; an accepted one sees the conversation start only
after the other person agreed; and a version 1 peer on either end still holds a
conversation.

---

## 4. A contacts screen

### The symptom

The address book can be added to and called from, and nothing else. A name typed
in a hurry is a name forever: there is no way to change it, and no way to remove
a contact who is gone. The main menu lists every contact as a line of its own,
which is fine for three and unreadable for thirty.

### What to build

`c) contacts` in the menu, opening a screen of its own that lists the address
book, numbered. Picking a number picks a contact, and then asks what to do with
them:

```
> c

  contacts
   1) BB          tcpGFwWCD2eo...
   2) babak       tcpGFwWCA74k...
> choice: 1

  BB
   c) call
   r) rename
   a) show their address
   f) forget
   b) back
> choice:
```

**rename** is the one that has to be built carefully. It changes the local name
and nothing else: the address and the key stay, because they are what the
contact *is*. That rules out remove-then-add, which would drop the key and make
the next call from them arrive as a stranger. `contacts.Book` needs a `Rename`
that moves the name and keeps the rest, refusing a name already taken the same
way `Add` refuses one.

The name is yours, not theirs. Somebody who calls themselves `babak` is worth
renaming `BB` to `babak` for, but that is a decision the person makes, not
something homa does on their behalf: a peer who could rename their own entry in
your address book could rename it to anything.

**call** is `dial`, which already exists. This is a second way to reach it, not
a second copy of it.

**forget** removes the contact. Ask first, and say what is lost: the key goes
with the name, so their next call arrives as `~` and whatever they call
themselves, and reaching them again means pasting the address again.

### An open question

The main menu still lists contacts as `1) call BB`. Nothing here asks for that
to change, and quick dialling is worth keeping, but with a contacts screen in
place the numbered list at the top has an obvious second home. Decide it when
the screen exists rather than now.

### Files

- `internal/ui/contacts.go`: new. The screen is its own responsibility, and
  `menu.go` is already the longest file in the package
- `internal/ui/menu.go`: the `c` entry, and routing to it
- `internal/contacts/contacts.go`: `Rename`

### Done when

A contact can be renamed and keeps their key, called from the contacts screen,
and forgotten after confirming; renaming to a name already in the book is
refused rather than silently merging two contacts; and a renamed contact's next
call still arrives under the new name rather than as a stranger.

---

## 5. A clear command

### What to build

`clear` at the menu and `/clear` in a conversation, both wiping the screen.

```go
// Clear wipes the screen. The sequence is the usual one: erase everything
// and move the cursor home. It is written to a terminal we control, never
// built from anything that arrived over the network.
func (u *UI) Clear() {
    u.Printf("\033[2J\033[H")
}
```

Redraw the menu afterwards, so the screen is not left blank.

### Files

`internal/ui/ui.go` (the method), `internal/ui/menu.go` and
`internal/ui/chat.go` (the commands), `/help` gains a line.

---

## 6. Drop input that is only control characters

### The symptom

Arrow keys have no meaning without line editing, so they arrive as escape
sequences. Sanitizing strips the escapes and sends the leftovers:

```
[BB] [A[A[B
```

Nothing dangerous, the sanitizer did its job, but it is noise.

### What to build

In `chatInput`, after reading a line, drop it silently if it contains no
printable characters. Do not warn: the person pressed a key that does nothing,
and a warning would be more annoying than the silence.

Real line editing, including history on the up arrow, is version 2.

### Files

`internal/ui/chat.go`

---

## 7. A reset

### What to build

`reset` at the menu, throwing everything away so the next start is a first run
again: settings, address book, and identity.

Ask first, and say what is being lost, because one of the three cannot be
recovered:

```
> reset
! This deletes your identity, your address book and your settings.
! Your address changes, and everyone who saved the old one can no
! longer reach you.
> type the word reset to confirm:
```

A yes-or-no `Confirm` is too easy to answer by reflex for something with no
undo. Make the person type the word.

### What it removes

Everything under the config directory: `key.json`, `config.json`,
`contacts.json`. `paths.Dir` is where they live, and each name already exists as
a constant in the package that owns the file. Remove the files rather than the
directory, so nothing else that happens to be in there is taken with them.

### Then what

The identity is loaded once at startup and the listener is bound to it, so
homa cannot carry on with a deleted key: the address on screen would be an
address nobody can reach. Two ways out, and the second is the one to build
unless there is a reason not to:

1. Re-run `bootstrap`. Correct, and needs the listener closed and replaced
   while an accept goroutine is running on it. That is real concurrency work
   for a command people will use once.
2. Say what was deleted, then exit cleanly. Starting homa again is the first
   run. Nothing has to be torn down mid-flight, and the person is told exactly
   what happened.

### Files

- `internal/ui/menu.go`: the menu entry and the confirmation
- `internal/paths`: removing a file from the config directory, with the same
  care `WriteAtomic` takes putting one there
- `internal/config`, `internal/contacts`, `internal/peer`: each already names
  its own file; the removal should use those names rather than repeating them

### Done when

Running reset, confirming it, and starting homa again gives the first-run
questions and an empty address book, and a person who refuses the confirmation
still has everything they had.

---

## 8. Tests

**The largest gap in the project.** There is no test file in the repository, and
version 3 adds rooms, which means more concurrency and more to get wrong.

Two kinds are wanted, and they catch different things. Write the unit tests
first: they are cheap, they need no network, and they cover the two places a
mistake is most expensive.

### Unit tests

**`internal/proto`** is the easiest and the most valuable. It is pure functions
over an `io.ReadWriter`, so a `net.Pipe` or a `bytes.Buffer` is the whole
fixture. Test: a round trip of every frame type; a frame arriving in pieces; a
declared length over `MaxPayload`; a truncated frame; concurrent writers not
interleaving.

**`internal/session/sanitize.go`** is a security boundary and a pure function,
which is the ideal combination. Test: escape sequences stripped; carriage
returns stripped; invalid UTF-8 replaced; truncation by runes rather than bytes;
`safeFileName` refusing `../`, absolute paths, empty names, and control
characters.

**`internal/contacts` and `internal/config`** with a temporary `XDG_CONFIG_HOME`
or `HOME`: save and load, a duplicate name refused, a corrupt file reported
clearly, an atomic write surviving a replaced file.

**`internal/ui`** is testable now that input arrives on a channel. `ui.New`
takes any `io.Reader`, so a pipe stands in for a keyboard: a line delivered, a
`ReadLine` returning `ErrCanceled` the moment its context is canceled, input
ending mid-prompt, a line longer than `maxInputLen` truncated.

### Integration tests

**`internal/session` end to end over `net.Pipe`**: two sessions, a handshake, a
message each way, a file offer accepted, a file offer rejected, a checksum
mismatch discarding the file, a connection dropped mid-transfer leaving no
`.part` behind. No network is involved, so this belongs in the normal test run.

**Two whole instances, over the real transport.** This is the layer with no
tests at all, and every claim about it has so far been checked by hand. Two
processes, each with its own `HOME`, standard input on a pipe:

- one calls the other and the conversation starts with no keypress
- the caller leaves, and the answering side returns to its menu on its own
- `SIGINT` at the menu exits at once, and `SIGINT` inside a conversation makes
  the peer see a goodbye
- a second caller is told the line is busy
- a file sent and received, with the digest checked at both ends

These need the network and a relay, so keep them behind a build tag or
`testing.Short`, and out of the normal `make test`. `make test-race` on
everything else must stay fast enough that nobody skips it.

The race detector is the point of `make test-race`: it is the only thing that
will catch a mistake in the locking around the address book and the settings,
or in the goroutines the input pump and each session start.

---

## 9. Install with brew and apt

### The symptom

There is no way to install homa except to clone the repository and build it.
Anyone who is not already a Go developer cannot run it at all.

The roadmap had this after everything else. It is in version 1 because a
release nobody can install is not a release.

### What to build

**GoReleaser**, configured so one tagged release produces everything:

- binaries for macOS and Linux, amd64 and arm64
- a `.deb`, which is what makes `apt` possible
- a Homebrew formula pushed to a tap
- checksums, and the version stamped in at build time with
  `-ldflags -X main.version=...` so `homa -version` reports the tag rather than
  `dev`

**A Homebrew tap.** A second repository, `Serajian/homebrew-homa`, holding the
formula GoReleaser writes. `brew install Serajian/homa/homa`.

**An apt repository.** This is the harder half and worth being honest about:
`apt` needs a signed repository served over HTTP, not just a `.deb` file. Two
routes:

1. Publish the `.deb` on the release page and tell people to
   `dpkg -i homa_*.deb`. One line of documentation, no infrastructure, and not
   really `apt`.
2. A real repository, which means a GPG key, a signed `Release` file, and
   somewhere to host it. GitHub Pages can serve it.

Decide which before starting. Route 2 is what the item asks for; route 1 is
what ships this week.

### Files

- `.goreleaser.yaml`: new
- `.github/workflows/`: a workflow that runs GoReleaser on a tag
- `Makefile`: a `release` target, or at least `snapshot` for testing locally
- `README.md`: the install instructions, which are the point of all of this
- `cmd/homa/version.go`: check that the ldflags path actually reaches `version`

### Done when

A tag produces a release with binaries, a `.deb` and a formula; `brew install`
works from a clean machine; the documented Debian route works from a clean
machine; and `homa -version` prints the tag.

---

## 10. Make it look like something

### What this is

The interface works and looks like nothing: one indent for information, an
exclamation mark for a warning, a plain line for everything else. No colour, no
welcome worth the name, no sense that any of it was designed.

**The design is not decided here.** It will be brainstormed when it is picked
up. What this item records is the ground it has to stand on, so the brainstorm
starts from what is already true rather than from a blank page and then walks
back into these one at a time.

### What is already true, and must survive

**Colour is never the only signal.** `internal/ui/const.go` says why the marks
exist: `>` for a prompt, two spaces for information, `!` for a warning, so a
reader can tell at a glance where a line came from *without any colour*. Colour
is allowed to reinforce that. It is not allowed to replace it. A terminal
without colour, a person who cannot see the difference, and a log file all have
to stay readable.

**Not everything homa writes to is a terminal.** Output gets piped and
redirected. Escape sequences written unconditionally end up in the file, which
`clearLine` already does and which is already noted as a wart. Whatever is built
should settle that properly: detect a terminal, honour `NO_COLOR`, and give the
flags a `-no-color` of their own.

**Nothing from the network is ever formatted into an escape sequence.** This is
the same boundary `session/sanitize.go` holds, and colour is exactly the kind of
feature that quietly breaks it: a nick styled by interpolating it into a colour
code is a nick that can carry its own codes. Style around network text, never
through it.

**Version 2 is a full-screen interface** built on bubbletea and lipgloss; see
[roadmap.md](roadmap.md). Time spent hand-rolling ANSI now is time that may be
thrown away, and a palette and a set of rules about what is emphasised are not.
Worth knowing which half is being built.

### Where it would show

- the first thing on screen: the name, the address, and that homa is listening
- the menu
- who said what in a conversation, and the difference between a contact's name
  and a `~` name they chose for themselves
- warnings, and file transfer progress

### Files

`internal/ui`, and nothing below it. If anything under `internal/ui` has to
change to make the interface prettier, something has leaked, and that is the bug
to fix first.

### Done when

To be decided with the design. At minimum: it still reads correctly with colour
switched off, piped to a file, and on a peer's text that is trying to be
clever.

---

## 11. A README worth arriving at

### The symptom

The README is 460 lines of prose and the first thing anyone sees. It is
accurate, and it reads like documentation rather than an introduction: sixteen
headings, no picture above the fold, and the reader has to get four screens down
before anything shows them what homa looks like in use.

It is also the only page most people will ever read. `docs/` is where depth
lives; this is where somebody decides whether to care.

### What to build

Not a rewrite. The prose is good and was argued over — this is about what a
reader meets first and how they move through it.

- **Above the fold**: what homa is in one line, what it looks like running, and
  how to install it. Right now `Install` is at line 88 and the sample
  conversation at line 20, which is the one thing already in the right place
- **Something to look at.** A terminal recording or a still of a real
  conversation. GitHub renders SVG, and a hand-made SVG is a file that has to
  be maintained; a recording is a file that ages. Pick knowingly
- **The long middle belongs in `docs/`.** The wire protocol, the architecture,
  the security notes and the file layout are all reference material with a home
  already. Link to them and keep the summary
- **Both themes.** GitHub renders light and dark, and an asset that assumes one
  is unreadable in the other

### What must stay true

**Every sample is a transcript, not an illustration.** The sample conversation
has been wrong twice already this month, once when the `[me]` label arrived and
once when it changed shape. If it is on the page it has to be what the program
actually prints, and it has to be checked whenever the interface moves.

**Nothing claims more than homa does.** No badge for a test suite that does not
exist yet, no "production ready", no benchmark nobody ran.

### Files

`README.md`, and whatever assets it needs. Nothing under `internal/`.

### Done when

Somebody who has never heard of homa can tell what it is, see it working, and
install it without scrolling past a protocol table; and every line of sample
output matches what the program prints today.

---

## 12. Diagrams that show the real thing

### The symptom

The architecture is an ASCII tree in [architecture.md](architecture.md) and six
mermaid blocks in the README. The tree carries the most important fact in the
codebase — that dependencies point one way and `internal/peer` is the only
package that imports tailcat — in a form nobody can see at a glance.

### What to build

Diagrams for the four things worth drawing, and nothing else:

- **the package graph**, showing the one-way dependency and the two rules that
  hold the shape: peer is the only tailcat importer, and session knows nothing
  about terminals
- **setting up a call**, end to end: dial, handshake, the greeting on the
  accepting side, the question, the accept, the conversation. This is the path
  that has changed three times and is the hardest to hold in your head
- **a file transfer**, offer to digest check to rename, including where a
  `.part` file lives and when it is discarded
- **a frame**, which the README already draws in ASCII and which is the one
  place ASCII is arguably right

### What must stay true

**A diagram that has drifted from the code is worse than no diagram**, because
it is believed. Two ways to keep that from happening, and the choice matters
more than the drawing: generate them from the code, or keep them few enough and
load-bearing enough that anyone changing that code will notice them. Four
diagrams is the second answer.

**They have to render where they are read.** GitHub renders mermaid natively and
`docs/` is read on GitHub, so mermaid costs nothing and needs no build step.
Anything richer is a file to maintain and a build to remember.

### Files

`docs/architecture.md`, `README.md`, and any assets. Nothing under `internal/`.

### Done when

The dependency rule can be seen rather than read, the call-setup path is on one
page from dial to conversation, and every diagram matches the code the day it
lands.

---

## 13. Security

**Version 1**, and last in it only because it has no content yet: an item
without requirements cannot be ordered against items that have them. Placing it
is the point of describing it.

Not yet specified. The heading is here because the work is wanted; what it
covers will be written down before anything is built.

Whoever fills this in: the ground it starts from is
[decisions.md](decisions.md), which already records what homa relies on and
what it deliberately does not claim.

- the tunnel authenticates, not the nick a peer announces, and not the address
  book, which only chooses a label
- an address is a secret and a capability: whoever holds it can call you, and
  it can be forwarded to anyone
- everything arriving from the network is sanitized before it is printed, and
  `session/sanitize.go` is the only place that happens
- a file name from a peer goes through `filepath.Base`, and a file is renamed
  into place only after its digest matches
- a frame is capped at 1 MiB so a peer cannot make homa allocate on request

---

# Version 2

Not to be started while version 1 is open. Detailed in
[roadmap.md](roadmap.md).

- **a full-screen interface**, on bubbletea and lipgloss: a chat pane, a
  separate input line, a contact list. It removes the two warts version 1 lives
  with, because input stops being a line the terminal owns. Only `internal/ui`
  should change; if anything below it has to, something has leaked and that is
  the bug to fix first
- **Android**: `gomobile bind` over the lower packages and a Compose interface.
  It is plausible at all because tailcat needs no VPN permission, and it is the
  reason `proto`, `peer` and `session` must stay free of any desktop assumption

# Version 3

- **rooms**: one host, several guests, join requests, broadcast, and rate
  limiting per guest. The first thing that breaks the two-equal-peers model, so
  it wants care and it wants the tests from version 1 already in place

# Still unplaced

Named in the roadmap and belonging to no version yet:

- a short human-readable invite code that resolves to an address, so nobody has
  to paste two hundred characters. The single biggest usability win still on
  the table
- a self-hosted DERP relay, so a group can run homa without touching
  Tailscale's
