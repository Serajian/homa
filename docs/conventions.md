# Conventions

- **Constants** live in each package's `const.go`. Enum values stay beside their
  type, so adding a frame type touches one file rather than two.
- **The logger variable is `lg`, never `logger`.** Tools that process one file
  at a time (`goimports`, `golines`, editor auto-import) cannot see that a
  package-level variable is declared in a sibling file. A variable named
  `logger` looks like a package selector to them, and they add an import for an
  unrelated package that happens to be called `logger`. This cost an afternoon.
  Do not rename it back. See [traps.md](traps.md).
- **Logging is off by default.** `logx` discards until `cmd/homa` says
  otherwise, because a log line landing mid-conversation would scramble the
  interface. Use `Debug` for detail, `Info` for things that happen once, `Warn`
  for something unexpected that was handled. Do not log on a hot path:
  `proto` deliberately logs nothing, since a large file is twenty thousand
  frames.
- **Errors wrap with context** and are prefixed by package: `peer: dialing:
  ...`. The UI strips that prefix before showing an error to a person.
- **Never `return x, nil` while holding a non-nil error.** `nilerr` catches it,
  and it is usually a bug being planted.
- **Files are split by responsibility**, not by size. When a file grows a second
  reason to change, split it.
