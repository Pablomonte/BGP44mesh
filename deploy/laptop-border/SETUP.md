# Border Router Setup (AS 65000)

## Before deploying

### 1. Configure IPs

Edit `bird.conf` and replace:
- `172.30.0.100` → This laptop's physical IP
- `172.30.0.1` → RPi ISP's physical IP (BGP neighbor)

### 2. Create `.env` file

```bash
cp <<EOF > .env
SERVER_HOST=<your-physical-ip>
MASTER_KEY=<generate-secure-key>
ENROLLMENT_TOKEN=
EOF
```

**Note:** `ENROLLMENT_TOKEN` is set after creating the network in Netmaker (see step 4).

### 3. Network requirements

- Same LAN as RPi ISP
- Ports needed:
  - 179/TCP (BGP)
  - 8081/TCP (Netmaker API)
  - 51821/UDP (WireGuard)
  - 1883/TCP (MQTT)

## Deploy

```bash
docker compose up -d
```

### 4. Create Netmaker network (manual step)

After deployment, create the mesh network via API:

```bash
# Create network
curl -X POST "http://localhost:8081/api/networks" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "netid": "mesh",
    "addressrange": "44.30.127.0/24"
  }'

# Create enrollment key
curl -X POST "http://localhost:8081/api/v1/enrollment-keys" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "networks": ["mesh"],
    "unlimited": true
  }'
```

Save the enrollment token from the response.

### 5. Enroll this node

Update `.env` with the `ENROLLMENT_TOKEN` and restart:

```bash
docker compose up -d netclient
```

## Verify

```bash
# Check BIRD/BGP
docker exec bird-border birdc show protocols
docker exec bird-border birdc show route

# Check Netmaker
docker exec netclient netclient list
docker exec netclient wg show
```

## Security Note

⚠️ Current setup uses plain-text credentials for testing. Before production:
- Use secrets management for `MASTER_KEY`
- Enable MQTT authentication in `mosquitto.conf`

