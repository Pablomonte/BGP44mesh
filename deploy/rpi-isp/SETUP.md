# RPi ISP Setup (AS 65001)

## Before deploying

### 1. Configure IPs

Edit `bird.conf` and replace:
- `172.30.0.1` → Your Raspberry Pi's physical IP
- `172.30.0.100` → Border Router's physical IP (BGP neighbor)

### 2. Network requirements

- The RPi must be on the same LAN as the Border Router (Laptop n1)
- Port 179/TCP must be reachable (BGP)

## Deploy

```bash
docker compose up -d
```

## Verify

```bash
# Check BIRD status
docker exec bird-isp birdc show status

# Check BGP session
docker exec bird-isp birdc show protocols

# Check routes being announced
docker exec bird-isp birdc show route
```

