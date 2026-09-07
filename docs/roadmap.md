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

## Version 2: commands offered as they are typed

Typing `/` should show what can be typed after it, and typing `/se` should
narrow that to `/send`. Neither is possible today for the same reason tab
completion is not: the terminal collects a whole line and hands it over on
Enter, so homa never sees a keystroke and cannot answer one.

Version 1 does what can be done without that — a lone `/` lists the commands
when it is entered, and an unrecognised one lists them rather than saying to
go and look. This item is the live version, and it belongs to the full-screen
interface rather than beside it: once that owns the input line it has every
keystroke already, and offering commands is a small thing on top rather than a
reason to go into raw mode.

Worth deciding then: whether it filters as more is typed, whether it takes
arrows and Enter to pick one, and whether the same thing offers contacts after
`/send` — the listing from `/files` is already a numbered set of candidates.

## Version 2: `/store`

Save the conversation you have been having, by typing `/store` at any point in
it.

**Working at any point is the whole of the problem.** homa prints a message and
forgets it: `chatHandler` hands each line to the interface and keeps nothing.
For `/store` to save what was said before it was typed, every conversation has
to be kept as it happens, whether or not it is ever saved.

That is a change in what homa is, not a feature bolted to the side of it, and it
should be decided as one. Today the program remembers nothing: no history, no
log of who said what, nothing on disk but settings, contacts and a key. After
this it holds every conversation in memory for as long as the conversation
lasts, and writes one to disk on request. Both are new.

**What has to be settled first**

- **whether it is always on.** Keeping every conversation so that one can be
  saved is the cost of the feature working the way it was asked for. A setting
  that turns the keeping off is the honest alternative to deciding for people,
  and it has to be off-by-default or on-by-default, which is the same kind of
  choice as the ringing in the item above
- **how much is kept.** An unbounded buffer is a leak on a long conversation.
  A cap by lines or by bytes, and what happens when it is reached: drop the
  oldest, or stop keeping and say so. Silently losing the start of what somebody
  is about to save is the one answer that is wrong
- **what a stored conversation contains.** The messages, certainly, in the form
  they were shown, with the labels that say who was speaking and the `~` on a
  name a peer chose. Whether it also carries the notices — files offered, a
  peer leaving — and whether it carries times, which are not kept today either

**What must stay true**

Store the sanitized text, never what arrived on the wire. `session/sanitize.go`
is where network content stops being dangerous, and a transcript is a file
somebody will open in something other than a terminal.

A stored conversation is plaintext on disk, made from something that was
encrypted end to end. That is the person's decision to make, but the interface
should say it plainly at the moment they make it, once, rather than in
documentation nobody reads.

**Where it fits**

The full-screen interface is in this same version and will own a scrollback
buffer of its own. That is the same data, and building the two without noticing
would mean keeping every conversation twice.

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
