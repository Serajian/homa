<p align="center">
  <img src="./docs/assets/hero.svg" width="100%"
       alt="homa, a peer-to-peer terminal chat. Two terminals side by side showing one real conversation: sara calls mohsen and waits, mohsen is asked whether to take the call and accepts, and the two exchange messages.">
</p>

**A peer-to-peer terminal chat.** Two machines exchange one address, then talk
directly: WireGuard between them, through NAT, on top of
[tailcat](https://github.com/tailscale/tailcat) — Tailscale's data plane with
the control plane taken out. A relay helps the two find each other and carries
traffic when no direct path can be made. There is no account, nothing in the
middle keeps your messages, and neither side is "the server": both listen and
either can call.

```sh
go install github.com/Serajian/homa/cmd/homa@latest
```

Run `homa` on both machines. The first run asks two questions and prints your
address. Give it to the other person, press `n` to save theirs, and call.

## What it is

- **A chat between exactly two people.** Rooms are version 3.
- **A file transfer**, in both directions, with a digest checked at the end.
- **A terminal program**, line-based today. A full-screen interface, with line
  editing and history, is version 2.

## What it is not

- Not a group chat, not a social network, not a bot platform.
- Not anonymous. Your peer sees a network path to you, as they would on a call.
- Not a place anything is stored. Nothing is kept but your settings, your
  address book, and your key — all on your own disk.

## A real session

Verbatim, from the side that placed the call:

```
What now?
  1  call alice

  n  add a contact
  b  contacts: rename, forget, call
  a  show my address

  s  settings
  c  clear the screen
  h  help

  r  start over: forget everything
  q  quit homa

> 
1
  calling alice...
  waiting for alice to answer... 59s  (Enter to give up)

  talking to alice  ·  they call themselves "alice"
  /help commands  ·  /quit leave  ·  files go to ~/homa-files

[me] salam
[alice] khoobam, to chetori?
[me] /send ~/notes.md
  offering /tmp/bob/notes.md, waiting for them to accept...
  sending: 100%
  sent.
```

The other side is asked before any of that happens, and sees the file arrive:

```
  ~bob is calling (expires in 1m0s).

> take the call from ~bob? [59s] [y/N]: y
  connected to ~bob

  talking to ~bob  ·  the name is theirs; they are not in your contacts
  /help commands  ·  /quit leave  ·  files go to ~/homa-files

[~bob] salam
[me] khoobam, to chetori?

  ~bob offers notes.md (11 B)  ·  y to accept, n to reject
[me] y
  receiving notes.md  ·  100%
  notes.md saved to /tmp/alice/homa-files/notes.md
```

Having somebody's address is not the same as being welcome to talk to them, so
nothing is put through until they say yes. Refusing is the default, and a call
nobody answers is hung up on inside a minute with both sides told why.

## In a conversation

| Command | What it does |
| --- | --- |
| `/files [dir]` | list a directory, numbered |
| `/files <n>` | list one from the last listing, `..` included |
| `/send <path>` | offer a file |
| `/send <n>` | offer one from the last listing |
| `/accept` | take the file being offered, or just `y` |
| `/reject` | refuse it, or just `n` |
| `/who` | who you are talking to |
| `/clear` | wipe the screen |
| `/help` | this list, or just `/` |
| `/quit` | leave the conversation, not homa |

Anything not starting with `/` is a message.

Typing a path exactly right, with no completion and nothing to look at, is not
a thing to ask of somebody mid-conversation. `/files` shows a directory and
`/send` takes a number out of it:

```
[me] /files ~/Downloads
  /Users/mohsen/Downloads
    1) ../                   dir
    2) archive/              dir
    3) gozaresh nahayi.pdf   4.2 KB
    4) poster.png            1.1 MB
[me] /send 3
```

Numbers work for walking as well as sending: `/files 2` goes into `archive`,
`/files 1` comes back out. A name is resolved against the directory you are
looking at rather than against wherever homa was started.

A file is never written without you accepting it, and an accepted file never
overwrites one already there: `poster.png` becomes `poster (2).png`. Progress is
reported every ten percent, so a large transfer says where it has got to and a
small one prints once.

## Who is calling

A peer announces a name during the handshake, but a name is just text they
typed. What identifies them is the key underneath the tunnel, which the
WireGuard handshake proved. So homa decides what to call somebody by key, never
by what they say:

- **a name from your address book**, when the key matches one. The first time
  you reach a contact their key is recorded, so their next call arrives under
  the name you gave them. Renaming them keeps that key.
- **`~` and the name they announced**, when it matches nothing. The `~` is what
  keeps the two apart: contact names never carry it, so a caller who names
  themselves `BB` shows up as `~BB` and cannot pass for the `BB` you saved.

## Your address is a secret

Treat it like a password. Whoever has it can call you, and it carries the
pre-shared key that guards your tunnel. Send it over a channel you already
trust.

Everything homa keeps lives in one directory — `~/.config/homa` on Linux,
`~/Library/Application Support/homa` on macOS, honoring `XDG_CONFIG_HOME` — as
`0600` files inside a `0700` directory:

| File | Holds |
| --- | --- |
| `key.json` | your identity and pre-shared key |
| `config.json` | display name, download directory |
| `contacts.json` | the names you gave people, and their addresses |

Deleting `key.json` gives you a new identity and a new address, and everyone who
saved the old one can no longer reach you. `r` at the menu does all three at
once, after making you type the word `reset`.

## Install

Requires Go 1.27.1 or newer. Packages for `brew` and `apt` are version 1 work
and not built yet.

```sh
go install github.com/Serajian/homa/cmd/homa@latest
```

From a clone:

```sh
git clone https://github.com/Serajian/homa.git
cd homa
make build          # ./build/homa
make install        # into $(go env GOPATH)/bin
make build-linux    # ./build/homa-linux-amd64
```

```
homa [flags]

  -log <path>   write diagnostics to a file
  -debug        write diagnostics to stderr, at debug level
  -no-color     never write color, even to a terminal (setting NO_COLOR does the same)
  -version      print the version and exit
```

Diagnostics go nowhere by default: homa draws a terminal interface, and a log
line landing in the middle of a conversation would scramble it.

## How it is built

Dependencies point one way, and two rules hold the shape: `internal/peer` is the
only package that imports tailcat, and `internal/session` knows nothing about
terminals while `internal/ui` knows nothing about wire formats.

| | |
| --- | --- |
| [docs/overview.md](docs/overview.md) | what homa is, where it came from, the prior art it replaces |
| [docs/architecture.md](docs/architecture.md) | package layout, dependency direction, startup and shutdown |
| [docs/protocol.md](docs/protocol.md) | the frame format, every message type, a transfer end to end |
| [docs/decisions.md](docs/decisions.md) | why things are the way they are, including every security choice |
| [docs/status.md](docs/status.md) | what works today and what is still rough |
| [docs/roadmap.md](docs/roadmap.md) | versions 2 and 3 |
| [docs/conventions.md](docs/conventions.md) | code style, and the reasons behind the odd ones |
| [docs/development.md](docs/development.md) | make targets, testing across two machines |

## Development

```sh
make            # every target, with descriptions
make at-first   # dev tools, deps, git hooks
make test       # hermetic, with the race detector
make test-live  # whole processes over the real transport
make lint
make run
```

`make test` needs no network and takes seconds. `make test-live` starts real
homa processes that reach each other through a relay, and is behind the `live`
build tag so the ordinary run stays quick.

## Roadmap

- [ ] **Version 1** two people, text, files, contacts, a line-based interface
      worth looking at, and installation through brew and apt. Most of it works;
      what is left is in [docs/todo.md](docs/todo.md)
- [ ] **Version 2** a full-screen interface, which brings line editing, history
      and tab completion with it, plus an Android build. The lower three
      packages are already free of any terminal assumption
- [ ] **Version 3** rooms: one host, several guests, join requests. It breaks
      the two-equal-peers model everything else rests on, which is why it is
      last

## Name

The Homa, the bird of Persian myth that never lands.

## License

MIT. See [LICENSE](LICENSE).
