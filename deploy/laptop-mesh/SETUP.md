# Mesh Node Setup (Laptop n2)

## Before deploying

### 1. Install Netmaker CA certificate

The Border Router uses a self-signed certificate. You must install it on this host:

```bash
# Copy certificate from border router (172.30.0.100)
scp user@172.30.0.100:/path/to/deploy/laptop-border/certs/server.crt /tmp/netmaker.crt

# Install certificate
sudo cp /tmp/netmaker.crt /usr/local/share/ca-certificates/netmaker.crt
sudo update-ca-certificates
```

### 2. Get enrollment token

From the Border Router, get the enrollment token:

```bash
curl -sk "https://172.30.0.100/api/v1/enrollment-keys" \
  -H "Authorization: Bearer $MASTER_KEY"
```

Copy the `token` field from the response.

### 3. Create `.env` file

```bash
echo "ENROLLMENT_TOKEN=<token-from-step-2>" > .env
```

### 4. Enable IP forwarding on host

```bash
sudo sysctl -w net.ipv4.ip_forward=1
# Make persistent:
echo "net.ipv4.ip_forward=1" | sudo tee /etc/sysctl.d/99-ipforward.conf
```

### 5. Network requirements

- UDP connectivity to Border Router on port 51821 (WireGuard)
- Can be on different LAN than other nodes (Netmaker handles NAT traversal)

## Deploy

```bash
docker compose up -d
```

## Verify

```bash
# Check Netmaker client logs
docker logs netclient

# Check WireGuard tunnel
docker exec netclient wg show

# Check mesh connectivity
ping -c 3 44.30.127.1  # Border router mesh IP

# Check received routes (from ISP via Border Router)
ip route | grep 192.0.2    # TEST-NET-1
ip route | grep 198.51.100 # TEST-NET-2
ip route | grep 203.0.113  # TEST-NET-3
```

## How routes arrive

1. ISP (AS 65001) announces test prefixes via BGP
2. Border Router (AS 65000) learns them via eBGP
3. Border Router announces mesh network (44.30.127.0/24) to ISP
4. Netmaker establishes WireGuard tunnel between nodes
5. Routes are distributed via the mesh

## Troubleshooting

### Certificate error
If you see `x509: certificate signed by unknown authority`:
- Ensure the CA certificate is installed (step 1)
- Run `update-ca-certificates` again

### Registration failed
If netclient can't register:
- Check the Border Router's Caddy proxy is running: `curl -k https://172.30.0.100/api/server/health`
- Verify the enrollment token is correct
- Check firewall allows HTTPS (443) to Border Router
