package peer

import (
	"fmt"

	"tailscale.com/wgengine/filter"
)

// servedPorts restricts the packet filter to homa's own port, so traffic to
// any other port is dropped before it ever reaches OnTCP. Defense in depth:
// OnTCP already refuses them, this stops them one layer earlier.
func servedPorts() []filter.PortRange {
	return []filter.PortRange{{First: Port, Last: Port}}
}

// tailcatLogf routes the transport's chatter into our own logger at Debug
// level, so it stays out of sight unless someone is debugging.
func tailcatLogf(format string, args ...any) {
	logger.Debug(fmt.Sprintf(format, args...))
}
