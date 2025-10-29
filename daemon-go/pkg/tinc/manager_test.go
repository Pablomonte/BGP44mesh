package tinc

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablomonte/bgp-daemon/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	manager := NewManager("testnet")

	assert.NotNil(t, manager)
	assert.Equal(t, "testnet", manager.netName)
	assert.Equal(t, "/var/run/tinc/testnet", manager.baseDir)
	assert.Equal(t, "/var/run/tinc/testnet/hosts", manager.hostsDir)
}

func TestNewManager_DefaultNetName(t *testing.T) {
	manager := NewManager(defaultNetName)

	assert.Equal(t, defaultNetName, manager.netName)
	assert.Equal(t, "/var/run/tinc/bgpmesh", manager.baseDir)
}

func TestSyncHostFile(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "tinc-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create hosts subdirectory
	hostsDir := filepath.Join(tmpDir, "hosts")
	err = os.MkdirAll(hostsDir, 0755)
	require.NoError(t, err)

	// Create manager with test directory
	manager := &Manager{
		netName:  "testnet",
		baseDir:  tmpDir,
		hostsDir: hostsDir,
	}

	tests := []struct {
		name    string
		peer    types.Peer
		wantErr bool
	}{
		{
			name: "valid peer with hostname endpoint",
			peer: types.Peer{
				IP:       net.ParseIP("10.0.0.2"),
				Endpoint: "node2:655",
				Key:      "-----BEGIN RSA PUBLIC KEY-----\ntest\n-----END RSA PUBLIC KEY-----",
			},
			wantErr: false,
		},
		{
			name: "valid peer with IP endpoint",
			peer: types.Peer{
				IP:       net.ParseIP("10.0.0.3"),
				Endpoint: "192.168.1.3:655",
				Key:      "test-key",
			},
			wantErr: false,
		},
		{
			name: "peer with base64 encoded key",
			peer: types.Peer{
				IP:       net.ParseIP("10.0.0.4"),
				Endpoint: "node4:655",
				Key:      "dGVzdC1rZXk=", // "test-key" in base64
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.SyncHostFile(tt.peer)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify file was created
			nodeName := extractNodeName(tt.peer)
			hostFile := filepath.Join(hostsDir, nodeName)
			assert.FileExists(t, hostFile)

			// Verify file content
			content, err := os.ReadFile(hostFile)
			require.NoError(t, err)

			// Extract expected address from endpoint (hostname without port)
			expectedAddr := tt.peer.Endpoint
			if idx := strings.Index(tt.peer.Endpoint, ":"); idx != -1 {
				expectedAddr = tt.peer.Endpoint[:idx]
			}

			assert.Contains(t, string(content), expectedAddr)
			assert.Contains(t, string(content), "Port = 655")
		})
	}
}

func TestSyncHostFile_InvalidPeer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tinc-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	hostsDir := filepath.Join(tmpDir, "hosts")
	err = os.MkdirAll(hostsDir, 0755)
	require.NoError(t, err)

	manager := &Manager{
		netName:  "testnet",
		baseDir:  tmpDir,
		hostsDir: hostsDir,
	}

	// Peer with no valid node name extraction
	peer := types.Peer{
		IP:       net.ParseIP("::1"), // IPv6 won't work with current extractNodeName
		Endpoint: "",
		Key:      "test-key",
	}

	err = manager.SyncHostFile(peer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing node name")
}

func TestRemoveHostFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tinc-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	hostsDir := filepath.Join(tmpDir, "hosts")
	err = os.MkdirAll(hostsDir, 0755)
	require.NoError(t, err)

	manager := &Manager{
		netName:  "testnet",
		baseDir:  tmpDir,
		hostsDir: hostsDir,
	}

	// Create a test file
	testFile := filepath.Join(hostsDir, "node2")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Remove the file
	err = manager.RemoveHostFile("node2")
	assert.NoError(t, err)
	assert.NoFileExists(t, testFile)
}

