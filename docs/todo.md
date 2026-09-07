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

Work them in the order given. Item 1 is first because it removes the cause of
three separate symptoms, and several later items get easier once it is done.

---

## 1. Replace blocking terminal reads with an input pump

**Priority: first. This one blocks the others.**

### The symptoms

Three bugs, one cause.

**a. Ctrl+C prints "shutting down..." and then nothing happens.** The program
stays alive at the menu until Enter is pressed, and sometimes not even then.

**b. After the other person leaves, homa asks for a keypress** before returning
to the menu: "press Enter to go back to the menu".

**c. An incoming call cannot be answered until a key is pressed.** It is
announced, parked, and waits.

### The cause

`UI.ReadLine` calls `bufio.Reader.ReadString`, which blocks in a `read` system
call on standard input. Nothing in Go can interrupt that read: not a context,
not a signal, not another goroutine.

The current workaround, `cmd/homa/shutdown.go`, closes standard input to make
the read return. That is unreliable: on macOS a `read` already in progress on a
descriptor is not guaranteed to return when the descriptor is closed, which is
exactly symptom (a).

### What to build

Move the blocking read into a goroutine of its own that never stops, and have it
deliver lines over a channel. Every consumer then selects between input and
whatever else it is waiting for.

```go
// in internal/ui

type UI struct {
    lines chan string   // one line per read, closed when input ends
    // ...existing fields
}

// New starts the pump.
func New(in io.Reader, out io.Writer) *UI

// ReadLine waits for a line, ctx cancellation, or the end of input.
func (u *UI) ReadLine(ctx context.Context) (string, error)
```

The pump goroutine is deliberately leaked when input never ends: it is one
goroutine for the life of the process, blocked on a read the process is about to
abandon anyway. Say so in a comment, so nobody "fixes" it later.

### What it fixes, and how

- **Ctrl+C**: `ReadLine` returns as soon as the context is canceled, no matter
  what the pump is doing. `cmd/homa/shutdown.go` can then be deleted entirely,
  along with the `finished` channel it needs, and `watchForShutdown` with it.
  Closing standard input stops being part of the design.
- **"press Enter"**: `chatInput` selects on input and on the session ending, so
  the conversation returns to the menu the moment the peer leaves. Delete the
  `press Enter to go back to the menu` line and the `ended` flag dance around it.
- **Parked calls**: `menuLoop` selects on input and on `a.incoming`, so a call
  is answered as it arrives rather than at the next keypress. The announcement
  becomes "bob is calling" and the conversation simply starts.

### Files

- `internal/ui/ui.go`: the pump, the new `ReadLine`
- `internal/ui/prompt.go`, `setup.go`, `menu.go`, `chat.go`: thread the context
  through; every `ReadLine` call gains a `ctx`
- `internal/ui/chat.go`: `chatInput` selects on input and on the session ending
- `internal/ui/menu.go`: `menuLoop` selects on input and on `a.incoming`
- `cmd/homa/shutdown.go`: delete
- `cmd/homa/main.go`: drop `watchForShutdown` and its channel

### Done when

- Ctrl+C at the menu exits immediately, printing nothing but a goodbye
- Ctrl+C inside a conversation ends it, the peer sees a goodbye, and the process
  exits at once
- when the peer leaves, the menu comes back on its own
- an incoming call connects without a keypress
- `README.md` and [status.md](status.md) lose the "known warts" that no longer
  exist

---

## 2. Show your own messages

### The symptom

Your own lines appear as bare text while theirs are labeled, so a conversation
reads as if only one person is in it:

```
salam
[bob] salam, chetori
hi
```

### What to build

Echo your own message with your own name, exactly like theirs:

```
[me] salam
[bob] salam, chetori
[me] hi
```

Use the literal `me` rather than the configured nick. It is shorter, it never
collides with the peer's name, and it needs no lookup.

This is an echo, not a round trip: print it when it is sent, not when anything
comes back.

### Files

`internal/ui/chat.go`, in `chatInput`, right after `SendText` succeeds.

### Caveat, and what to do about it

The line the person typed is already on the screen, because the terminal echoed
it as they typed. Printing it again shows it twice.

Two options:

1. **Accept it for now.** Simple, and the duplicate disappears in phase 3 when a
   full-screen interface owns the input line.
