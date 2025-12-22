# Border Router Setup

> [← Back to main README](../../README.md)

Border node that connects the Netmaker mesh to external networks via BGP.

## Components

| Service | Purpose | Network |
|---------|---------|---------|
| netclient | WireGuard mesh client | host (creates netmaker interface) |
| bird | BGP daemon (AS 65000) | host (peers with 172.30.0.1) |

## Prerequisites

1. **Netmaker server running** at `netmaker.altermundi.net` (see [../netmaker/README.md](../netmaker/README.md))
2. **Enrollment token** from Netmaker (create via API or UI)
3. **Host requirements**:
   - `net.ipv4.ip_forward=1` enabled
   - IP address in the 172.30.0.x network (for BGP peering with RPi ISP)

## Network Configuration

| Address | Role |
|---------|------|
| 172.30.0.100 | This host (secondary IP for BGP) |
| 172.30.0.1 | RPi ISP (BGP neighbor) |
| 44.30.127.0/24 | Mesh network (Netmaker) |
| 44.30.127.x | This host (mesh IP, assigned by Netmaker) |

## Deploy

### 1. Configure environment

```bash
cp .env.example .env
# Edit .env and add ENROLLMENT_TOKEN
```

**Environment variables:**

| Variable | Description |
|----------|-------------|
| ENROLLMENT_TOKEN | Token from Netmaker server (get via API or UI) |

### 2. Add secondary IP for BGP peering

The border router needs an IP in the 172.30.0.x network to peer with the RPi ISP.

**Temporary (until reboot):**
```bash
sudo ip addr add 172.30.0.100/24 dev wlp0s20f3
```

**Persistent with NetworkManager:**
```bash
# Find your WiFi connection name
nmcli con show

# Add secondary IP
nmcli con mod "<wifi-connection-name>" +ipv4.addresses 172.30.0.100/24
nmcli con up "<wifi-connection-name>"
```

**Persistent with /etc/network/interfaces.d/:**
```bash
cat <<EOF | sudo tee /etc/network/interfaces.d/bgp-peering
# Secondary IP for BGP peering with RPi ISP
auto wlp0s20f3:1
iface wlp0s20f3:1 inet static
    address 172.30.0.100
    netmask 255.255.255.0
EOF
```

Verify connectivity:
```bash
ping -c 2 172.30.0.1
```

### 3. Enable IP forwarding on host

```bash
sudo sysctl -w net.ipv4.ip_forward=1
# Make persistent:
echo "net.ipv4.ip_forward=1" | sudo tee /etc/sysctl.d/99-forward.conf
```

### 4. Start services

```bash
docker compose up -d
```

The startup sequence:
1. `netclient` starts and creates WireGuard interface `netmaker`
2. `bird` waits for interface (healthcheck)
3. `bird` starts BGP peering with RPi ISP

### 5. Verify

```bash
# Check netclient/WireGuard
docker exec netclient wg show

# Check BIRD status
docker exec bird-border birdc show status
docker exec bird-border birdc show protocols

# Check BGP routes
docker exec bird-border birdc show route
docker exec bird-border birdc "show route export isp"

# Test mesh connectivity (to another mesh node)
ping 44.30.127.1
```

## Architecture

```
                         netmaker.altermundi.net
                                  │
                                  │ WireGuard (51821/udp)
                                  │
┌─────────────────────────────────┼──────────────────────────┐
│            Border Router        │                          │
│                                 ▼                          │
│  ┌───────────────┐      ┌─────────────┐                   │
│  │   netclient   │──────│  netmaker   │ 44.30.127.x       │
│  │               │      │  interface  │                   │
│  └───────────────┘      └──────┬──────┘                   │
│                                │                           │
│                         ┌──────┴──────┐                   │
│  BGP :179               │    BIRD     │                   │
│  ◄──────────────────────│   AS 65000  │                   │
│                         └─────────────┘                   │
│                                                            │
│  Secondary IP: 172.30.0.100 (on WiFi)                     │
└────────────────────────────────────────────────────────────┘
         │
         │ BGP peering
         ▼
   RPi ISP (172.30.0.1)
       AS 65001
```

## BGP Configuration

Current `bird.conf` announces:
- **Export to ISP**: `44.30.127.0/24` (mesh network)
- **Import from ISP**: `192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24` (test prefixes)

## Troubleshooting

### netclient won't connect
```bash
docker logs netclient
# Check token is valid and netmaker.altermundi.net is reachable
curl -s https://netmaker.altermundi.net/api/server/health
```

### BIRD won't start
```bash
docker logs bird-border
# Usually waiting for nm-mesh interface
docker exec netclient wg show
```

### BGP session not established
```bash
docker exec bird-border birdc show protocols all isp
# Check RPi ISP is reachable
ping 172.30.0.1
```

### Interface not detected
If BIRD can't find the WireGuard interface:
```bash
# Check actual interface name
ip link | grep -E 'netmaker|nm-'

# The entrypoint auto-detects interfaces matching 'netmaker' or 'nm-*'
docker logs bird-border
```

For detailed Netmaker configuration and troubleshooting, see [../../docs/NETMAKER.md](../../docs/NETMAKER.md).
