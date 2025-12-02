# Mesh Node Setup (Laptop n2)

## Before deploying

### 1. Get enrollment token

From the Border Router (Laptop n1), get the enrollment token created during its setup.

### 2. Create `.env` file

```bash
echo "ENROLLMENT_TOKEN=<token-from-border-router>" > .env
```

### 3. Network requirements

- UDP connectivity to Border Router on port 51821 (WireGuard)
- Can be on different LAN than other nodes (Netmaker handles NAT traversal)

## Deploy

```bash
docker compose up -d
```

## Verify

```bash
# Check Netmaker client
docker exec netclient netclient list

# Check WireGuard tunnel
docker exec netclient wg show

# Check received routes (from ISP via Border Router)
ip route | grep 192.0.2    # TEST-NET-1
ip route | grep 198.51.100 # TEST-NET-2
ip route | grep 203.0.113  # TEST-NET-3
```

## How routes arrive

1. ISP (AS 65001) announces test prefixes via BGP
2. Border Router (AS 65000) learns them via eBGP
3. Netmaker distributes routes to all mesh nodes
4. This node receives routes through the WireGuard tunnel

