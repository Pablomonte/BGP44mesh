# Border Router Setup (AS 65000)

## Before deploying

### 1. Configure IPs

Edit `bird.conf` and replace:
- `172.30.0.100` → This laptop's physical IP
- `172.30.0.1` → RPi ISP's physical IP (BGP neighbor)

### 2. Create `.env` file

```bash
cat <<EOF > .env
SERVER_HOST=172.30.0.100
MASTER_KEY=$(openssl rand -base64 32)
ENROLLMENT_TOKEN=
EOF
```

**Note:** `ENROLLMENT_TOKEN` is set after creating the network in Netmaker (see step 5).

### 3. Network requirements

- Same LAN as RPi ISP
- Ports needed:
  - 179/TCP (BGP)
  - 443/TCP (Netmaker API via Caddy TLS proxy)
  - 51821/UDP (WireGuard)
  - 1883/TCP (MQTT)

### 4. Install CA certificate on host

The self-signed certificate must be trusted by the host for netclient to work:

```bash
sudo cp certs/server.crt /usr/local/share/ca-certificates/netmaker.crt
sudo update-ca-certificates
```

## Deploy

```bash
docker compose up -d
```

### 5. Create Netmaker network

After deployment, create the mesh network via API:

```bash
source .env

# Create network
curl -sk -X POST "https://localhost/api/networks" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "netid": "mesh",
    "addressrange": "44.30.127.0/24"
  }'

# Create enrollment key
curl -sk -X POST "https://localhost/api/v1/enrollment-keys" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "networks": ["mesh"],
    "tags": ["mesh-node"],
    "unlimited": true
  }'
```

Save the `token` field from the enrollment key response.

### 6. Enroll this node

Update `.env` with the `ENROLLMENT_TOKEN` and restart netclient:

```bash
# Add token to .env
echo "ENROLLMENT_TOKEN=<token>" >> .env

# Restart netclient
docker compose up -d --force-recreate netclient
```

### 7. Reload BIRD to detect netmaker interface

After netclient creates the WireGuard interface, restart BIRD:

```bash
docker restart bird-border
```

## Verify

```bash
# Check BIRD/BGP
docker exec bird-border birdc show protocols
docker exec bird-border birdc show route
docker exec bird-border birdc "show route export isp"

# Check Netmaker
docker logs netmaker
docker logs netclient
docker exec netclient wg show

# Check mesh connectivity from netmaker IP
ping -I 44.30.127.1 -c 3 172.30.0.1

# Check API health
curl -sk https://localhost/api/server/health
```

## Architecture

```
                    ┌─────────────────────────────────────────────┐
                    │           laptop-border (this)              │
                    │                                             │
   ISP (172.30.0.1) │  ┌─────────┐    ┌─────────┐   ┌──────────┐ │
   ◄────BGP:179────►│  │  BIRD   │    │Netmaker │   │ Caddy    │ │
                    │  │ AS65000 │    │ Server  │◄──│ :443 TLS │ │
                    │  └────┬────┘    └────┬────┘   └──────────┘ │
                    │       │              │                      │
                    │       │         ┌────┴────┐                 │
                    │       └────────►│Netclient│◄── WireGuard    │
                    │                 │44.30.127.1   :51821       │
                    │                 └─────────┘                 │
                    └─────────────────────────────────────────────┘
                                          │
                                          │ WireGuard tunnel
                                          ▼
                                   Other mesh nodes
```

## Security Note

⚠️ Current setup uses self-signed certificates for testing. For production:
- Use Let's Encrypt or proper CA certificates
- Use secrets management for `MASTER_KEY`
- Enable MQTT authentication in `mosquitto.conf`
