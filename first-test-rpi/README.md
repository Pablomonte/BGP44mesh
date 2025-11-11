# First Hardware Test - Mock ISP Ping via BGP + TINC

## Goal
Configure 3 physical devices so **Mock-ISP (Raspberry Pi) can ping Laptop n2** through BGP routing and TINC VPN.

## Quick Start

Follow these documents **in order**:

1. **[00-OVERVIEW.md](./00-OVERVIEW.md)** - Architecture and prerequisites (~5 min read)
2. **[01-MOCK-ISP-RPI.md](./01-MOCK-ISP-RPI.md)** - Raspberry Pi setup (~20 min)
3. **[02-BORDER-ROUTER-LAPTOP-N1.md](./02-BORDER-ROUTER-LAPTOP-N1.md)** - Laptop n1 setup (~30 min)
4. **[03-MESH-NODE-LAPTOP-N2.md](./03-MESH-NODE-LAPTOP-N2.md)** - Laptop n2 setup (~15 min)

**Total time**: ~75 minutes

## Architecture

```
Raspberry Pi          Laptop n1              Laptop n2
172.30.0.1       ←BGP→ 172.30.0.100   ←TINC→ 44.30.127.2
AS 65001, BIRD        + 44.30.127.1          TINC only
                      AS 65000
                      BIRD + TINC
```

## Device Configuration Summary

| Device | Software | IPs | Config Files |
|--------|----------|-----|--------------|
| Raspberry Pi | BIRD | 172.30.0.1 | 01-MOCK-ISP-RPI.md |
| Laptop n1 | BIRD + TINC | 172.30.0.100 + 44.30.127.1 | 02-BORDER-ROUTER-LAPTOP-N1.md |
| Laptop n2 | TINC | 44.30.127.2 | 03-MESH-NODE-LAPTOP-N2.md |

## Success Test

After completing all setup:

```bash
# On Raspberry Pi
ping -c 5 44.30.127.2
# Should succeed ✅
```

## Repository Info

**⚠️ Important**: This repository is Docker-focused. The Makefile commands are for Docker only.

**What we use**:
- Configuration files from `configs/isp-bird/`, `configs/bird/`, `configs/tinc/`
- Setup logic from `docker/*/entrypoint.sh` (as reference)

**What we install manually**:
- BIRD and TINC packages (not provided by repository)

## Files

- `00-OVERVIEW.md` (3.8 KB) - General info, architecture, how ping works
- `01-MOCK-ISP-RPI.md` (4.5 KB) - Raspberry Pi setup with BIRD
- `02-BORDER-ROUTER-LAPTOP-N1.md` (7.0 KB) - Laptop n1 with BIRD + TINC
- `03-MESH-NODE-LAPTOP-N2.md` (6.1 KB) - Laptop n2 with TINC only

---

**Start with**: `00-OVERVIEW.md`

