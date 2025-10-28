package types

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeer_String(t *testing.T) {
	tests := []struct {
		name     string
		peer     Peer
		expected string
	}{
		{
			name: "normal peer with short key",
			peer: Peer{
				IP:       net.ParseIP("10.0.0.2"),
				Endpoint: "192.168.1.2:655",
				Key:      "short-key",
			},
			expected: "Peer{IP: 10.0.0.2, Endpoint: 192.168.1.2:655, Key: short-key}",
		},
		{
			name: "peer with long key gets truncated",
			peer: Peer{
				IP:       net.ParseIP("10.0.0.3"),
				Endpoint: "192.168.1.3:655",
				Key:      "this-is-a-very-long-key-that-should-be-truncated",
			},
			expected: "Peer{IP: 10.0.0.3, Endpoint: 192.168.1.3:655, Key: this-is-a-very-long-...}",
		},
		{
			name: "peer with empty key",
			peer: Peer{
				IP:       net.ParseIP("10.0.0.4"),
				Endpoint: "192.168.1.4:655",
				Key:      "",
			},
			expected: "Peer{IP: 10.0.0.4, Endpoint: 192.168.1.4:655, Key: }",
		},
		{
			name: "peer with nil IP",
			peer: Peer{
				IP:       nil,
				Endpoint: "192.168.1.5:655",
				Key:      "test-key",
			},
			expected: "Peer{IP: <nil>, Endpoint: 192.168.1.5:655, Key: test-key}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.peer.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPeer_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		peer     Peer
		expected bool
	}{
		{
			name: "valid peer with all fields",
			peer: Peer{
				IP:       net.ParseIP("10.0.0.2"),
				Endpoint: "192.168.1.2:655",
				Key:      "some-key",
			},
			expected: true,
		},
		{
			name: "valid peer without key",
			peer: Peer{
				IP:       net.ParseIP("10.0.0.3"),
				Endpoint: "192.168.1.3:655",
				Key:      "",
			},
			expected: true,
		},
		{
			name: "invalid peer with nil IP",
			peer: Peer{
				IP:       nil,
				Endpoint: "192.168.1.4:655",
				Key:      "some-key",
			},
			expected: false,
		},
		{
			name: "invalid peer with empty endpoint",
			peer: Peer{
				IP:       net.ParseIP("10.0.0.5"),
				Endpoint: "",
				Key:      "some-key",
			},
			expected: false,
		},
		{
			name: "invalid peer with both nil IP and empty endpoint",
			peer: Peer{
				IP:       nil,
				Endpoint: "",
				Key:      "some-key",
			},
			expected: false,
		},
		{
			name: "valid peer with IPv6",
			peer: Peer{
				IP:       net.ParseIP("2001:db8::1"),
				Endpoint: "[2001:db8::2]:655",
				Key:      "ipv6-key",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.peer.IsValid()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPeer_EdgeCases(t *testing.T) {
	t.Run("peer with exactly 20 char key not truncated", func(t *testing.T) {
		peer := Peer{
			IP:       net.ParseIP("10.0.0.1"),
			Endpoint: "test:655",
			Key:      "12345678901234567890", // Exactly 20 chars
		}
		str := peer.String()
		assert.Contains(t, str, "12345678901234567890")
		assert.NotContains(t, str, "...")
	})

	t.Run("peer with 21 char key gets truncated", func(t *testing.T) {
		peer := Peer{
			IP:       net.ParseIP("10.0.0.1"),
			Endpoint: "test:655",
			Key:      "123456789012345678901", // 21 chars
		}
		str := peer.String()
		assert.Contains(t, str, "12345678901234567890...")
	})

	t.Run("valid peer with whitespace in endpoint", func(t *testing.T) {
		peer := Peer{
			IP:       net.ParseIP("10.0.0.1"),
			Endpoint: " 192.168.1.1:655 ",
			Key:      "test",
		}
		// Endpoint is not empty, so it's technically valid
		assert.True(t, peer.IsValid())
	})

	t.Run("empty struct is invalid", func(t *testing.T) {
		peer := Peer{}
		assert.False(t, peer.IsValid())
	})
}

func TestPeer_Construction(t *testing.T) {
	t.Run("create peer with ParseIP", func(t *testing.T) {
		ip := net.ParseIP("10.0.0.100")
		require.NotNil(t, ip)

		peer := Peer{
			IP:       ip,
			Endpoint: "external.example.com:655",
			Key:      "rsa-public-key",
		}

		assert.True(t, peer.IsValid())
		assert.Equal(t, "10.0.0.100", peer.IP.String())
		assert.Contains(t, peer.String(), "10.0.0.100")
	})

	t.Run("create peer with invalid IP string", func(t *testing.T) {
		ip := net.ParseIP("invalid-ip")
		assert.Nil(t, ip)

		peer := Peer{
			IP:       ip,
			Endpoint: "test:655",
			Key:      "key",
		}

		assert.False(t, peer.IsValid())
	})
}
