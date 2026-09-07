# homa: project context

This file is the briefing for anyone, human or agent, picking this project up.
It says where homa came from, what has been decided and why, what exists today,
and what comes next. Read it before changing anything.

---

## 1. What homa is

A peer-to-peer terminal chat with file transfer, written in Go.

Two machines exchange one address, then talk directly: encrypted, through NAT,
with no account, no coordination server, and no shared network. Neither side is
"the server". Both listen, both can call.

## 2. Where it started

The project began from Tailscale's release of
[tailcat](https://github.com/tailscale/tailcat), which takes Tailscale's data
plane, WireGuard encryption plus NAT traversal plus DERP relays, and strips away
the control plane: no accounts, no tailnet, no admin panel. Reaching a peer needs
only a compact address string that encodes their public keys, a pre-shared key,
and which relay to meet at.

That removed the hard part of building a direct peer-to-peer chat. Prior art was
surveyed before starting:

- **Tox / toxic** is the closest existing thing, but runs its own DHT and has
  fought NAT traversal for years. homa borrows that layer instead of building it.
- **croc** and **magic-wormhole** have the best "just connect" experience,
  built on a short human-readable code rather than a long key. Worth stealing
  later; see Phase 5.
- **chat-tails** is a terminal chat over Tailscale, but requires everyone to be
  in the same tailnet, is centralized around one host, and has no file transfer.
  homa's whole point is that two people share nothing beforehand.

So the gap homa fills: terminal chat, plus file transfer, plus peer-to-peer with
no account, on top of infrastructure that already works.

The name is the Homa, the bird of Persian myth that never lands.

## 3. Working agreement

These are the rules the project has been built under. Keep to them.

1. **Propose before acting.** Nothing gets created, renamed, or restructured
   without being described first and agreed to. That includes files, packages,
   dependencies, and "helpful" extras.
2. **One file at a time.** Deliver a file, get it reviewed, then move on. Do not
   dump five files and hope.
3. **Explain decisions in the code.** Comments say *why*, not *what*. If a line
   exists because of a trap, name the trap. Future readers will otherwise
   "simplify" it back into the bug.
4. **Nothing not asked for.** No speculative features, no extra columns, no
   convenience wrappers that add a name and no meaning.
5. **`make lint` must pass before a commit**, and the git hooks enforce it.
6. **Commit messages explain the reasoning**, not just the change. Look at
   `git log` for the established shape: a subject line, then what and why, with
   the decisions worth remembering spelled out.

## 4. Architecture

Dependencies point one way. Nothing below knows about anything above.

```
cmd/homa
   |
   v
internal/ui           menu, chat screen, prompts, everything a person sees
   |     \
   |      +---> internal/config     the settings a person chose
   |      +---> internal/contacts   the address book
   v
internal/session      one conversation: handshake, read loop, file transfer
   |
   v
internal/proto        framing and message types

internal/peer         listening, dialing, identity   (the only tailcat importer)
internal/paths        where files live, writing them safely
internal/logx         one logging switch for the whole program
```

### Two rules that hold the shape

**`internal/peer` is the only package that imports tailcat.** Its exported API
speaks in `net.Conn` and plain `string`. No tailcat or tailscale type appears in
any signature outside it. Swapping the transport later means rewriting that one
package. Do not leak those types outward, even "just for now".

**`internal/session` knows nothing about terminals; `internal/ui` knows nothing
about wire formats.** A session reports through a `Handler` interface that the
UI implements. This is the seam the full-screen interface will slot into, and
the seam an Android UI would reuse.

### Package responsibilities

| Package | Owns |
| --- | --- |
| `cmd/homa` | flags, logging setup, signals, wiring, exit codes |
| `internal/ui` | menu, chat, prompts, first-run setup, all output |
| `internal/session` | handshake, read loop, text, file transfer, sanitizing |
| `internal/proto` | frame layout, message structs, encode and decode |
| `internal/peer` | identity, relay choice, listen, dial, remote key |
| `internal/config` | display name, download directory, validation |
| `internal/contacts` | the address book, with its own locking |
| `internal/paths` | config directory, permissions, atomic writes, `~` |
| `internal/logx` | one switchable logger, discarding by default |

## 5. Decisions, and why

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

## 6. Conventions

- **Constants** live in each package's `const.go`. Enum values stay beside their
  type, so adding a frame type touches one file rather than two.
- **The logger variable is `lg`, never `logger`.** Tools that process one file
  at a time (`goimports`, `golines`, editor auto-import) cannot see that a
  package-level variable is declared in a sibling file. A variable named
  `logger` looks like a package selector to them, and they add an import for an
  unrelated package that happens to be called `logger`. This cost an afternoon.
  Do not rename it back.
- **Logging is off by default.** `logx` discards until `cmd/homa` says
  otherwise, because a log line landing mid-conversation would scramble the
  interface. Use `Debug` for detail, `Info` for things that happen once, `Warn`
  for something unexpected that was handled. Do not log on a hot path:
  `proto` deliberately logs nothing, since a large file is twenty thousand
  frames.
- **Errors wrap with context** and are prefixed by package: `peer: dialing:
  ...`. The UI strips that prefix before showing an error to a person.
- **Never `return x, nil` while holding a non-nil error.** `nilerr` catches it,
  and it is usually a bug being planted.
- **Files are split by responsibility**, not by size. When a file grows a second
  reason to change, split it.

## 7. Current state: Phase 1 is complete

Working today:

- first-run setup, saved settings, a saved identity with a stable address
- an address book: add, list, call by name, keys learned on first contact
- listening and dialing at the same time, incoming calls parked and announced,
  a busy caller told why
- text chat with slash commands
- file transfer in both directions, with confirmation, progress, a digest check,
  no overwriting, and cleanup of partial files
- graceful shutdown from either screen
- `make lint` clean, hooks wired, Makefile covering build, format, lint, test,
  cross-compile

Known warts, all fixed by Phase 3:

- a message arriving while you type is printed over your half-finished line
- after the other person leaves, a keypress is needed to return to the menu
- a message typed but not yet sent when the peer leaves is dropped silently

Not yet built: tests. There is no test file in the repository. That is the
largest gap in the project and should be closed before Phase 2 adds concurrency.

## 8. What comes next

### Phase 2: rooms

One person hosts a room; several guests join. This is the first thing that
breaks the "two equal peers" model, so it needs care.

- a host mode that accepts several connections instead of one
- a join request a host approves or refuses, reusing the parked-call idea
- broadcast: a message from one guest reaches all the others
- rate limiting per guest, which chat-tails does and homa currently does not
- the protocol gains a sender field on TEXT, or a per-guest id assigned at join

The framing does not change. `proto` should need only new message types.

### Phase 3: a full-screen interface

Using bubbletea and lipgloss, the same tools chat-tails uses.

- a chat pane, a separate input line, a contact list
- this removes all three known warts at once, because input stops being a
  blocking read
- only `internal/ui` changes. If anything below has to change, something has
  leaked, and that is the bug to fix first

### Phase 4: Android

- `gomobile bind` over the lower packages, a Kotlin and Compose interface
- tailcat needs no VPN permission because everything is userspace, which is the
  reason this is even plausible
- the hard parts are the foreground service for staying connected and battery
  behavior, not the networking
- keep `proto`, `peer` and `session` free of any desktop assumption: no direct
  terminal reads, no assumptions about file paths. Everything is injected

### Phase 5: distribution and polish

- GoReleaser: binaries, `.deb` and `.rpm`, a Homebrew tap, one tagged release
  producing all of them
- version stamped at build time with `-ldflags -X main.version=...`
- a short human-readable invite code that resolves to an address, in the spirit
  of croc, so nobody has to paste two hundred characters. This is the single
  biggest usability win still on the table
- self-hosted DERP, so a group can run homa without touching Tailscale's relays

## 9. Traps already hit

Recorded so nobody pays for them twice.

**`golines` invents imports.** It formats each file independently, so a
package-level `logger` variable looked like a missing import and it added
`bytedance/gopkg/util/logger` to one file and `gorm.io/gorm/logger` to another.
Renaming the variable to `lg` fixed it at the root. Editors do the same thing
for the same reason.

**`proxy.golang.org` is unreliable from some networks.** If `go get` fails with
403 or a TLS timeout, set a mirror: `go env -w GOPROXY=https://goproxy.io,direct`.
The Go toolchain itself is fetched the same way, so this also fixes
"cannot download go1.x".

**`golangci-lint` must be built with a Go at least as new as `go.mod` targets.**
A Homebrew binary built against an older Go refuses to run. `go install` it
instead, so it is compiled with the toolchain in use.

**A Homebrew Go upgrade breaks the shell's command cache.** `rehash` in zsh, or
open a new terminal.

## 10. Working on this project

```sh
make            # every target, with descriptions
make at-first   # dev tools, deps, git hooks
make check      # what CI would run
make lint
make build
make run
make doc        # the public API of every internal package
```

Testing across two machines: `make build-linux`, copy the binary to the second
machine, run `homa` on both.

To watch what homa is doing without disturbing the interface:

```sh
homa -log homa.log -debug
tail -f homa.log
```
