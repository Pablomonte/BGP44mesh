package tinc

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pablomonte/bgp-daemon/pkg/types"
)

const (
	defaultTincDir = "/var/run/tinc"
	defaultNetName = "bgpmesh"
	hostsSubdir    = "hosts"
)

// Manager handles TINC configuration and operations
type Manager struct {
	netName  string
	baseDir  string
	hostsDir string
}

// NewManager creates a new TINC manager
func NewManager(netName string) *Manager {
	baseDir := filepath.Join(defaultTincDir, netName)
	hostsDir := filepath.Join(baseDir, hostsSubdir)

	return &Manager{
		netName:  netName,
		baseDir:  baseDir,
		hostsDir: hostsDir,
	}
}

// SyncHostFile creates or updates a host file for a peer
// nodeName is the TINC node name (e.g., "node2") used for the filename
// peer.Endpoint contains the DNS-resolvable hostname (e.g., "tinc2:655") for the Address field
func (m *Manager) SyncHostFile(nodeName string, peer types.Peer) error {
	if nodeName == "" {
		return fmt.Errorf("invalid peer: missing node name")
	}

	hostFilePath := filepath.Join(m.hostsDir, nodeName)

	// Decode key if base64 encoded
	keyData := peer.Key
	if decoded, err := base64.StdEncoding.DecodeString(peer.Key); err == nil {
		keyData = string(decoded)
	}

	// Extract address from endpoint (remove port if present)
	// This is the DNS-resolvable hostname (e.g., "tinc2")
	address := peer.Endpoint
	if idx := strings.Index(peer.Endpoint, ":"); idx != -1 {
		address = peer.Endpoint[:idx]
	}

	// Create host file content
	// File is named with TINC node name (node2), but Address uses Docker hostname (tinc2)
	// Subnet declaration is required for TINC switch mode to map IPs to nodes
	content := fmt.Sprintf(`# Host configuration for %s
Address = %s
Port = 655
Subnet = %s/32

%s
`, nodeName, address, peer.IP.String(), keyData)

	// Write host file
	if err := os.WriteFile(hostFilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write host file: %w", err)
	}

	return nil
}

// RemoveHostFile deletes a host file for a peer
func (m *Manager) RemoveHostFile(nodeName string) error {
	hostFilePath := filepath.Join(m.hostsDir, nodeName)

	if err := os.Remove(hostFilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove host file: %w", err)
	}

	return nil
}

// Reload triggers TINC daemon to reload configuration
// Uses SIGHUP signal (TINC 1.0 mechanism) with shared PID namespace
// Includes retry logic with exponential backoff for robustness
func (m *Manager) Reload() error {
	// Retry with exponential backoff for transient errors
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		// Find tincd process PID (visible due to shared PID namespace)
		pidCmd := exec.Command("pidof", "tincd")
		output, err := pidCmd.Output()

		if err != nil {
			lastErr = fmt.Errorf("attempt %d: tincd process not found: %w", attempt, err)
			if attempt < 3 {
				// Wait before retry - tincd might be starting
				backoff := time.Duration(1<<uint(attempt-1)) * time.Second
				time.Sleep(backoff)
				continue
			}
			return fmt.Errorf("tincd process not found after %d attempts: %w", attempt, lastErr)
		}

		pidStr := strings.TrimSpace(string(output))
		if pidStr == "" {
			lastErr = fmt.Errorf("attempt %d: no tincd PID found", attempt)
			if attempt < 3 {
				backoff := time.Duration(1<<uint(attempt-1)) * time.Second
				time.Sleep(backoff)
				continue
			}
			return fmt.Errorf("no tincd PID found after %d attempts", attempt)
		}

		// Send SIGHUP signal to reload configuration
		killCmd := exec.Command("kill", "-HUP", pidStr)
		if err := killCmd.Run(); err != nil {
			lastErr = fmt.Errorf("attempt %d: failed to send SIGHUP to tincd (PID %s): %w", attempt, pidStr, err)
			if attempt < 3 {
				backoff := time.Duration(1<<uint(attempt-1)) * time.Second
				time.Sleep(backoff)
				continue
			}
			return fmt.Errorf("failed to reload tincd after %d attempts: %w", attempt, lastErr)
		}

		// Success
		return nil
	}

	return fmt.Errorf("reload failed after 3 attempts: %w", lastErr)
}

// GetPublicKey reads the public key from the local host file
func (m *Manager) GetPublicKey(nodeName string) (string, error) {
	hostFilePath := filepath.Join(m.hostsDir, nodeName)

	data, err := os.ReadFile(hostFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read host file: %w", err)
	}

	// Extract public key section (everything after the first blank line)
	parts := strings.Split(string(data), "\n\n")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid host file format")
	}

	return strings.TrimSpace(parts[1]), nil
}

