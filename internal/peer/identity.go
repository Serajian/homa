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

	"github.com/Serajian/homa/internal/logx"
	"github.com/tailscale/tailcat"
)

var logger = logx.For("peer")

// Identity is this machine's permanent name on the network: a private key
// plus the relay it can be reached through. It is opaque on purpose, so the
// transport's types never leak into the rest of homa.
//
// The whole thing is a secret. Anyone holding it can impersonate this machine.
type Identity struct {
	pk *tailcat.PrivateKey
}

// LoadOrCreateIdentity returns this machine's identity, creating and saving
// one on first run. Creation reaches the network to measure relay latency,
// so ctx should allow for that; later runs read only from disk.
func LoadOrCreateIdentity(ctx context.Context) (*Identity, error) {
	p, err := keyPath()
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(p)
	switch {
	case err == nil:
		return load(b, p)
	case errors.Is(err, os.ErrNotExist):
		logger.Info("no identity on disk, creating one", "path", p)
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

	logger.Info("identity loaded", "path", p, "region", pk.Public.RegionID)
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
		logger.Warn("generated identity had no pre-shared key, adding one")
		pk.Public.PresharedKey = tailcat.NewPresharedKey()
	}

	b, err := json.MarshalIndent(pk, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("peer: encoding new identity: %w", err)
	}

	// WriteFile creates the file with filePerm from the start, so the key is
	// never momentarily world-readable.
	if err := os.WriteFile(p, b, filePerm); err != nil {
		return nil, fmt.Errorf("peer: writing %s: %w", p, err)
	}

	logger.Info("identity created", "path", p, "region", region)
	return &Identity{pk: pk}, nil
}
