package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pablomonte/bgp-daemon/pkg/discovery"
	"github.com/pablomonte/bgp-daemon/pkg/metrics"
	"github.com/pablomonte/bgp-daemon/pkg/tinc"
	"github.com/pablomonte/bgp-daemon/pkg/types"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	etcdEndpoints = flag.String("etcd", "localhost:2379", "etcd endpoints (comma-separated)")
	iface         = flag.String("iface", "tinc0", "TINC interface name")
	nodeName      = flag.String("node", "node1", "Node name for mDNS advertisement")
	tincNetName   = flag.String("tinc-net", "bgpmesh", "TINC network name")
	metricsAddr   = flag.String("metrics-addr", ":2112", "Metrics HTTP server address")
	verbose       = flag.Bool("v", false, "verbose logging")
)

func main() {
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("============================================")
	log.Println("BGP Propagation Daemon")
	log.Println("============================================")
	log.Printf("Node name: %s", *nodeName)
	log.Printf("TINC network: %s", *tincNetName)
	log.Printf("TINC interface: %s", *iface)
	log.Printf("etcd endpoints: %s", *etcdEndpoints)
	log.Printf("Metrics address: %s", *metricsAddr)
	log.Printf("Verbose: %v", *verbose)
	log.Println()

	// Start Prometheus metrics HTTP server
	log.Println("Starting metrics HTTP server...")
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(*metricsAddr, nil); err != nil {
			log.Printf("⚠ Metrics server error: %v", err)
		}
	}()
	log.Printf("✓ Metrics available at http://localhost%s/metrics", *metricsAddr)
	log.Println()

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v", sig)
		log.Println("Shutting down gracefully...")
		cancel()
	}()

	// Connect to etcd
	log.Println("Connecting to etcd...")
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{*etcdEndpoints},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
	}
	defer cli.Close()

	log.Println("✓ Connected to etcd")

	// Initialize TINC manager
	log.Println()
	log.Println("Initializing TINC manager...")
	tincManager := tinc.NewManager(*tincNetName)
	log.Println("✓ TINC manager initialized")

	// Get local public key for mDNS advertisement
	localKey, err := tincManager.GetPublicKey(*nodeName)
	keyFingerprint := ""
	if err != nil {
		log.Printf("⚠ Failed to read local key: %v", err)
		log.Println("  Continuing without mDNS advertisement...")
		keyFingerprint = "unknown"
	} else {
		log.Printf("✓ Read local public key (%d bytes)", len(localKey))
		// Use first 20 chars as fingerprint
		if len(localKey) > 20 {
			keyFingerprint = localKey[:20]
		} else {
			keyFingerprint = localKey
		}
	}

	// Store own key in etcd at /peers/<nodename>
	nodeIP := os.Getenv("NODE_IP")
	tincEndpoint := os.Getenv("TINC_ENDPOINT")
	if localKey != "" && nodeIP != "" && tincEndpoint != "" {
		log.Println()
		log.Println("Storing own key in etcd...")

		peerData := types.Peer{
			IP:       net.ParseIP(nodeIP),
			Key:      localKey,
			Endpoint: tincEndpoint,
		}

		if peerData.IsValid() {
			peerJSON, err := json.Marshal(peerData)
			if err == nil {
				peerKey := "/peers/" + *nodeName
				if _, err := cli.Put(ctx, peerKey, string(peerJSON)); err != nil {
					log.Printf("⚠ Failed to store own key in etcd: %v", err)
				} else {
					log.Printf("✓ Stored own key in etcd at %s", peerKey)
				}
			} else {
				log.Printf("⚠ Failed to marshal peer JSON: %v", err)
			}
		} else {
			log.Printf("⚠ Invalid peer data, skipping etcd storage")
		}
	} else {
		log.Println()
		log.Println("⚠ Skipping etcd key storage (missing NODE_IP or TINC_ENDPOINT env vars)")
	}

	// Start mDNS service advertisement
	log.Println()
	log.Println("Starting mDNS service advertisement...")
	mdnsServer, err := discovery.AdvertiseService(*nodeName, 655, keyFingerprint)
	if err != nil {
		log.Printf("⚠ mDNS advertisement failed: %v", err)
	} else {
		defer mdnsServer.Shutdown()
		log.Printf("✓ Advertising as '%s._bgp-node._tcp.local'", *nodeName)
	}

	// Start continuous mDNS monitoring in background
	log.Println()
	log.Println("Starting mDNS peer monitoring...")
	go discovery.MonitorPeers(ctx, *iface, 30*time.Second, func(peers []types.Peer) {
		log.Printf("📡 mDNS: Discovered %d peers", len(peers))
		metrics.PeersDiscovered.Set(float64(len(peers)))
		if *verbose {
			for i, peer := range peers {
				log.Printf("   [%d] %v", i+1, peer)
			}
		}
		// TODO: Optionally sync discovered peers to etcd
	})
	log.Println("✓ mDNS monitoring started (30s interval)")

	// Initial peer discovery
	log.Println()
	log.Println("Performing initial mDNS discovery...")
	peers, err := discovery.LookupPeers(*iface)
	if err != nil {
		log.Printf("⚠ Initial mDNS lookup failed: %v", err)
	} else {
		log.Printf("✓ Discovered %d peers initially", len(peers))
		if *verbose {
			for i, peer := range peers {
				log.Printf("  [%d] %v", i+1, peer)
			}
		}
	}

	// Perform initial peer sync from etcd
	log.Println()
	log.Println("Syncing TINC keys from etcd...")
	resp, err := cli.Get(ctx, "/peers/", clientv3.WithPrefix())
	if err != nil {
		log.Printf("⚠ Failed to fetch peers from etcd: %v", err)
	} else {
		peerNames := make([]string, 0)
		syncedCount := 0

		for _, kv := range resp.Kvs {
			var peer types.Peer
			if err := json.Unmarshal(kv.Value, &peer); err != nil {
				log.Printf("⚠ Failed to parse peer JSON for %s: %v", string(kv.Key), err)
				continue
			}

			if !peer.IsValid() {
				log.Printf("⚠ Invalid peer data for %s, skipping", string(kv.Key))
				continue
			}

			peerNodeName := extractNodeNameFromKey(string(kv.Key))

			// Skip syncing own host file
			if peerNodeName == *nodeName {
				if *verbose {
					log.Printf("  Skipping own node: %s", peerNodeName)
				}
				continue
			}

			// Sync host file
			if err := tincManager.SyncHostFile(peer); err != nil {
				log.Printf("⚠ Failed to sync %s: %v", peerNodeName, err)
			} else {
				peerNames = append(peerNames, peerNodeName)
				syncedCount++
				if *verbose {
					log.Printf("  ✓ Synced host file for %s", peerNodeName)
				}
			}
		}

		log.Printf("✓ Synced %d peer host files", syncedCount)

		// Update ConnectTo directives
		if len(peerNames) > 0 {
			log.Println("Updating ConnectTo directives...")
			if err := tincManager.UpdateConnectTo(peerNames); err != nil {
				log.Printf("⚠ Failed to update ConnectTo: %v", err)
			} else {
				log.Printf("✓ Updated ConnectTo with %d peers", len(peerNames))
			}
		}

		// Reload TINC daemon
		reloadFailed := false
		if syncedCount > 0 {
			log.Println("Reloading TINC daemon...")
			if err := tincManager.Reload(); err != nil {
				log.Printf("⚠ Failed initial TINC reload: %v", err)
				log.Println("  (Will retry after TINC startup)")
				reloadFailed = true
			} else {
				log.Println("✓ TINC daemon reloaded with peers")
			}
		}

		// If initial reload failed, retry after TINC has had time to start
		if reloadFailed {
			go func() {
				time.Sleep(10 * time.Second)
				log.Println("Retrying TINC reload...")
				if err := tincManager.Reload(); err != nil {
					log.Printf("⚠ Retry reload failed: %v", err)
				} else {
					log.Println("✓ TINC daemon reloaded successfully on retry")
				}
			}()
		}
	}

	// Watch etcd for peer changes
	log.Println()
	log.Println("Watching /peers/ in etcd for changes...")
	watchChan := cli.Watch(ctx, "/peers/", clientv3.WithPrefix())

	log.Println("✓ Daemon running (Ctrl+C to stop)")
	log.Println("============================================")
	log.Println()

	// Main event loop
	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, exiting...")
			return

		case watchResp := <-watchChan:
			if watchResp.Err() != nil {
				log.Printf("Watch error: %v", watchResp.Err())
				metrics.EtcdWatchErrors.Inc()
				continue
			}

			for _, event := range watchResp.Events {
				key := string(event.Kv.Key)

				switch event.Type {
				case clientv3.EventTypePut:
					log.Printf("📥 etcd PUT: %s", key)

					// Parse peer from JSON
					var peer types.Peer
					if err := json.Unmarshal(event.Kv.Value, &peer); err != nil {
						log.Printf("⚠ Failed to parse peer JSON: %v", err)
						metrics.PeerSyncTotal.WithLabelValues("error", "PUT").Inc()
						continue
					}

					if !peer.IsValid() {
						log.Printf("⚠ Invalid peer data, skipping")
						metrics.PeerSyncTotal.WithLabelValues("error", "PUT").Inc()
						continue
					}

					if *verbose {
						log.Printf("   Peer: %v", peer)
					}

					// Sync host file to TINC with duration measurement
					syncStart := time.Now()
					if err := tincManager.SyncHostFile(peer); err != nil {
						log.Printf("❌ Failed to sync host file: %v", err)
						metrics.PeerSyncTotal.WithLabelValues("error", "PUT").Inc()
						continue
					}
					metrics.HostFileSyncDuration.Observe(time.Since(syncStart).Seconds())
					log.Printf("✓ Synced host file for peer")

					// Update ConnectTo directives with current peers
					resp, err := cli.Get(ctx, "/peers/", clientv3.WithPrefix())
					if err == nil {
						peerNames := make([]string, 0)
						for _, kv := range resp.Kvs {
							peerNodeName := extractNodeNameFromKey(string(kv.Key))
							// Skip own node
							if peerNodeName != *nodeName && peerNodeName != "" {
								peerNames = append(peerNames, peerNodeName)
							}
						}
						if err := tincManager.UpdateConnectTo(peerNames); err != nil {
							log.Printf("⚠ Failed to update ConnectTo: %v", err)
						}
					}

					// Reload TINC daemon with duration measurement
					reloadStart := time.Now()
					if err := tincManager.Reload(); err != nil {
						log.Printf("⚠ Failed to reload tincd: %v", err)
						metrics.PeerSyncTotal.WithLabelValues("error", "PUT").Inc()
					} else {
						metrics.TincReloadDuration.Observe(time.Since(reloadStart).Seconds())
						metrics.PeerSyncTotal.WithLabelValues("success", "PUT").Inc()
						log.Printf("✓ Reloaded TINC daemon")
					}

				case clientv3.EventTypeDelete:
					log.Printf("🗑️  etcd DELETE: %s", key)

					// Extract node name from key (e.g., /peers/node2 -> node2)
					deletedNodeName := extractNodeNameFromKey(key)
					if deletedNodeName == "" {
						log.Printf("⚠ Could not extract node name from key")
						metrics.PeerSyncTotal.WithLabelValues("error", "DELETE").Inc()
						continue
					}

					log.Printf("   Removing host file for: %s", deletedNodeName)

					// Remove host file
					if err := tincManager.RemoveHostFile(deletedNodeName); err != nil {
						log.Printf("⚠ Failed to remove host file: %v", err)
						metrics.PeerSyncTotal.WithLabelValues("error", "DELETE").Inc()
						continue
					}
					log.Printf("✓ Removed host file")

					// Update ConnectTo directives with current peers
					resp, err := cli.Get(ctx, "/peers/", clientv3.WithPrefix())
					if err == nil {
						peerNames := make([]string, 0)
						for _, kv := range resp.Kvs {
							peerNodeName := extractNodeNameFromKey(string(kv.Key))
							// Skip own node
							if peerNodeName != *nodeName && peerNodeName != "" {
								peerNames = append(peerNames, peerNodeName)
							}
						}
						if err := tincManager.UpdateConnectTo(peerNames); err != nil {
							log.Printf("⚠ Failed to update ConnectTo: %v", err)
						}
					}

					// Reload TINC daemon with duration measurement
					reloadStart := time.Now()
					if err := tincManager.Reload(); err != nil {
						log.Printf("⚠ Failed to reload tincd: %v", err)
						metrics.PeerSyncTotal.WithLabelValues("error", "DELETE").Inc()
					} else {
						metrics.TincReloadDuration.Observe(time.Since(reloadStart).Seconds())
						metrics.PeerSyncTotal.WithLabelValues("success", "DELETE").Inc()
						log.Printf("✓ Reloaded TINC daemon")
					}
				}
			}
		}
	}
}

// extractNodeNameFromKey extracts the node name from an etcd key
// Examples:
//   - "/peers/node2" -> "node2"
//   - "/peers/tinc3" -> "tinc3"
func extractNodeNameFromKey(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
