# Border Router Setup

> [← Back to main README](../../README.md)

Border node that connects the Netmaker mesh to external networks via BGP.

## Components

| Service | Purpose | Network |
|---------|---------|---------|
| netclient | WireGuard mesh client | host (creates netmaker interface) |
| bird | BGP daemon (AS ${BORDER_ROUTER_AS}) | host (peers with ${ISP_IP}) |

## Prerequisites

1. **Netmaker server running** at `${SERVER_HOST}` (see [../netmaker/README.md](../netmaker/README.md))
2. **Enrollment token** from Netmaker (create via API or UI)
3. **Host requirements**:
   - `net.ipv4.ip_forward=1` enabled
   - IP address in the BGP peering network (for BGP peering with ISP)

## Network Configuration

| Address | Role |
|---------|------|
| ${BORDER_ROUTER_IP} | This host (secondary IP for BGP) |
| ${ISP_IP} | ISP/Peer (BGP neighbor) |
| ${MESH_ADDRESS_RANGE} | Mesh network (Netmaker) |
| (from mesh range) | This host (mesh IP, assigned by Netmaker) |

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

The border router needs an IP in the BGP peering network to peer with the ISP.

**Temporary (until reboot):**
```bash
sudo ip addr add ${BORDER_ROUTER_IP}/24 dev ${BORDER_ROUTER_INTERFACE}
```

**Persistent with NetworkManager:**
```bash
# Find your connection name
nmcli con show

# Add secondary IP
nmcli con mod "<connection-name>" +ipv4.addresses ${BORDER_ROUTER_IP}/24
nmcli con up "<connection-name>"
```

**Persistent with /etc/network/interfaces.d/:**
```bash
cat <<EOF | sudo tee /etc/network/interfaces.d/bgp-peering
# Secondary IP for BGP peering with ISP
auto ${BORDER_ROUTER_INTERFACE}:1
iface ${BORDER_ROUTER_INTERFACE}:1 inet static
    address ${BORDER_ROUTER_IP}
    netmask 255.255.255.0
EOF
```

Verify connectivity:
```bash
ping -c 2 ${ISP_IP}
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
ping <mesh-node-ip>  # any IP in ${MESH_ADDRESS_RANGE}
```

## Architecture

```
                         ${SERVER_HOST}
                                  │
                                  │ WireGuard
                                  │
┌─────────────────────────────────┼──────────────────────────┐
│            Border Router        │                          │
│                                 ▼                          │
│  ┌───────────────┐      ┌─────────────┐                   │
│  │   netclient   │──────│  netmaker   │ mesh IP           │
│  │               │      │  interface  │                   │
│  └───────────────┘      └──────┬──────┘                   │
│                                │                           │
│                         ┌──────┴──────┐                   │
│  BGP :179               │    BIRD     │                   │
│  ◄──────────────────────│ AS ${BORDER │                   │
│                         │  _ROUTER_AS}│                   │
│                         └─────────────┘                   │
│                                                            │
│  Secondary IP: ${BORDER_ROUTER_IP}                        │
└────────────────────────────────────────────────────────────┘
         │
         │ BGP peering
         ▼
   ISP (${ISP_IP})
   AS ${ISP_AS}
```

## BGP Configuration

Configuration is managed via environment variables in `.env`:

- **Export to ISP**: `${MESH_ADDRESS_RANGE}` (mesh network)
- **Import from ISP**: Test prefixes (`${TEST_PREFIX_1}`, `${TEST_PREFIX_2}`, `${TEST_PREFIX_3}`)

The BIRD configuration is generated from `bird.conf.template` at container startup using values from your `.env` file.

## Troubleshooting

### netclient won't connect
```bash
docker logs netclient
# Check token is valid and Netmaker server is reachable
curl -s https://${SERVER_HOST}/api/server/health
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
# Check ISP/peer is reachable
ping ${ISP_IP}
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
