package peer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/tailscale/tailcat"
	"tailscale.com/tailcfg"
)

// Relay is where this machine meets the world: the region frozen in the
// identity, named from the DERP map, and whether the relay has us.
type Relay struct {
	Code      string // "fra"
	Name      string // "Frankfurt"
	Connected bool
}

// Path is how one conversation travels. Latency is zero when it is not
// known: only the side that dialed can ping. The far side's IP is
// deliberately not here; direct is all a screen needs to say.
type Path struct {
	Direct  bool
	Relay   string // region code when not direct
	Latency time.Duration
	Rx, Tx  int64
}

// ErrPathUnknown is Probe on a connection whose path the transport has not
// reported yet. It is not a failure of the conversation.
var ErrPathUnknown = errors.New("peer: the path is not known yet")

// Relay reports the relay this listener sits behind. The name comes from
// the DERP map tailcat fetched at start, which its in-process cache still
// holds, so no request is made for it; a map that cannot be had leaves
// the name empty and the code stands alone.
func (l *Listener) Relay() Relay {
	r := Relay{}
	if st := l.srv.Status(); st != nil && st.Self != nil {
		r.Code = st.Self.Relay
		r.Connected = r.Code != ""
	}
	if r.Code == "" {
		return r
	}
	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()
	if dm, err := tailcat.FetchDERPMap(ctx); err == nil {
		r.Name = regionName(dm, r.Code)
	}
	return r
}

func regionName(dm *tailcfg.DERPMap, code string) string {
	for _, region := range dm.Regions {
		if region != nil && region.RegionCode == code {
			return region.RegionName
		}
	}
	return ""
}

// Probe reports how conn travels. A dialed connection asks its client,
// which can ping; an accepted one reads the server's status table, which
// may not know the peer yet, in which case ErrPathUnknown.
func Probe(ctx context.Context, conn net.Conn) (Path, error) {
	ctx, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()

	switch c := conn.(type) {
	case *dialedConn:
		res, err := c.client.DiscoPing(ctx)
		if err != nil {
			return Path{}, fmt.Errorf("peer: probing the path: %w", err)
		}
		p := Path{
			Direct:  res.Endpoint != "",
			Relay:   res.DERPRegionCode,
			Latency: time.Duration(res.LatencySeconds * float64(time.Second)),
		}
		if ps := peerStatus(c.client, conn); ps != nil {
			p.Rx, p.Tx = ps.rx, ps.tx
		}
		return p, nil

	case *heldConn:
		if c.listener == nil {
			return Path{}, ErrPathUnknown
		}
		ps := serverPeerStatus(c.listener.srv, conn)
		if ps == nil {
			return Path{}, ErrPathUnknown
		}
		return Path{Direct: ps.direct, Relay: ps.relay, Rx: ps.rx, Tx: ps.tx}, nil
	}
	return Path{}, errors.New("peer: not a homa connection")
}

// stat is the few things read out of a status table entry.
type stat struct {
	direct bool
	relay  string
	rx, tx int64
}

// serverPeerStatus finds conn's peer in the server's status table by its
// tunnel address, the way lookupKey does.
func serverPeerStatus(srv *tailcat.Server, conn net.Conn) *stat {
	ap, err := netip.ParseAddrPort(conn.RemoteAddr().String())
	if err != nil {
		return nil
	}
	st := srv.Status()
	if st == nil {
		return nil
	}
	for _, p := range st.Peer {
		if p == nil {
			continue
		}
		for _, ip := range p.TailscaleIPs {
			if ip == ap.Addr() {
				return &stat{direct: p.CurAddr != "", relay: p.Relay, rx: p.RxBytes, tx: p.TxBytes}
			}
		}
	}
	return nil
}

// peerStatus is the dialing side's view of the same table, if the client
// exposes one; today it does not, and bytes stay zero there.
func peerStatus(_ *tailcat.Client, _ net.Conn) *stat { return nil }

// Fingerprint is the far side of conn, shortened the way a book records
// it: the whole key, when the connection was dialed and carries one, cut
// to the prefix; otherwise the prefix the tunnel address gives.
func Fingerprint(conn net.Conn) string {
	if k := RemoteKey(conn); k != "" {
		return shortKey(k)
	}
	return RemoteKeyPrefix(conn)
}

func shortKey(k string) string {
	k = strings.TrimPrefix(k, keyMark)
	if len(k) > 2*addrKeyBytes {
		k = k[:2*addrKeyBytes]
	}
	return keyMark + k
}

// KeyPrefix is the start of this machine's public key, in the form a peer's
// RemoteKeyPrefix gives for it: what a contact's book records about us.
func (i *Identity) KeyPrefix() string { return shortKey(i.PublicKey()) }
