# Working agreement

These are the rules the project has been built under. Keep to them.

1. **Propose before acting.** Nothing gets created, renamed, or restructured
   without being described first and agreed to. That includes files, packages,
   dependencies, and "helpful" extras.
2. **One file at a time.** Deliver a file, get it reviewed, then move on. Do not
   dump five files and hope.
3. **Explain decisions in the code.** Comments say *why*, not *what*. If a line
   exists because of a trap, name the trap. Future readers will otherwise
   "simplify" it back into the bug.
4. **Nothing not asked for.** No speculative features, no extra columns, no
   convenience wrappers that add a name and no meaning.
5. **`make lint` must pass before a commit**, and the git hooks enforce it.
6. **Commit messages explain the reasoning**, not just the change. Look at
   `git log` for the established shape: a subject line, then what and why, with
   the decisions worth remembering spelled out.
