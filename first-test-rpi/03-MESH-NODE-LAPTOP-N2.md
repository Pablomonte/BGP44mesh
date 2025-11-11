# Laptop n2 - Mesh Node Setup

Configure Laptop n2 as a TINC mesh node (no BGP).

## Device Info

- **Role**: TINC mesh node
- **IP**: `44.30.127.2/24` (TINC only)
- **Software**: TINC only
- **Purpose**: Participate in VPN mesh, be reachable from Mock-ISP

---

## Step 1: Install TINC

**⚠️ Repository does NOT handle this - manual installation required**

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install TINC
sudo apt install -y tinc

# Verify
tincd --version
```

---

## Step 2: Configure TINC

### 2.1 Setup Directories

```bash
sudo mkdir -p /etc/tinc/bgpmesh/hosts
```

### 2.2 Generate Keys

```bash
sudo tincd -n bgpmesh -K4096
# Creates:
# - /etc/tinc/bgpmesh/rsa_key.priv
# - /etc/tinc/bgpmesh/hosts/node2
```

### 2.3 Create TINC Config

**Use repository template**: `configs/tinc/tinc.conf.j2`

```bash
sudo nano /etc/tinc/bgpmesh/tinc.conf
```

Add:
```conf
Name = node2
Mode = switch
Cipher = aes-256-cbc
Digest = sha256
Port = 655
Interface = tinc0

# Connect to node1 (border router)
ConnectTo = node1
```

### 2.4 Create tinc-up Script

**Use repository template**: `configs/tinc/tinc-up.j2`

```bash
sudo nano /etc/tinc/bgpmesh/tinc-up
```

Add:
```bash
#!/bin/sh
ip link set $INTERFACE up mtu 1400
ip addr add 44.30.127.2/24 dev $INTERFACE
ip -6 addr add 2001:db8::2/64 dev $INTERFACE
echo "TINC interface $INTERFACE configured: 44.30.127.2/24"
```

Make executable:
```bash
sudo chmod +x /etc/tinc/bgpmesh/tinc-up
```

### 2.5 Create tinc-down Script

```bash
sudo nano /etc/tinc/bgpmesh/tinc-down
```

Add:
```bash
#!/bin/sh
ip link set $INTERFACE down
```

Make executable:
```bash
sudo chmod +x /etc/tinc/bgpmesh/tinc-down
```

### 2.6 Edit Host File

```bash
sudo nano /etc/tinc/bgpmesh/hosts/node2
```

Add at the top (before public key):
```conf
Address = <LAPTOP_N2_IP_OR_HOSTNAME>
Port = 655
Subnet = 44.30.127.2/32
```

---

## Step 3: Exchange TINC Host Files

**Critical for connectivity!**

### Receive node1 host file from Laptop n1:

```bash
sudo nano /etc/tinc/bgpmesh/hosts/node1
# Paste the content that Laptop n1 provided
# (From Laptop n1's: sudo cat /etc/tinc/bgpmesh/hosts/node1)
```

### Send node2 host file to Laptop n1:

```bash
# Display for copying
sudo cat /etc/tinc/bgpmesh/hosts/node2
# Copy entire output and send to Laptop n1
```

### Verify both host files exist:

```bash
ls -la /etc/tinc/bgpmesh/hosts/
# Should show: node1, node2
```

---

## Step 4: Start TINC

```bash
# Enable service
sudo systemctl enable tinc@bgpmesh

# Start TINC
sudo systemctl start tinc@bgpmesh

# Check status
sudo systemctl status tinc@bgpmesh

# Verify interface
ip addr show tinc0
# Should show: 44.30.127.2/24 UP
```

---

## Step 5: Verify Connectivity

### Check TINC Interface

```bash
# Interface should be up
ip addr show tinc0
# Expected: 44.30.127.2/24 UP

# Check logs
sudo journalctl -u tinc@bgpmesh -n 30
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
ip route
# Should show: 44.30.127.0/24 dev tinc0 proto kernel

# Laptop n2 should have default route or route to 172.30.0.0/24
# This allows responses to Mock-ISP pings to work
```

---

## Step 6: Make Laptop n2 Reachable from Mock-ISP

For Mock-ISP to successfully ping Laptop n2, ensure routing:

### Option A: Add Default Route via Laptop n1

```bash
# Add default route through TINC to Laptop n1
sudo ip route add default via 44.30.127.1 dev tinc0 metric 100

# This allows responses to go back through Laptop n1 to Mock-ISP
```

### Option B: Add Specific Route to ISP Network

```bash
# Add route to ISP network via Laptop n1
sudo ip route add 172.30.0.0/24 via 44.30.127.1 dev tinc0

# This ensures responses to Mock-ISP go via Laptop n1
```

### Make Route Persistent (Optional)

Add to `/etc/network/interfaces` or create systemd service to add route on boot.

---

## Step 7: Test from Mock-ISP

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
sudo tcpdump -i tinc0 icmp
# Should see echo requests from Mock-ISP and echo replies
```

---

## Troubleshooting

### TINC Not Starting

```bash
# Check config syntax
sudo tincd -n bgpmesh -D -d5
# Watch for errors

# Check host files
ls -la /etc/tinc/bgpmesh/hosts/
# Must have both node1 and node2

# Check logs
sudo journalctl -u tinc@bgpmesh -f
```

### Ping from Laptop n1 Works, but Mock-ISP Ping Fails

```bash
# Check routing on Laptop n2
ip route
# Must have route back to 172.30.0.0/24 via 44.30.127.1

# Add route
sudo ip route add 172.30.0.0/24 via 44.30.127.1 dev tinc0

# Test again from Mock-ISP
```

### TINC Interface Not Coming Up

```bash
# Check tinc-up permissions
ls -l /etc/tinc/bgpmesh/tinc-up
# Should be executable (chmod +x)

# Check TUN device
ls -l /dev/net/tun
# Should exist

# Restart TINC
sudo systemctl restart tinc@bgpmesh
```

### No Connection to node1

```bash
# Check node1 host file exists and has Address line
cat /etc/tinc/bgpmesh/hosts/node1
# Must have: Address = <laptop_n1_ip>

# Manual connection attempt
sudo tinc -n bgpmesh connect node1

# Check network connectivity to Laptop n1
# (if on same physical network, should be reachable)
```

---

## Configuration Files Used

From repository:
- **TINC config**: `configs/tinc/tinc.conf.j2`
- **TINC up**: `configs/tinc/tinc-up.j2`
- **TINC down**: `configs/tinc/tinc-down.j2`
- **Setup reference**: `docker/tinc/entrypoint.sh`

---

## Verification Checklist

- [ ] TINC service running
- [ ] tinc0 interface UP with `44.30.127.2/24`
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

