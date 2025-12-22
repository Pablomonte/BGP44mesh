# Mock ISP Setup (AS 65001)

> [← Back to main README](../../README.md)

Simulates an upstream ISP for testing BGP peering without real Internet connectivity.

## Purpose

This mock ISP allows you to:
- Test BGP session establishment with your border router
- Verify route announcements in both directions
- Test end-to-end connectivity from "external" networks to mesh nodes

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Mock ISP (this)                              │
│                      AS 65001                                   │
│                                                                 │
│   Announces test prefixes:        Receives from border router:  │
│   - ${TEST_PREFIX_1}             - ${MESH_ADDRESS_RANGE}       │
│   - ${TEST_PREFIX_2}                                           │
│   - ${TEST_PREFIX_3}                                           │
│                                                                 │
│   IP: ${ISP_IP}                                                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ BGP (eBGP, port 179)
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Border Router                                │
│                      AS ${BORDER_ROUTER_AS}                     │
│                                                                 │
│   IP: ${BORDER_ROUTER_IP} (secondary IP for BGP peering)       │
│   Mesh: from ${MESH_ADDRESS_RANGE}                             │
│   Egress Gateway: announces external routes to mesh            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ WireGuard mesh
                              ▼
                         Mesh Nodes
                    ${MESH_ADDRESS_RANGE}
```

## Prerequisites

1. Raspberry Pi on same LAN as Border Router
2. Docker Engine (not Docker Desktop)
3. Port 179/TCP accessible

## Configuration

Copy and customize the environment file:

```bash
cp .env.example .env
# Edit .env with your network configuration
```

All IP addresses, AS numbers, and network ranges are configured via environment variables.

## Deploy

```bash
docker compose up -d
```

## Verify

```bash
# BGP session status
docker exec bird-isp birdc show protocols

# Routes announced (should show TEST-NET prefixes)
docker exec bird-isp birdc show route

# Routes received from Border Router (mesh network)
docker exec bird-isp birdc "show route protocol border_router"
```

## Test End-to-End Connectivity

Once the border router is configured as **egress gateway** in Netmaker (see [../bird-border/README.md](../bird-border/README.md) for setup):

```bash
# From RPi, ping any mesh node
ping <mesh-node-ip>  # any IP in ${MESH_ADDRESS_RANGE}

# Should work because:
# 1. RPi has route to ${MESH_ADDRESS_RANGE} via Border Router (BGP)
# 2. Border Router forwards to mesh node via WireGuard
# 3. Mesh node responds via Border Router (egress gateway route)
# 4. Border Router forwards response back to RPi
```

## Test Prefixes

| Prefix | Purpose |
|--------|---------|
| 192.0.2.0/24 | TEST-NET-1 (RFC 5737) |
| 198.51.100.0/24 | TEST-NET-2 (RFC 5737) |
| 203.0.113.0/24 | TEST-NET-3 (RFC 5737) |

These are documentation prefixes that should never appear on the real Internet.

## Troubleshooting

### BGP session stuck in "Active" or "Connect"

```bash
# Check connectivity
ping ${BORDER_ROUTER_IP}

# Check port
nc -zv ${BORDER_ROUTER_IP} 179

# Check BIRD logs
docker logs bird-isp
```

### Ping to mesh nodes fails

1. Verify BGP is Established:
   ```bash
   docker exec bird-isp birdc show protocols
   ```

2. Verify route to mesh exists:
   ```bash
   docker exec bird-isp birdc show route ${MESH_ADDRESS_RANGE}
   ```

3. Verify Border Router is egress gateway:
   ```bash
   # On border router host
   curl -s "https://${SERVER_HOST}/api/nodes" \
     -H "Authorization: Bearer $MASTER_KEY" | jq '.[] | select(.isegressgateway==true)'
   ```

### Routes not being exchanged

Check filters in BIRD configuration (generated from `bird.conf.template`):
- Import filter accepts `${MESH_ADDRESS_RANGE}`
- Export filter sends test prefix routes
