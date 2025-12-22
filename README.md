# BGP4mesh

Run your own Autonomous System (AS) with BGP peering and a WireGuard mesh network.

## Goal

Create an independent AS that:
- Announces your IP block to the Internet via BGP
- Provides connectivity to distributed nodes through a WireGuard mesh
- Enables you to host services accessible from the public Internet

## Architecture

```
                              INTERNET
                                  │
                                  │ BGP (your AS announced globally)
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          ISP / IXP (datacenter)                             │
│                          (not under your control)                           │
└─────────────────────────────────────────────────────────────────────────────┘
                                  │
                                  │ BGP peering (eBGP)
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         BORDER ROUTER (your AS)                             │
│                                                                             │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐                  │
│   │    BIRD     │     │  netclient  │     │  Egress GW  │                  │
│   │  AS 65000   │     │  (WireGuard)│     │  announces  │                  │
│   │             │     │             │     │  external   │                  │
│   │ announces:  │     │ mesh IP:    │     │  routes to  │                  │
│   │ 44.30.127.0 │     │ 44.30.127.x │     │  mesh       │                  │
│   │ /24         │     │             │     │             │                  │
│   └─────────────┘     └─────────────┘     └─────────────┘                  │
│                                                                             │
│   Connects your AS to both: Internet (BGP) and your mesh (WireGuard)       │
└─────────────────────────────────────────────────────────────────────────────┘
                                  │
                                  │ WireGuard mesh (Netmaker)
                                  ▼
┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐
│   Mesh Node 1    │    │   Mesh Node 2    │    │   Mesh Node N    │
│   44.30.127.2    │    │   44.30.127.3    │    │   44.30.127.x    │
│                  │    │                  │    │                  │
│   netclient      │    │   netclient      │    │   netclient      │
│   (anywhere)     │    │   (anywhere)     │    │   (anywhere)     │
└──────────────────┘    └──────────────────┘    └──────────────────┘
```

## How It Works

### Outbound (mesh → Internet)

1. A mesh node (44.30.127.3) wants to reach the Internet
2. Traffic goes to the **border router** (egress gateway)
3. Border router forwards to ISP via BGP peering
4. Response comes back the same path

### Inbound (Internet → mesh)

1. Someone on the Internet wants to reach 44.30.127.3
2. BGP routing directs traffic to your ISP (your AS is announced)
3. ISP sends to your **border router**
4. Border router forwards via WireGuard mesh to the node

### Key Insight: Egress Gateway

The border router must be configured as an **egress gateway** in Netmaker. This announces external routes (e.g., `0.0.0.0/0` or specific ranges) to all mesh nodes, so they know how to reach the Internet through the border router.

Without this, mesh nodes wouldn't know how to route responses back to external IPs.

## Components

| Component | Location | Purpose |
|-----------|----------|---------|
| `deploy/netmaker/` | Public server | Netmaker control plane (mesh management) |
| `deploy/bird-border/` | Datacenter | Border router: BGP + mesh gateway |
| `deploy/netclient/` | Any location | Standalone mesh node |

### Netmaker Server (`deploy/netmaker/`)

