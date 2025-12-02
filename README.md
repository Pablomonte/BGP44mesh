# BGP4mesh - BGP + Netmaker VPN Overlay

Two autonomous systems communicating via BGP, with Netmaker providing the VPN mesh.

## Architecture

```
Raspberry Pi (Mock-ISP)     Laptop n1 (Border Router)      Laptop n2 (Mesh Node)
AS 65001, 172.30.0.1        AS 65000, 172.30.0.100         Netmaker client
                            Netmaker: 44.30.127.1          Netmaker: 44.30.127.2
        │                           │                              │
        │◄─── BGP eBGP ────────────►│◄───── Netmaker VPN ─────────►│
        │                           │                              │
   Announces                  Border Router                 Mesh Node
   Test-Net ranges            Routes ISP ↔ Mesh             Receives routes via Netmaker
```

## Components

| Device | Role | AS | IP (physical) | IP (Netmaker) |
|--------|------|-----|---------------|---------------|
| Raspberry Pi | Mock ISP (BIRD) | 65001 | 172.30.0.1 | - |
| Laptop n1 | Border Router (BIRD + Netmaker) | 65000 | 172.30.0.100 | 44.30.127.1 |
| Laptop n2 | Mesh Node (Netmaker only) | - | 172.30.0.101 | 44.30.127.2 |

## Quick Start

Each device runs its own docker-compose from the `deploy/` folder:

```bash
# On Raspberry Pi (ISP)
cd deploy/rpi-isp && docker compose up -d

# On Laptop n1 (Border Router)
cd deploy/laptop-border && docker compose up -d

# On Laptop n2 (Mesh Node)
cd deploy/laptop-mesh && docker compose up -d
```

## Configuration

**Border Router (laptop-border):** Create `.env` file:
```bash
SERVER_HOST=172.30.0.100        # Your physical IP
MASTER_KEY=your-secure-key      # Netmaker API key
ENROLLMENT_TOKEN=               # Set after creating network in Netmaker
```

**Mesh Node (laptop-mesh):** Create `.env` file:
```bash
ENROLLMENT_TOKEN=<token-from-netmaker>
```

## Goal

Test BGP route propagation: ISP announces test prefixes → Border Router learns them → Mesh nodes receive them via Netmaker.