2. **Erase the typed line first** by writing `\033[1A\033[2K` (up one line, clear
   it) before printing `[me] ...`. Works in any normal terminal, but it is a
   guess about terminal state and will look wrong if the typed line wrapped.

Take option 1 unless the double line is unbearable. If option 2 is taken, put the
escape sequence behind a named constant in `internal/ui/const.go` with a comment
saying exactly what it does, and never build it from network input.

### Done when

Both sides of a conversation are labeled, and it is obvious at a glance who said
what.

---

## 3. Make sending a file bearable

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

## 4. Name incoming callers correctly

### The symptom

Every incoming call is "someone unrecognized", even from a contact whose key was
recorded on a previous call. The log shows why:

```
level=DEBUG msg="no key found for connection" pkg=peer addr=fd7a:115c:a1e0:3be2:...
level=DEBUG msg="incoming connection accepted" pkg=peer known_key=false
```

So `contacts.ByPubKey` never matches, and the address book only half works.

### The cause, as far as it is known

`peer.lookupKey` maps a connection's tunnel address back to a node key by
searching `Server.Status().Peer`. On the accepting side that map appears to be
empty or to lack `TailscaleIPs`. A fresh server's network map shows
`"Peers":null`.

### Step one: find out what is actually there

Add temporary logging inside `lookupKey` before doing anything else:

```go
status := srv.Status()
lg.Debug("status peers", "count", len(status.Peer))
for k, p := range status.Peer {
    lg.Debug("status peer", "key", k.String(), "ips", p.TailscaleIPs)
}
```

Take one call, read `/tmp/b.log`, and decide from evidence. Do not guess.

### Likely outcomes

**If the peers are there but arrive late**, the lookup runs before the network
map updates. Retry briefly: look, and if nothing matches, wait 50ms and look
again, up to about half a second. Do it inside `lookupKey` so no caller has to
know.

**If the peers are never there**, use what the address itself carries. A tailcat
address embeds the first eight bytes of the node key:

```
nodekey:3be24627f2f1914f7592f144...
fd7a:115c:a1e0:3be2:4627:f2f1:914f:7592
             ^^^^ ^^^^ ^^^^ ^^^^ ^^^^
```

Eight bytes is not an identity, but it is enough to match against keys already
in the address book, which is all this feature needs. Then:

- `peer.RemoteKey` keeps returning the full key when it is known, from the
  dialing side
- add `peer.RemoteKeyPrefix`, returning the hex prefix taken from the address
- `contacts` gains `ByPubKeyPrefix`, matching a stored key by its start

Be honest in the comments about what this proves. A prefix match says "this is
consistent with being that contact", not "this is cryptographically that
contact". The tunnel is what actually authenticates; the address book is only
choosing a label. Write that down where the function lives, so nobody later
mistakes it for authentication.

### Files

`internal/peer/remote.go`, `internal/contacts/contacts.go`,
`internal/ui/menu.go`

### Done when

Calling a contact once, then having them call back, shows their name rather than
"someone unrecognized", and the reasoning about what a prefix does and does not
prove is written in the code.

---

## 5. Expire a parked call

### The symptom

If nobody answers, a call waits forever. The caller sits in a conversation with
somebody who is not there.

Less pressing once item 1 lands, since calls are then answered as they arrive,
but a call still parks whenever the person is inside another conversation or a
prompt.

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

## 6. A clear command

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

## 7. Drop input that is only control characters

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

## 8. Tests

**The largest gap in the project.** There is no test file in the repository, and
phase 2 adds rooms, which means more concurrency and more to get wrong.

Where the value is, in order:

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

**`internal/session`** end to end over `net.Pipe`: two sessions, a handshake, a
message each way, a file offer accepted, a file offer rejected, a checksum
mismatch discarding the file, a connection dropped mid-transfer leaving no
`.part` behind.

**`internal/contacts` and `internal/config`** with a temporary `XDG_CONFIG_HOME`
or `HOME`: save and load, a duplicate name refused, a corrupt file reported
clearly, an atomic write surviving a replaced file.

Run them with `make test-race`. The race detector is the point: it is the only
thing that will catch a mistake in the locking added for the address book and
the settings.

---

## Not in this list, on purpose

These belong to later phases in [roadmap.md](roadmap.md) and should not be
started here:

- rooms and broadcast (phase 2)
- a full-screen interface, line editing, history (phase 3)
- Android (phase 4)
- packaging, short invite codes, self-hosted relays (phase 5)
