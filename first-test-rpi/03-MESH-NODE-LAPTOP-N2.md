# Laptop n2 - Mesh Node Setup (Docker)

Configure Laptop n2 as a TINC mesh node (no BGP) using Docker containers.

## Device Info

- **Role**: TINC mesh node
- **IP**: `44.30.127.2/24` (TINC only)
- **Docker Services**: `tinc2`, `etcd1`
- **Purpose**: Participate in VPN mesh, be reachable from Mock-ISP

---

## Step 1: Prerequisites

```bash
# Install Docker and Docker Compose
sudo apt update
sudo apt install -y docker.io docker-compose-v2

# Add user to docker group (optional)
sudo usermod -aG docker $USER
# Log out and back in

# Verify Docker
docker --version
docker compose version
```

---

## Step 2: Clone Repository

```bash
# Clone or copy repository to Laptop n2
cd ~
git clone <repository-url> BGP4mesh
cd BGP4mesh
```

---

## Step 3: Create Minimal Docker Compose File

Create a compose file for just TINC node2:

```bash
nano docker-compose.node2.yml
```

Add:
```yaml
version: '3.8'

services:
  tinc2:
    build: ./docker/tinc
    container_name: tinc2
    hostname: tinc2
    cap_add:
      - NET_ADMIN
    devices:
      - /dev/net/tun
    ports:
      - "655:655/udp"
    volumes:
      - ./configs/tinc:/etc/tinc:ro
      - tinc2-data:/var/run/tinc
    depends_on:
      - etcd1
    networks:
      - mesh-net
      - cluster-net
    environment:
      - TINC_NAME=node2
      - TINC_PORT=655
      - TINC_NETNAME=bgpmesh
    restart: unless-stopped

  etcd1:
    image: quay.io/coreos/etcd:v3.5.14
    container_name: etcd1
    hostname: etcd1
    command:
      - etcd
      - --name=etcd1
      - --data-dir=/etcd-data
      - --listen-client-urls=http://0.0.0.0:2379
      - --advertise-client-urls=http://etcd1:2379
      - --listen-peer-urls=http://0.0.0.0:2380
      - --initial-advertise-peer-urls=http://etcd1:2380
      - --initial-cluster=etcd1=http://etcd1:2380
      - --initial-cluster-state=new
    ports:
      - "2379:2379"
      - "2380:2380"
    volumes:
      - etcd1-data:/etcd-data
    networks:
      - cluster-net
      - mesh-net
    restart: unless-stopped

networks:
  mesh-net:
    driver: bridge
    ipam:
      config:
        - subnet: 172.22.0.0/16
  cluster-net:
    driver: bridge
    internal: true
    ipam:
      config:
        - subnet: 172.23.0.0/16

volumes:
  etcd1-data:
  tinc2-data:
```

---

## Step 4: Update TINC Config Template (Optional)

The TINC config template should work as-is, but verify it includes `ConnectTo` for node1:

```bash
# Check template
cat configs/tinc/tinc.conf.j2
```

If it doesn't have `ConnectTo = node1`, you may need to manually configure after deployment (see Step 7).

---

## Step 5: Deploy Services

```bash
# Deploy TINC node2
docker compose -f docker-compose.node2.yml up -d --build

# Check status
docker ps
# Should show: tinc2, etcd1 running
```

---

## Step 6: Fix TINC Host File Address

**Critical!** The auto-generated TINC host file has `Address = tinc2` (container name) which won't resolve on separate devices. Fix it:

```bash
# View current host file
docker exec tinc2 cat /var/run/tinc/bgpmesh/hosts/node2

# Fix the Address line to use actual IP
# For same-switch test (all devices on 172.30.0.0/24):
docker exec tinc2 sed -i 's/Address = tinc2/Address = 172.30.0.101/' /var/run/tinc/bgpmesh/hosts/node2

# For separate-network test (Laptop n2 on different internet):
# Use Laptop n2's public/reachable IP instead

# Verify the change
docker exec tinc2 cat /var/run/tinc/bgpmesh/hosts/node2
# Should show: Address = 172.30.0.101 (or your reachable IP)
```

