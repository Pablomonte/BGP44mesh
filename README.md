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

## Project Structure

```
BGP/
├── docker-compose.yml          # 9 services (3 bird + 3 tinc + 3 etcd + monitoring)
├── Makefile                    # Build/deploy automation
├── configs/                    # BIRD/TINC templates (Jinja2)
├── docker/                     # Container builds
├── ansible/                    # Infrastructure orchestration
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

## Sprint Roadmap

- **Sprint 1** (current): Local 3-node MVP, Docker orchestration, basic tests
- **Sprint 2**: Automation (mDNS discovery, key distribution, Ansible roles)
- **Sprint 3**: Production hardening (rolling updates, chaos testing)
- **Sprint 4**: Scalability (route reflectors, RPKI, multi-region)

## License

TBD

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) (coming in Sprint 2)

---

**AI-assisted development**
