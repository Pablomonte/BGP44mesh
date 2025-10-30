# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**BGP Overlay Network over TINC Mesh** - A minimalist but robust 5-node local development environment (Sprint 1.5) that integrates:
- **BIRD 2.x**: BGP routing daemon with dynamic peer configuration
- **TINC 1.0**: Layer 2 mesh VPN (switch mode, RSA-2048, AES-256)
- **etcd 3.5+**: Distributed key-value store for peer propagation
- **Prometheus/Grafana**: Monitoring and metrics
- **Go daemon**: Custom propagation logic (etcd watch, TINC topology management)
- **Ansible**: Orchestration with atomic roles (for production deployment)

**Total Files**: 29, optimized for quick bootstrap
**Convergence Time**: <2min on hosts with >8GB RAM
**Architecture Focus**: Separation of concerns with idempotent configs, dynamic peer discovery
**Scalability**: Full mesh topology supports any number of nodes (tested with 3-5 nodes)

## Quick Start

```bash
# Prerequisites check
docker --version    # Need 24+
go version         # Need 1.21+
ansible --version  # Need 2.16+

# Setup
cp .env.example .env
make deploy-local  # Converges in <2min

# Verify
make test          # Integration tests

# Monitor
make monitor       # Open Grafana at localhost:3000

# Cleanup
make clean
```

## Project Structure (Exact)

```
project-bgp/
├── .gitignore                  # Go builds, .env, Docker caches, TINC keys
├── README.md                   # Overview linking to QUICKSTART, stack description
├── Makefile                    # Automation: deploy-local, test, monitor, clean, validate, help
├── docker-compose.yml          # 15 services: 5×bird/tinc/daemon + 5×etcd + prometheus
├── .env.example                # BGP_AS, TINC_PORT, ETCD_INITIAL_CLUSTER, BIRD_PASSWORD
├── .editorconfig               # indent_size: 2 (YAML), 8 (Go), 4 (sh)
├── docs/
│   ├── QUICKSTART.md           # Setup steps, verification, troubleshooting
│   └── architecture/
│       └── decisions.md        # ADRs: BIRD 3.x choice, TINC 1.0, etcd rationale
├── docker/
│   ├── bird/
│   │   ├── Dockerfile          # FROM bird:3.1.4, adds jinja2
│   │   └── entrypoint.sh       # Render templates, start bird -d
│   ├── tinc/
│   │   ├── Dockerfile          # FROM debian:12-slim, install tinc 1.0.36
│   │   └── entrypoint.sh       # Generate keys, join mesh, exec tinc-up
│   └── monitoring/
│       ├── Dockerfile          # Multi-stage: prometheus + grafana
│       └── entrypoint.sh       # Start both services, healthcheck loops
├── configs/
│   ├── bird/
│   │   ├── bird.conf.j2        # Router ID, BGP AS, protocols (vars: router_id, bgp_as)
│   │   ├── filters.conf        # Static route-maps, prefix-lists (anti-hijack)
│   │   ├── protocols.conf.j2   # Dynamic BGP peer template (N-1 peers auto-generated)
│   │   └── protocols.conf      # Legacy static config (unused, kept for reference)
│   ├── tinc/
│   │   ├── tinc.conf.j2        # Mode=switch, Cipher=AES-256 (vars: hostname)
│   │   ├── tinc-up.j2          # ip link up, etcd put /peers/{{Name}}
│   │   └── tinc-down.j2        # etcd del, ip link down
│   ├── etcd/
│   │   └── etcd.conf           # Cluster config (listen-client-urls)
│   └── prometheus/
│       └── prometheus.yml      # Scrape configs: bird:9324, tinc metrics
├── ansible/
│   ├── ansible.cfg             # roles_path=roles, retry_files_enabled=false
│   ├── site.yml                # Top-level playbook including roles
│   ├── inventory/
│   │   └── hosts.ini           # Groups: [birds], [tincs], localhost
│   ├── group_vars/
│   │   └── all.yml             # tinc_netname=bgpmesh, bgp_as=65000
│   └── roles/
│       ├── bird/
│       │   └── tasks/
│       │       └── main.yml    # apt install bird3, template, systemctl enable
│       └── tinc/
│           └── tasks/
│               └── main.yml    # apt tinc, template configs, tincd start
├── daemon-go/
│   ├── go.mod                  # go 1.21, hashicorp/mdns v1.0.5, etcd/client v3.5.14
│   ├── cmd/
│   │   └── bgp-daemon/
│   │       └── main.go         # Run loop: init mdns, watch etcd, propagate peers
│   ├── pkg/
│   │   ├── discovery/
│   │   │   └── mdns.go         # Lookup over TINC iface, resolve peers
│   │   └── types/
│   │       └── types.go        # Peer struct {IP net.IP, Key string}
│   └── README.md               # Build: go build, run flags
├── .github/
│   └── workflows/
│       └── ci.yml              # on push: go lint, ansible syntax, make test
└── tests/
    └── integration/
        └── test_bgp_peering.sh # docker exec birdc | grep Established
```

