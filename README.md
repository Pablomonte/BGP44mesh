# BGP Overlay Network over TINC Mesh

A production-grade BGP routing framework with automated orchestration, combining BIRD 3.x, TINC 1.0 mesh VPN, etcd distributed storage, and custom Go daemon for peer discovery.

## Stack

- **BIRD 3.x**: BGP routing daemon (MP-BGP, RPKI validation)
- **TINC 1.0**: Layer 2 mesh VPN (switch mode, RSA-2048, AES-256)
- **etcd 3.5+**: Distributed config/state storage with HA
- **Ansible**: Infrastructure orchestration
- **Go daemon**: Custom propagation (mDNS discovery, key distribution, config sync)
- **Prometheus + Grafana**: Metrics and monitoring
- **Docker**: Service containerization

## Quick Start

```bash
# Setup
cp .env.example .env
make deploy-local

# Verify (wait ~90s for convergence)
docker exec bird1 birdc show protocols
docker exec tinc1 tinc -n bgpmesh info
docker exec etcd1 etcdctl endpoint health

# Monitor
make monitor  # Opens Grafana at http://localhost:3000

# Test
make test-all

# Cleanup
make clean
```

See [QUICKSTART.md](docs/QUICKSTART.md) for detailed instructions.

## Common Commands

```bash
# Container status
make status
docker ps

# BIRD (BGP routing)
docker exec bird1 birdc show protocols        # All protocols
docker exec bird1 birdc show protocols all peer1  # Peer detail
docker exec bird1 birdc show route            # Routing table

# TINC (VPN mesh)
docker exec tinc1 ip addr show tinc0          # Interface status

# etcd (distributed storage)
docker exec etcd1 etcdctl member list         # Cluster members
docker exec etcd1 etcdctl endpoint health     # Cluster health
docker exec etcd1 etcdctl put /key "value"    # Write
docker exec etcd1 etcdctl get /key            # Read

# Logs
docker logs -f bird1                          # Follow logs
docker compose logs bird1 bird2 bird3         # Multiple services

# Access containers
docker exec -it bird1 /bin/bash               # Interactive shell
```

## Project Structure

```
BGP/
├── docker-compose.yml          # 15 services (5 bird + 5 tinc + 5 etcd + monitoring)
├── Makefile                    # Build/deploy automation
├── configs/                    # BIRD/TINC templates (Jinja2)
├── docker/                     # Container builds
├── ansible/                    # Infrastructure orchestration (4 roles)
├── daemon-go/                  # Custom Go propagation daemon
├── tests/                      # Validation, integration, E2E tests
└── docs/                       # Documentation

## Architecture

- **Layer 2**: TINC mesh (switch mode) with UDP hole punching
- **Layer 3**: BIRD BGP sessions over TINC tunnels
- **State**: etcd cluster for peer propagation
- **Discovery**: Go daemon with mDNS over TINC interface
- **Monitoring**: Prometheus scraping BIRD metrics, Grafana dashboards

See [docs/architecture/decisions.md](docs/architecture/decisions.md) for design decisions.

## Development

```bash
# Run all tests
make test-all

# Test individual components
make test-env          # Environment variables
make test-configs      # Configuration templates
make test-builds       # Docker builds
make test-integration  # BGP/TINC/etcd integration
make test-e2e          # Full stack workflow

# Development workflow
vim configs/bird/bird.conf.j2
make validate
docker restart bird1 bird2 bird3
```

## Requirements

- Docker 24+ with Compose v2
- Go 1.21+ (for daemon development)
- Ansible 2.16+ (for production deployment)
- >8GB RAM (>16GB recommended for parallel builds)

## Performance Targets

- Deployment: <2min convergence
- BGP: <30s reconvergence with BFD
- etcd: <10ms quorum reads
- TINC: <50ms overhead vs direct

## Sprint Status

### Sprint 2 Phase 1 (Completed 2025-10-28)

- **Testing**: Makefile (20+ targets), CI coverage enforcement
  - pkg/tinc: 92.7% coverage (11 test functions)
  - pkg/discovery: 89.8% coverage (9 test functions)
  - pkg/types: 100% coverage
- **Scaling**: 5-node Docker deployment (15 containers, etcd quorum)
- **Automation**: 4 Ansible roles (etcd, tinc, bird, bgp-daemon)
- **Docs**: Testing guide, Deployment guide, Status report

**Commands**:
```bash
cd daemon-go && make test-coverage  # Run tests with coverage
cd daemon-go && make test-unit      # Fast tests (skip integration)
make deploy-local                    # Docker 5-node
cd ansible && ansible-playbook -i inventory/hosts.ini playbook.yml  # Ansible
```

**Next**: Sprint 2 Phase 2 (custom Grafana dashboards, additional integration tests)

### Sprint 1 (Completed)

Local 3-node MVP, Docker orchestration, basic tests, Grafana monitoring

### Roadmap

- **Sprint 2 Phase 2**: Unit tests completion, custom Grafana dashboards
- **Sprint 3**: Production hardening (rolling updates, chaos testing)
- **Sprint 4**: Scalability (route reflectors, RPKI, multi-region)

## License

TBD

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) (coming in Sprint 2)

---

**AI-assisted development**
