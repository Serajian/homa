package peer

import (
	"context"
	"errors"
	"fmt"

	"github.com/tailscale/tailcat"
	"tailscale.com/tailcfg"
)

// pickRegion measures the DERP relays and returns the nearest one.
//
// This runs once in a machine's lifetime. The chosen region is frozen into
// the identity file because its number is encoded in our address: letting it
// drift would change our address and break every contact who saved it.
func pickRegion(ctx context.Context) (tailcfg.DERPRegionID, error) {
	ctx, cancel := context.WithTimeout(ctx, regionPickTimeout)
	defer cancel()

	logger.Debug("fetching relay list")
	dm, err := tailcat.FetchDERPMap(ctx, tailcat.ExpandForServer)
	if err != nil {
		return 0, fmt.Errorf("peer: fetching the relay list: %w", err)
	}

	logger.Debug("measuring relay latency", "regions", len(dm.Regions))
	region, err := tailcat.PickBestRegion(ctx, dm)
	if err != nil {
		return 0, fmt.Errorf("peer: measuring relays: %w", err)
	}
	if region == 0 {
		// Documented outcome when no relay answered: usually no internet.
		return 0, errors.New("peer: no relay responded; check your connection and try again")
	}

	logger.Info("relay region chosen", "region", region)
	return region, nil
}
