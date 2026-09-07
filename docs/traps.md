# Traps already hit

Recorded so nobody pays for them twice.

**`golines` invents imports.** It formats each file independently, so a
package-level `logger` variable looked like a missing import and it added
`bytedance/gopkg/util/logger` to one file and `gorm.io/gorm/logger` to another.
Renaming the variable to `lg` fixed it at the root. Editors do the same thing
for the same reason.

**`proxy.golang.org` is unreliable from some networks.** If `go get` fails with
403 or a TLS timeout, set a mirror: `go env -w GOPROXY=https://goproxy.io,direct`.
The Go toolchain itself is fetched the same way, so this also fixes
"cannot download go1.x".

**`golangci-lint` must be built with a Go at least as new as `go.mod` targets.**
A Homebrew binary built against an older Go refuses to run. `go install` it
instead, so it is compiled with the toolchain in use.

**A Homebrew Go upgrade breaks the shell's command cache.** `rehash` in zsh, or
open a new terminal.
