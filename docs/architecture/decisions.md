# Architecture Decision Records (ADRs)

## ADR-001: BIRD 3.x over BIRD 1.6 and FRR

**Date**: 2025-10-27
**Status**: Accepted

### Context

Need a BGP routing daemon that supports both IPv4 and IPv6 (MP-BGP) for our overlay network. Options considered:
- BIRD 1.6 (legacy, still in use)
- BIRD 3.x (current, MP-BGP unified)
- FRR (feature-rich, Quagga fork)

### Decision

Use **BIRD 3.x** (3.1.4+)

### Rationale

**Pros:**
- MP-BGP support unified in single daemon (no need for separate bird/bird6)
- Modern config syntax (30-40% less boilerplate vs 1.6)
- RPKI validation built-in (RFC 6811) for route origin validation
- BFD integration for fast reconvergence (<30s)
- Active development and security updates
- Lower memory footprint than FRR (~100MB per 10k routes vs ~200MB)

**Cons:**
- Higher memory usage than BIRD 1.6 (~20% increase)
- Config syntax incompatible with 1.6 (migration required)
- Fewer production deployments than 1.6 (maturity trade-off)

**Trade-offs:**
- Memory overhead acceptable given modern hardware targets (>8GB RAM)
- Syntax migration one-time cost, payoff in maintainability
- Faster reconvergence (BFD) worth the ~20MB extra RAM

### Alternatives Discarded

- **BIRD 1.6**: EOL, no MP-BGP, legacy syntax
- **FRR**: 2x memory overhead on embedded hardware, overkill for our use case
- **Quagga**: Obsolete (forked to FRR)

### Consequences

- Docker image size: ~100MB (acceptable)
- Config templates use BIRD 3.x syntax
- RPKI integration available for future (Sprint 4)
- Reconvergence <30s with BFD (vs ~90s without)

---

## ADR-002: TINC 1.0 over TINC 1.1 and WireGuard

**Date**: 2025-10-27
**Status**: Accepted

### Context

Need a Layer 2 mesh VPN for BGP overlay. Options:
- TINC 1.0 (legacy, stable, switch mode)
- TINC 1.1 (modern, invitation system)
- WireGuard (fast, point-to-point, kernel-level)

### Decision

Use **TINC 1.0** (1.0.36+)

### Rationale

**Pros:**
- **Switch mode**: Full Layer 2 mesh, transparent to BGP
- **Legacy compatibility**: Works on OpenWrt 23.05+ (important for future production)
- **Stable**: Battle-tested in production environments
- **NAT traversal**: UDP hole punching for nodes behind firewalls
- **RSA-2048**: Strong encryption, upgrade path to 4096

**Cons:**
- Manual key exchange (no invitations like 1.1)
- Older codebase (less active development)
- Higher overhead than WireGuard (~50ms vs ~20ms)

**Trade-offs:**
- Manual key exchange mitigated by automation (Go daemon in Sprint 2)
- Latency overhead acceptable for dev/test (production tuning later)
- Switch mode essential for L2 BGP adjacency

### Alternatives Discarded

- **TINC 1.1**: Invitations nice but incompatible with OpenWrt legacy kernels (<5.10)
- **WireGuard**: Point-to-point only, would need custom mesh logic (complexity)
- **VXLAN**: Requires multicast, not suitable for public internet

### Consequences

- Need custom key distribution automation (Sprint 2 Go daemon)
- MTU tuning required (1400 on tun0 vs 1500 on host)
- Compatible with OpenWrt gateways in production
- UDP port 655 must be open in firewalls

---

## ADR-003: etcd over Consul and IPFS

**Date**: 2025-10-27
**Status**: Accepted

### Context

Need distributed key-value store for:
- Peer propagation (IPs, keys, endpoints)
- Config sync (bird.conf, tinc.conf)
- Health status monitoring

Options:
- etcd (Raft consensus, watch API)
- Consul (service discovery, DNS)
- IPFS (content-addressable, fully distributed)

### Decision

Use **etcd** (v3.5.14+)

### Rationale

**Pros:**
- **Lightweight**: 50MB/node vs 200MB Kafka or 500MB+ Consul
- **Raft consensus**: Strong consistency, 3-node quorum tolerates 1 failure
- **Watch API**: Real-time updates for Go daemon
- **Low latency**: <10ms reads for peer lookups
- **Simple ops**: No Zookeeper dependency (unlike Kafka)

**Cons:**
- Centralized cluster (vs fully distributed IPFS)
- Needs quorum (>50% nodes) for writes
- No built-in service discovery (vs Consul)

**Trade-offs:**
- Centralized but HA (3-node raft) acceptable for 50-node target scale
- Quorum requirement mitigated by running etcd on stable servers
- Service discovery handled by mDNS in Go daemon (simpler)

### Alternatives Discarded

- **Consul**: Heavier, overkill for our needs, DNS features unused
- **IPFS**: Slow cold starts (~30s), high bandwidth overhead (10%), storage issues on OpenWrt
- **Redis**: No consensus, single point of failure without complex Sentinel setup
- **Git**: No real-time updates, not suitable for dynamic state

