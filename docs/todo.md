# homa: outstanding work

Everything below was found by testing phase 1 end to end. Each item says what
is wrong, why, what to build, which files to touch, and how to know it is done.

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

## 1. Make sending a file bearable

### The symptom

`/send` needs a full path, typed by hand, with no completion and no listing:

```
/send ./docs/architecture.md
```

Tab completion needs raw terminal mode, which is phase 3 work. Something useful
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

### The symptom

If nobody answers, a call waits forever. The caller sits in a conversation with
somebody who is not there.

Less pressing now that calls are answered as they arrive, but a call still
parks whenever the person is inside another conversation or a prompt, and that
one has no deadline.

### What to build

Give a parked call a deadline of about a minute. When it passes:

- take it out of `a.incoming`
- tell the caller: `no answer`
- close it
- tell this side too, so the announcement does not sit on screen as a lie

Say the limit in the announcement: `bob is calling (expires in 60s)`.

Put the timeout in `internal/ui/const.go` next to `dialTimeout`.

### Files

`internal/ui/menu.go`, `internal/ui/const.go`

### Done when

An unanswered call closes itself, and the caller is told why rather than being
left in an empty conversation.

---

## 3. A clear command

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

## 4. Drop input that is only control characters

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

Real line editing, including history on the up arrow, is phase 3.

### Files

`internal/ui/chat.go`

---

## 5. A reset

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

## 6. Tests

**The largest gap in the project.** There is no test file in the repository, and
phase 2 adds rooms, which means more concurrency and more to get wrong.

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

## 7. Install with brew and apt

### The symptom

There is no way to install homa except to clone the repository and build it.
Anyone who is not already a Go developer cannot run it at all.

The roadmap has this under Phase 5. It is here because it is wanted now.

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

## Not in this list, on purpose

These belong to later phases in [roadmap.md](roadmap.md) and should not be
started here:

- rooms and broadcast (phase 2)
- a full-screen interface, line editing, history (phase 3)
- Android (phase 4)
- packaging, short invite codes, self-hosted relays (phase 5)
