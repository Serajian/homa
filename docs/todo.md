# homa: outstanding work

Work is grouped by version, newest front first. Versions 1 and 2 are closed and
kept here only as markers; what is open starts at **Faults** and is worked from
there down.

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

---

# Version 1

Complete, and released as v0.1.0: two people, text, files, contacts, an
interface with a design, and installation through brew and apt. Nothing is
left here.

---

# Version 2

Complete, and released as v0.2.0: the full-screen interface, commands offered
as they are typed, and the bell. The interface's design and plan stay in
[design/](design/) for the record. Android was planned here and is now
unplaced, at the bottom. Nothing is left here.

# Next

What a review on 2026-09-09 turned up, less what has since been done. These are
small beside version 3's items and they belong to what is already built, so
they ship in the 0.2 line, in this order.

- **Call an address without saving it.** The menu can only call a contact, so a
  one-off call or a single file means inventing a contact and deleting it
  afterwards. Add a menu action that takes an address, dials it, and offers to
  save the person when the call ends. Files: `internal/ui` (a menu action, a
  form, the model), both READMEs. Done when an address pasted at the menu calls,
  and nothing reaches `contacts.json` unless the person asks for it

- **Refuse a caller by key, and a do-not-disturb.** An address is a bearer
  capability with no revocation short of a reset, which changes your address
  for everyone who has it; today somebody you refuse can call again
  immediately. Build a list of keys homa hangs up on, and a setting that
  refuses every call while it is on. Decide, and write down, whether a blocked
  caller is told they are blocked or told the same thing anybody hears when
  nobody is taking calls — the second says less about you. Files:
  `internal/config`, a blocked list beside `internal/contacts`,
  `internal/ui/calls.go`, `internal/ui/model.go`, `docs/decisions.md`. Done when
  a blocked key is hung up on without the screen ever lighting up, and the
  setting turns every call away

- **Resume an interrupted transfer.** A transfer that breaks starts from zero;
  chunks carry an id and no offset, and `FILE_ACCEPT` has no "start at". For a
  gigabyte on a line that drops, that is the difference between a feature and a
  frustration. Build an offset in the accept, a digest of what is already on
  disk so a resume cannot silently glue two different files together, and a
  partial file that survives on purpose rather than by accident. Files:
  `internal/proto/messages.go`, `internal/session/files.go`, `internal/ui`.
  Done when a transfer killed at half resumes on the next offer of the same
  file and the digest still matches at the end

# Version 3

Version 2 is closed; this is the open front. Detailed in
[roadmap.md](roadmap.md).

- **`/store`**, saving the conversation you have been having, typed at any point
  in it. Working at any point is the whole difficulty: it means homa keeps every
  conversation as it happens, whether or not it is ever asked to save one, and
  today homa remembers nothing. Moved here from version 2: it changes what homa
  keeps, which is a question for the version that settles security

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

Real items, written out like the rest so that whoever picks one up is not
starting from a sentence. They have no version yet because each is waiting on a
decision rather than on time.

- **A short invite code instead of two hundred characters.** An address is
  around two hundred characters and a secret, and its length is why people
  paste it into whatever channel is at hand rather than the one they trust.
  croc's word codes are the shape people expect. Before any code is written,
  one question has to be answered in [decisions.md](decisions.md), because it
  decides whether this is possible at all: **a short code cannot carry an
  address.** A tailcat address holds three independent 32-byte keys — the
  server's node key, its disco key and the pre-shared key — which is 96 bytes
  before the relay region and before any framing, or 128 characters of
  base64url at the theoretical best. No compression touches that: it is random
  key material. So a code short enough to read aloud has to be a *pointer*, and
  a pointer needs something to resolve it, which is a server, which homa does
  not have and has said it does not want. The options to weigh and write down:
  a rendezvous that holds an address for a few minutes and hands it to whoever
  quotes the code, which is a server and changes what homa is; the relay doing
  that job, which keeps the parts list the same but hands the relay operator
  the thing the pre-shared key was added to keep from them; or dropping the
  pre-shared key for short-lived invites, which weakens exactly the peer we
  chose not to trust. Files: `docs/decisions.md` first and alone. Done when the
  option is chosen and the reasoning is written down; only then does code
  follow

- **A relay of your own.** Every homa meets the world at one of Tailscale's
  DERP relays, whose rate limits and continued existence are not ours; if
  `tailcat.dev` stops serving a relay list, a new identity cannot pick a region
  at all. tailcat already supports the alternative: an address can carry a
  `tailcfg.DERPRegion` naming a host instead of a region number, and then the
  far side needs no setting and no flag, because the address itself says where
  to meet. What homa adds is one setting for the relay's host name, read when
  an identity is created, and the relay itself is Tailscale's `derper` on a
  machine with a name and a TLS certificate, which `derper` can get for itself.
  Two things to say plainly in the docs: the address grows, because the host
  name travels in it, and the relay is frozen into it like the region number
  is, so it must be a name that will outlive the address. A side benefit worth
  taking: the live tests could then run against a relay of their own instead of
  the network. Files: `internal/config` (the setting), `internal/peer`
  (`identity.go`, `region.go`), `internal/ui` (settings, first run), both
  READMEs, `docs/decisions.md`, `docs/development.md`. Done when two homas
  configured with a private relay talk with no traffic to Tailscale's, and an
  address made that way is dialled by a homa that was told nothing

- **Android.** `gomobile bind` over the lower packages and a Kotlin and Compose
  interface. What is known: `proto`, `peer`, `session`, `contacts`, `config`,
  `paths` and `logx` — and tailcat with them — compile for `GOOS=android` on
  arm64 and amd64 with `CGO_ENABLED=0`, so the transport is not the obstacle,
  and tailcat needs no VPN permission because everything is userspace. What was
  not tested is behaviour on a real network, which needs a device. It stays
  here by decision, not by oversight: the cost is not the networking but
  staying alive, which means a foreground service, a permanent notification,
  battery, and the vendors' own process killers; and the cheap alternative, an
  app that only works while it is open, is an app nobody can be called on.
  Until it is picked up, the constraint it puts on everything else holds:
  `proto`, `peer` and `session` stay free of any desktop assumption — no
  terminal read, no assumption about file paths, everything injected. Done, if
  it is ever begun, when `gomobile bind` produces an `.aar` and a phone and a
  desktop hold a conversation
