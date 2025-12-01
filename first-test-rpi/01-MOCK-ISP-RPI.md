# Raspberry Pi - Mock ISP Setup (Docker)

Configure Raspberry Pi as a simulated ISP with BIRD BGP daemon using Docker.

## Device Info

- **Role**: Mock ISP (AS 65001)
- **IP**: `172.30.0.1/24`
- **Docker Service**: `isp-bird`
- **Network Mode**: Host network (for direct interface access)
- **Purpose**: Provide BGP upstream, receive routes from Laptop n1

---

## Step 1: Prerequisites

```bash
# Install Docker and Docker Compose
sudo apt update
sudo apt install -y docker.io docker-compose-v2

# Add user to docker group (optional, to avoid sudo)
sudo usermod -aG docker $USER
# Log out and back in for group change to take effect

# Verify Docker
docker --version
docker compose version
```

---

## Step 2: Clone Repository

```bash
# Clone or copy repository to Raspberry Pi
cd ~
git clone <repository-url> BGP4mesh
cd BGP4mesh
```

---

## Step 3: Configure Network Interface

Set static IP `172.30.0.1/24` on the physical interface (e.g., `eth0`):

```bash
# Example for systemd-networkd
sudo nano /etc/systemd/network/10-eth0.network
```

Add:
```ini
[Match]
Name=eth0

[Network]
Address=172.30.0.1/24
```

Apply:
```bash
sudo systemctl restart systemd-networkd
ip addr show eth0
# Verify: 172.30.0.1/24 assigned
```

**Alternative**: If using NetworkManager or `/etc/network/interfaces`, configure accordingly.

---

## Step 4: Update ISP BIRD Configuration

The repository's ISP config needs to be updated for the hardware test IPs.

```bash
# Backup original config
cp configs/isp-bird/bird.conf configs/isp-bird/bird.conf.original

# Edit config
nano configs/isp-bird/bird.conf
```

**Update the BGP protocol section** (lines 40-80):

Change:
```conf
protocol bgp customer {
    description "Customer AS 65000 (Border Router)";
    local 10.42.0.228 as 65001;  # ← Change this
    neighbor 10.42.0.100 as 65000;  # ← Change this
```

To:
```conf
protocol bgp customer {
    description "Customer AS 65000 (Border Router)";
    local 172.30.0.1 as 65001;  # ← Raspberry Pi IP
    neighbor 172.30.0.100 as 65000;  # ← Laptop n1 IP
```

**Update the import filter** to accept TINC mesh subnet (lines 48-64):

Change:
```conf
        import filter {
            # Accept customer prefixes
            if net ~ [10.100.0.0/24, 10.200.0.0/24] then {
                print "ISP: Accepting customer route ", net, " from AS65000";
                accept;
            }

            # Reject TINC mesh internal network (should not be announced)
            if net ~ [10.0.0.0/24] then {
                print "ISP: Rejecting internal mesh route ", net;
                reject;
            }

            # Reject anything else
            print "ISP: Rejecting unknown route ", net;
            reject;
        };
```

To:
```conf
        import filter {
            # Accept customer prefixes
            if net ~ [10.100.0.0/24, 10.200.0.0/24] then {
                print "ISP: Accepting customer route ", net, " from AS65000";
                accept;
            }

            # CRITICAL: Accept TINC mesh subnet so Mock-ISP can ping Laptop n2
            if net ~ [44.30.127.0/24] then {
                print "ISP: Accepting TINC mesh route ", net, " from AS65000";
                accept;
            }

            # Reject anything else
            print "ISP: Rejecting unknown route ", net;
            reject;
        };
```

---

## Step 5: Deploy ISP with Docker Compose

Use the standalone ISP compose file:

```bash
# Deploy ISP container
docker compose -f deploy/hardware-test/docker-compose.isp.yml up -d --build

# Check status
docker ps | grep isp-bird
docker logs isp-bird
```

The container runs in **host network mode**, so it uses the host's `eth0` interface directly.

---

## Step 6: Verify Configuration

```bash
# Check container is running
docker ps | grep isp-bird

# Check BIRD status inside container
docker exec isp-bird birdc show status

# Check protocols
docker exec isp-bird birdc show protocols

# Expected output:
# device1    Device     ---        up
# kernel1    Kernel     master4    up
# isp_routes Static     master4    up
# customer   BGP        ---        start/Active  ← Waiting for Laptop n1

# Check static routes
docker exec isp-bird birdc show route protocol isp_routes
# Should show: 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24
```

---

## Step 7: Verify After Laptop n1 is Configured

Once Laptop n1 is running:

```bash
# Check BGP session
docker exec isp-bird birdc show protocols customer
# Should show: Established

# Check routes learned from customer
docker exec isp-bird birdc show route protocol customer
# Should include: 44.30.127.0/24 via 172.30.0.100

# Check kernel routing table (on host)
ip route | grep 44.30.127
# Should show: 44.30.127.0/24 via 172.30.0.100 dev eth0

# TEST: Ping Laptop n2 via TINC mesh
ping -c 5 44.30.127.2
# Should succeed! ✅ Goal achieved
```

---

## Troubleshooting

### Container Not Starting

```bash
# Check logs
docker logs isp-bird

# Check if port 179 is already in use
sudo netstat -tlnp | grep 179
# If BIRD is running on host, stop it: sudo systemctl stop bird
```

### BGP Not Establishing

```bash
# Check connectivity to Laptop n1
ping -c 3 172.30.0.100

# Check BIRD logs
docker logs isp-bird

# Check firewall (BGP port 179)
sudo iptables -L -n | grep 179
# Allow BGP: sudo iptables -A INPUT -p tcp --dport 179 -j ACCEPT

# Restart container
docker compose -f deploy/hardware-test/docker-compose.isp.yml restart isp-bird
```

### No Route to 44.30.127.0/24

```bash
# Verify import filter accepts it
docker exec isp-bird birdc show protocols all customer | grep -A 10 "Import filter"

# Check if Laptop n1 is announcing it
docker exec isp-bird birdc show route protocol customer

# If not present, check Laptop n1 export configuration
```

### Ping to 44.30.127.2 Fails

```bash
# Check route exists
ip route | grep 44.30.127
# Must show: 44.30.127.0/24 via 172.30.0.100

# Verify next hop is reachable
ping -c 3 172.30.0.100

# Check BIRD exported route to kernel
docker exec isp-bird birdc show route all 44.30.127.0/24
# Should show "kernel1" protocol
```

---

## Configuration Files Used

From repository:
- **Docker Compose**: `deploy/hardware-test/docker-compose.isp.yml`
- **BIRD config**: `configs/isp-bird/bird.conf` (modified for hardware test)
- **Docker image**: `docker/bird/Dockerfile`
- **Entrypoint**: `docker/bird/entrypoint.sh`

---

## Next Step

Configure **Laptop n1** → See `02-BORDER-ROUTER-LAPTOP-N1.md`
