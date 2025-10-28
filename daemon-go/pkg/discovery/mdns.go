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
			Service: "_bgp-node._tcp",
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

// AdvertiseService broadcasts this node's BGP service via mDNS
// Advertises on _bgp-node._tcp.local with node info
func AdvertiseService(nodeName string, port int, keyFingerprint string) (*mdns.Server, error) {
	// Create service info
	info := []string{
		fmt.Sprintf("key=%s", keyFingerprint),
		fmt.Sprintf("version=1.0"),
	}

	// Define service
	service, err := mdns.NewMDNSService(
		nodeName,           // Instance name (e.g., "node1")
		"_bgp-node._tcp",   // Service type
		"",                 // Domain (empty = .local)
		"",                 // Host name (empty = use hostname)
		port,               // Port
		nil,                // IPs (nil = use all interfaces)
		info,               // TXT records
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create mDNS service: %w", err)
	}

	// Start mDNS server
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return nil, fmt.Errorf("failed to start mDNS server: %w", err)
	}

	return server, nil
}

// MonitorPeers continuously discovers peers and calls callback on changes
// Runs until context is cancelled
func MonitorPeers(ctx context.Context, iface string, interval time.Duration, callback func([]types.Peer)) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastPeers []types.Peer

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			// Discover current peers
			peers, err := LookupPeers(iface)
			if err != nil {
				// Log error but continue monitoring
				continue
			}

			// Check if peers changed
			if !peersEqual(peers, lastPeers) {
				callback(peers)
				lastPeers = peers
			}
		}
	}
}

// peersEqual compares two peer lists for equality
func peersEqual(a, b []types.Peer) bool {
	if len(a) != len(b) {
		return false
	}

	// Create map for O(n) comparison
	aMap := make(map[string]bool)
	for _, peer := range a {
		aMap[peer.Endpoint] = true
	}

	for _, peer := range b {
		if !aMap[peer.Endpoint] {
			return false
		}
	}

	return true
}
