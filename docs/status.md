# Current state: Phase 1 is complete

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

Known warts, all fixed by Phase 3 (see [roadmap.md](roadmap.md)):

- a message arriving while you type is printed over your half-finished line
- after the other person leaves, a keypress is needed to return to the menu
- a message typed but not yet sent when the peer leaves is dropped silently

Not yet built: tests. There is no test file in the repository. That is the
largest gap in the project and should be closed before Phase 2 adds concurrency.
