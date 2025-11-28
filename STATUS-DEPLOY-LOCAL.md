# Deploy Local Environment Report

## Runtime State

- `make deploy-local` runs `docker compose up -d --build`, rebuilding the stack; all services are `Up ~14m` with health checks passing (`bird1-5`, `tinc1-5`, `daemon1-5`, `etcd1-5`, `prometheus`) (see `Makefile:4`).
- BIRD routers share the TINC network namespace via `network_mode: "service:tincX"` and maintain AS 65000 peerings; `birdc` confirms four established neighbors per node (see `docker-compose.yml:7`).
- Go daemons (one per node) share PID/network namespaces with their TINC twins, mount `/var/run/tinc`, publish keys to etcd, and watch `/peers/` to reconcile host files; logs show the initial sync of five peers and recurring mDNS scans (see `docker-compose.yml:89`, `daemon-go/cmd/bgp-daemon/main.go:74`).
- The five-member etcd quorum elected a leader and exposes client ports 2379/2380 as configured; `etcdctl` reports healthy endpoints (see `docker-compose.yml:341`).
- Monitoring packages Prometheus + Grafana into one container, exposing 9090/3000 with supervisor-managed health checks for metrics visibility (see `docker/monitoring/Dockerfile:5`).

## Repository Layout

- Compose models five identical edge nodes (TINC + BIRD + daemon) plus etcd quorum and monitoring plane, using `mesh-net` for data and internal `cluster-net` for control (see `docker-compose.yml:224`, `docker-compose.yml:460`).
- BIRD images render configs from Jinja templates into `/var/run/bird` before launching the daemon in foreground mode (see `docker/bird/entrypoint.sh:26`).
- TINC entrypoints generate RSA keys on first boot, rebuild host files each start, and leave `ConnectTo` empty so the Go daemon manages peer wiring (see `docker/tinc/entrypoint.sh:27`).
- The Go control-plane binary exposes Prometheus metrics, stores node metadata in etcd, monitors mDNS, and reconciles connections on `/peers/` changes (see `daemon-go/cmd/bgp-daemon/main.go:49`).
- Architectural decisions for BIRD/TINC/etcd and Docker Compose are recorded in ADRs for traceability (see `docs/architecture/decisions.md:1`).

## Notable Observations

- Docker Compose warns that the top-level `version` key is obsolete; removing the line keeps output clean without behavior change (see `docker-compose.yml:1`).
- Grafana occasionally logs "database is locked" during routine tasks; retries succeed but monitor these if dashboard edits stall.
- Go daemon logs include periodic `mdns: Closing client` entries—normal cleanup every 30 seconds, but spikes could signal discovery issues.
- etcd currently serves over plain HTTP, prompting warnings about insecure traffic; enable TLS before exposing beyond localhost (see `docker-compose.yml:349`).

## Recommended Next Steps

1. Run `make monitor` to open Grafana/Prometheus and confirm metrics match the healthy state (`Makefile:10`).
2. Drop the deprecated `version` line from `docker-compose.yml` before the next `make deploy-local` to silence compose warnings.
3. Plan TLS for etcd (certs plus endpoint updates) if this cluster will be reachable from outside the host.