**Note for same-switch test**: Laptop n2 also needs an IP on eth0:
```bash
# On Laptop n2 host (not in container)
sudo ip addr add 172.30.0.101/24 dev eth0
sudo ip link set eth0 up
```

---

## Step 7: Exchange TINC Host Files

**Critical for connectivity!**

### Receive node1 host file from Laptop n1:

```bash
# Create node1 host file (with corrected Address from Laptop n1)
docker exec tinc2 sh -c 'cat > /var/run/tinc/bgpmesh/hosts/node1' << 'EOF'
# Paste content from Laptop n1 here
# (From Laptop n1: docker exec tinc1 cat /var/run/tinc/bgpmesh/hosts/node1)
# Make sure Address = 172.30.0.100 (not "tinc1")
EOF
```

### Send node2 host file to Laptop n1:

```bash
# Display host file for Laptop n1 (with corrected Address)
docker exec tinc2 cat /var/run/tinc/bgpmesh/hosts/node2
# Copy this entire output and send to Laptop n1
```

### Verify both host files exist with correct Address:

```bash
docker exec tinc2 ls -la /var/run/tinc/bgpmesh/hosts/
# Should show: node1, node2

# Verify Address lines are IPs (not container names)
docker exec tinc2 grep "Address" /var/run/tinc/bgpmesh/hosts/*
# node1: Address = 172.30.0.100 (Laptop n1)
# node2: Address = 172.30.0.101 (Laptop n2) or reachable IP
```

---

## Step 8: Configure TINC to Connect to node1

If the template doesn't include `ConnectTo`, add it:

```bash
# Check current config
docker exec tinc2 cat /var/run/tinc/bgpmesh/tinc.conf

# If ConnectTo is missing, restart with ConnectTo
docker exec tinc2 sh -c 'echo "ConnectTo = node1" >> /var/run/tinc/bgpmesh/tinc.conf'

# Restart TINC
docker compose -f docker-compose.node2.yml restart tinc2
```

---

## Step 9: Verify Connectivity

### Check TINC Interface

```bash
# Interface should be up
docker exec tinc2 ip addr show tinc0
# Expected: 44.30.127.2/24 UP

# Check logs
docker logs tinc2 | tail -30
# Should show connection to node1
```

### Ping Laptop n1

```bash
# Test TINC mesh connectivity
ping -c 5 44.30.127.1
# Should succeed
```

### Check Routing Table

```bash
# View routes
docker exec tinc2 ip route
# Should show: 44.30.127.0/24 dev tinc0 proto kernel
```

---

## Step 10: Configure Return Route for Mock-ISP

For Mock-ISP to successfully ping Laptop n2, ensure routing back to ISP network:

### Option A: Add Default Route via Laptop n1

```bash
# Add default route through TINC to Laptop n1
docker exec tinc2 ip route add default via 44.30.127.1 dev tinc0 metric 100

# This allows responses to go back through Laptop n1 to Mock-ISP
```

### Option B: Add Specific Route to ISP Network

```bash
# Add route to ISP network via Laptop n1
docker exec tinc2 ip route add 172.30.0.0/24 via 44.30.127.1 dev tinc0

# This ensures responses to Mock-ISP go via Laptop n1
```

**Note**: These routes are temporary. For persistence, you could:
1. Add to a startup script
2. Create a systemd service
3. Use a Docker entrypoint script modification

---

## Step 11: Test from Mock-ISP

Once all devices are configured:

### On Mock-ISP (Raspberry Pi):

```bash
# Ping Laptop n2
ping -c 5 44.30.127.2
# Should succeed ✅ Goal achieved!

# Trace route
traceroute 44.30.127.2
# Should show: RPi → Laptop n1 (172.30.0.100) → Laptop n2
```

### On Laptop n2 (verify responses):

```bash
# Monitor ICMP
docker exec tinc2 tcpdump -i tinc0 icmp
# Should see echo requests from Mock-ISP and echo replies
```

