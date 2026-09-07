package peer

import (
	"net"
	"strings"
	"testing"
)

// This package is the one that touches tailcat, so most of it needs a
// network, a relay and a real key. What can be tested without any of that is
// the part that decides who somebody is — and that is the part worth testing
// anyway.

// fakeConn is a net.Conn with nothing behind it, carrying only the remote
// address these functions read.
type fakeConn struct {
	net.Conn
	remote string
}

func (c fakeConn) RemoteAddr() net.Addr { return fakeAddr(c.remote) }

type fakeAddr string

func (fakeAddr) Network() string  { return "tcp" }
func (a fakeAddr) String() string { return string(a) }

// A connection this package did not hand out cannot claim to know its remote
// key. The interface behind RemoteKey is unexported for exactly this reason,
// and nothing outside peer can satisfy it.
func TestRemoteKeyIsEmptyForAConnectionFromElsewhere(t *testing.T) {
	t.Parallel()

	a, b := net.Pipe()
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })

	if got := RemoteKey(a); got != "" {
		t.Errorf("RemoteKey of an ordinary connection = %q, want empty", got)
	}
}

// The last ten bytes of a tunnel address are the first ten of the peer's key.
// That was measured rather than assumed: this contact
//
//	nodekey:cbbc522530057d33c1bf5de665b8be09690c90af37d8a19d1919e59a096ebd06
//
// called in from fd7a:115c:a1e0:cbbc:5225:3005:7d33:c1bf.
func TestRemoteKeyPrefixReadsTheKeyOutOfTheAddress(t *testing.T) {
	t.Parallel()

	conn := fakeConn{remote: "[fd7a:115c:a1e0:cbbc:5225:3005:7d33:c1bf]:42348"}

	got := RemoteKeyPrefix(conn)
	want := "nodekey:cbbc522530057d33c1bf"

	if got != want {
		t.Errorf("RemoteKeyPrefix = %q, want %q", got, want)
	}
}

// The prefix has to be spelled the way RemoteKey spells a key, or a
// comparison against a stored contact would never match however right it was.
func TestRemoteKeyPrefixIsAPrefixOfARealKey(t *testing.T) {
	t.Parallel()

	const stored = "nodekey:cbbc522530057d33c1bf5de665b8be09690c90af37d8a19d1919e59a096ebd06"
	conn := fakeConn{remote: "[fd7a:115c:a1e0:cbbc:5225:3005:7d33:c1bf]:1"}

	if prefix := RemoteKeyPrefix(conn); !strings.HasPrefix(stored, prefix) {
		t.Errorf("%q is not the start of %q", prefix, stored)
	}
}

// An address from outside tailcat's range says nothing about anybody's key,
// and guessing from one would put a name on a stranger.
func TestRemoteKeyPrefixRefusesAnythingButATunnelAddress(t *testing.T) {
	t.Parallel()

	for _, remote := range []string{
		"192.168.1.10:7777",
		"[2001:db8::1]:7777",
		"[fe80::1]:7777",
		"not an address at all",
		"",
	} {
		if got := RemoteKeyPrefix(fakeConn{remote: remote}); got != "" {
			t.Errorf("RemoteKeyPrefix(%q) = %q, want empty", remote, got)
		}
	}
}

func TestValidAddr(t *testing.T) {
	t.Parallel()

	for _, s := range []string{
		"",
		"   ",
		"not an address",
		"tcp-too-short",
		strings.Repeat("a", 300),
	} {
		if ValidAddr(s) {
			t.Errorf("ValidAddr(%q) said yes", truncate(s))
		}
	}
}

// An address is a secret and two hundred characters long, so an error about
// one must not spray it across a terminal or a log file.
func TestParseAddrDoesNotPutTheWholeAddressInItsError(t *testing.T) {
	t.Parallel()

	const long = "tcpGFwWCD2eoBWgizbbv24DyRq1Uny0AI4docW_zu0ibIxCTKPSGFrWCDwc3KCJOgu8xS9eRgffIlZV_sRd7qQOZbe802VuR1pL2NOT-A-REAL-ADDRESS"

	_, err := ParseAddr(long)
	if err == nil {
		t.Fatal("parsed something that is not an address")
	}
	if strings.Contains(err.Error(), long) {
		t.Errorf("the error carries the whole address: %v", err)
	}
	// Only the first few characters, enough to tell two addresses apart.
	// Nothing is asserted about the length of the whole message: it wraps
	// whatever the transport said, and that is not this package's to
	// promise.
	if !strings.Contains(err.Error(), long[:12]+"...") {
		t.Errorf("the error does not show the start of what was pasted: %v", err)
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	if got := truncate("short"); got != "short" {
		t.Errorf("truncate kept %q, want it whole", got)
	}
	if got := truncate(strings.Repeat("a", 50)); got != strings.Repeat("a", 12)+"..." {
		t.Errorf("truncate = %q", got)
	}
}

// The port is part of the protocol rather than a setting: both peers must
// agree on it, and it lives inside the tunnel so it collides with nothing.
func TestTheServedPortIsTheOneWeSpeakOn(t *testing.T) {
	t.Parallel()

	ranges := servedPorts()
	if len(ranges) == 0 {
		t.Fatal("nothing is served")
	}

	var found bool
	for _, r := range ranges {
		if Port >= r.First && Port <= r.Last {
			found = true
		}
	}
	if !found {
		t.Errorf("port %d is not in the served ranges %+v", Port, ranges)
	}
}

func TestKeyPathIsShowable(t *testing.T) {
	t.Parallel()

	// It goes inside messages to a person, so it must produce something
	// even when the config directory cannot be worked out.
	if got := KeyPath(); got == "" || !strings.Contains(got, "key.json") {
		t.Errorf("KeyPath = %q", got)
	}
}