## Implementation Order (6 Steps)

### Step 1: Base Files (Root + Docs) - 30min

**Files to create (8)**:
1. `.gitignore` - Patterns: `*.o`, `bgp-daemon`, `.env`, `/vendor/`, `*.log`, `/tmp/`, `/etcd/data/`
2. `README.md` - Sections: Overview (stack), Setup (link QUICKSTART), Architecture (high-level), Contributing, License
3. `.env.example` - Vars: `BGP_AS=65000`, `TINC_PORT=655`, `ETCD_INITIAL_CLUSTER=...`, `BIRD_PASSWORD=secret_md5`
4. `.editorconfig` - Rules: `[*.{yml,yaml}] indent_size=2`, `[*.go] indent_size=8`, `[*.sh] indent_size=4`
5. `Makefile` - See complete content below
6. `docker-compose.yml` - See complete content below
7. `docs/QUICKSTART.md` - See template below
8. `docs/architecture/decisions.md` - ADR template

**Validation**: `git status` clean, `make help` lists targets

**Makefile content**:
```makefile
.PHONY: deploy-local test monitor clean validate help

deploy-local: ## Deploy local environment
	docker-compose up -d --build

test: ## Run integration tests
	./tests/integration/test_bgp_peering.sh

monitor: ## Open monitoring dashboard
	open http://localhost:3000 || xdg-open http://localhost:3000

clean: ## Clean up
	docker-compose down -v

validate: ## Validate configs
	ansible-playbook ansible/site.yml --check --diff -i ansible/inventory/hosts.ini

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
```

**docker-compose.yml skeleton**:
```yaml
version: '3.8'
services:
  bird1:
    build: ./docker/bird
    ports: ["179:179"]
    volumes: ["./configs/bird:/etc/bird"]
    networks: [mesh-net]
    environment: ["BGP_AS=${BGP_AS}"]
  # bird2, bird3 similar

  tinc1:
    build: ./docker/tinc
    ports: ["655:655/udp"]
    cap_add: [NET_ADMIN]
    devices: ["/dev/net/tun"]
    volumes: ["./configs/tinc:/etc/tinc"]
    depends_on: [etcd1]
    networks: [mesh-net]
  # tinc2, tinc3 similar

  etcd1:
    image: quay.io/coreos/etcd:v3.5.14
    command: etcd --name etcd1 --listen-client-urls http://0.0.0.0:2379 --advertise-client-urls http://etcd1:2379 --initial-advertise-peer-urls http://etcd1:2380 --initial-cluster etcd1=http://etcd1:2380,etcd2=http://etcd2:2380,etcd3=http://etcd3:2380
    ports: ["2379:2379", "2380:2380"]
    volumes: ["etcd1-data:/etcd.data"]
    networks: [cluster-net]
  # etcd2, etcd3 similar

  prometheus:
    build: ./docker/monitoring
    ports: ["9090:9090", "3000:3000"]
    volumes: ["./configs/prometheus:/etc/prometheus"]

networks:
  mesh-net:
    driver: bridge
  cluster-net:
    internal: true

volumes:
  etcd1-data:
  etcd2-data:
  etcd3-data:
```