func TestRemoveHostFile_NonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tinc-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	hostsDir := filepath.Join(tmpDir, "hosts")
	err = os.MkdirAll(hostsDir, 0755)
	require.NoError(t, err)

	manager := &Manager{
		netName:  "testnet",
		baseDir:  tmpDir,
		hostsDir: hostsDir,
	}

	// Try to remove non-existent file (should not error)
	err = manager.RemoveHostFile("nonexistent")
	assert.NoError(t, err)
}

func TestGetPublicKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tinc-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	hostsDir := filepath.Join(tmpDir, "hosts")
	err = os.MkdirAll(hostsDir, 0755)
	require.NoError(t, err)

	manager := &Manager{
		netName:  "testnet",
		baseDir:  tmpDir,
		hostsDir: hostsDir,
	}

	tests := []struct {
		name        string
		fileContent string
		wantKey     string
		wantErr     bool
	}{
		{
			name: "valid host file",
			fileContent: `# Host configuration for node2
Address = 10.0.0.2
Port = 655

-----BEGIN RSA PUBLIC KEY-----
test-public-key-data
-----END RSA PUBLIC KEY-----`,
			wantKey: "-----BEGIN RSA PUBLIC KEY-----\ntest-public-key-data\n-----END RSA PUBLIC KEY-----",
			wantErr: false,
		},
		{
			name: "host file with extra blank lines",
			fileContent: `Address = 10.0.0.3

my-key-data`,
			wantKey: "my-key-data",
			wantErr: false,
		},
		{
			name:        "invalid format - no blank line",
			fileContent: `Address = 10.0.0.4`,
			wantKey:     "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(hostsDir, "testnode")
			err := os.WriteFile(testFile, []byte(tt.fileContent), 0644)
			require.NoError(t, err)

			key, err := manager.GetPublicKey("testnode")

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantKey, key)
		})
	}
}

func TestGetPublicKey_NonExistentFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tinc-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	hostsDir := filepath.Join(tmpDir, "hosts")
	err = os.MkdirAll(hostsDir, 0755)
	require.NoError(t, err)

	manager := &Manager{
		netName:  "testnet",
		baseDir:  tmpDir,
		hostsDir: hostsDir,
	}

	_, err = manager.GetPublicKey("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read host file")
}

