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
				// Add 5-second timeout for Put operation
				putCtx, putCancel := context.WithTimeout(ctx, 5*time.Second)
				defer putCancel()
				if _, err := cli.Put(putCtx, peerKey, string(peerJSON)); err != nil {
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

	// Heurística de "ventana de calma" para peer discovery
	// Espera hasta que no haya nuevos peers registrándose (convergencia natural)
	// con timeout de seguridad para no esperar indefinidamente
	calmWindow := 2 * time.Second        // Tiempo sin cambios = todos registrados
	checkInterval := 500 * time.Millisecond
	maxWaitTime := 10 * time.Second

	startTime := time.Now()
	lastPeerCount := 0
	calmDuration := time.Duration(0)

	var resp *clientv3.GetResponse

	// Loop de discovery con heurística adaptativa
	for {
		// Add 3-second timeout for each Get attempt
		getCtx, getCancel := context.WithTimeout(ctx, 3*time.Second)
		resp, err = cli.Get(getCtx, "/peers/", clientv3.WithPrefix())
		getCancel()
		if err != nil {
			log.Printf("⚠ Failed to fetch peers: %v", err)
			time.Sleep(checkInterval)
			continue
		}

		currentPeerCount := len(resp.Kvs)

		// Nuevos peers detectados - resetear ventana de calma
		if currentPeerCount > lastPeerCount {
			if lastPeerCount == 0 {
				log.Printf("⏳ Discovered %d peers, waiting for cluster to stabilize...", currentPeerCount)
			} else {
				log.Printf("⏳ Discovered %d peers (was %d), waiting for more...", currentPeerCount, lastPeerCount)
			}
			lastPeerCount = currentPeerCount
			calmDuration = 0
			time.Sleep(checkInterval)
			continue
		}

		// No hay cambios - incrementar tiempo de calma
		calmDuration += checkInterval

		// Condición 1: Ventana de calma alcanzada (estable)
		if calmDuration >= calmWindow {
			log.Printf("✓ Peer discovery stable (%d peers found)", currentPeerCount)
			break
		}

		// Condición 2: Timeout máximo de seguridad
		if time.Since(startTime) >= maxWaitTime {
			log.Printf("⚠ Discovery timeout reached, proceeding with %d peers", currentPeerCount)
			break
		}

		time.Sleep(checkInterval)
	}

	// Procesar todos los peers descubiertos
	if err == nil {
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
			if err := tincManager.SyncHostFile(peerNodeName, peer); err != nil {
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

		// Reconcile TINC connections (file-based for TINC 1.0)
		// Updates tinc.conf and reloads daemon
		if len(peerNames) > 0 {
			log.Println()
			log.Println("Reconciling TINC connections...")
			added, removed, err := tincManager.ReconcileConnections(peerNames)
			if err != nil {
				log.Printf("⚠ Reconciliation failed: %v", err)
			} else {
				log.Printf("✓ Connections reconciled (added: %d, removed: %d)", added, removed)

				// Update metrics
				if added > 0 {
					metrics.TincConnectionOperations.WithLabelValues("add", "success").Add(float64(added))
				}
				if removed > 0 {
					metrics.TincConnectionOperations.WithLabelValues("remove", "success").Add(float64(removed))
				}

				// Display current topology
				if *verbose {
					currentConns, _ := tincManager.GetCurrentConnections()
					log.Printf("📊 Current connections: %v", currentConns)
					metrics.TincConnectionsActive.Set(float64(len(currentConns)))
				}
			}
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

					peerNodeName := extractNodeNameFromKey(key)

					// Skip own node
					if peerNodeName == *nodeName {
						if *verbose {
							log.Printf("  Skipping own node update")
						}
						continue
					}

					if *verbose {
						log.Printf("   Peer: %v", peer)
					}

					// Step 1: Sync host file (persistent)
					syncStart := time.Now()
					if err := tincManager.SyncHostFile(peerNodeName, peer); err != nil {
						log.Printf("❌ Failed to sync host file: %v", err)
						metrics.PeerSyncTotal.WithLabelValues("error", "PUT").Inc()
						continue
					}
					metrics.HostFileSyncDuration.Observe(time.Since(syncStart).Seconds())
					log.Printf("✓ Synced host file for %s", peerNodeName)

					// Step 2: Get all current peers from etcd
					getCtx, getCancel := context.WithTimeout(ctx, 3*time.Second)
					resp, err := cli.Get(getCtx, "/peers/", clientv3.WithPrefix())
					getCancel()
					if err != nil {
						log.Printf("⚠ Failed to fetch all peers: %v", err)
						metrics.PeerSyncTotal.WithLabelValues("error", "PUT").Inc()
						continue
					}

					allPeerNames := make([]string, 0)
					for _, kv := range resp.Kvs {
						pn := extractNodeNameFromKey(string(kv.Key))
						if pn != *nodeName && pn != "" {
							allPeerNames = append(allPeerNames, pn)
						}
					}

					// Step 3: Reconcile connections (TINC 1.0 file-based)
					// Updates tinc.conf and reloads daemon
					added, removed, err := tincManager.ReconcileConnections(allPeerNames)
					if err != nil {
						log.Printf("❌ Failed to reconcile connections: %v", err)
						metrics.PeerSyncTotal.WithLabelValues("error", "PUT").Inc()
					} else {
						log.Printf("✓ Connections reconciled for %s (added: %d, removed: %d)", peerNodeName, added, removed)
						metrics.PeerSyncTotal.WithLabelValues("success", "PUT").Inc()

						if added > 0 {
							metrics.TincConnectionOperations.WithLabelValues("add", "success").Add(float64(added))
						}
						if removed > 0 {
							metrics.TincConnectionOperations.WithLabelValues("remove", "success").Add(float64(removed))
						}

						// Display current topology
						if *verbose {
							currentConns, _ := tincManager.GetCurrentConnections()
							log.Printf("📊 Current connections: %v", currentConns)
							metrics.TincConnectionsActive.Set(float64(len(currentConns)))
						}
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

					log.Printf("   Removing peer: %s", deletedNodeName)

					// Step 1: Remove host file (persistent)
					if err := tincManager.RemoveHostFile(deletedNodeName); err != nil {
						log.Printf("⚠ Failed to remove host file: %v", err)
					} else {
						log.Printf("✓ Removed host file for %s", deletedNodeName)
					}

					// Step 2: Get remaining peers from etcd
					getCtx, getCancel := context.WithTimeout(ctx, 3*time.Second)
					resp, err := cli.Get(getCtx, "/peers/", clientv3.WithPrefix())
					getCancel()
					if err != nil {
						log.Printf("⚠ Failed to fetch remaining peers: %v", err)
						metrics.PeerSyncTotal.WithLabelValues("error", "DELETE").Inc()
						continue
					}

					remainingPeerNames := make([]string, 0)
					for _, kv := range resp.Kvs {
						pn := extractNodeNameFromKey(string(kv.Key))
						if pn != *nodeName && pn != "" {
							remainingPeerNames = append(remainingPeerNames, pn)
						}
					}

					// Step 3: Reconcile connections (TINC 1.0 file-based)
					// Updates tinc.conf and reloads daemon
					added, removed, err := tincManager.ReconcileConnections(remainingPeerNames)
					if err != nil {
						log.Printf("❌ Failed to reconcile connections: %v", err)
						metrics.PeerSyncTotal.WithLabelValues("error", "DELETE").Inc()
					} else {
						log.Printf("✓ Connections reconciled after removing %s (added: %d, removed: %d)", deletedNodeName, added, removed)
						metrics.PeerSyncTotal.WithLabelValues("success", "DELETE").Inc()

						if removed > 0 {
							metrics.TincConnectionOperations.WithLabelValues("remove", "success").Add(float64(removed))
						}

						// Display current topology
						if *verbose {
							currentConns, _ := tincManager.GetCurrentConnections()
							log.Printf("📊 Current connections: %v", currentConns)
							metrics.TincConnectionsActive.Set(float64(len(currentConns)))
						}
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
