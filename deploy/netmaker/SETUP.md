# Netmaker Server Setup (Core Components)

Minimal deployment with Netmaker server and Mosquitto MQTT broker.

## Components

| Service | Purpose | Port |
|---------|---------|------|
| netmaker | Mesh VPN server | 8443 (HTTPS API) |
| mq | MQTT broker (Mosquitto) | 1883 |

## Before deploying

### 1. DNS Setup

Point a domain to this server's public IP:
```
netmaker.example.com -> YOUR_PUBLIC_IP
```

### 2. Create `.env` file

```bash
cat <<EOF > .env
SERVER_HOST=netmaker.example.com
MASTER_KEY=$(openssl rand -base64 32)
EOF
```

### 3. Firewall / Network requirements

Open ports:
- 8443/TCP (Netmaker API)
- 51821/UDP (WireGuard)
- 1883/TCP (MQTT)

## Deploy

```bash
docker compose up -d
```

## Create Netmaker network

```bash
source .env

# Create network
curl -sk -X POST "https://${SERVER_HOST}:8443/api/networks" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "netid": "mesh",
    "addressrange": "44.30.127.0/24"
  }'

# Create enrollment key
curl -sk -X POST "https://${SERVER_HOST}:8443/api/v1/enrollment-keys" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "networks": ["mesh"],
    "tags": ["mesh-node"],
    "unlimited": true
  }'
```

Save the `token` field for client enrollment.

## Verify

```bash
docker compose ps
docker logs netmaker
docker logs netmaker-mq

# API health
curl -sk https://${SERVER_HOST}:8443/api/server/health
```

## Enroll clients

```bash
# Install netclient
curl -sL 'https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/netclient-install.sh' | sudo VERSION=v0.24.2 bash

# Join network
netclient join -t <ENROLLMENT_TOKEN>
```

## Architecture

```
┌─────────────────────────────────┐
│       Netmaker Server           │
│                                 │
│  ┌─────────┐    ┌───────────┐  │
│  │Netmaker │    │ Mosquitto │  │
│  │ :8443   │    │  :1883    │  │
│  └────┬────┘    └───────────┘  │
│       │                         │
│   :51821/UDP WireGuard         │
└─────────────────────────────────┘
            │
            ▼
      Mesh clients
```

## Security Note

For production:
- Use secrets management for `MASTER_KEY`
- Enable MQTT authentication in `mosquitto.conf`
- Consider firewall rules to restrict MQTT access
