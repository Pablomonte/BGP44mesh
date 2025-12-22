# Mock ISP Setup (AS 65001)

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
│   - 192.0.2.0/24 (TEST-NET-1)    - 44.30.127.0/24 (mesh)       │
│   - 198.51.100.0/24 (TEST-NET-2)                               │
│   - 203.0.113.0/24 (TEST-NET-3)                                │
│                                                                 │
│   IP: 172.30.0.1                                               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ BGP (eBGP, port 179)
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Border Router                                │
│                      AS 65000                                   │
│                                                                 │
│   IP: 172.30.0.100 (secondary IP for BGP peering)              │
│   Mesh: 44.30.127.x                                            │
│   Egress Gateway: announces external routes to mesh            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ WireGuard mesh
                              ▼
                         Mesh Nodes
                        44.30.127.x
```

## Prerequisites

1. Raspberry Pi on same LAN as Border Router
2. Docker Engine (not Docker Desktop)
3. Port 179/TCP accessible

## Configuration

Edit `bird.conf` if your IPs differ:
- `172.30.0.1` → RPi's IP
- `172.30.0.100` → Border Router's IP

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

Once the border router is configured as **egress gateway** in Netmaker:

```bash
# From RPi, ping any mesh node
ping 44.30.127.3

# Should work because:
# 1. RPi has route to 44.30.127.0/24 via Border Router (BGP)
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
ping 172.30.0.100

# Check port
nc -zv 172.30.0.100 179

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
   docker exec bird-isp birdc show route 44.30.127.0/24
   ```

3. Verify Border Router is egress gateway:
   ```bash
   # On border router host
   curl -s "https://netmaker.altermundi.net/api/nodes" \
     -H "Authorization: Bearer $MASTER_KEY" | jq '.[] | select(.isegressgateway==true)'
   ```

### Routes not being exchanged

Check filters in `bird.conf`:
- Import filter accepts `44.30.127.0/24`
- Export filter sends `isp_routes` protocol
