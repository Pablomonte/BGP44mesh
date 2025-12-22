# Mesh Node Setup

> [← Back to main README](../../README.md)

Connects to the Netmaker server at `${SERVER_HOST}`.

## Prerequisites

- The Netmaker server must be deployed first (see [../netmaker/README.md](../netmaker/README.md))
- An enrollment token from the Netmaker server

## Before deploying

### 1. Get enrollment token

From the Netmaker server, get the enrollment token:

```bash
# On the Netmaker server (or any machine with API access)
source ../netmaker/.env

curl -s "https://${SERVER_HOST}/api/v1/enrollment-keys" \
  -H "Authorization: Bearer $MASTER_KEY" | jq -r '.[0].token'
```

Or create a new enrollment key:

```bash
curl -X POST "https://${SERVER_HOST}/api/v1/enrollment-keys" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d "{
    \"networks\": [\"${MESH_NETWORK_ID}\"],
    \"tags\": [\"mesh-node\"],
    \"unlimited\": true
  }"
```

### 2. Create `.env` file

```bash
cp .env.example .env
# Edit .env and set ENROLLMENT_TOKEN from step 1
```

**Environment variables:**

| Variable | Description |
|----------|-------------|
| ENROLLMENT_TOKEN | Token from Netmaker server (get via API or UI) |

### 3. Enable IP forwarding on host

```bash
sudo sysctl -w net.ipv4.ip_forward=1
# Make persistent:
echo "net.ipv4.ip_forward=1" | sudo tee /etc/sysctl.d/99-ipforward.conf
```

### 4. Network requirements

- UDP connectivity to Netmaker server on port 51821 (WireGuard)
- HTTPS connectivity to Netmaker server on port 443
- Can be on different LAN than other nodes (Netmaker handles NAT traversal)

## Deploy

```bash
# For fresh install (or if changing Netmaker servers):
docker compose down -v  # Remove old volumes with cached config
docker compose up -d
```

> **Note:** The `netclient_data` volume caches server configuration. If you previously
> connected to a different Netmaker server, remove the volume first or the client
> will use stale broker addresses.

## Verify

```bash
# Check Netmaker client logs
docker logs netclient

# Check WireGuard tunnel
docker exec netclient wg show

# Check mesh connectivity (ping other mesh nodes)
ping -c 3 <mesh-node-ip>  # any IP in ${MESH_ADDRESS_RANGE}

# Check routes received via mesh
ip route | grep "$(echo ${MESH_ADDRESS_RANGE} | cut -d/ -f1)"
```

## Troubleshooting

### Registration failed

If netclient can't register:

```bash
# Check Netmaker API health
curl https://${SERVER_HOST}/api/server/health

# Verify the enrollment token is correct
echo $ENROLLMENT_TOKEN | base64 -d | jq .

# Check firewall allows HTTPS and WireGuard
curl -I https://${SERVER_HOST}
```

### WireGuard tunnel not established

```bash
# Check netclient status
docker exec netclient netclient list

# Force reconnect
docker compose restart netclient
```

### No routes received

Ensure the Border Router is connected to the mesh and announcing routes via BGP.

## Architecture

```text
                    Netmaker Server
                    ${SERVER_HOST}
                         :443
                          │
         ┌────────────────┼────────────────┐
         │                │                │
         ▼                ▼                ▼
    ┌─────────┐     ┌─────────┐     ┌─────────┐
    │ Border  │     │  Mesh   │     │  Mesh   │
    │ Router  │     │ Node 1  │     │ Node 2  │
    │ (BIRD)  │     │         │     │         │
    └────┬────┘     └─────────┘     └─────────┘
         │
    BGP to ISP
```