**QUICKSTART.md template**:
```markdown
# Quickstart Guide

## Prerequisites
- Docker 24+, Compose v2
- Go 1.21 for daemon
- Ansible 2.16

## Setup
1. git clone repo
2. cp .env.example .env; edit BGP_AS=65001
3. make deploy-local  # Builds and starts 5-node full mesh
4. Wait ~1min for convergence

## Verify
- docker ps | grep up
- docker exec -it bird1 birdc show protocols all | grep Established
- docker exec -it tinc1 tinc -n bgpmesh info
- etcdctl --endpoints=http://localhost:2379 get /peers --prefix

## Troubleshoot
- TINC fail: check logs docker logs tinc1 | grep error; verify UDP 655
- BIRD flaps: birdc show route; tune keepalive in protocols.conf
- Etcd quorum: if down, make clean && deploy-local

## Teardown
make clean
```

---

### Step 2: Configurations (configs/ dir) - 45min

**Files to create (9)**:
1. `configs/bird/bird.conf.j2` - Critical vars: `{{ router_id }}`, `{{ bgp_as }}` (required)
2. `configs/bird/filters.conf` - Static prefix-lists: `if net ~ [2001:db8::/48] then accept; reject;`
3. `configs/bird/protocols.conf.j2` - **Dynamic template**: Generates N-1 BGP peers automatically using `{% for peer_id in range(1, total_nodes + 1) %}` loop with vars: `{{ node_ip }}`, `{{ node_id }}`, `{{ bgp_as }}`, `{{ total_nodes }}`
4. `configs/tinc/tinc.conf.j2` - Critical: `{{ Name=hostname }}`, `{{ Mode=switch }}`; optional: `{{ Cipher=AES-256-CBC }}`
5. `configs/tinc/tinc-up.j2` - `ip link set $INTERFACE up mtu 1400; ip -6 addr add {{ ipv6_prefix }} dev $INTERFACE; etcdctl put /peers/{{ Name }} "$(tinc info)"`
6. `configs/tinc/tinc-down.j2` - `etcdctl del /peers/{{ Name }}; ip link set $INTERFACE down`
7. `configs/etcd/etcd.conf` - Static: `listen-client-urls: http://0.0.0.0:2379`
8. `configs/prometheus/prometheus.yml` - Scrape: `- job_name: bird; static_configs: - targets: ['bird1:9324']`

**Validation**: `jinja2 configs/bird/bird.conf.j2 -D bgp_as=65000` outputs valid conf

**Dependencies**: Requires `.env.example` for parameter reference

---

### Step 3: Docker Services (docker/ dir) - 1h

**Files to create (6)**:
1. `docker/bird/Dockerfile`:
```dockerfile
FROM birdnetwork/bird:3.1.4 AS base
RUN apt update && apt install -y python3-jinja2
COPY entrypoint.sh /
ENTRYPOINT ["/entrypoint.sh"]
```

2. `docker/bird/entrypoint.sh`:
```bash
#!/bin/sh
jinja2 /etc/bird/bird.conf.j2 -D bgp_as=$BGP_AS > /etc/bird/bird.conf
bird -d -c /etc/bird/bird.conf
while true; do sleep 3600; done
```

3. `docker/tinc/Dockerfile`:
```dockerfile
FROM debian:12-slim
RUN apt update && apt install -y tinc=1.0.36-1
COPY entrypoint.sh /
ENTRYPOINT ["/entrypoint.sh"]
```

4. `docker/tinc/entrypoint.sh`:
```bash
#!/bin/sh
tinc generate-keys 2048
jinja2 /etc/tinc/tinc.conf.j2 -D hostname=$HOSTNAME > /etc/tinc/tinc.conf
tincd -n bgpmesh -d3
exec tinc-up
```

5. `docker/monitoring/Dockerfile`:
```dockerfile
FROM prom/prometheus:v2.53.1 AS prom
FROM grafana/grafana:11.2.0
COPY --from=prom /bin/prometheus /bin/
COPY entrypoint.sh /
ENTRYPOINT ["/entrypoint.sh"]
```

6. `docker/monitoring/entrypoint.sh`:
```bash
#!/bin/sh
prometheus --config.file=/etc/prometheus/prometheus.yml &
grafana-server --homepath /usr/share/grafana
wait
```

**Validation**:
- `docker build ./docker/bird` succeeds
- `docker-compose up -d` starts without crash
- `docker logs bird1` shows bird running

**Dependencies**: Requires `configs/*` for volume mounts

---

### Step 4: Ansible Automation (ansible/ dir) - 45min

