# Laptop n1 - Border Router Setup (Docker)

Configure Laptop n1 as border router with BIRD (BGP) + TINC (VPN mesh) using Docker containers.

## Device Info

- **Role**: Border Router (AS 65000)
- **IPs**: 
  - ISP-facing: `172.30.0.100/24` (via macvlan)
  - TINC mesh: `44.30.127.1/24` (via TINC container)
- **Docker Services**: `bird1`, `tinc1`, `etcd1`
- **Purpose**: Connect ISP to TINC mesh, route traffic between them

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

# Verify macvlan support (required)
lsmod | grep macvlan
# Should show macvlan module loaded
```

---

## Step 2: Clone Repository

```bash
# Clone or copy repository to Laptop n1
cd ~
git clone <repository-url> BGP4mesh
cd BGP4mesh
```

---

## Step 3: Identify Network Interface

Find the physical interface connected to the ISP network:

```bash
# Find default route interface
ip route | grep default
# Example output: default via 172.30.0.1 dev eth0 ...

# Or list all interfaces
ip addr show
# Look for interface with IP in 172.30.0.0/24 range
```

**Note the interface name** (e.g., `eth0`, `enp0s3`, `enxa0cec8992ed8`). You'll need this for macvlan configuration.

---

## Step 4: Create Environment File

Create `.env` file for Docker Compose:

```bash
cd ~/BGP4mesh
nano .env
```

Add:
```bash
# BGP Configuration
BGP_AS=65000
ISP_ENABLED=true
ISP_NEIGHBOR=172.30.0.1
ISP_LOCAL_IP=172.30.0.100

# Macvlan Configuration (for ISP connectivity)
LAN_INTERFACE=eth0  # ← Change to your interface name
LAN_SUBNET=172.30.0.0/24
LAN_GATEWAY=172.30.0.1
LAN_IP_RANGE=172.30.0.100/31
TINC1_LAN_IP=172.30.0.100

# TINC Configuration
TINC_PORT=655
TINC_NETNAME=bgpmesh
```

**Important**: Replace `eth0` with your actual interface name from Step 3.

---

## Step 5: Verify Standalone Docker Compose File

The repository includes a **standalone** compose file for hardware test:

```bash
# Verify file exists
cat deploy/hardware-test/docker-compose.border-router.yml
```

This file contains only the services needed for Laptop n1:
- `bird1` - BGP daemon (shares network with tinc1)
- `tinc1` - VPN mesh node with macvlan for ISP connectivity
- `etcd1` - Service discovery

**Note**: Unlike `docker-compose.yml` (for local simulation with 5 nodes), this standalone file is designed specifically for the hardware test and uses macvlan for real ISP connectivity.

**Note**: The file is located at `deploy/hardware-test/docker-compose.border-router.yml`.

---

## Step 6: Update BIRD Export Filter

The repository's filter needs to export TINC mesh subnet to ISP:

```bash
# Edit filters config
nano configs/bird/filters.conf
```

**Update the `export_to_isp` filter** (lines 14-32):

Change:
```conf
filter export_to_isp {
    # Accept customer prefixes
    if net ~ [10.100.0.0/24, 10.200.0.0/24] then {
        print "Announcing customer prefix ", net, " to ISP";
        accept;
    }

    # Reject TINC mesh internal network
    if net ~ [44.30.127.0/24] then {
        print "Blocking internal mesh route ", net, " from ISP";
        reject;
    }

    # Reject everything else
    print "Rejecting unknown prefix ", net, " to ISP";
    reject;
}
```

To:
```conf
filter export_to_isp {
    # CRITICAL: Export TINC mesh subnet so ISP can route to it
    if net ~ [44.30.127.0/24] then {
        print "Announcing TINC mesh ", net, " to ISP";
        accept;
    }

    # Accept customer prefixes
    if net ~ [10.100.0.0/24, 10.200.0.0/24] then {
        print "Announcing customer prefix ", net, " to ISP";
        accept;
    }

    # Reject everything else
    print "Rejecting unknown prefix ", net, " to ISP";
    reject;
}
```

---

## Step 7: Enable IP Forwarding

**Critical!** Laptop n1 must route packets between the ISP network and TINC mesh:

```bash
# Enable IP forwarding (temporary)
sudo sysctl -w net.ipv4.ip_forward=1