// UpdateConnectTo updates tinc.conf with ConnectTo directives for specified peers
// Removes all existing ConnectTo lines and adds new ones
func (m *Manager) UpdateConnectTo(peerNames []string) error {
	confPath := filepath.Join(m.baseDir, "tinc.conf")

	// Read existing config
	data, err := os.ReadFile(confPath)
	if err != nil {
		// If tinc.conf doesn't exist yet, skip update gracefully
		// This can happen during startup when TINC container is still initializing
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read tinc.conf: %w", err)
	}

	// Parse config lines and filter out ConnectTo directives
	lines := strings.Split(string(data), "\n")
	filtered := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Keep lines that are not ConnectTo directives
		if !strings.HasPrefix(trimmed, "ConnectTo") && trimmed != "" {
			filtered = append(filtered, line)
		}
	}

	// Add new ConnectTo directives
	for _, peerName := range peerNames {
		filtered = append(filtered, fmt.Sprintf("ConnectTo = %s", peerName))
	}

	// Ensure file ends with newline
	newData := strings.Join(filtered, "\n") + "\n"

	// Write updated config
	if err := os.WriteFile(confPath, []byte(newData), 0644); err != nil {
		return fmt.Errorf("failed to write tinc.conf: %w", err)
	}

	return nil
}

// AddConnection is a no-op for TINC 1.0 (file-based configuration)
// TINC 1.0 doesn't have CLI commands like 'tinc add ConnectTo'
// Connections are managed via UpdateConnectTo() + Reload() instead
func (m *Manager) AddConnection(peerName string) error {
	// No-op: TINC 1.0 uses file-based config only
	// Use UpdateConnectTo() + Reload() for connection management
	return nil
}

// RemoveConnection is a no-op for TINC 1.0 (file-based configuration)
// TINC 1.0 doesn't have CLI commands like 'tinc del ConnectTo'
// Connections are managed via UpdateConnectTo() + Reload() instead
func (m *Manager) RemoveConnection(peerName string) error {
	// No-op: TINC 1.0 uses file-based config only
	// Use UpdateConnectTo() + Reload() for connection management
	return nil
}

// GetCurrentConnections returns list of ConnectTo peers from tinc.conf
// For TINC 1.0, we read the config file since CLI commands don't exist
func (m *Manager) GetCurrentConnections() ([]string, error) {
	confPath := filepath.Join(m.baseDir, "tinc.conf")

	data, err := os.ReadFile(confPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tinc.conf: %w", err)
	}

	peers := make([]string, 0)
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Look for "ConnectTo = <peer>" lines
		if strings.HasPrefix(trimmed, "ConnectTo") {
			// Parse: "ConnectTo = node2" or "ConnectTo=node2"
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				peerName := strings.TrimSpace(parts[1])
				if peerName != "" {
					peers = append(peers, peerName)
				}
			}
		}
	}

	return peers, nil
}

// GetDesiredConnections returns list of peers that SHOULD be connected (full mesh logic)
// Excludes own node name
func (m *Manager) GetDesiredConnections(allPeers []string, ownNodeName string) []string {
	desired := make([]string, 0)
	for _, peer := range allPeers {
		if peer != ownNodeName && peer != "" {
			desired = append(desired, peer)
		}
	}
	return desired
}

// ReconcileConnections implements full mesh for TINC 1.0 (file-based)
// Updates tinc.conf with desired peers and reloads daemon
// Returns (added, removed, error)
func (m *Manager) ReconcileConnections(desiredPeers []string) (int, int, error) {
	// Get current connections from tinc.conf
	current, err := m.GetCurrentConnections()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get current connections: %w", err)
	}

	// Calculate diffs for metrics
	currentSet := make(map[string]bool)
	for _, peer := range current {
		currentSet[peer] = true
	}

	desiredSet := make(map[string]bool)
	for _, peer := range desiredPeers {
		desiredSet[peer] = true
	}

	added := 0
	removed := 0

	for _, peer := range desiredPeers {
		if !currentSet[peer] {
			added++
		}
	}

	for _, peer := range current {
		if !desiredSet[peer] {
			removed++
		}
	}

	// Update tinc.conf with full peer list
	if err := m.UpdateConnectTo(desiredPeers); err != nil {
		return 0, 0, fmt.Errorf("failed to update tinc.conf: %w", err)
	}

	// Reload TINC daemon to apply changes (SIGHUP)
	if err := m.Reload(); err != nil {
		return added, removed, fmt.Errorf("failed to reload tincd: %w", err)
	}

	return added, removed, nil
}

// extractNodeName extracts the node name from a peer
// Tries to parse from IP or endpoint
func extractNodeName(peer types.Peer) string {
	// If endpoint contains a hostname, extract it
	endpoint := peer.Endpoint
	if strings.Contains(endpoint, ":") {
		parts := strings.Split(endpoint, ":")
		if len(parts) > 0 {
			return parts[0]
		}
	}

	// Fallback: Use IP to generate node name (e.g., 10.0.0.2 -> node2)
	ip := peer.IP.String()
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return fmt.Sprintf("node%s", parts[3])
	}

	return ""
}