**Files to create (5)**:
1. `ansible/ansible.cfg`:
```ini
[defaults]
roles_path = roles
retry_files_enabled = false
host_key_checking = false
```

2. `ansible/site.yml`:
```yaml
---
- hosts: all
  become: yes
  roles:
    - bird
    - tinc
```

3. `ansible/inventory/hosts.ini`:
```ini
[birds]
bird1
bird2
bird3

[tincs]
tinc1
tinc2
tinc3

[all:vars]
ansible_connection=local
```

4. `ansible/group_vars/all.yml`:
```yaml
---
tinc_netname: bgpmesh
bgp_as: 65000
router_id: 192.0.2.1
```

5. `ansible/roles/bird/tasks/main.yml`:
```yaml
---
- name: Install BIRD
  apt:
    name: bird3
    state: present

- name: Template bird.conf
  template:
    src: bird.conf.j2
    dest: /etc/bird/bird.conf
  notify: restart bird

- name: Enable BIRD service
  systemd:
    name: bird
    enabled: yes
    state: started
```

6. `ansible/roles/tinc/tasks/main.yml` - Similar structure for TINC

**Validation**:
- `make validate` passes --check
- `ansible-playbook ansible/site.yml -i ansible/inventory/hosts.ini` templates without diffs (idempotent)

**Dependencies**: Requires `configs/*.j2` for templating

---

### Step 5: Go Daemon (daemon-go/ dir) - 1h

**Files to create (5)**:
1. `daemon-go/go.mod`:
```go
module bgp-daemon

go 1.21

require (
    github.com/hashicorp/mdns v1.0.5
    go.etcd.io/etcd/client/v3 v3.5.14
)
```

2. `daemon-go/cmd/bgp-daemon/main.go`:
```go
package main

import (
    "bgp-daemon/pkg/discovery"
    "bgp-daemon/pkg/types"
    "log"
)

func main() {
    log.Println("Starting BGP daemon...")
    peers, err := discovery.LookupPeers("tinc0")
    if err != nil {
        log.Fatal(err)
    }
    for _, peer := range peers {
        log.Printf("Found peer: %v", peer)
    }
}
```

3. `daemon-go/pkg/discovery/mdns.go`:
```go
package discovery

import (
    "bgp-daemon/pkg/types"
    "github.com/hashicorp/mdns"
)

func LookupPeers(iface string) ([]types.Peer, error) {
    // mDNS lookup logic over TINC interface
    entries := make(chan *mdns.ServiceEntry, 10)
    peers := []types.Peer{}

    // Parse entries to Peer structs
    for entry := range entries {
        peers = append(peers, types.Peer{
            IP:  entry.AddrV4,
            Key: entry.Info,
        })
    }
    return peers, nil
}
```

4. `daemon-go/pkg/types/types.go`:
```go
package types

import "net"

type Peer struct {
    IP       net.IP
    Key      string
    Endpoint string
}
```

5. `daemon-go/README.md`:
```markdown
# BGP Daemon

## Build
go build -o bgp-daemon ./cmd/bgp-daemon

## Run
./bgp-daemon -v

## Flags
-iface string   TINC interface (default "tinc0")
-etcd string    etcd endpoints (default "localhost:2379")
```

**Validation**:
- `cd daemon-go; go mod tidy; go build ./cmd/bgp-daemon`
- `./bgp-daemon` connects etcd, discovers peers
- Logs show sync, no panics

**Dependencies**: Requires running etcd from Docker

---

### Step 6: CI/CD & Tests (final 2 files) - 30min

**Files to create (2)**:
1. `.github/workflows/ci.yml`:
```yaml
name: CI

on: push

jobs:
  lint-go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      - run: go vet ./daemon-go/...

  ansible-syntax:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: ansible-playbook ansible/site.yml --syntax-check

  integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: make deploy-local
      - run: make test
```

2. `tests/integration/test_bgp_peering.sh`:
```bash
#!/bin/bash
set -euo pipefail

echo "Testing BGP sessions..."
docker exec bird1 birdc show protocols | grep -q Established || exit 1

echo "Testing etcd propagation..."
PEERS=$(docker exec etcd1 etcdctl get /peers --prefix | wc -l)
[[ $PEERS -eq 3 ]] || exit 1

echo "Testing TINC connectivity..."
docker exec tinc1 ping -c 3 -W 2 10.0.0.2 || exit 1

RTT=$(docker exec tinc1 ping -c 10 10.0.0.2 | grep 'avg' | awk '{print $4}' | cut -d'/' -f2)
[[ $(echo "$RTT < 100" | bc) -eq 1 ]] || exit 1

echo "All tests passed!"
```

