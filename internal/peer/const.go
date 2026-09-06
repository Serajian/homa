package peer

import "time"

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
