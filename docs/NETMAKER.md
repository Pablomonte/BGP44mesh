# Netmaker configuration

## Components

| Container | Image | Function |
|-----------|-------|----------|
| netmaker | gravitl/netmaker:v0.24.2 | Server, manages mesh topology |
| netmaker-mq | eclipse-mosquitto:2 | MQTT broker for node communication |
| caddy | caddy:2-alpine | TLS termination (HTTPS required by netclient) |
| netclient | gravitl/netclient:v0.24.2 | WireGuard client on each mesh node |

## Network

| Parameter | Value |
|-----------|-------|
| Network ID | mesh |
| Address range | 44.30.127.0/24 |
| WireGuard port | 51821/UDP |
| API port | 443/TCP (via Caddy) |
| MQTT port | 1883/TCP |

## API

### Authentication

All API calls require the `Authorization: Bearer <MASTER_KEY>` header.

### Create network

```bash
curl -sk -X POST "https://<SERVER>/api/networks" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"netid": "mesh", "addressrange": "44.30.127.0/24"}'
```

### Create enrollment key

```bash
curl -sk -X POST "https://<SERVER>/api/v1/enrollment-keys" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"networks": ["mesh"], "tags": ["node"], "unlimited": true}'
```

Response contains `token` field (base64-encoded JSON with server address and key value).

### List hosts

```bash
curl -sk "https://<SERVER>/api/hosts" \
  -H "Authorization: Bearer $MASTER_KEY"
```

### List networks

```bash
curl -sk "https://<SERVER>/api/networks" \
  -H "Authorization: Bearer $MASTER_KEY"
```

### Health check

```bash
curl -sk "https://<SERVER>/api/server/health"
```

## TLS requirement

Netmaker v0.24.x netclient requires HTTPS. The project uses Caddy with self-signed certificates.

Certificate generation:
```bash
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout certs/server.key -out certs/server.crt \
  -subj "/CN=<IP>" -addext "subjectAltName=IP:<IP>"
```

Hosts running netclient must trust this certificate:
```bash
sudo cp server.crt /usr/local/share/ca-certificates/netmaker.crt
sudo update-ca-certificates
```

## Environment variables

### Server (netmaker)

| Variable | Description |
|----------|-------------|
| SERVER_HOST | Physical IP address |
| SERVER_API_CONN_STRING | `<IP>` (no scheme, no port) |
| SERVER_HTTP_HOST | `<IP>` (no scheme, no port) |
| API_PORT | Internal API port (8081) |
| BROKER_ENDPOINT | `mqtt://<IP>:1883` |
| MQ_HOST | MQTT broker IP |
| MQ_PORT | MQTT port (1883) |
| MASTER_KEY | API authentication key |
| DATABASE | `sqlite` |

### Client (netclient)

| Variable | Description |
|----------|-------------|
| TOKEN | Enrollment token from API |

## WireGuard interface

Netclient creates interface named `netmaker` (not `nm-*` as documented elsewhere).

BIRD configuration must reference this interface:
```
protocol direct {
    ipv4;
    interface "netmaker";
}
```

## Troubleshooting

### netclient: certificate signed by unknown authority

Install the CA certificate on the host (not just in the container).

### netclient: https://http//...

Token contains `http://` in server field. Ensure `SERVER_HTTP_HOST` and `SERVER_API_CONN_STRING` do not include scheme.

### BIRD not exporting mesh route

BIRD must be restarted after the `netmaker` interface is created:
```bash
docker restart bird-border
```

### netmaker crash loop: could not connect to broker

Check `BROKER_ENDPOINT` uses `mqtt://` scheme (not `ws://`). Mosquitto default config does not support WebSocket.
