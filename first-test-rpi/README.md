# First Hardware Test - Mock ISP Ping via BGP + TINC

## Goal
Configure 3 physical devices so **Mock-ISP (Raspberry Pi) can ping Laptop n2** through BGP routing and TINC VPN using **Docker containers**.

## Quick Start

Follow these documents **in order**:

1. **[00-OVERVIEW.md](./00-OVERVIEW.md)** - Architecture and prerequisites (~5 min read)
2. **[01-MOCK-ISP-RPI.md](./01-MOCK-ISP-RPI.md)** - Raspberry Pi Docker setup (~15 min)
3. **[02-BORDER-ROUTER-LAPTOP-N1.md](./02-BORDER-ROUTER-LAPTOP-N1.md)** - Laptop n1 Docker setup (~20 min)
4. **[03-MESH-NODE-LAPTOP-N2.md](./03-MESH-NODE-LAPTOP-N2.md)** - Laptop n2 Docker setup (~15 min)

**Total time**: ~55 minutes

## Architecture

```
Raspberry Pi (Docker)          Laptop n1 (Docker)              Laptop n2 (Docker)
isp-bird container          bird1 + tinc1 containers          tinc2 container
172.30.0.1/24              172.30.0.100/24 + 44.30.127.1/24    44.30.127.2/24
AS 65001, BIRD             AS 65000, BIRD + TINC               TINC only
     │                          │                                  │
     │◄─────── BGP eBGP ────────►│◄──── TINC VPN Mesh ────────────►│
     │                          │                                  │
```

## Device Configuration Summary

| Device | Docker Services | IPs | Network Setup |
|--------|----------------|-----|---------------|
| Raspberry Pi | `isp-bird` | 172.30.0.1/24 | Host network mode |
| Laptop n1 | `bird1` + `tinc1` + `etcd1` | 172.30.0.100/24 (macvlan) + 44.30.127.1/24 (TINC) | Macvlan + Docker networks |
| Laptop n2 | `tinc2` + `etcd1` | 44.30.127.2/24 (TINC) | Docker networks |

## Success Test

After completing all setup:

```bash
# On Raspberry Pi (from host or inside isp-bird container)
ping -c 5 44.30.127.2
# Should succeed ✅
```

## Repository Info

**✅ This repository uses Docker for all services**. All BIRD and TINC services run in containers.

**What the repository provides**:
- Docker Compose files for orchestration
- Docker images for BIRD and TINC
- Configuration templates in `configs/`
- Entrypoint scripts that render configurations
- Network setup via Docker networks and macvlan

**Prerequisites**:
- Docker 24+ and Docker Compose v2
- Linux kernel with macvlan support (for Laptop n1)
- Physical network connectivity between devices

## Files

- `00-OVERVIEW.md` - General info, Docker architecture, how ping works
- `01-MOCK-ISP-RPI.md` - Raspberry Pi Docker setup with BIRD
- `02-BORDER-ROUTER-LAPTOP-N1.md` - Laptop n1 Docker setup with BIRD + TINC
- `03-MESH-NODE-LAPTOP-N2.md` - Laptop n2 Docker setup with TINC only

---

**Start with**: `00-OVERVIEW.md`

