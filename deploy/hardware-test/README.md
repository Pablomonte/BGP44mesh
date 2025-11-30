# Hardware Test Deployment Files

Docker Compose files for the 3-device hardware test setup (RPi + 2 Laptops).

## Files

| File | Device | Description |
|------|--------|-------------|
| `docker-compose.isp.yml` | Raspberry Pi | Mock ISP (AS 65001, BIRD in host network mode) |
| `docker-compose.border-router.yml` | Laptop n1 | Border Router (AS 65000, BIRD + TINC with macvlan) |
| `docker-compose.mesh-node.yml` | Laptop n2 | Mesh Node (TINC only) |

## Network Topology

```
RPi (Mock-ISP)          Laptop n1 (Border Router)      Laptop n2 (Mesh Node)
172.30.0.1              172.30.0.100 + 44.30.127.1     172.30.0.101 + 44.30.127.2
AS 65001                AS 65000                       TINC only
    │                        │                              │
    │◄──── BGP eBGP ────────►│◄──── TINC VPN Mesh ─────────►│
    │                        │                              │
```

## Usage

### On Raspberry Pi (Mock-ISP):
```bash
cd /path/to/BGP4mesh
docker compose -f deploy/hardware-test/docker-compose.isp.yml up -d --build
```

### On Laptop n1 (Border Router):
```bash
cd /path/to/BGP4mesh
# Configure .env first (see documentation)
docker compose -f deploy/hardware-test/docker-compose.border-router.yml up -d --build
```

### On Laptop n2 (Mesh Node):
```bash
cd /path/to/BGP4mesh
docker compose -f deploy/hardware-test/docker-compose.mesh-node.yml up -d --build
```

## Documentation

See `first-test-rpi/` folder for detailed setup guides:
- `00-OVERVIEW.md` - Architecture overview
- `01-MOCK-ISP-RPI.md` - RPi setup
- `02-BORDER-ROUTER-LAPTOP-N1.md` - Laptop n1 setup
- `03-MESH-NODE-LAPTOP-N2.md` - Laptop n2 setup
- `RESULTS.md` - Test results

## Prerequisites

- Docker 24+ and Docker Compose v2
- Linux kernel with macvlan support (Laptop n1 only)
- All devices connected via Ethernet switch (172.30.0.0/24)