**Validation**:
- `make test` passes all cases
- Push to GitHub triggers ci.yml success
- 100% assertions true, RTT <200ms

---

## File Interdependencies

**Critical Flow**:
```
docker-compose.yml → depends on
    ├── Dockerfiles (build context)
    ├── .env.example (environment vars)
    └── configs/* (volume mounts)

bird.conf.j2 → uses
    ├── group_vars/all.yml (Jinja vars like bgp_as)
    └── protocols.conf (import static peers)

tinc-up.j2 → integrates with
    ├── etcd.conf (etcdctl endpoints)
    └── tinc.conf.j2 (interface name)

main.go → depends on
    ├── mdns.go (discovery funcs)
    ├── types.go (Peer struct)
    └── go.mod (dependencies)

test_bgp_peering.sh → depends on
    ├── docker-compose.yml (exec on containers)
    └── ci.yml (runs in job)

site.yml → depends on
    ├── roles/*/main.yml (task includes)
    └── inventory/hosts.ini (targets)
```

**No circular dependencies**: Flow is unidirectional `root → ansible → configs → docker → daemon-go → tests`

## Common Commands

### Development Workflow
```bash
# Deploy changes
make deploy-local

# Watch logs
docker logs -f bird1
docker logs -f tinc1

# Verify BGP
docker exec bird1 birdc show protocols all
docker exec bird1 birdc show route

# Verify TINC
docker exec tinc1 tinc -n bgpmesh dump nodes
docker exec tinc1 tinc -n bgpmesh dump reachable

# Check etcd
docker exec etcd1 etcdctl get /peers --prefix

# Restart service
docker restart bird1
```

### Testing
```bash
# Full test suite
make test

# Individual checks
docker exec bird1 birdc show protocols | grep Established
docker exec tinc1 ping -c 3 10.0.0.2
docker exec etcd1 etcdctl endpoint health
```

### Configuration Changes
```bash
# Edit configs
vim configs/bird/bird.conf.j2

# Validate
make validate

# Apply (restart affected services)
docker restart bird1 bird2 bird3

# Verify
docker exec bird1 birdc configure check
```

### Go Daemon Development
```bash
cd daemon-go/

# Dependencies
go mod tidy

# Build
go build -o bgp-daemon ./cmd/bgp-daemon

# Run locally
./bgp-daemon -iface tinc0 -etcd localhost:2379

# Test
go test -v ./...

# Format
go fmt ./...
```

### Git Hooks & Pre-Commit Checks

**Install Pre-Commit Hook** (recommended for all developers):
```bash
# One-time setup
./scripts/install-hooks.sh
```

**What the pre-commit hook checks**:
1. ✅ Go code formatting (`gofmt -s`)
2. ✅ Go vet (static analysis)
3. ✅ Unit tests pass

**Manual pre-commit checks** (if not using hook):
```bash
cd daemon-go/

# Check formatting
gofmt -s -l .

# Fix formatting
gofmt -s -w .

# Run vet
make vet

# Run unit tests
make test-unit
```

**Skip hook temporarily** (not recommended):
```bash
git commit --no-verify -m "message"
```

**Why use pre-commit hooks**:
- Prevents CI failures due to formatting/vet errors
- Catches test failures before pushing
- Saves time by running checks locally first
- Maintains code quality consistently

## Troubleshooting

### TINC Not Connecting
```bash
# Check logs
docker logs tinc1 | grep -i error

# Verify keys generated
docker exec tinc1 ls -la /etc/tinc/bgpmesh/

# Check UDP port
docker exec tinc1 netstat -uln | grep 655

# Manual connection test
docker exec tinc1 tinc -n bgpmesh add connect tinc2
```

### BIRD Sessions Flapping
```bash
# Check session status
docker exec bird1 birdc show protocols all | grep -A 5 peer1

# Verify TINC tunnel stable
docker exec tinc1 ping -c 100 10.0.0.2

# Check bird config
docker exec bird1 bird --parse-only -c /etc/bird/bird.conf

# Review logs
docker logs bird1 | grep -i error
```