# Make persistent across reboots
echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.conf

# Verify
sysctl net.ipv4.ip_forward
# Should show: net.ipv4.ip_forward = 1
```

**Optional - Allow forwarding in firewall** (if you have restrictive iptables rules):

```bash
sudo iptables -A FORWARD -i tinc0 -j ACCEPT
sudo iptables -A FORWARD -o tinc0 -j ACCEPT
```

---

## Step 8: Deploy Services

```bash
# Deploy with standalone hardware test file
docker compose -f deploy/hardware-test/docker-compose.border-router.yml up -d --build

# Check status
docker ps
# Should show: bird1, tinc1, etcd1 running
```

---

## Step 9: Verify Configuration

### Check TINC

```bash
# Check container is running
docker ps | grep tinc1

# Check TINC interface
docker exec tinc1 ip addr show tinc0
# Should show: 44.30.127.1/24 UP

# Check logs
docker logs tinc1 | tail -20
```

### Check BIRD

```bash
# Check container is running
docker ps | grep bird1

# Check BIRD status
docker exec bird1 birdc show status

# Check protocols
docker exec bird1 birdc show protocols
# Should show: isp_primary BGP up/Established (after ISP is running)

# Check BGP session details
docker exec bird1 birdc show protocols all isp_primary

# Routes from ISP
docker exec bird1 birdc show route protocol isp_primary
# Should show: 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24

# Routes exported to ISP
docker exec bird1 birdc show route export isp_primary
# Should include: 44.30.127.0/24 ← CRITICAL
```

### Check Macvlan Network

```bash
# Check macvlan interface exists
ip addr show | grep 172.30.0.100
# Should show macvlan interface with 172.30.0.100/24

# Test connectivity to ISP
ping -c 3 172.30.0.1
# Should succeed
```

### Check Kernel Routes

```bash
# Kernel should have TINC subnet
ip route | grep 44.30.127
# Should show: 44.30.127.0/24 dev tinc0 proto kernel
```

---

## Step 10: Fix TINC Host File Address

**Critical!** The auto-generated TINC host file has `Address = tinc1` (container name) which won't resolve on separate devices. Fix it:

```bash
# View current host file
docker exec tinc1 cat /var/run/tinc/bgpmesh/hosts/node1

# Fix the Address line to use actual IP
# For same-switch test (all devices on 172.30.0.0/24):
docker exec tinc1 sed -i 's/Address = tinc1/Address = 172.30.0.100/' /var/run/tinc/bgpmesh/hosts/node1

# For separate-network test (Laptop n2 on different internet):
# Use Laptop n1's public/reachable IP instead of 172.30.0.100

# Verify the change
docker exec tinc1 cat /var/run/tinc/bgpmesh/hosts/node1
# Should show: Address = 172.30.0.100 (or your reachable IP)
```

---

## Step 11: Exchange TINC Host Files with Laptop n2

**Critical for TINC connectivity!**

### Get node1 host file:

```bash
# Display host file for Laptop n2 (with corrected Address)
docker exec tinc1 cat /var/run/tinc/bgpmesh/hosts/node1
# Copy this entire output and send to Laptop n2
```

### Receive node2 host file from Laptop n2:

Once Laptop n2 provides its host file:

```bash
# Create node2 host file
docker exec tinc1 sh -c 'cat > /var/run/tinc/bgpmesh/hosts/node2' << 'EOF'
# Paste content from Laptop n2 here
EOF

# Restart TINC to establish connection
docker compose -f deploy/hardware-test/docker-compose.border-router.yml restart tinc1
```

---

## Step 12: Verify After Laptop n2 is Configured

```bash
# Ping Laptop n2 via TINC
ping -c 5 44.30.127.2
# Should succeed

# Check TINC connection
docker exec tinc1 tinc -n bgpmesh dump nodes
# Should show node2

