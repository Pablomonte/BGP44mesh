package discovery

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/pablomonte/bgp-daemon/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeersEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []types.Peer
		b        []types.Peer
		expected bool
	}{
		{
			name:     "empty lists",
			a:        []types.Peer{},
			b:        []types.Peer{},
			expected: true,
		},
		{
			name: "identical single peer",
			a: []types.Peer{
				{IP: net.ParseIP("10.0.0.2"), Endpoint: "10.0.0.2:655"},
			},
			b: []types.Peer{
				{IP: net.ParseIP("10.0.0.2"), Endpoint: "10.0.0.2:655"},
			},
			expected: true,
		},
		{
			name: "identical multiple peers",
			a: []types.Peer{
				{IP: net.ParseIP("10.0.0.2"), Endpoint: "10.0.0.2:655"},
				{IP: net.ParseIP("10.0.0.3"), Endpoint: "10.0.0.3:655"},
			},
			b: []types.Peer{
				{IP: net.ParseIP("10.0.0.2"), Endpoint: "10.0.0.2:655"},
				{IP: net.ParseIP("10.0.0.3"), Endpoint: "10.0.0.3:655"},
			},
			expected: true,
		},
		{
			name: "same peers different order",
			a: []types.Peer{
				{IP: net.ParseIP("10.0.0.3"), Endpoint: "10.0.0.3:655"},
				{IP: net.ParseIP("10.0.0.2"), Endpoint: "10.0.0.2:655"},
			},
			b: []types.Peer{
				{IP: net.ParseIP("10.0.0.2"), Endpoint: "10.0.0.2:655"},
				{IP: net.ParseIP("10.0.0.3"), Endpoint: "10.0.0.3:655"},
			},
			expected: true,
		},
		{
			name: "different lengths",
			a: []types.Peer{
				{IP: net.ParseIP("10.0.0.2"), Endpoint: "10.0.0.2:655"},
			},
			b: []types.Peer{
				{IP: net.ParseIP("10.0.0.2"), Endpoint: "10.0.0.2:655"},
				{IP: net.ParseIP("10.0.0.3"), Endpoint: "10.0.0.3:655"},
			},
			expected: false,
		},
		{
			name: "different endpoints",
			a: []types.Peer{
				{IP: net.ParseIP("10.0.0.2"), Endpoint: "10.0.0.2:655"},
			},
			b: []types.Peer{
				{IP: net.ParseIP("10.0.0.3"), Endpoint: "10.0.0.3:655"},
			},
			expected: false,
		},
		{
			name: "one empty one populated",
			a: []types.Peer{
				{IP: net.ParseIP("10.0.0.2"), Endpoint: "10.0.0.2:655"},
			},
			b:        []types.Peer{},
			expected: false,
		},
		{
			name:     "nil vs empty",
			a:        nil,
			b:        []types.Peer{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := peersEqual(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLookupPeers_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mDNS integration test in short mode")
	}

	// Test that LookupPeers returns within timeout even with no peers
	start := time.Now()
	peers, err := LookupPeers("eth0")
	elapsed := time.Since(start)

	// Should complete within ~5 seconds (with some buffer)
	assert.Less(t, elapsed, 6*time.Second)

	// May return timeout error or nil depending on whether any responses came
	if err != nil {
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	}

	// Peers list should be valid (empty or populated)
	assert.NotNil(t, peers)
}

func TestLookupPeers_InvalidInterface(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mDNS integration test in short mode")
	}

	// LookupPeers should handle non-existent interface gracefully
	// (hashicorp/mdns queries all interfaces when iface not supported)
	peers, err := LookupPeers("nonexistent0")

	// Should not panic and return valid result
	if err != nil {
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	}
	assert.NotNil(t, peers)
}

func TestAdvertiseService(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mDNS integration test in short mode")
	}

	tests := []struct {
		name           string
		nodeName       string
		port           int
		keyFingerprint string
		wantErr        bool
	}{
		{
			name:           "valid service",
			nodeName:       "testnode",
			port:           655,
			keyFingerprint: "test-key-fingerprint",
			wantErr:        false,
		},
		{
			name:           "valid service with empty key",
			nodeName:       "testnode2",
			port:           655,
			keyFingerprint: "",
			wantErr:        false,
		},
		{
			name:           "high port number",
			nodeName:       "testnode3",
			port:           65535,
			keyFingerprint: "key",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := AdvertiseService(tt.nodeName, tt.port, tt.keyFingerprint)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, server)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, server)

			// Cleanup
			server.Shutdown()
		})
	}
}

func TestAdvertiseService_InvalidPort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mDNS integration test in short mode")
	}

	// Test with invalid port (0)
	server, err := AdvertiseService("testnode", 0, "key")

	// Should fail with missing port error
	assert.Error(t, err)
	assert.Nil(t, server)
	assert.Contains(t, err.Error(), "missing service port")
}

func TestMonitorPeers_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mDNS integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	callbackCalled := false
	callback := func(peers []types.Peer) {
		callbackCalled = true
	}

	// Monitor with short interval
	err := MonitorPeers(ctx, "eth0", 500*time.Millisecond, callback)

	// Should return context.DeadlineExceeded
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Callback may or may not have been called depending on timing
	// We just verify it doesn't panic
	_ = callbackCalled
}

func TestMonitorPeers_ImmediateCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mDNS integration test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	callbackCalled := false
	callback := func(peers []types.Peer) {
		callbackCalled = true
	}

	start := time.Now()
	err := MonitorPeers(ctx, "eth0", 1*time.Second, callback)
	elapsed := time.Since(start)

	// Should return quickly
	assert.Less(t, elapsed, 100*time.Millisecond)
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, callbackCalled, "callback should not be called if context cancelled immediately")
}

func TestMonitorPeers_CallbackOnChange(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mDNS integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	callCount := 0
	var lastPeers []types.Peer

	callback := func(peers []types.Peer) {
		callCount++
		lastPeers = peers
	}

	// Monitor with short interval
	// Note: In a real environment with mDNS traffic, callback might be called
	// In test environment with no peers, callback is only called if peers change
	err := MonitorPeers(ctx, "eth0", 500*time.Millisecond, callback)

	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// If callback was called, verify peers structure
	if callCount > 0 {
		assert.NotNil(t, lastPeers)
	}
}

func TestAdvertiseAndDiscover_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mDNS integration test in short mode")
	}

	// Start advertising a service
	server, err := AdvertiseService("test-node", 655, "test-key")
	require.NoError(t, err)
	require.NotNil(t, server)
	defer server.Shutdown()

	// Give mDNS time to propagate
	time.Sleep(1 * time.Second)

	// Try to discover it
	peers, err := LookupPeers("eth0")

	// Should complete without error or with timeout
	if err != nil {
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	}

	// Peers list should be valid
	assert.NotNil(t, peers)

	// Note: In containerized/isolated test environments, mDNS may not work
	// This test verifies the functions work together without panicking
	// but doesn't assert specific peer discovery due to network constraints
}
