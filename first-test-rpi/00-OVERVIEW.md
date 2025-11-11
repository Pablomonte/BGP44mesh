# Hardware Test Setup - Overview

## Goal
Get **Mock-ISP (Raspberry Pi)** to ping **Laptop n2** through BGP routing and TINC VPN mesh using **Docker containers**.

## Architecture

```
Raspberry Pi (Docker)          Laptop n1 (Docker)              Laptop n2 (Docker)
isp-bird container          bird1 + tinc1 + etcd1            tinc2 + etcd1
AS 65001, BIRD              AS 65000, BIRD + TINC             TINC only
172.30.0.1/24              172.30.0.100/24 + 44.30.127.1/24  44.30.127.2/24
     │                          │                                  │
     │◄─────── BGP eBGP ────────►│◄──── TINC VPN Mesh ────────────►│
     │                          │                                  │
   Announces                  Routes between                 Receives routes
   192.0.2.0/24               ISP & TINC mesh                via kernel
```

## Network Subnets

- **ISP Network**: `172.30.0.0/24` (physical connection between RPi and Laptop n1)
- **TINC Mesh**: `44.30.127.0/24` (VPN overlay between Laptop n1 and n2)

## Docker Services

Each device runs Docker containers:

- **Raspberry Pi**: `isp-bird` (BIRD daemon in host network mode)
- **Laptop n1**: `bird1`, `tinc1`, `etcd1` (BIRD shares network with TINC, uses macvlan for ISP connectivity)
- **Laptop n2**: `tinc2`, `etcd1` (TINC mesh node)

## How Mock-ISP Pings Laptop n2

1. **Laptop n2** announces `44.30.127.2/32` via TINC to **Laptop n1**
2. **Laptop n1** (BIRD) learns this route from kernel
3. **Laptop n1** announces `44.30.127.0/24` to **Mock-ISP** via BGP
4. **Mock-ISP** learns route: `44.30.127.0/24 via 172.30.0.100` (next hop: Laptop n1)
5. **Mock-ISP** pings `44.30.127.2` → routes to Laptop n1 → TINC forwards to Laptop n2

## Setup Order

1. **Raspberry Pi**: Deploy Mock-ISP with Docker → `01-MOCK-ISP-RPI.md`
2. **Laptop n1**: Deploy BIRD + TINC with Docker → `02-BORDER-ROUTER-LAPTOP-N1.md`
3. **Laptop n2**: Deploy TINC with Docker → `03-MESH-NODE-LAPTOP-N2.md`
4. **Verify**: Mock-ISP can ping Laptop n2

## Repository Information

**✅ This repository uses Docker for all services**. All setup is done via Docker Compose.

### What Repository Provides

✅ **Docker Compose files**: `docker-compose.yml`, `docker-compose.isp.yml`  
✅ **Docker images**: `docker/bird/`, `docker/tinc/` with entrypoint scripts  
✅ **BIRD configurations**: `configs/isp-bird/bird.conf`, `configs/bird/*.conf`  
✅ **TINC templates**: `configs/tinc/*.j2` (rendered by entrypoint scripts)  
✅ **Network setup**: Docker networks and macvlan for physical connectivity  
✅ **Makefile commands**: `make deploy-local-isp`, etc.  

### How It Works

1. **Docker Compose** orchestrates all services
2. **Entrypoint scripts** render configuration templates from environment variables
3. **Docker networks** provide virtual interfaces (isp-net, mesh-net)
4. **Macvlan** provides direct L2 access to physical network (for Laptop n1)
5. **Host network mode** used on Raspberry Pi for direct interface access

## Prerequisites (All Devices)

- Linux OS (Debian/Ubuntu recommended)
- Docker 24+ and Docker Compose v2
- Root/sudo access (for Docker and network configuration)
- Network connectivity between devices
- **Laptop n1 only**: Linux kernel with macvlan support (for physical network access)

## Time Estimate

- Raspberry Pi: 15 minutes
- Laptop n1: 20 minutes
- Laptop n2: 15 minutes
- Verification: 5 minutes
- **Total**: ~55 minutes

## Critical Configuration Points

1. **Route export on Laptop n1**: Must export TINC subnet (44.30.127.0/24) to ISP
2. **BGP session**: Must establish between RPi (172.30.0.1) and Laptop n1 (172.30.0.100 via macvlan)
3. **TINC connectivity**: Laptop n1 and n2 must connect via TINC mesh (44.30.127.x)
4. **Macvlan setup**: Laptop n1 needs macvlan network for physical ISP connectivity
5. **ISP import filter**: Must accept 44.30.127.0/24 route from customer

## Verification Checklist

- [ ] BGP session `Established` between RPi and Laptop n1
- [ ] Laptop n1 can ping Laptop n2 via TINC (44.30.127.2)
- [ ] Mock-ISP has route to `44.30.127.0/24` via `172.30.0.100`
- [ ] **Mock-ISP can ping `44.30.127.2`** ✅ Goal achieved!

## Next Steps

1. Read device-specific guides (01, 02, 03)
2. Install Docker and Docker Compose on each device
3. Clone repository and configure environment variables
4. Deploy services with Docker Compose
5. Exchange TINC host files between Laptop n1 and n2
6. Verify connectivity and test ping

---

**Start with**: `01-MOCK-ISP-RPI.md`

