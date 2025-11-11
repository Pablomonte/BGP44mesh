# Raspberry Pi - Mock ISP Setup

Configure Raspberry Pi as a simulated ISP with BIRD BGP daemon.

## Device Info

- **Role**: Mock ISP (AS 65001)
- **IP**: `172.30.0.1/24`
- **Software**: BIRD only
- **Purpose**: Provide BGP upstream, receive routes from Laptop n1

---

## Step 1: Install BIRD

**⚠️ Repository does NOT handle this - manual installation required**

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install BIRD 2.x
sudo apt install -y bird2

# Verify
bird --version
# Should show: BIRD version 2.x
```

---

## Step 2: Configure Network Interface

Set static IP `172.30.0.1/24`:

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

---

## Step 3: Configure BIRD

**Use repository config**: `configs/isp-bird/bird.conf`

```bash
# Copy config from repository
sudo mkdir -p /etc/bird
sudo cp ~/BGP4mesh/configs/isp-bird/bird.conf /etc/bird/
```

**Required edits**:
```bash
sudo nano /etc/bird/bird.conf
```

Change line 7:
```conf
router id 172.30.0.1;  # ← Already correct
```

Change line 44 (BGP neighbor):
```conf
neighbor 172.30.0.100 as 65000;  # ← Must match Laptop n1 IP
```

**Key sections in config**:

1. **Static routes** (lines 27-38): Announces `192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`
2. **BGP import filter** (lines 50-66): 
   - ✅ Accepts customer routes (10.100.0.0/24, 10.200.0.0/24)
   - ✅ **SHOULD accept 44.30.127.0/24** (mesh subnet) ← Critical for ping to work!
3. **BGP export filter** (lines 69-76): Announces ISP routes to customer

**⚠️ Important**: The default config **rejects** `44.30.127.0/24`. To allow Mock-ISP to ping Laptop n2, **modify import filter**:

```bash
sudo nano /etc/bird/bird.conf
```

Change lines 57-61 to:
```conf
        import filter {
            # Accept customer prefixes
            if net ~ [10.100.0.0/24, 10.200.0.0/24, 44.30.127.0/24] then {
                print "ISP (Primary): Accepting customer route ", net, " from AS65000";
                accept;
            }
```

---

## Step 4: Start BIRD

```bash
# Enable service
sudo systemctl enable bird

# Start BIRD
sudo systemctl start bird

# Check status
sudo systemctl status bird

# Verify BIRD is running
sudo birdc show status
```

---

## Step 5: Verify Configuration

```bash
# Check protocols
sudo birdc show protocols

# Expected output:
# device1    Device     ---        up
# kernel1    Kernel     master4    up
# isp_routes Static     master4    up
# customer_primary BGP  ---        start/Active  ← Waiting for Laptop n1

# Check static routes
sudo birdc show route protocol isp_routes
# Should show: 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24
```

---

## Step 6: Verify After Laptop n1 is Configured

Once Laptop n1 is running:

```bash
# Check BGP session
sudo birdc show protocols customer_primary
# Should show: Established

# Check routes learned from customer
sudo birdc show route protocol customer_primary
# Should include: 44.30.127.0/24 via 172.30.0.100

# Check kernel routing table
ip route | grep 44.30.127
# Should show: 44.30.127.0/24 via 172.30.0.100 dev eth0

# TEST: Ping Laptop n2 via TINC mesh
ping -c 5 44.30.127.2
# Should succeed! ✅ Goal achieved
```

---

## Troubleshooting

### BGP Not Establishing

```bash
# Check connectivity to Laptop n1
ping -c 3 172.30.0.100

# Check BIRD logs
sudo journalctl -u bird -n 50

# Check firewall
sudo iptables -L -n | grep 179
# Allow BGP: sudo iptables -A INPUT -p tcp --dport 179 -j ACCEPT

# Restart BIRD
sudo systemctl restart bird
```

### No Route to 44.30.127.0/24

```bash
# Verify import filter accepts it
sudo birdc show protocols all customer_primary | grep -A 10 "Import filter"

# Check if Laptop n1 is announcing it
sudo birdc show route protocol customer_primary

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
sudo birdc show route all 44.30.127.0/24
# Should show "kernel1" protocol
```

---

## Configuration Files Used

From repository:
- **Main config**: `configs/isp-bird/bird.conf`
- **Reference**: `docker/bird/Dockerfile` (shows BIRD setup)

---

## Next Step

Configure **Laptop n1** → See `02-BORDER-ROUTER-LAPTOP-N1.md`

