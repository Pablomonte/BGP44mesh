package types

import (
	"fmt"
	"net"
)

// Peer represents a discovered BGP peer in the TINC mesh
type Peer struct {
	IP       net.IP // IPv4 address on TINC mesh (e.g., 10.0.0.2)
	Key      string // RSA public key (base64 or fingerprint)
	Endpoint string // External endpoint for TINC connection (IP:port)
}

// String returns a human-readable representation of the peer
func (p Peer) String() string {
	keyPreview := p.Key
	if len(keyPreview) > 20 {
		keyPreview = keyPreview[:20] + "..."
	}

	return fmt.Sprintf("Peer{IP: %s, Endpoint: %s, Key: %s}",
		p.IP.String(), p.Endpoint, keyPreview)
}

// IsValid checks if the peer has all required fields
func (p Peer) IsValid() bool {
	return p.IP != nil && p.Endpoint != ""
}

// TODO Sprint 2: Add more peer metadata
// - Hostname/NodeName
// - BGP AS number
// - TINC subnet assignments
// - Health status (last seen, RTT)
// - Capabilities/features

// TODO Sprint 2: Add config sync types
// type Config struct {
//     BirdConf string
//     TincConf string
//     Version  int
// }

// TODO Sprint 2: Add health check types
// type HealthStatus struct {
//     NodeName    string
//     LastSeen    time.Time
//     RTT         time.Duration
//     BGPStatus   string // "Established", "Idle", etc.
//     TINCStatus  string // "Connected", "Disconnected"
// }
