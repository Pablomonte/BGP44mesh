# Netmaker Server Setup (Core Components)

> [← Back to main README](../../README.md)

Runs behind nginx reverse proxy with certbot for Let's Encrypt.

## Components

| Service | Purpose | Port |
|---------|---------|------|
| netmaker | Mesh VPN server | 8443 (HTTP) |
| mq | MQTT broker (Mosquitto) | 1883 |

## Before deploying

### 1. Create `.env` file

```bash
cat <<EOF > .env
SERVER_HOST=your-netmaker-domain.com
MASTER_KEY=$(openssl rand -base64 32)
EOF
```

### 2. Nginx configuration

Add to your nginx config:

```nginx
server {
    listen 443 ssl;
    server_name ${SERVER_HOST};

    ssl_certificate /etc/letsencrypt/live/${SERVER_HOST}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/${SERVER_HOST}/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8443;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Then obtain certificate:
```bash
certbot --nginx -d ${SERVER_HOST}
```

### 3. Firewall requirements

Open ports:
- 443/TCP (nginx - HTTPS)
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
curl -X POST "https://${SERVER_HOST}/api/networks" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d "{
    \"netid\": \"${MESH_NETWORK_ID}\",
    \"addressrange\": \"${MESH_ADDRESS_RANGE}\"
  }"

# Create enrollment key
curl -X POST "https://${SERVER_HOST}/api/v1/enrollment-keys" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d "{
    \"networks\": [\"${MESH_NETWORK_ID}\"],
    \"tags\": [\"mesh-node\"],
    \"unlimited\": true
  }"
```

Save the `token` field for client enrollment.

## Verify

```bash
docker compose ps
docker logs netmaker
docker logs netmaker-mq

# API health
curl https://${SERVER_HOST}/api/server/health
```

## Enroll clients

```bash
# Install netclient
curl -sL 'https://raw.githubusercontent.com/gravitl/netmaker/master/scripts/netclient-install.sh' | sudo VERSION=v0.24.2 bash

# Join network
netclient join -t <ENROLLMENT_TOKEN>
```

For containerized deployments, see:
- [deploy/netclient/README.md](../netclient/README.md) - Standalone mesh nodes
- [deploy/bird-border/README.md](../bird-border/README.md) - Border router with BGP

## Architecture

```
                         nginx (certbot)
                              :443
                               │
┌──────────────────────────────┼──────────────────┐
│         Netmaker Server      │                  │
│                              ▼                  │
│  ┌─────────────┐       ┌───────────┐           │
│  │  Netmaker   │◄──────│  nginx    │           │
│  │  :8443 HTTP │       │  proxy    │           │
│  └──────┬──────┘       └───────────┘           │
│         │                                       │
│  ┌──────┴──────┐                               │
│  │  Mosquitto  │ :1883                         │
│  └─────────────┘                               │
│                                                 │
│      :51821/UDP WireGuard                      │
└─────────────────────────────────────────────────┘
                    │
                    ▼
              Mesh clients
```