---

## Troubleshooting

### TINC Not Starting

```bash
# Check logs
docker logs tinc2 | tail -50

# Check config syntax
docker exec tinc2 tincd -n bgpmesh -D -d5
# Watch for errors (Ctrl+C to exit)

# Check host files
docker exec tinc2 ls -la /var/run/tinc/bgpmesh/hosts/
# Must have both node1 and node2

# Check TUN device
docker exec tinc2 ls -l /dev/net/tun
# Should exist
```

### Ping from Laptop n1 Works, but Mock-ISP Ping Fails

```bash
# Check routing on Laptop n2
docker exec tinc2 ip route
# Must have route back to 172.30.0.0/24 via 44.30.127.1

# Add route if missing
docker exec tinc2 ip route add 172.30.0.0/24 via 44.30.127.1 dev tinc0

# Test again from Mock-ISP
```

### TINC Interface Not Coming Up

```bash
# Check tinc-up permissions
docker exec tinc2 ls -l /var/run/tinc/bgpmesh/tinc-up
# Should be executable

# Check tinc-up content
docker exec tinc2 cat /var/run/tinc/bgpmesh/tinc-up
# Should configure 44.30.127.2/24

# Restart TINC
docker compose -f docker-compose.node2.yml restart tinc2
```

### No Connection to node1

```bash
# Check node1 host file exists and has correct Address line
docker exec tinc2 cat /var/run/tinc/bgpmesh/hosts/node1
# Must have: Address = 172.30.0.100 (not "tinc1"!)

# Check tinc.conf has ConnectTo
docker exec tinc2 cat /var/run/tinc/bgpmesh/tinc.conf | grep ConnectTo
# Should show: ConnectTo = node1

# Manual connection attempt
docker exec tinc2 tinc -n bgpmesh connect node1

# Check network connectivity to Laptop n1
# (if on same physical network, should be reachable)
ping 172.30.0.100  # Test physical connectivity to Laptop n1
```

### TINC Host Files Have Wrong Address (Container Names)

If host files have `Address = tinc1` or `Address = tinc2` instead of IPs:

```bash
# Check Address lines
docker exec tinc2 grep "Address" /var/run/tinc/bgpmesh/hosts/*

# If node1 has "Address = tinc1", get corrected file from Laptop n1
# If node2 has "Address = tinc2", fix it:
docker exec tinc2 sed -i 's/Address = tinc2/Address = 172.30.0.101/' /var/run/tinc/bgpmesh/hosts/node2

# Restart TINC
docker compose -f docker-compose.node2.yml restart tinc2
```

### etcd Connection Issues

```bash
# Check etcd is running
docker ps | grep etcd1

# Check etcd logs
docker logs etcd1

# Verify etcd connectivity from tinc2
docker exec tinc2 etcdctl --endpoints=http://etcd1:2379 endpoint health
# Should show healthy
```

---

## Configuration Files Used

From repository:
- **Docker Compose**: `docker-compose.node2.yml` (created)
- **TINC templates**: `configs/tinc/tinc.conf.j2`, `configs/tinc/tinc-up.j2`, `configs/tinc/tinc-down.j2`
- **Docker image**: `docker/tinc/Dockerfile`
- **Entrypoint**: `docker/tinc/entrypoint.sh`

---

## Verification Checklist

- [ ] TINC service running (`docker ps | grep tinc2`)
- [ ] tinc0 interface UP with `44.30.127.2/24` (`docker exec tinc2 ip addr show tinc0`)
- [ ] Can ping Laptop n1 (`44.30.127.1`)
- [ ] Route to ISP network exists (via `44.30.127.1`)
- [ ] **Mock-ISP can ping this device** ✅

---

## Final Test

From **Raspberry Pi**:
```bash
ping -c 10 44.30.127.2
# Success! Goal achieved!
```

This proves:
- BGP routing works (RPi → Laptop n1)
- TINC mesh works (Laptop n1 → Laptop n2)
- Full end-to-end connectivity established
