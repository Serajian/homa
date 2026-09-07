package peer

import (
	"encoding/hex"
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
// owns it, using the server's status table.
//
// On the accepting side that table is empty. It was measured: on a server
// that had just taken a call, Status().Peer held no entries at all and
// Status().Self carried no addresses either. So this returns nothing there,
// and RemoteKeyPrefix is what names an incoming caller.
//
// It is kept because it is the right answer whenever the table is
// populated, and it costs a map walk over nothing. Note before relying on
// it that it returns a node key, while a dialed connection's key comes from
// the address as a ServerPublic: whether those are the same string for the
// same peer has not been established.
func lookupKey(srv *tailcat.Server, conn net.Conn) string {
	ap, err := netip.ParseAddrPort(conn.RemoteAddr().String())
	if err != nil {
		lg.Debug("unparsable remote address", "addr", conn.RemoteAddr())
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

	lg.Debug("no key found for connection", "addr", ap.Addr())
	return ""
}

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

// RemoteKeyPrefix returns the start of the remote peer's key, read out of
// the tunnel address the connection carries. It is empty when the address
// is not one of tailcat's.
//
// This exists because RemoteKey has nothing to work with on the accepting
// side: the address is all that arrives with a call. Ten bytes is not an
// identity, but it is enough to pick a contact out of an address book,
// which is all a label needs.
//
// Be careful what this is taken to mean. A match says "consistent with
// being that contact", not "cryptographically is that contact". What
// authenticates a peer is the tunnel: WireGuard completed a handshake with
// whoever holds the private half of that key, and no name in an address
// book adds to or subtracts from that. This only chooses what to call them
// on screen.
func RemoteKeyPrefix(conn net.Conn) string {
	ap, err := netip.ParseAddrPort(conn.RemoteAddr().String())
	if err != nil {
		lg.Debug("unparsable remote address", "addr", conn.RemoteAddr())
		return ""
	}

	addr := ap.Addr()
	if !addrPrefix.Contains(addr) {
		lg.Debug("remote address is not a tunnel address", "addr", addr)
		return ""
	}

	b := addr.As16()
	return keyMark + hex.EncodeToString(b[len(b)-addrKeyBytes:])
}
