# Laptop n1 - Border Router Setup

Configure Laptop n1 as border router with BIRD (BGP) + TINC (VPN mesh).

## Device Info

- **Role**: Border Router (AS 65000)
- **IPs**: 
  - ISP-facing: `172.30.0.100/24`
  - TINC mesh: `44.30.127.1/24`
- **Software**: BIRD + TINC
- **Purpose**: Connect ISP to TINC mesh, route traffic between them

---

## Step 1: Install Software

**⚠️ Repository does NOT handle this - manual installation required**

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install BIRD 2.x and TINC
sudo apt install -y bird2 tinc python3-jinja2

# Verify
bird --version
tincd --version
```

---

## Step 2: Configure ISP-Facing Network Interface

Set static IP `172.30.0.100/24`:

```bash
sudo nano /etc/systemd/network/20-eth0.network
```

Add:
```ini
[Match]
Name=eth0

[Network]
Address=172.30.0.100/24
Gateway=172.30.0.1
```

Apply:
```bash
sudo systemctl restart systemd-networkd
ip addr show eth0
# Verify: 172.30.0.100/24

# Test ISP connectivity
ping -c 3 172.30.0.1
# Should succeed
```

---

## Step 3: Configure TINC VPN

### 3.1 Setup Directories

```bash
sudo mkdir -p /etc/tinc/bgpmesh/hosts
```

### 3.2 Generate Keys

```bash
sudo tincd -n bgpmesh -K4096
# Creates:
# - /etc/tinc/bgpmesh/rsa_key.priv
# - /etc/tinc/bgpmesh/hosts/node1
```

### 3.3 Create TINC Config

**Use repository template**: `configs/tinc/tinc.conf.j2`

```bash
# Create config (manually render Jinja2 template)
sudo nano /etc/tinc/bgpmesh/tinc.conf
```

Add (from template):
```conf
Name = node1
Mode = switch
Cipher = aes-256-cbc
Digest = sha256
Port = 655
Interface = tinc0
```

### 3.4 Create tinc-up Script

**Use repository template**: `configs/tinc/tinc-up.j2`

```bash
sudo nano /etc/tinc/bgpmesh/tinc-up
```

Add (adapted from template):
```bash
#!/bin/sh
ip link set $INTERFACE up mtu 1400
ip addr add 44.30.127.1/24 dev $INTERFACE
ip -6 addr add 2001:db8::1/64 dev $INTERFACE
echo "TINC interface $INTERFACE configured: 44.30.127.1/24"
```

Make executable:
```bash
sudo chmod +x /etc/tinc/bgpmesh/tinc-up
```

### 3.5 Create tinc-down Script

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

### 3.6 Edit Host File

```bash
sudo nano /etc/tinc/bgpmesh/hosts/node1
```

Add at the top (before public key):
```conf
Address = <LAPTOP_N1_IP_OR_HOSTNAME>
Port = 655
Subnet = 44.30.127.1/32
```

### 3.7 Save Host File for Exchange

```bash
# Display for copying to Laptop n2
sudo cat /etc/tinc/bgpmesh/hosts/node1
# Copy this entire content - you'll need it for Laptop n2
```

---

## Step 4: Configure BIRD

### 4.1 Main Config

**Reference**: `configs/bird/bird.conf.j2`

```bash
sudo mkdir -p /etc/bird
sudo nano /etc/bird/bird.conf
```

Add:
```conf
router id 192.0.2.1;
log syslog all;
debug protocols all;

protocol device { scan time 10; }

protocol kernel {
    ipv4 {
        import all;
        export all;
    };
}

protocol static { ipv4; }

include "/etc/bird/filters.conf";
include "/etc/bird/protocols.conf";
```

### 4.2 Filters Config

**Reference**: `configs/bird/filters.conf`

```bash
sudo nano /etc/bird/filters.conf
```

Add:
```conf
# Export to ISP: Announce TINC mesh subnet
filter export_to_isp {
    # CRITICAL: Export TINC mesh so ISP can route to it
    if net ~ [44.30.127.0/24] then {
        print "Announcing TINC mesh ", net, " to ISP";
        accept;
    }
    
    # Optionally announce customer prefixes
    if net ~ [10.100.0.0/24, 10.200.0.0/24] then accept;
    
    reject;
}

