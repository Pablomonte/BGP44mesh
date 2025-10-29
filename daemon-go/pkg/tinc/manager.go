package tinc

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
func (m *Manager) SyncHostFile(peer types.Peer) error {
	// Extract node name from peer endpoint or key
	nodeName := extractNodeName(peer)
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
	address := peer.Endpoint
	if idx := strings.Index(peer.Endpoint, ":"); idx != -1 {
		address = peer.Endpoint[:idx]
	}

	// Create host file content
	content := fmt.Sprintf(`# Host configuration for %s
Address = %s
Port = 655

%s
`, nodeName, address, keyData)

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
// Uses TINC 1.0's signal mechanism via pidfile
// This works across separate containers sharing /var/run/tinc volume
func (m *Manager) Reload() error {
	// TINC 1.0 uses tincd -k to send signals via pidfile
	// -k = --kill signal (HUP = reload configuration)
	// -n = network name
	// -c = config directory
	cmd := exec.Command("tincd", "-n", m.netName, "-c", m.baseDir, "-kHUP")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to reload tincd: %w (output: %s)", err, string(output))
	}

	return nil
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
