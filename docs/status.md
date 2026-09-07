# Current state: Phase 1 is complete

Working today:

- first-run setup, saved settings, a saved identity with a stable address
- an address book: add, list, call by name, keys learned on first contact
- listening and dialing at the same time, an incoming call greeted as it
  arrives and put through only when you agree to take it, a refused or busy
  caller told why
- a caller you have saved shown by your name for them, one you have not shown
  by the name they chose, marked so the two cannot be confused
- text chat with slash commands
- file transfer in both directions, with confirmation, progress, a digest check,
  no overwriting, and cleanup of partial files
- graceful shutdown from either screen
- `make lint` clean, hooks wired, Makefile covering build, format, lint, test,
  cross-compile

Known warts, fixed by Phase 3 (see [roadmap.md](roadmap.md)):

- a message arriving while you type is printed over your half-finished line
- a message typed but not yet sent when the peer leaves is dropped silently

Both are the same limitation: input is a line at a time, and the terminal owns
the line until Enter. A full-screen interface owns it instead.

Not yet built: tests. There is no test file in the repository. That is the
largest gap in the project and should be closed before Phase 2 adds concurrency.
