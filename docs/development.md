# Working on this project

```sh
make            # every target, with descriptions
make at-first   # dev tools, deps, git hooks
make check      # what CI would run
make lint
make build
make run
make doc        # the public API of every internal package
make test       # hermetic, with the race detector: no network, no relay
make test-live  # whole processes over the real transport; needs the network
```

Testing across two machines: `make build-linux`, copy the binary to the second
machine, run `homa` on both.

To watch what homa is doing without disturbing the interface:

```sh
homa -log homa.log -debug
tail -f homa.log
```

If the toolchain itself misbehaves, check [traps.md](traps.md) first.

## Releasing

A tag that starts with `v` is a release. `.github/workflows/release.yml` runs GoReleaser
on it: binaries for macOS and Linux on amd64 and arm64, a `.deb` for each, checksums, a
GitHub release, and the Homebrew cask pushed to `Serajian/homebrew-homa`. The tag reaches
`homa -version` through `-X main.version`; a local `make build` stamps what `git describe`
says instead, so a binary from a working tree names its commit.

`make snapshot` runs the whole pipeline locally without a tag and without publishing, into
`./dist`, which is where to look before tagging. The tap push needs a token of its own —
the `HOMEBREW_TAP_GITHUB_TOKEN` repository secret — because the token Actions provides can
only write to this repository.

To release: `git tag v0.1.0 && git push origin v0.1.0`, then watch the Actions run.

Publishing the release also runs `.github/workflows/apt.yml`, which takes the `.deb`s
from the release, adds them to the apt repository on the `gh-pages` branch
(`.github/apt/build.sh` builds the `pool/` and `dists/` tree and signs `Release`), and
GitHub Pages serves it at https://serajian.github.io/homa. It signs with the
`APT_GPG_PRIVATE_KEY` secret, a key made for this alone. `workflow_dispatch` republishes an
existing release, which is how the repository was first populated.
