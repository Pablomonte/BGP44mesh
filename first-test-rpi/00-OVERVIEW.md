# Hardware Test Setup - Overview

## Goal
Get **Mock-ISP (Raspberry Pi)** to ping **Laptop n2** through BGP routing and TINC VPN mesh.

## Architecture

```
Raspberry Pi (Mock-ISP)          Laptop n1 (Border Router)      Laptop n2 (Mesh Node)
AS 65001, BIRD                   AS 65000, BIRD + TINC          TINC only
172.30.0.1/24                    172.30.0.100 + 44.30.127.1     44.30.127.2/24
     │                                  │                              │
     │◄─────── BGP eBGP ──────────────►│◄──── TINC VPN Mesh ────────►│
     │                                  │                              │
   Announces                      Routes between                 Receives routes
   192.0.2.0/24                   ISP & TINC mesh                via kernel
```

## Network Subnets

- **ISP Network**: `172.30.0.0/24` (physical connection between RPi and Laptop n1)
- **TINC Mesh**: `44.30.127.0/24` (VPN overlay between Laptop n1 and n2)

## How Mock-ISP Pings Laptop n2

1. **Laptop n2** announces `44.30.127.2/32` via TINC to **Laptop n1**
2. **Laptop n1** (BIRD) learns this route from kernel
3. **Laptop n1** announces `44.30.127.0/24` to **Mock-ISP** via BGP
4. **Mock-ISP** learns route: `44.30.127.0/24 via 172.30.0.100` (next hop: Laptop n1)
5. **Mock-ISP** pings `44.30.127.2` → routes to Laptop n1 → TINC forwards to Laptop n2

## Setup Order

1. **Raspberry Pi**: Configure Mock-ISP BIRD → `01-MOCK-ISP-RPI.md`
2. **Laptop n1**: Configure BIRD + TINC → `02-BORDER-ROUTER-LAPTOP-N1.md`
3. **Laptop n2**: Configure TINC only → `03-MESH-NODE-LAPTOP-N2.md`
4. **Verify**: Mock-ISP can ping Laptop n2

## Repository Information

**⚠️ IMPORTANT**: This repository is **Docker-focused**. The Makefile commands (`make deploy-local-isp`, etc.) are for Docker deployments only.

For **physical hardware**:
- **Manual installation required**: BIRD and TINC packages
- **Use repository configs**: All configurations in `configs/` directory
- **Reference Docker scripts**: `docker/*/entrypoint.sh` for setup logic

### What Repository Provides

✅ **BIRD configurations**: `configs/isp-bird/bird.conf`, `configs/bird/*.conf`  
✅ **TINC templates**: `configs/tinc/*.j2`  
✅ **Setup logic**: `docker/bird/entrypoint.sh`, `docker/tinc/entrypoint.sh`  
✅ **Network architecture**: `docker-compose.yml` shows complete setup  

### What Repository Does NOT Provide

❌ Physical hardware installation scripts  
❌ OS-level package management  
❌ Bare-metal deployment automation  

You must manually:
- Install BIRD2 and TINC packages on each device
- Adapt Docker configs for bare-metal
- Configure network interfaces

## Prerequisites (All Devices)

- Linux OS (Debian/Ubuntu recommended)
- Root/sudo access
- Network connectivity between devices

## Time Estimate

- Raspberry Pi: 20 minutes
- Laptop n1: 30 minutes
- Laptop n2: 15 minutes
- Verification: 10 minutes
- **Total**: ~75 minutes

## Critical Configuration Points

1. **Route export on Laptop n1**: Must export TINC subnet to ISP
2. **BGP session**: Must establish between RPi (172.30.0.1) and Laptop n1 (172.30.0.100)
3. **TINC connectivity**: Laptop n1 and n2 must ping via 44.30.127.x
4. **Kernel routes**: BIRD must sync routes to/from kernel

## Verification Checklist

- [ ] BGP session `Established` between RPi and Laptop n1
- [ ] Laptop n1 can ping Laptop n2 via TINC (44.30.127.2)
- [ ] Mock-ISP has route to `44.30.127.0/24` via `172.30.0.100`
- [ ] **Mock-ISP can ping `44.30.127.2`** ✅ Goal achieved!

## Next Steps

1. Read device-specific guides (01, 02, 03)
2. Install packages on each device
3. Copy/adapt repository configs
4. Start services and verify

---

**Start with**: `01-MOCK-ISP-RPI.md`

