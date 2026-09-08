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

Complete, and released as v0.1.0: two people, text, files, contacts, an
interface with a design, and installation through brew and apt. Nothing is
left here.

---

# Version 2

Version 1 is closed; this is the open front. Detailed in
[roadmap.md](roadmap.md). The full-screen interface is in progress: design in
[design/2026-09-08-full-screen-interface.md](design/2026-09-08-full-screen-interface.md),
plan beside it.

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

- **security**, the parts that are actually weak, decided after the questions
  were answered (the tunnel is end to end and forward secret, and a compromised
  machine is beyond any layer; a second encryption layer over tailcat buys
  nothing and was rejected — see [decisions.md](decisions.md)):
  - the address is the secret, and the channel people send it over is the
    weakest link: shorten what has to travel, or verify the key after a first
    call so a leaked address cannot be quietly replayed as somebody else
  - `key.json` at rest: `0600` today; a passphrase would keep a copied file
    useless, which is the one place a second layer belongs
  - releases: checksums exist, signatures do not; sign the release and the
    binaries, and put `govulncheck` in CI so the transport's advisories are
    seen when they land
  - what already holds and must keep holding: the tunnel authenticates, not
    the nick; everything from the network goes through `session/sanitize.go`;
    a file name goes through `filepath.Base` and is renamed into place only
    after its digest matches; a frame is capped at 1 MiB
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
