# What comes next

Work is grouped by version. Version 1 is the first release and its items live in
[todo.md](todo.md); this file is what comes after it, in the order it comes.

Rooms used to be next and are now last. They break the two-equal-peers model
that everything else rests on, and they want the tests from version 1 already in
place before they add concurrency to a program that has none.

## Version 2: a full-screen interface

Using bubbletea and lipgloss, the same tools chat-tails uses.

- a chat pane, a separate input line, a contact list
- this removes both warts version 1 lives with, because input stops being a
  line the terminal owns until Enter
- only `internal/ui` changes. If anything below has to change, something has
  leaked, and that is the bug to fix first

## Version 2: Android

- `gomobile bind` over the lower packages, a Kotlin and Compose interface
- tailcat needs no VPN permission because everything is userspace, which is the
  reason this is even plausible
- the hard parts are the foreground service for staying connected and battery
  behavior, not the networking
- keep `proto`, `peer` and `session` free of any desktop assumption: no direct
  terminal reads, no assumptions about file paths. Everything is injected

## Version 3: rooms

One person hosts a room; several guests join. This is the first thing that
breaks the "two equal peers" model, so it needs care.

- a host mode that accepts several connections instead of one
- a join request a host approves or refuses, reusing the parked-call idea
- broadcast: a message from one guest reaches all the others
- rate limiting per guest, which chat-tails does and homa currently does not
- the protocol gains a sender field on TEXT, or a per-guest id assigned at join

The framing does not change. `proto` should need only new message types.

## Not placed in any version

- a short human-readable invite code that resolves to an address, in the spirit
  of croc, so nobody has to paste two hundred characters. This is the single
  biggest usability win still on the table
- self-hosted DERP, so a group can run homa without touching Tailscale's relays

Packaging used to be here. It is item 9 of version 1 now: a release nobody can
install is not a release.
