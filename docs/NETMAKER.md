# Netmaker Setup Guide

## What is Netmaker?

Netmaker creates WireGuard-based VPN mesh networks. Nodes connect to a central server that manages the mesh topology and distributes WireGuard configurations.

## Why Netmaker over TINC?

- **Route distribution**: Netmaker automatically propagates routes to all mesh nodes
- No need for iBGP between mesh nodes — Netmaker handles it
- Modern WireGuard-based (faster, simpler than TINC)

## Architecture for this project

- **Netmaker Server**: Runs on Laptop n1 (Border Router) - manages the mesh
- **Netmaker Clients**: All mesh nodes including Laptop n1 and n2

## Docker Setup

### Server (on Border Router - Laptop n1)

The Netmaker server needs:
- PostgreSQL or SQLite for data
- CoreDNS for DNS (optional)
- Caddy/Traefik for HTTPS (production)

For local testing, we use the minimal setup without HTTPS.

### Client (on all mesh nodes)

Netclient runs as a container or directly on host. It:
- Registers with the Netmaker server
- Receives WireGuard config
- Maintains the VPN tunnel

## Key Configuration

```yaml
# Essential environment variables for Netmaker server
NETMAKER_BASE_DOMAIN: nm.local          # Your domain
SERVER_HOST: 172.30.0.100               # Server's physical IP
MASTER_KEY: <generate-secure-key>       # API master key
MQ_HOST: mq                             # Message queue host
```

## Network Design

| Network | CIDR | Purpose |
|---------|------|---------|
| Physical LAN | 172.30.0.0/24 | Device-to-device (BGP runs here) |
| Netmaker Mesh | 44.30.127.0/24 | VPN overlay (mesh traffic) |

## ⚠️ Manual Setup Required

After deploying Netmaker server, you must manually create the network via API:

```bash
# 1. Create network
curl -X POST "http://<SERVER_HOST>:8081/api/networks" \
  -H "Authorization: Bearer <MASTER_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"netid": "mesh", "addressrange": "44.30.127.0/24"}'

# 2. Create enrollment key for nodes
curl -X POST "http://<SERVER_HOST>:8081/api/v1/enrollment-keys" \
  -H "Authorization: Bearer <MASTER_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"networks": ["mesh"], "unlimited": true}'
```

Save the enrollment token — needed for all nodes to join.

## Commands

```bash
# Check mesh status
docker exec netclient netclient list

# View WireGuard interfaces
docker exec netclient wg show
```

## Documentation

- Official docs: https://docs.netmaker.io/
- Docker install: https://docs.netmaker.io/quick-start.html
- API reference: https://docs.netmaker.io/api.html

## Notes

- Netmaker uses WireGuard under the hood (port 51821 by default)
- The server needs ports: 8081 (API), 51821/UDP (WireGuard), 1883 (MQTT)
- Clients need UDP connectivity to server and peers

## Security (TODO for production)

⚠️ Current setup uses insecure defaults for testing:
- `MASTER_KEY` in plain text `.env` files
- MQTT broker allows anonymous connections
- No HTTPS/TLS

Before production deployment:
- [ ] Use secrets management (Docker secrets, Vault, etc.)
- [ ] Enable MQTT authentication
- [ ] Add Caddy/Traefik for HTTPS
- [ ] Restrict network access with firewall rules

