package peer

import (
	"net"
	"net/netip"

	"github.com/tailscale/tailcat"
)

// keyed is implemented by the connections this package hands out. It is
// unexported so only peer can satisfy it: a connection claiming to know its
// remote key has to have come from here.
type keyed interface {
	remoteKey() string
}

// RemoteKey returns the public key of whoever is on the other end of conn,
// or an empty string if it cannot be determined.
//
// This is the only trustworthy way to tell peers apart. The nick announced
// during a handshake is whatever the far side typed; this key is what the
// tunnel itself proved.
func RemoteKey(conn net.Conn) string {
	if k, ok := conn.(keyed); ok {
		return k.remoteKey()
	}
	return ""
}

// PublicKey is this machine's own key, in the same form RemoteKey returns.
// Give it to a peer who wants to restrict who may connect to them.
func (i *Identity) PublicKey() string {
	return i.pk.Public.ServerPublic.String()
}

// lookupKey maps a connection's tunnel address back to the peer key that
// owns it. tailcat hands out connections carrying a synthetic IPv6 address
// rather than a key, and the server's status table is what ties the two
// together.
func lookupKey(srv *tailcat.Server, conn net.Conn) string {
	ap, err := netip.ParseAddrPort(conn.RemoteAddr().String())
	if err != nil {
		logger.Debug("unparsable remote address", "addr", conn.RemoteAddr())
		return ""
	}

	status := srv.Status()
	if status == nil {
		return ""
	}

	for nodeKey, peer := range status.Peer {
		if peer == nil {
			continue
		}
		for _, ip := range peer.TailscaleIPs {
			if ip == ap.Addr() {
				return nodeKey.String()
			}
		}
	}

	logger.Debug("no key found for connection", "addr", ap.Addr())
	return ""
}