# Import from ISP
filter import_from_isp {
    bgp_local_pref = 200;
    accept;
}
```

### 4.3 Protocols Config

**Reference**: `configs/bird/protocols.conf.j2`

```bash
sudo nano /etc/bird/protocols.conf
```

Add:
```conf
# BGP to ISP (eBGP)
protocol bgp isp {
    description "ISP Upstream AS 65001";
    local 172.30.0.100 as 65000;
    neighbor 172.30.0.1 as 65001;
    
    ipv4 {
        import filter import_from_isp;
        export filter export_to_isp;
    };
    
    hold time 90;
    keepalive time 30;
}
```

---

## Step 5: Start Services

### Start TINC

```bash
sudo systemctl enable tinc@bgpmesh
sudo systemctl start tinc@bgpmesh

# Verify
sudo systemctl status tinc@bgpmesh
ip addr show tinc0
# Should show: 44.30.127.1/24
```

### Start BIRD

```bash
sudo systemctl enable bird
sudo systemctl start bird

# Verify
sudo systemctl status bird
sudo birdc show status
```

---

## Step 6: Verify Configuration

### Check TINC

```bash
# Interface up
ip addr show tinc0
# Should show: 44.30.127.1/24 UP

# Logs
sudo journalctl -u tinc@bgpmesh -n 20
```

### Check BIRD

```bash
# Protocols
sudo birdc show protocols
# Should show: isp BGP up/Established

# BGP session details
sudo birdc show protocols all isp

# Routes from ISP
sudo birdc show route protocol isp
# Should show: 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24

# Routes exported to ISP
sudo birdc show route export isp
# Should include: 44.30.127.0/24 ← CRITICAL
```

### Check Kernel Routes

```bash
# Kernel should have TINC subnet
ip route | grep 44.30.127
# Should show: 44.30.127.0/24 dev tinc0 proto kernel
```

---

## Step 7: Exchange TINC Host Files

**Critical for TINC connectivity!**

### Send to Laptop n2:
```bash
# Already saved in Step 3.7
sudo cat /etc/tinc/bgpmesh/hosts/node1
# Copy this to Laptop n2
```

### Receive from Laptop n2:
Once Laptop n2 sends its host file:
```bash
sudo nano /etc/tinc/bgpmesh/hosts/node2
# Paste content from Laptop n2

# Restart TINC
sudo systemctl restart tinc@bgpmesh
```

---

## Step 8: Verify After Laptop n2 is Configured

```bash
# Ping Laptop n2 via TINC
ping -c 5 44.30.127.2
# Should succeed

# Check TINC connection
sudo tinc -n bgpmesh dump nodes
# Should show node2

# Verify BIRD sees kernel route to Laptop n2
sudo birdc show route
# Should include routes via tinc0
```

---

## Troubleshooting

### BGP Not Establishing

```bash
ping -c 3 172.30.0.1  # Test ISP connectivity
sudo journalctl -u bird -n 50  # Check logs
sudo birdc show protocols all isp  # Detailed BGP info
sudo systemctl restart bird  # Restart
```

### TINC Not Connecting

```bash
sudo journalctl -u tinc@bgpmesh -n 50  # Check logs
ls -la /etc/tinc/bgpmesh/hosts/  # Verify node2 file exists
sudo systemctl restart tinc@bgpmesh  # Restart
```

### 44.30.127.0/24 Not Announced to ISP

```bash
# Check kernel has route
ip route | grep 44.30.127

# Check BIRD export filter
sudo birdc show route export isp | grep 44.30.127

# Verify filter accepts it
sudo nano /etc/bird/filters.conf
# Ensure: if net ~ [44.30.127.0/24] then accept;
```

---

## Configuration Files Used

From repository:
- **BIRD main**: `configs/bird/bird.conf.j2`
- **BIRD filters**: `configs/bird/filters.conf`
- **BIRD protocols**: `configs/bird/protocols.conf.j2`
- **TINC config**: `configs/tinc/tinc.conf.j2`
- **TINC up**: `configs/tinc/tinc-up.j2`
- **TINC down**: `configs/tinc/tinc-down.j2`
- **Setup reference**: `docker/bird/entrypoint.sh`, `docker/tinc/entrypoint.sh`

---

## Next Step

Configure **Laptop n2** → See `03-MESH-NODE-LAPTOP-N2.md`

