# Decisions, and why

These were argued through once. Reopen them only with a reason.

**The relay region is frozen at first run.** tailcat can pick the nearest relay
at every startup, but the region number is encoded in the address. Letting it
drift would change the address on every launch and break every contact who saved
it. The address must be stable, so latency is measured once and the answer is
written into `key.json`.

**Both peers are equal.** homa listens from the moment it starts. There is no
"host" and no coordination about who waits.

**A call is greeted the moment it arrives, and answered without a keypress.**
The handshake happens in the accept goroutine, so a caller is connected while
they wait rather than timing out after fifteen seconds. The menu then waits on
the keyboard and on the channel of arrived calls together, in one select, and
takes whichever comes first. A call still waits when the person is already in a
conversation or answering a prompt; that is the only case left where anything
is parked.

**A second caller is told why they are turned away.** The greeting completes, a
message says "busy: another call is already waiting", then the connection
closes. Guessing why a connection died is worse than being told.

**Peers are identified by key, never by the name they announce.** The nick in a
handshake is text they typed. `peer.RemoteKey` returns what the WireGuard
handshake proved. The interface behind it is unexported so no other package can
fabricate a connection that claims to know its remote key.

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