# Verify BIRD sees kernel route to Laptop n2
docker exec bird1 birdc show route
# Should include routes via tinc0
```

---

## Troubleshooting

### BGP Not Establishing

```bash
# Test ISP connectivity
ping -c 3 172.30.0.1

# Check BIRD logs
docker logs bird1 | tail -50

# Check BGP session details
docker exec bird1 birdc show protocols all isp_primary

# Verify macvlan IP is correct
ip addr show | grep 172.30.0.100

# Restart services
docker compose -f deploy/hardware-test/docker-compose.border-router.yml restart bird1
```

### isp_secondary Protocol Failing (Expected)

The BIRD configuration includes a secondary ISP uplink (`isp_secondary`) that expects a peer at `172.31.0.2`. **This is expected to fail** in the hardware test since we only have one ISP link.

```bash
# Check protocols - isp_secondary will show "start" or "Active"
docker exec bird1 birdc show protocols
# isp_primary    BGP    ---    up      Established  ← This is what matters
# isp_secondary  BGP    ---    start   Active       ← Expected to fail, ignore
```

**This does not affect the test** - only `isp_primary` needs to establish.

### TINC Not Connecting

```bash
# Check logs
docker logs tinc1 | tail -50

# Verify host files exist with correct Address
docker exec tinc1 ls -la /var/run/tinc/bgpmesh/hosts/
# Should show: node1, node2

# Check Address lines in host files (must be reachable IPs, not container names)
docker exec tinc1 grep "Address" /var/run/tinc/bgpmesh/hosts/*
# node1 should have: Address = 172.30.0.100 (or reachable IP)
# node2 should have: Address = <laptop_n2_ip>

# Check TINC interface
docker exec tinc1 ip addr show tinc0

# Restart TINC
docker compose -f deploy/hardware-test/docker-compose.border-router.yml restart tinc1
```

### TINC Host File Has Wrong Address

If host files still have container names like `Address = tinc1`:

```bash
# Fix node1's Address
docker exec tinc1 sed -i 's/Address = tinc1/Address = 172.30.0.100/' /var/run/tinc/bgpmesh/hosts/node1

# Restart to apply
docker compose -f deploy/hardware-test/docker-compose.border-router.yml restart tinc1
```

### 44.30.127.0/24 Not Announced to ISP

```bash
# Check kernel has route
ip route | grep 44.30.127

# Check BIRD export filter
docker exec bird1 birdc show route export isp_primary | grep 44.30.127

# Verify filter configuration
cat configs/bird/filters.conf | grep -A 5 "export_to_isp"
# Should show: if net ~ [44.30.127.0/24] then accept;

# Reload BIRD config
docker exec bird1 birdc configure
```

### Macvlan Not Working

```bash
# Check interface exists
ip link show | grep macvlan

# Check parent interface is correct
docker network inspect bgp4mesh-fork-santi_lan-macvlan | grep parent

# Verify IP assignment
ip addr show | grep 172.30.0.100

# If macvlan not created, recreate network
docker compose -f deploy/hardware-test/docker-compose.border-router.yml down
docker compose -f deploy/hardware-test/docker-compose.border-router.yml up -d --build
```

### IP Forwarding Not Enabled

If packets don't route between ISP and TINC:

```bash
# Check if forwarding is enabled
sysctl net.ipv4.ip_forward
# Must show: net.ipv4.ip_forward = 1

# Enable if not
sudo sysctl -w net.ipv4.ip_forward=1
```

---

## Configuration Files Used

From repository:
- **Docker Compose**: `deploy/hardware-test/docker-compose.border-router.yml` (standalone file for hardware test)
- **Environment**: `.env` (created)
- **BIRD configs**: `configs/bird/bird.conf.j2`, `configs/bird/protocols.conf.j2`, `configs/bird/filters.conf` (modified)
- **TINC templates**: `configs/tinc/*.j2`
- **Docker images**: `docker/bird/`, `docker/tinc/`

---

## Next Step

Configure **Laptop n2** → See `03-MESH-NODE-LAPTOP-N2.md`