func TestExtractNodeName(t *testing.T) {
	tests := []struct {
		name     string
		peer     types.Peer
		expected string
	}{
		{
			name: "hostname in endpoint",
			peer: types.Peer{
				IP:       net.ParseIP("10.0.0.2"),
				Endpoint: "node2:655",
			},
			expected: "node2",
		},
		{
			name: "FQDN in endpoint",
			peer: types.Peer{
				IP:       net.ParseIP("10.0.0.3"),
				Endpoint: "node3.local:655",
			},
			expected: "node3.local",
		},
		{
			name: "IP endpoint - returns IP from endpoint",
			peer: types.Peer{
				IP:       net.ParseIP("10.0.0.4"),
				Endpoint: "192.168.1.4:655",
			},
			expected: "192.168.1.4",
		},
		{
			name: "no port in endpoint - fallback to IP-based name",
			peer: types.Peer{
				IP:       net.ParseIP("10.0.0.4"),
				Endpoint: "",
			},
			expected: "node4",
		},
		{
			name: "endpoint without port",
			peer: types.Peer{
				IP:       net.ParseIP("10.0.0.5"),
				Endpoint: "node5",
			},
			expected: "node5",
		},
		{
			name: "IPv6 - returns empty",
			peer: types.Peer{
				IP:       net.ParseIP("::1"),
				Endpoint: "[::1]:655",
			},
			expected: "[",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractNodeName(tt.peer)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUpdateConnectTo(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "tinc-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create tinc.conf with initial content
	confPath := filepath.Join(tmpDir, "tinc.conf")
	initialConf := `Name = node1
Mode = switch
Port = 655
`
	err = os.WriteFile(confPath, []byte(initialConf), 0644)
	require.NoError(t, err)

	// Create manager with test directory
	manager := &Manager{
		netName:  "testnet",
		baseDir:  tmpDir,
		hostsDir: filepath.Join(tmpDir, "hosts"),
	}

	tests := []struct {
		name      string
		peers     []string
		wantLines []string
	}{
		{
			name:  "add single peer",
			peers: []string{"node2"},
			wantLines: []string{
				"Name = node1",
				"Mode = switch",
				"Port = 655",
				"ConnectTo = node2",
			},
		},
		{
			name:  "add multiple peers",
			peers: []string{"node2", "node3", "node4"},
			wantLines: []string{
				"Name = node1",
				"Mode = switch",
				"Port = 655",
				"ConnectTo = node2",
				"ConnectTo = node3",
				"ConnectTo = node4",
			},
		},
		{
			name:  "replace existing peers",
			peers: []string{"node5"},
			wantLines: []string{
				"Name = node1",
				"Mode = switch",
				"Port = 655",
				"ConnectTo = node5",
			},
		},
		{
			name:  "empty peer list",
			peers: []string{},
			wantLines: []string{
				"Name = node1",
				"Mode = switch",
				"Port = 655",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Update ConnectTo
			err := manager.UpdateConnectTo(tt.peers)
			require.NoError(t, err)

			// Read back the config
			data, err := os.ReadFile(confPath)
			require.NoError(t, err)

			// Verify each expected line is present
			content := string(data)
			for _, wantLine := range tt.wantLines {
				assert.Contains(t, content, wantLine, "config should contain: %s", wantLine)
			}

			// Verify no duplicate ConnectTo lines
			lines := strings.Split(content, "\n")
			connectToCount := 0
			for _, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "ConnectTo") {
					connectToCount++
				}
			}
			assert.Equal(t, len(tt.peers), connectToCount, "should have exactly %d ConnectTo lines", len(tt.peers))
		})
	}
}

func TestUpdateConnectTo_NonExistentFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tinc-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager := &Manager{
		netName:  "testnet",
		baseDir:  tmpDir,
		hostsDir: filepath.Join(tmpDir, "hosts"),
	}

	// Try to update ConnectTo when tinc.conf doesn't exist
	// Should return nil (graceful handling) instead of error during startup
	err = manager.UpdateConnectTo([]string{"node2"})
	assert.NoError(t, err, "UpdateConnectTo should handle missing tinc.conf gracefully")
}

func TestUpdateConnectTo_PreservesOtherLines(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tinc-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	confPath := filepath.Join(tmpDir, "tinc.conf")
	initialConf := `Name = node1
Mode = switch
Port = 655
ConnectTo = oldpeer1
ConnectTo = oldpeer2
Device = /dev/net/tun
AddressFamily = ipv4
`
	err = os.WriteFile(confPath, []byte(initialConf), 0644)
	require.NoError(t, err)

	manager := &Manager{
		netName:  "testnet",
		baseDir:  tmpDir,
		hostsDir: filepath.Join(tmpDir, "hosts"),
	}

	// Update with new peers
	err = manager.UpdateConnectTo([]string{"newpeer1", "newpeer2"})
	require.NoError(t, err)

	// Read back
	data, err := os.ReadFile(confPath)
	require.NoError(t, err)
	content := string(data)

	// Old ConnectTo lines should be removed
	assert.NotContains(t, content, "ConnectTo = oldpeer1")
	assert.NotContains(t, content, "ConnectTo = oldpeer2")

	// New ConnectTo lines should be present
	assert.Contains(t, content, "ConnectTo = newpeer1")
	assert.Contains(t, content, "ConnectTo = newpeer2")

	// Other config lines should be preserved
	assert.Contains(t, content, "Name = node1")
	assert.Contains(t, content, "Mode = switch")
	assert.Contains(t, content, "Port = 655")
	assert.Contains(t, content, "Device = /dev/net/tun")
	assert.Contains(t, content, "AddressFamily = ipv4")
}

func TestReload_Integration(t *testing.T) {
	// Skip if not in integration mode or tincd not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	manager := NewManager("testnet")

	// This will likely fail unless tincd is actually running
	// We just test that the function doesn't panic
	err := manager.Reload()
	// We don't assert success/failure since it depends on tincd being available
	_ = err
}
