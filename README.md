# BGP Overlay Network over TINC Mesh

A production-grade BGP routing framework with ISP multi-homing, combining BIRD 2.x, TINC 1.0 mesh VPN, and etcd distributed storage.

## Stack

- **BIRD 2.x**: BGP routing daemon with multi-homing support
- **TINC 1.0**: Layer 2 mesh VPN (switch mode, RSA-2048, AES-256)
- **etcd 3.5+**: Distributed storage for TINC peer discovery
- **Ansible**: Infrastructure orchestration (production deployment)
- **Docker**: Service containerization (8 containers)

## Quick Start

```bash
# Setup
cp .env.example .env
make deploy-local-isp  # Deploys 8 containers with ISP multi-homing

# Verify BGP multi-homing (2 uplinks to ISP)
docker exec bird1 birdc show protocols        # Should show 2 Established
docker exec isp-bird birdc show protocols     # Should show 2 Established
docker exec bird1 birdc show route all 192.0.2.0/24  # Check local-pref

# Test
./tests/integration/test_isp_integrated.sh    # 8/8 tests

# Cleanup
make clean
```

See [QUICKSTART.md](docs/QUICKSTART.md) for detailed instructions.

### External ISP Integration

To connect the mesh network to an external ISP:

```bash
# Configure .env with ISP settings (see docs for details)
make deploy-with-external-isp

# Verify BGP session
make verify-isp
```

See [docs/EXTERNAL-ISP-INTEGRATION.md](docs/EXTERNAL-ISP-INTEGRATION.md) for complete ISP integration guide.

## Common Commands

```bash
# Container status
docker ps  # 8 containers: 5 TINC + 1 BIRD + 1 ISP + 1 etcd

# BIRD (BGP routing - multi-homing)
docker exec bird1 birdc show protocols              # 2 ISP uplinks (Established)
docker exec bird1 birdc show protocols all isp_primary   # Primary link detail
docker exec bird1 birdc show route all 192.0.2.0/24 # Check local-pref (200 vs 150)

# ISP mock
docker exec isp-bird birdc show protocols           # 2 customer sessions
docker exec isp-bird birdc show route               # ISP routes (no 44.30.127.0/24)

# TINC (VPN mesh - 5 nodes)
docker exec tinc1 ip addr show tinc0                # 44.30.127.1/24
docker exec tinc2 ping -c 3 44.30.127.1             # Mesh connectivity

# etcd (single node)
docker exec etcd1 etcdctl get /peers --prefix       # TINC peer info

# Logs
docker logs -f bird1                                # Border router logs
docker logs -f isp-bird                             # ISP mock logs
```

## Project Structure

```
BGP/
├── docker-compose.yml          # 8 services (5 TINC + 1 BIRD + 1 ISP + 1 etcd)
├── Makefile                    # Build/deploy automation
├── configs/
│   ├── bird/                   # BIRD border router (multi-homing)
│   ├── isp-bird/               # ISP mock (dual BGP sessions)
│   └── tinc/                   # TINC mesh templates
├── docker/                     # Container builds
├── tests/integration/          # Multi-homing integration tests
└── docs/                       # Documentation

## Architecture

- **TINC mesh**: 5 nodes (44.30.127.0/24) - Layer 2 VPN only
- **Border router**: bird1 with dual ISP uplinks (eBGP multi-homing)
  - Primary: 172.30.0.3 → 172.30.0.2 (local-pref 200)
  - Secondary: 172.31.0.3 → 172.31.0.2 (local-pref 150)
- **ISP mock**: Dual BGP sessions, announces TEST-NET prefixes
- **State**: Single etcd node for TINC peer discovery

See [docs/architecture/decisions.md](docs/architecture/decisions.md) for design decisions.

## Development

```bash
# Run integration tests
./tests/integration/test_isp_integrated.sh  # Multi-homing validation

# Development workflow
vim configs/bird/protocols.conf.j2  # Modify BGP configuration
docker restart bird1                 # Apply changes
docker exec bird1 birdc show protocols  # Verify
```

## Requirements

- Docker 24+ with Compose v2
- Go 1.21+ (for daemon development - optional)
- Ansible 2.16+ (for production deployment - optional)
- >4GB RAM

## Performance

- Deployment: <1min convergence (8 containers)
- BGP: Dual uplink with automatic failover (local-pref based)
- TINC: 5-node mesh with <50ms overhead

## Sprint Status

### Current: Multi-homing Refactor (2025-11-10)

**Architecture change**: Full mesh iBGP (5 routers) → Single border router with ISP multi-homing

- **Simplification**: 22 containers → 8 containers
- **Multi-homing**: Dual ISP uplinks with BGP local-pref (200 primary, 150 backup)
- **Networks**:
  - TINC mesh: 44.30.127.0/24 (5 VPN nodes)
  - ISP primary: 172.30.0.0/24
  - ISP secondary: 172.31.0.0/24
- **Testing**: 8/8 integration tests passing

**Deploy**:
```bash
make deploy-local-isp                        # 8 containers with multi-homing
./tests/integration/test_isp_integrated.sh   # Verify
```

### Previous Sprints

- **Sprint 2 Phase 1**: Go daemon testing (92%+ coverage), 5-node scaling, Ansible roles
- **Sprint 1**: 3-node MVP, Docker orchestration, monitoring

### Roadmap

- **Next**: Production hardening, route reflectors
- **Future**: RPKI validation, multi-region support

## License

TBD

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) (coming in Sprint 2)

---

**AI-assisted development**
