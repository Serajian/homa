package peer

import (
	"net"
	"strings"
	"testing"

	"github.com/tailscale/tailcat"
	"tailscale.com/tailcfg"
)

// newTestIdentity is an identity that never touched the network: a fresh
// key with a made-up region.
func newTestIdentity(t *testing.T) *Identity {
	t.Helper()
	pk := tailcat.NewPrivateKey()
	pk.Public.RegionID = 1
	return &Identity{pk: pk}
}

func TestRegionNameComesFromTheMap(t *testing.T) {
	t.Parallel()

	dm := &tailcfg.DERPMap{Regions: map[tailcfg.DERPRegionID]*tailcfg.DERPRegion{
		1: {RegionCode: "nyc", RegionName: "New York City"},
		4: {RegionCode: "fra", RegionName: "Frankfurt"},
	}}
	if got := regionName(dm, "fra"); got != "Frankfurt" {
		t.Errorf("fra: %q", got)
	}
	if got := regionName(dm, "xyz"); got != "" {
		t.Errorf("unknown code named %q", got)
	}
}

// A loopback connection is not homa's; Probe says so rather than guessing.
func TestProbeRefusesAConnectionThatIsNotHomas(t *testing.T) {
	t.Parallel()

	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	if _, err := Probe(t.Context(), a); err == nil ||
		!strings.Contains(err.Error(), "not a homa connection") {
		t.Errorf("err = %v", err)
	}
}

// The key prefix is the form RemoteKeyPrefix gives for a peer: the mark
// and the first addrKeyBytes of the key.
func TestKeyPrefixIsWhatAContactRecords(t *testing.T) {
	t.Parallel()

	id := newTestIdentity(t)
	full := id.PublicKey()
	got := id.KeyPrefix()
	if !strings.HasPrefix(got, keyMark) || len(got) != len(keyMark)+2*addrKeyBytes {
		t.Fatalf("prefix %q", got)
	}
	if !strings.HasPrefix(full, got) {
		t.Errorf("%q is not the start of %q", got, full)
	}
}
