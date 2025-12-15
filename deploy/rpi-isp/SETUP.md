# RPi ISP Setup (AS 65001)

This is a mock ISP that announces test prefixes via BGP to the Border Router.

## Before deploying

### 1. Configure IPs

Edit `bird.conf` and replace:
- `172.30.0.1` → Your Raspberry Pi's physical IP
- `172.30.0.100` → Border Router's physical IP (BGP neighbor)

### 2. Network requirements

- The RPi must be on the same LAN as the Border Router (Laptop n1)
- Port 179/TCP must be reachable (BGP)

### 3. Use native Docker (not Docker Desktop)

If using Docker Desktop on the RPi, `network_mode: host` won't work properly.
Use native Docker Engine:

```bash
docker context use default
```

## Deploy

```bash
docker compose up -d
```

## Verify

```bash
# Check BIRD status
docker exec bird-isp birdc show status

# Check BGP session (should show "Established")
docker exec bird-isp birdc show protocols

# Check routes being announced
docker exec bird-isp birdc show route

# Check routes received from Border Router
docker exec bird-isp birdc "show route protocol border_router"
```

## Test prefixes announced

| Prefix | Description |
|--------|-------------|
| 192.0.2.0/24 | TEST-NET-1 (RFC 5737) |
| 198.51.100.0/24 | TEST-NET-2 (RFC 5737) |
| 203.0.113.0/24 | TEST-NET-3 (RFC 5737) |

## Expected routes received

Once the Border Router and mesh are up, you should receive:

| Prefix | Description |
|--------|-------------|
| 44.30.127.0/24 | Netmaker mesh network |

## Troubleshooting

### BGP session not establishing
- Check both devices are on the same LAN
- Verify port 179 is not blocked by firewall
- Check IPs in bird.conf match actual interfaces

### No routes received
- Verify Border Router has netclient running
- Check BIRD on Border Router sees the `netmaker` interface:
  ```bash
  docker exec bird-border birdc show protocols all direct1
  ```
