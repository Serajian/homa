package peer

import (
	"net/netip"
	"time"
)

// Port is the TCP port homa speaks on inside the tunnel. It is part of the
// protocol, not a setting: both peers must agree on it. Being inside the
// tunnel, it collides with nothing on the host.
const Port uint16 = 7777

// File layout of the config directory.
const (
	keyFile = "key.json"
)

// regionPickTimeout bounds the one-time latency probe across DERP relays.
// It runs once, on first launch, so a generous bound is fine.
const regionPickTimeout = 30 * time.Second

// addrKeyBytes is how much of a peer's key its tunnel address carries.
//
// tailcat addresses run fd7a:115c:a1e0 followed by the start of the key, so
// ten of the sixteen bytes are the key's own. Measured rather than assumed:
// a contact stored as
//
//	nodekey:cbbc522530057d33c1bf5de665b8be09690c90af37d8a19d1919e59a096ebd06
//
// called in from
//
//	fd7a:115c:a1e0:cbbc:5225:3005:7d33:c1bf
const addrKeyBytes = 10

// keyMark is the marker a tailscale key string carries. RemoteKey returns
// keys wearing it, so a prefix built here has to wear it too or nothing
// would ever compare equal.
const keyMark = "nodekey:"

// addrPrefix is the range tailcat hands its tunnel addresses out of.
var addrPrefix = netip.MustParsePrefix("fd7a:115c:a1e0::/48")

// relayNameTimeout bounds the lookup of a relay region's name. The DERP
// map is in the transport's process cache after start, so this is a cache
// read; the bound is short because a stale cache must not stall a screen,
// and the transport falls back to the stored map when a revalidation runs
// out of time.
const relayNameTimeout = 500 * time.Millisecond

// statusTimeout bounds a look at the connection's path or the relay's name:
// a ping through the relay, or a DERP map read from cache. A person is
// waiting on a screen for the answer.
const statusTimeout = 3 * time.Second
