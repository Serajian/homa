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
make test-live  # whole processes on pseudo-terminals over the real transport; needs the network
```

Testing across two machines: `make build-linux`, copy the binary to the second
machine, run `homa` on both.

To watch what homa is doing without disturbing the interface:

```sh
homa -log homa.log -debug
tail -f homa.log
```

If the toolchain itself misbehaves, check [traps.md](traps.md) first.

## The card people see when they share the link

A repository link pasted into a chat or a timeline is unfurled into a card, and
by default GitHub draws that card from the owner's avatar and a few counts,
which says nothing about homa. `docs/assets/social-preview.png` replaces it: the
logo package's horizontal lockup on the ink background, the tagline, and one
line, at 1280 by 640, which is the size GitHub asks for.

It is set by hand, once, and stays set: **Settings → General → Social preview →
Edit → Upload an image**. There is no API for it — the repository object has no
such field — so it cannot be part of the release pipeline.

`docs/assets/social-preview.svg` is where it comes from; it takes the mark and
the wordmark straight out of `docs/assets/logo/homa-horisontal.svg`, so a change
to the logo reaches the card by re-rendering rather than by redrawing:

```sh
rsvg-convert -w 1280 -h 640 docs/assets/social-preview.svg -o docs/assets/social-preview.png
```

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

The release workflow ends by starting `.github/workflows/apt.yml` (a release made with
`GITHUB_TOKEN` raises no event other workflows see, so it is dispatched by name), which takes the `.deb`s
from the release, adds them to the apt repository on the `gh-pages` branch
(`.github/apt/build.sh` builds the `pool/` and `dists/` tree and signs `Release`), and
GitHub Pages serves it at https://serajian.github.io/homa. It signs with the
`APT_GPG_PRIVATE_KEY` secret, a key made for this alone. `workflow_dispatch` republishes an
existing release, which is how the repository was first populated.

## The live tests and the screen

`test/live` starts each homa on a pseudo-terminal and applies what it writes to
a small terminal grid (`screen.go`): cursor moves, tab stops, erasing, the
lot. The full-screen interface draws in place, so what a person sees is the
grid, not the byte stream, and every wait in those tests looks at the grid. A
failure prints the screen, the control sequences met, and keeps the raw bytes
in a file it names, which is how the grid itself was debugged.

The README's screens come from the same place: `HOMA_FRAMES=<dir> go test
-tags live -run 'TestACallIsAskedAboutAndPutThrough|TestAFileCrossesAndKeepsItsContents' ./test/live/`
writes each snapshot the two scenarios take into the directory, from homes
under `/tmp/<name>` so the paths on screen read as a person's would.
