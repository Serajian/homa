# Working on this project

```sh
make            # every target, with descriptions
make at-first   # dev tools, deps, git hooks
make check      # what CI would run
make lint
make build
make run
make doc        # the public API of every internal package
```

Testing across two machines: `make build-linux`, copy the binary to the second
machine, run `homa` on both.

To watch what homa is doing without disturbing the interface:

```sh
homa -log homa.log -debug
tail -f homa.log
```

If the toolchain itself misbehaves, check [traps.md](traps.md) first.
