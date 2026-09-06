package peer

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/tailscale/tailcat"
)

// dialedConn owns the client engine behind a dialed connection, so closing
// the connection also shuts the engine down. Without this the tunnel would
// stay up after a conversation ended.
type dialedConn struct {
	net.Conn
	client *tailcat.Client
}

func (d *dialedConn) Close() error {
	err := d.Conn.Close()
	if cerr := d.client.Close(); err == nil {
		err = cerr
	}
	return err
}

// Dial connects to the peer at addr, presenting id as our identity so the
// other side can recognize us by key rather than by a self-declared name.
//
// addr is the string the peer's Listener printed. It is a secret: it carries
// the pre-shared key that guards their tunnel.
//
// The caller owns the returned connection and must Close it. Give ctx a
// deadline: reaching a peer can take a while when a direct path has to be
// negotiated through the relay.
func Dial(ctx context.Context, id *Identity, addr string) (net.Conn, error) {
	a, err := ParseAddr(addr)
	if err != nil {
		return nil, err
	}

	logger.Info("dialing peer")

	c := tailcat.NewClient(a)
	c.Key = id.pk.Private
	c.Logf = tailcatLogf

	conn, err := c.DialTCPPort(ctx, Port)
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("peer: connecting: %w", err)
	}

	logger.Info("connected to peer")
	return &dialedConn{Conn: conn, client: c}, nil
}

// ParseAddr checks that s looks like a peer address, so a typo is caught the
// moment it is pasted rather than as a confusing failure later. The parsed
// form is unexported on purpose: callers outside this package handle
// addresses as opaque strings.
func ParseAddr(s string) (tailcat.Addr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("peer: empty address")
	}

	a := tailcat.Addr(s)
	if _, err := tailcat.ParseAddr(a); err != nil {
		return "", fmt.Errorf("peer: %q is not a valid homa address: %w", truncate(s), err)
	}
	return a, nil
}

// ValidAddr reports whether s is a well-formed peer address. It exists so the
// UI can validate what a user pasted without importing the transport.
func ValidAddr(s string) bool {
	_, err := ParseAddr(s)
	return err == nil
}

// truncate shortens an address for an error message. Addresses run to a
// couple of hundred characters, and the whole thing is a secret, so error
// text should not spray it across a terminal or a log file.
func truncate(s string) string {
	const keep = 12
	if len(s) <= keep {
		return s
	}
	return s[:keep] + "..."
}
