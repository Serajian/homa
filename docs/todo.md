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

## 1. Show your own messages

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

## 2. Make sending a file bearable

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

## 3. Name incoming callers correctly

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

## 4. Expire a parked call

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

Real line editing, including history on the up arrow, is phase 3.

### Files

`internal/ui/chat.go`

---

## 7. Tests

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

## Not in this list, on purpose

These belong to later phases in [roadmap.md](roadmap.md) and should not be
started here:

- rooms and broadcast (phase 2)
- a full-screen interface, line editing, history (phase 3)
- Android (phase 4)
- packaging, short invite codes, self-hosted relays (phase 5)
