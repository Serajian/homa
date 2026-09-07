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

## Version 2: a sound when a call arrives

homa can be left running in a window nobody is looking at, and today the only
sign of a call is a line of text on a screen that is not in front of anyone.

**The terminal bell is the mechanism.** `\a`, one byte, written to a terminal
homa already owns. No dependency, no audio file to ship, no CGO — and CGO is
the reason nothing heavier is worth it: it would break `make build-linux` and
complicate the `gomobile` work in this same version. Playing a real sound means
either shelling out to `afplay` or `paplay`, which is a dependency on whatever
the machine happens to have, or linking an audio library, which is a dependency
on a C toolchain. Neither buys enough.

**Where it rings**, and no more than that:

- a call arrives. This is the one that matters: the screen is not being watched
- your call is taken, so the person who dialled can look away while it connects

Not on every message. A conversation that beeps is a conversation people mute,
and then it does not ring for a call either.

**A setting decides.** `config.Config` gains a field and the settings screen a
question. Not the first-run questions: those ask only what homa cannot guess,
and this it can. Whether it defaults to ringing or to silence is a real choice —
a program that makes noise on first use without asking is a program people
distrust — and it should be made deliberately.

**What must stay true.** The bell is a control character, so it lives behind a
named constant in `internal/ui/const.go` with the others, and is never built
from anything that arrived over the network. It is also best-effort: many
terminals turn it into a visual flash and some ignore it entirely, so nothing
may depend on it having been heard. And it is written unconditionally like
`clearLine`, so redirected output gets the byte — the same wart, to be settled
in the same place when terminal detection is added.

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