### Consequences

- etcd cluster must be highly available (3+ nodes)
- Quorum loss blocks writes (acceptable for config sync, not critical path)
- Ansible etcd3 module for config management integration
- Encryption at rest needed for secrets (Sprint 3)

---

## ADR-004: Docker Compose over Kubernetes for Sprint 1

**Date**: 2025-10-27 (updated 2025-11-03 for Sprint 1.5)
**Status**: Accepted (Sprint 1 & 1.5)

### Context

Need local development orchestration for services:
- **Sprint 1**: 9 services (3 bird + 3 tinc + 3 etcd + monitoring)
- **Sprint 1.5**: 21 containers (5 bird + 5 tinc + 5 daemon + 5 etcd + prometheus)

### Decision

Use **Docker Compose** for Sprint 1 & 1.5, migrate to **systemd** for production (Sprint 3+)

### Rationale

**Pros:**
- **Simplicity**: Single docker-compose.yml, `make deploy-local` converges <2min
- **Low overhead**: No control plane (vs k8s)
- **Local dev optimized**: Fast iteration cycles
- **Multi-stage builds**: Reduce image sizes ~20-30%

**Cons:**
- Not production-ready (no HA, no auto-scaling)
- Single-host only (no multi-node orchestration)

**Trade-offs:**
- Perfect for MVP and testing, production uses systemd on Debian/OpenWrt
- Kubernetes overkill for 3-50 node target scale

### Future Migration

- Sprint 3: systemd units for production Debian servers
- Sprint 4: OpenWrt native packages (opkg) for gateways

---

## ADR-005: Ansible Push + Pull Hybrid

**Date**: 2025-10-27
**Status**: Accepted

### Context

Need config management for production deployment and continuous sync.

### Decision

Use **Ansible push** for initial provisioning, **ansible-pull** for continuous config management (5min cron)

### Rationale

**Pros:**
- **Idempotent**: Safe repeated runs, no config drift
- **Agentless**: SSH-based, works with OpenWrt dropbear
- **ansible-pull**: Mitigates firewall/NAT issues for nodes behind firewalls
- **Fast**: <1min per node for config updates

**Cons:**
- Not real-time (vs Salt/Puppet agents)
- ansible-pull needs Git repo access

**Trade-offs:**
- 5min update interval sufficient for config changes (not realtime critical)
- Git dependency acceptable (already using for versioning)

### Consequences

- All nodes need Git + Ansible packages
- Secrets management via Ansible Vault (Sprint 3)
- CI/CD integration in Sprint 2

---

## ADR-006: Go for Custom Daemon over Python

**Date**: 2025-10-27
**Status**: Accepted

### Context

Need custom daemon for:
- mDNS peer discovery over TINC
- etcd integration (watch /peers/)
- Config sync automation

### Decision

Use **Go** (1.21+)

### Rationale

**Pros:**
- **Cross-platform**: Single binary for Linux/OpenWrt ARM/x86
- **Low overhead**: <10MB RAM, <1% CPU idle
- **Concurrency**: Goroutines for etcd watches + mDNS lookup
- **Static binary**: No runtime dependencies (vs Python venv)
- **Performance**: Fast startup (<100ms)

**Cons:**
- Larger team familiarity with Python
- Compilation step (vs interpreted Python)

**Trade-offs:**
- Learning curve acceptable given performance benefits
- Static binary deployment simpler than Python deps on OpenWrt

### Consequences

- Go 1.21+ required for development
- `go build` produces single binary
- Containerized for dev, native binary for production OpenWrt

---

## ADR-007: TDD Moderate Approach (Opción A)

**Date**: 2025-10-28
**Status**: Accepted

### Context

Need testing strategy balancing coverage with MVP speed.

### Decision

**TDD Moderate** (Opción A): Tests mínimos críticos - config validation, Docker builds, integration, E2E

**Skip:** Extensive unit tests (Python pytest, Go >80% coverage)
**Focus:** End-to-end functional validation

### Rationale

**Pros:**
- **Speed**: ~3-4h implementation (vs 7-8h full TDD)
- **Pragmatic**: Tests critical path without over-engineering
- **Bash-based**: Simple, fast, minimal dependencies

**Cons:**
- Lower coverage metrics (~60% vs 80%)
- Fewer edge cases caught by unit tests

**Trade-offs:**
- Sprint 1 MVP prioritizes functional validation
- Unit test expansion in Sprint 2 when daemon matures

### Consequences

- `make test-all` validates: env vars, configs, builds, integration, E2E
- CI runs on every push (GitHub Actions)
- Coverage expansion tracked for Sprint 2

---

## Future ADRs (Planned)

- **ADR-008**: Route Reflectors for scalability (Sprint 4)
- **ADR-009**: RPKI validation integration (Sprint 4)
- **ADR-010**: Multi-region etcd replication (Sprint 4)
- **ADR-011**: BGP MD5 vs TCP-AO authentication (Sprint 3)
- **ADR-012**: Chaos testing strategy (Sprint 3)