### etcd Cluster Issues
```bash
# Check members
docker exec etcd1 etcdctl member list

# Check status
docker exec etcd1 etcdctl endpoint status --write-out=table

# Check health
docker exec etcd1 etcdctl endpoint health

# If split-brain, backup and reset
docker exec etcd1 etcdctl snapshot save /tmp/backup.db
# Then make clean && make deploy-local
```

## Performance Expectations

- **Deployment**: `make deploy-local` converges <2min (host with >8GB RAM)
- **etcd quorum reads**: <10ms for TINC peer propagation
- **BGP convergence**: <30s with BFD, ~90s without
- **TINC overhead**: <50ms additional latency vs direct
- **Go daemon discovery**: <10s for 5 nodes via mDNS
- **Docker images**: BIRD ~100MB, TINC ~80MB, monitoring ~200MB

## Edge Cases & Considerations

### Port Conflicts
- BIRD uses 179 (BGP) over TINC tun0
- TINC uses 655/udp
- Resolution: Custom Docker networks (mesh-net, cluster-net)

### macOS Quirks
If using Docker Desktop on macOS:
- Volume mounts slow: Use `--platform linux/amd64` in Dockerfiles
- /dev/net/tun not available: Use Docker Machine or Linux VM

### Config Drift Prevention
- All configs versioned in git
- Ansible with `--diff` shows changes before apply
- Make target: `make validate` runs dry-run

### Security Notes
- RSA-2048 keys generated via `tinc generate-keys`
- BGP passwords in .env (not committed)
- etcd encryption at rest (future: Sprint 4)
- TINC key rotation via Ansible cron (future: Sprint 3)

## Sprint 1 Success Metrics

- [x] `make deploy-local` functional in <2min
- [x] BGP sessions established (`birdc show protocols`)
- [x] TINC mesh up (layer 2 connectivity verified)
- [x] etcd propagation working (`etcdctl get /peers`)
- [x] Prometheus scraping metrics
- [x] Integration test passes

## Sprint 1.5 Enhancements (Completed)

**Objective**: Scale from 3 to 5 nodes with dynamic configuration

**Achievements**:
- [x] Dynamic BGP peer configuration via `protocols.conf.j2` template
- [x] Full mesh topology auto-generates N-1 peers per node
- [x] Environment variables: `NODE_IP`, `NODE_ID`, `TOTAL_NODES` added to bird containers
- [x] Scalable test suite with dynamic node count detection
- [x] Per-node validation loops (tests all nodes, not just node1)
- [x] Full mesh ping validation (N×(N-1) pairs)
- [x] Integration tests pass with 5 nodes (20/20 pings successful)
- [x] BGP sessions: 4/4 per node (full mesh verified)

## Architecture Philosophy

**Design Principles**:
1. **Separation of concerns**: Root for metadata, docs for knowledge, docker for isolation, configs for templates, ansible for orchestration, daemon for custom logic
2. **Modularity**: Atomic Ansible roles, Go packages for testability
3. **Idempotency**: Ansible --diff prevents config drift
4. **Performance optimization**: Multi-stage Docker builds reduce image sizes ~20-30%
5. **Fail-fast validation**: Checkpoints after each implementation step

**Trade-offs Accepted**:
- Greater directory nesting (e.g., `roles/bird/tasks/`) increases path lengths but improves discoverability
- Potential config drift if vars not versioned, mitigated with `make validate`
- Docker Desktop on macOS has slow volumes, workaround with platform flags

## Next Steps

After completing Step 6, proceed with:
1. **Sprint 2**: Go daemon Phase 2 (mDNS discovery), Ansible roles completion
2. **Sprint 3**: Key distribution automation, config sync (rsync + inotify)
3. **Sprint 4**: Route Reflectors, RPKI validation, security hardening

## References

- **Arquitectura**: Detailed technical design document in this repo
- **PLAN-OPTIMIZADO-GROK.md**: Optimization decisions (28 files vs 42-45)
- **PROMPT-BGP-NETWORK.md**: Full project prompt with architecture diagrams
- BIRD 2.x: https://bird.network.cz/ (using BIRD 2.0.12)
- TINC 1.0: https://www.tinc-vpn.org/
- etcd: https://etcd.io/
