# Decisions, and why

These were argued through once. Reopen them only with a reason.

**The relay region is frozen at first run.** tailcat can pick the nearest relay
at every startup, but the region number is encoded in the address. Letting it
drift would change the address on every launch and break every contact who saved
it. The address must be stable, so latency is measured once and the answer is
written into `key.json`.

**Both peers are equal.** homa listens from the moment it starts. There is no
"host" and no coordination about who waits.

**A call arriving at the menu is parked, not answered.** A blocking read on a
terminal cannot be interrupted from another goroutine. The call is announced,
and the next keypress picks it up. This is a limitation of a line-based
interface, and Phase 3 removes it without changing the architecture.

**A second caller is told why they are turned away.** The greeting completes, a
message says "busy: another call is already waiting", then the connection
closes. Guessing why a connection died is worse than being told.

**Peers are identified by key, never by the name they announce.** The nick in a
handshake is text they typed. `peer.RemoteKey` returns what the WireGuard
handshake proved. The interface behind it is unexported so no other package can
fabricate a connection that claims to know its remote key.

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

**Ctrl+C closes standard input.** Cancelling a context does not wake a goroutine
blocked on the keyboard, so without this Ctrl+C did nothing until Enter was
pressed. Closing the input makes the read return, and every loop already treats
the end of input as "we are done".

**Locks live with the thing they guard.** `contacts.Book` takes its own RWMutex,
because the accept goroutine reads it while the person edits it. `EditSettings`
returns a new config rather than writing through the old pointer, so the lock is
held for a pointer swap rather than for as long as somebody takes to answer a
question.
