# Current state: version 2 in progress

v0.1.0 is out with everything version 1 set out to do. Version 2's first item,
the full-screen interface, is built; what is left of version 2 is in
[todo.md](todo.md). Working today:

- first-run setup, saved settings, a saved identity with a stable address
- an address book: add, list, call by name, keys learned on first contact, and
  a screen of its own for renaming, forgetting and calling. A rename keeps the
  key, so somebody renamed still arrives under the name you gave them
- listening and dialing at the same time, an incoming call greeted as it
  arrives and put through only when you agree to take it, a refused or busy
  caller told why, and a call nobody answers hung up on inside a minute with
  both sides told
- a caller who waits to be let in rather than being told they are talking
  before anyone agreed, can give up on the wait with a keypress without
  leaving homa, and still works against a peer speaking version 1 of the
  protocol
- a caller you have saved shown by your name for them, one you have not shown
  by the name they chose, marked so the two cannot be confused
- text chat with slash commands, and a way to wipe the screen from either
  the menu or a conversation
- file transfer in both directions, with confirmation, progress, a digest check,
  no overwriting, and cleanup of partial files
- a file picked by listing a directory and choosing a number, rather than by
  typing a path exactly right with no help
- a reset that throws away the identity, address book and settings, guarded by
  having to type the word rather than a letter
- a help page at the menu, saying what the menu cannot say for itself: which
  quit leaves the program and which leaves a conversation, what the marks on a
  name mean, and that the address is a secret
- graceful shutdown from either screen
- a welcome screen: the logo drawn in block characters and color when the
  output is a terminal that can show them, one plain line when it is a pipe, a
  file, a narrow terminal or a locale without UTF-8
- an interface with one meaning per color — cream is you, green is them, grey
  is homa, the terminal's yellow is a warning — that reads the same with color
  off, menus in groups with the people first, a status line at the top of every
  conversation, a next step under every warning, and no escape sequence ever
  written to anything that is not a terminal. The logo's 24-bit palette on a
  terminal that declares it, the sixteen ANSI colors everywhere else, and "you"
  in the terminal's own foreground so a light theme reads too. `NO_COLOR`,
  `TERM=dumb` and `-no-color` turn color off
- documentation with the four diagrams that matter drawn in mermaid, which
  GitHub renders, and the call-setup one also as a pannable page
- `make lint` clean, hooks wired, Makefile covering build, format, lint, test,
  cross-compile
- a release, v0.1.0: `brew install --cask Serajian/homa/homa` on macOS, a `.deb`
  on the release page for Debian and Ubuntu, archives for the rest, and
  `homa -version` naming the tag — all of it built by GoReleaser from a tag,
  the cask pushed to the tap by the same run
- an apt repository at https://serajian.github.io/homa, signed, rebuilt from the
  release's `.deb`s by a workflow on every release and served by GitHub Pages
  from the `gh-pages` branch, so `apt install homa` works on Debian and Ubuntu

- a full-screen interface: the screen drawn whole, so a message arriving while
  you type lands in the pane and never on your line, and a line not yet sent
  when the far side leaves stays where it was; up and down walk what you sent;
  PgUp/PgDn scroll what was said. Version 1's two warts, and the arrow keys
  that did nothing, went with the line-based interface they came from

- commands offered as they are typed: a `/` shows every command in the row
  above the input, each letter narrows the row, left and right walk it, Tab or
  Enter take the one marked, and a word that is no command is warned about
  before Enter. `/help` and the hint read one table, so they cannot disagree

Tests: `make test` runs them, with the race detector, and they are hermetic —
no network, no relay, nothing outside a temporary directory. `make test-live`
runs the other kind: whole homa processes reaching each other through a relay,
behind the `live` build tag so the ordinary run stays quick.
