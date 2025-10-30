package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// PeerSyncTotal counts total peer sync operations
	PeerSyncTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bgp_daemon_peer_sync_total",
			Help: "Total number of peer sync operations",
		},
		[]string{"status", "event_type"},
	)

	// TincReloadDuration tracks TINC reload operation duration
	TincReloadDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "bgp_daemon_tinc_reload_duration_seconds",
			Help:    "Duration of TINC daemon reload operations",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1},
		},
	)

	// PeersDiscovered tracks number of peers discovered via mDNS
	PeersDiscovered = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "bgp_daemon_peers_discovered",
			Help: "Number of peers currently discovered via mDNS",
		},
	)

	// EtcdWatchErrors counts etcd watch errors
	EtcdWatchErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "bgp_daemon_etcd_watch_errors_total",
			Help: "Total number of etcd watch errors",
		},
	)

	// HostFileSyncDuration tracks host file sync duration
	HostFileSyncDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "bgp_daemon_hostfile_sync_duration_seconds",
			Help:    "Duration of host file sync operations",
			Buckets: []float64{.001, .005, .01, .025, .05, .1},
		},
	)

	// TincConnectionsActive tracks number of active TINC connections
	TincConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "bgp_daemon_tinc_connections_active",
			Help: "Number of active TINC connections maintained by daemon",
		},
	)

	// TincConnectionOperations tracks TINC connection add/remove operations
	TincConnectionOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bgp_daemon_tinc_connection_operations_total",
			Help: "Total number of TINC connection operations (add/remove)",
		},
		[]string{"operation", "status"},
	)
)