Central control plane for the WireGuard mesh. Runs on a public server with:
- Netmaker API (behind nginx with Let's Encrypt)
- Mosquitto MQTT broker
- WireGuard coordination (no traffic passes through it)

### Border Router (`deploy/bird-border/`)

The critical component that bridges your AS to the Internet:
- **BIRD**: BGP daemon, announces your IP block to the ISP
- **netclient**: Connects to the mesh
- **Egress Gateway**: Announces external routes to mesh nodes

### Mesh Nodes (`deploy/netclient/`)

Simple nodes that join the mesh:
- Run netclient to establish WireGuard tunnels
- Receive routes from egress gateway
- Can host services accessible from the Internet

## Network Addressing

| Network | CIDR | Purpose |
|---------|------|---------|
| Your AS block | 44.30.127.0/24 | Public IPs announced via BGP |
| Mesh overlay | (same as above) | WireGuard mesh uses your public block |
| BGP peering | (varies) | Link between you and ISP |

**Note:** In this design, mesh IPs are your public IPs. This means services on mesh nodes are directly reachable from the Internet once BGP is established.

## Deployment

### Prerequisites

1. **IP allocation**: Obtain IP block from RIR (LACNIC, ARIN, etc.) or lease from provider
2. **AS number**: Obtain from RIR or use private AS (64512-65534) for testing
3. **BGP peering**: Agreement with ISP or IXP for BGP session
4. **Public server**: For Netmaker control plane
5. **Datacenter presence**: For border router (colocation or VPS with BGP support)

### 1. Deploy Netmaker Server

```bash
cd deploy/netmaker
cp .env.example .env
# Edit .env with your domain and MASTER_KEY
docker compose up -d
```

See `deploy/netmaker/SETUP.md` for full instructions.

### 2. Create Mesh Network

```bash
source .env
curl -X POST "https://your-netmaker-domain/api/networks" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"netid": "mynet", "addressrange": "44.30.127.0/24"}'
```

### 3. Deploy Border Router

```bash
cd deploy/bird-border
cp .env.example .env
# Add ENROLLMENT_TOKEN from Netmaker
docker compose up -d
```

See `deploy/bird-border/SETUP.md` for BGP configuration.

### 4. Configure Egress Gateway

Make the border router announce external routes to the mesh:

```bash
# Via Netmaker API
curl -X POST "https://your-netmaker-domain/api/nodes/mynet/<node-id>/creategateway" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"ranges":["0.0.0.0/0"],"natEnabled":"no"}'
```

Or use Netmaker UI: Node → Edit → Enable Egress Gateway → Add range `0.0.0.0/0`

### 5. Deploy Mesh Nodes

```bash
cd deploy/netclient
cp .env.example .env
# Add ENROLLMENT_TOKEN
docker compose up -d
```

## Testing with Mock ISP

For development without real BGP peering, use a Raspberry Pi as mock ISP:

```
┌─────────────────┐                    ┌─────────────────┐
│   RPi (mock)    │◄───── BGP ────────►│  Border Router  │
│   AS 65001      │     172.30.0.x     │    AS 65000     │
│   172.30.0.1    │                    │   172.30.0.100  │
│                 │                    │   44.30.127.x   │
│ announces test  │                    │                 │
│ prefixes        │                    │ egress gateway  │
└─────────────────┘                    └─────────────────┘
                                              │
                                              │ mesh
                                              ▼
                                       ┌─────────────────┐
                                       │  Mesh Nodes     │
                                       │  44.30.127.x    │
                                       └─────────────────┘
```

The border router needs a secondary IP in the RPi's network for BGP peering:
```bash
sudo ip addr add 172.30.0.100/24 dev wlp0s20f3
```

## Verification

### BGP Session
```bash
docker exec bird-border birdc show protocols
docker exec bird-border birdc show route
```

### Mesh Connectivity
```bash
docker exec netclient wg show
ping 44.30.127.x  # other mesh nodes
```

### End-to-End (from mock ISP)
```bash
# From RPi, should reach any mesh node
ping 44.30.127.3
```

## Production Considerations

### Security
- Use proper TLS certificates (Let's Encrypt)
- Enable MQTT authentication
- Use secrets management for MASTER_KEY
- Configure firewalls appropriately

### High Availability
- Multiple border routers with BGP failover
- Netmaker can run in HA mode
- Consider anycast for critical services

### IP Space
- For real deployment, use legitimately obtained IP space
- 44.30.127.0/24 is used here for testing (AMPRNet allocation)
- Contact your RIR for production IP allocation

## Project Structure

```
deploy/
├── netmaker/           # Netmaker server (public)
│   ├── docker-compose.yml
│   ├── mosquitto.conf
│   └── SETUP.md
├── bird-border/        # Border router + netclient
│   ├── docker-compose.yml
│   ├── bird.conf
│   ├── Dockerfile
│   ├── entrypoint.sh
│   └── SETUP.md
└── netclient/          # Standalone mesh node
    ├── docker-compose.yml
    └── SETUP.md
```

## References

- [Netmaker Documentation](https://docs.netmaker.io/)
- [BIRD Internet Routing Daemon](https://bird.network.cz/)
- [BGP RFC 4271](https://datatracker.ietf.org/doc/html/rfc4271)
- [WireGuard](https://www.wireguard.com/)

## License

MIT
