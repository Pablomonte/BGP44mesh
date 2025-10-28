package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/pablomonte/bgp-daemon/pkg/types"
)

// LookupPeers discovers BGP peers via mDNS over the specified interface
// In Sprint 1, this is a skeleton implementation
// Sprint 2 will add full mDNS service discovery with TINC integration
func LookupPeers(iface string) ([]types.Peer, error) {
	entries := make(chan *mdns.ServiceEntry, 10)

	// Query for BGP service on local network
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start mDNS query in goroutine
	go func() {
		defer close(entries)

		params := &mdns.QueryParam{
			Service: "_bgp._tcp",
			Domain:  "local",
			Timeout: 5 * time.Second,
			Entries: entries,
		}

		// Run query
		if err := mdns.Query(params); err != nil {
			// Error is logged but not fatal for Sprint 1
			return
		}
	}()

	// Collect discovered peers
	peers := []types.Peer{}

	for {
		select {
		case <-ctx.Done():
			// Timeout or cancelled
			return peers, ctx.Err()

		case entry, ok := <-entries:
			if !ok {
				// Channel closed, query complete
				return peers, nil
			}

			if entry == nil {
				continue
			}

			// Convert mDNS entry to Peer struct
			peer := types.Peer{
				IP:       entry.AddrV4,
				Endpoint: fmt.Sprintf("%s:%d", entry.AddrV4.String(), entry.Port),
			}

			// Extract key from TXT records if available
			if entry.InfoFields != nil && len(entry.InfoFields) > 0 {
				peer.Key = entry.InfoFields[0]
			}

			peers = append(peers, peer)
		}
	}
}

// TODO Sprint 2: Implement AdvertiseService for mDNS service registration
// func AdvertiseService(iface string, port int, key string) error

// TODO Sprint 2: Implement continuous peer monitoring
// func MonitorPeers(iface string, callback func([]Peer)) error
