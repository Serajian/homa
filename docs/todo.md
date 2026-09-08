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

## 3. Security

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
