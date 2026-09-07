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

## 1. Install with brew and apt

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

## 2. Make it look like something

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

## 3. Diagrams that show the real thing

### The symptom

The architecture is an ASCII tree in [architecture.md](architecture.md) and six
mermaid blocks in the README. The tree carries the most important fact in the
codebase — that dependencies point one way and `internal/peer` is the only
package that imports tailcat — in a form nobody can see at a glance.

### What to build

Diagrams for the four things worth drawing, and nothing else:

- ~~**the package graph**~~ — drawn, in [architecture.md](architecture.md),
  alongside startup and shutdown
- **setting up a call**, end to end: dial, handshake, the greeting on the
  accepting side, the question, the accept, the conversation. This is the path
  that has changed three times and is the hardest to hold in your head
- **a file transfer**, offer to digest check to rename, including where a
  `.part` file lives and when it is discarded
- ~~**a frame**~~ — the ASCII drawing moved to [protocol.md](protocol.md) with
  the rest of the wire format, and ASCII is the right answer there

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

`docs/architecture.md`, `README.md`, and any assets, which live in `docs/assets/`.
Nothing under `internal/`.

### Done when

The dependency rule can be seen rather than read, the call-setup path is on one
page from dial to conversation, and every diagram matches the code the day it
lands.

---

## 4. Security

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
- **a sound when a call arrives**, so homa can be left in a window nobody is
  watching. The terminal bell is the whole mechanism; anything richer costs a
  dependency this project should not take. A setting decides whether it rings
- **commands offered as they are typed**: `/` showing what can follow it, and
  narrowing as more is typed. Impossible without raw mode, and nearly free once
  the full-screen interface owns the input line. Version 1 answers a lone `/`
  on Enter, which is what can be done from a line-based interface
- **`/store`**, saving the conversation you have been having, typed at any point
  in it. Working at any point is the whole difficulty: it means homa keeps every
  conversation as it happens, whether or not it is ever asked to save one, and
  today homa remembers nothing

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
