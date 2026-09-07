// Package live holds the tests that need the real transport: two whole homa
// processes, reaching each other through tailcat and a relay.
//
// They are behind the "live" build tag, so `make test` never runs them.
// Everything else in this repository is hermetic and quick, and it has to
// stay that way or people stop running it. These are the opposite: they
// need the network, they take the better part of a minute each, and they can
// fail because a relay was slow rather than because homa was wrong.
//
// Run them with `make test-live`. They exist because this is the one layer
// nothing else covers: every claim about what two instances do together was
// checked by hand until now, and by hand it stays checked only as long as
// somebody remembers to check it.
package live
