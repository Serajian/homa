// Package peer is homa's only door to the network. Every dependency on
// tailcat, WireGuard and DERP lives inside this package; the rest of the
// program sees nothing but net.Conn values and plain strings. Swapping the
// transport later means rewriting this package and nothing else.
package peer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/tailscale/tailcat"

	"github.com/Serajian/homa/internal/logx"
	"github.com/Serajian/homa/internal/paths"
)

var lg = logx.For("peer")

// KeyPath reports where the identity is stored, for messages to the user.
func KeyPath() string { return paths.Display(keyFile) }

// Identity is this machine's permanent name on the network: a private key
// plus the relay it can be reached through. It is opaque on purpose, so the
// transport's types never leak into the rest of homa.
//
// The whole thing is a secret. Anyone holding it can impersonate this machine.
type Identity struct {
	pk *tailcat.PrivateKey
}

// RemoveIdentity deletes the saved key, so the next start makes a new one.
//
// This is the part of a reset that cannot be undone. A new key is a new
// address, and everybody holding the old one loses the way to reach you. The
// file name lives in this package, so the deleting does too.
func RemoveIdentity() error { return paths.Remove(keyFile) }

// HasIdentity reports whether an identity is already saved, so a caller can
// say what the wait on the first run is for. It is a check on the file, not
// on its contents; LoadOrCreateIdentity is what reads it.
func HasIdentity() bool {
	p, err := paths.File(keyFile)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// LoadOrCreateIdentity returns this machine's identity, creating and saving
// one on first run. Creation reaches the network to measure relay latency,
// so ctx should allow for that; later runs read only from disk.
func LoadOrCreateIdentity(ctx context.Context) (*Identity, error) {
	p, err := paths.File(keyFile)
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(p)
	switch {
	case err == nil:
		return load(b, p)
	case errors.Is(err, os.ErrNotExist):
		lg.Info("no identity on disk, creating one", "path", p)
		return create(ctx, p)
	default:
		return nil, fmt.Errorf("peer: reading %s: %w", p, err)
	}
}

func load(b []byte, p string) (*Identity, error) {
	var pk tailcat.PrivateKey
	if err := json.Unmarshal(b, &pk); err != nil {
		return nil, fmt.Errorf("peer: %s is corrupt (%w); delete it to start over, "+
			"but your contacts will need your new address", p, err)
	}
	if pk.Public.RegionID <= 0 && len(pk.Public.Region) == 0 {
		return nil, fmt.Errorf("peer: %s has no relay region; delete it to start over", p)
	}

	lg.Info("identity loaded", "path", p, "region", pk.Public.RegionID)
	return &Identity{pk: &pk}, nil
}

// create builds a fresh identity and writes it to disk.
func create(ctx context.Context, p string) (*Identity, error) {
	region, err := pickRegion(ctx)
	if err != nil {
		return nil, err
	}

	pk := tailcat.NewPrivateKey()
	pk.Public.RegionID = region

	// The pre-shared key is what makes our address a secret worth guarding:
	// without it, whoever runs the relay could join the tunnel. NewPrivateKey
	// is expected to set it, but the docs do not promise so.
	if pk.Public.PresharedKey.IsZero() {
		lg.Warn("generated identity had no pre-shared key, adding one")
		pk.Public.PresharedKey = tailcat.NewPresharedKey()
	}

	b, err := json.MarshalIndent(pk, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("peer: encoding new identity: %w", err)
	}

	// WriteFile creates the file with filePerm from the start, so the key is
	// never momentarily world-readable.
	if err := os.WriteFile(p, b, paths.FilePerm); err != nil {
		return nil, fmt.Errorf("peer: writing %s: %w", p, err)
	}

	lg.Info("identity created", "path", p, "region", region)
	return &Identity{pk: pk}, nil
}
