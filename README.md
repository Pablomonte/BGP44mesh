# BGP4mesh

BGP route distribution over a Netmaker WireGuard mesh network.

## Overview

This project implements BGP peering between two autonomous systems, with routes distributed to mesh nodes via Netmaker (WireGuard-based VPN).

```
┌────────────────┐         ┌─────────────────────────────────┐         ┌────────────────┐
│   rpi-isp      │         │       laptop-border             │         │  laptop-mesh   │
│   AS 65001     │◄──BGP──►│         AS 65000                │◄──WG───►│   mesh node    │
│  172.30.0.1    │  :179   │       172.30.0.100              │  :51821 │                │
│                │         │                                 │         │                │
│  BIRD          │         │  BIRD + Netmaker + Caddy        │         │  Netclient     │
│  announces:    │         │  44.30.127.1 (mesh)             │         │  44.30.127.x   │
│  192.0.2.0/24  │         │                                 │         │                │
│  198.51.100/24 │         │  exports to BGP:                │         │  receives:     │
│  203.0.113/24  │         │  44.30.127.0/24                 │         │  ISP routes    │
└────────────────┘         └─────────────────────────────────┘         └────────────────┘
```

## Components

| Directory | Device | Function | Software |
|-----------|--------|----------|----------|
| `deploy/rpi-isp` | Raspberry Pi | Mock ISP, AS 65001 | BIRD 2 |
| `deploy/laptop-border` | Laptop | Border router AS 65000 + Netmaker server | BIRD 2, Netmaker, Caddy, Mosquitto |
| `deploy/laptop-mesh` | Laptop/other | Mesh node | Netclient |

## Network addressing

| Network | CIDR | Purpose |
|---------|------|---------|
| Physical LAN | 172.30.0.0/24 | BGP peering between rpi-isp and laptop-border |
| Netmaker mesh | 44.30.127.0/24 | WireGuard overlay, distributed to all mesh nodes |
| TEST-NET-1 | 192.0.2.0/24 | Announced by rpi-isp (RFC 5737) |
| TEST-NET-2 | 198.51.100.0/24 | Announced by rpi-isp (RFC 5737) |
| TEST-NET-3 | 203.0.113.0/24 | Announced by rpi-isp (RFC 5737) |

## Requirements

- Docker Engine (not Docker Desktop - `network_mode: host` requires native Docker)
- Devices on same LAN for BGP peering (rpi-isp ↔ laptop-border)
- UDP connectivity for WireGuard (port 51821)

## Deployment order

### 1. rpi-isp (Mock ISP)

```bash
cd deploy/rpi-isp
# Edit bird.conf: set correct IPs
docker compose up -d
```

### 2. laptop-border (Border Router + Netmaker Server)

```bash
cd deploy/laptop-border

# Create .env
cat <<EOF > .env
SERVER_HOST=172.30.0.100
MASTER_KEY=$(openssl rand -base64 32)
EOF

# Generate TLS certificate (required for netclient)
mkdir -p certs
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout certs/server.key -out certs/server.crt \
  -subj "/CN=172.30.0.100" -addext "subjectAltName=IP:172.30.0.100"

# Install CA on host
sudo cp certs/server.crt /usr/local/share/ca-certificates/netmaker.crt
sudo update-ca-certificates

# Start services
docker compose up -d

# Wait for netmaker to start, then create network
source .env
sleep 10

curl -sk -X POST "https://localhost/api/networks" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"netid": "mesh", "addressrange": "44.30.127.0/24"}'

# Create enrollment key
curl -sk -X POST "https://localhost/api/v1/enrollment-keys" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"networks": ["mesh"], "tags": ["node"], "unlimited": true}'

# Copy the "token" field from response, add to .env
echo "ENROLLMENT_TOKEN=<token>" >> .env

# Restart netclient with token
docker compose up -d --force-recreate netclient

# Restart BIRD to detect netmaker interface
docker restart bird-border
```

### 3. laptop-mesh (Mesh Node)

```bash
cd deploy/laptop-mesh

# Install CA certificate from border router
scp user@172.30.0.100:/path/to/certs/server.crt /tmp/netmaker.crt
sudo cp /tmp/netmaker.crt /usr/local/share/ca-certificates/netmaker.crt
sudo update-ca-certificates

# Enable IP forwarding
sudo sysctl -w net.ipv4.ip_forward=1

# Create .env with enrollment token from step 2
echo "ENROLLMENT_TOKEN=<token>" > .env

docker compose up -d
```

## Verification

### BGP status (rpi-isp)
```bash
docker exec bird-isp birdc show protocols
docker exec bird-isp birdc show route
```

### BGP status (laptop-border)
```bash
docker exec bird-border birdc show protocols
docker exec bird-border birdc show route
docker exec bird-border birdc "show route export isp"
```

### Netmaker status
```bash
# Server health
curl -sk https://172.30.0.100/api/server/health

# WireGuard interface
docker exec netclient wg show

# Mesh connectivity
ping -I 44.30.127.1 172.30.0.1
```

## Ports

| Port | Protocol | Service | Node |
|------|----------|---------|------|
| 179 | TCP | BGP | rpi-isp, laptop-border |
| 443 | TCP | Netmaker API (Caddy TLS) | laptop-border |
| 1883 | TCP | MQTT (Mosquitto) | laptop-border |
| 51821 | UDP | WireGuard | laptop-border |

## Files

```
deploy/
├── laptop-border/
│   ├── docker-compose.yml    # BIRD, Netmaker, Caddy, Mosquitto, Netclient
│   ├── bird.conf             # BGP config AS 65000
│   ├── Caddyfile             # TLS reverse proxy
│   ├── mosquitto.conf        # MQTT broker
│   ├── Dockerfile            # BIRD container
│   ├── entrypoint.sh
│   ├── certs/                # TLS certificates (generated)
│   └── SETUP.md
├── laptop-mesh/
│   ├── docker-compose.yml    # Netclient only
│   └── SETUP.md
└── rpi-isp/
    ├── docker-compose.yml    # BIRD only
    ├── bird.conf             # BGP config AS 65001
    ├── Dockerfile
    ├── entrypoint.sh
    └── SETUP.md
```

## Known issues

- Netmaker v0.24.x requires HTTPS. Caddy provides TLS termination with self-signed certificates.
- `network_mode: host` does not work with Docker Desktop (uses VM). Use native Docker Engine.
- BIRD must be restarted after netclient creates the WireGuard interface to learn the route.
- `sysctls` in docker-compose is ignored with `network_mode: host`. Set `ip_forward` on the host.

## Security considerations

This setup uses insecure defaults for testing:

- Self-signed TLS certificates
- MQTT broker allows anonymous connections
- MASTER_KEY stored in plaintext .env files

For production: use proper CA certificates, enable MQTT authentication, use secrets management.
