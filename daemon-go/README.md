# BGP Propagation Daemon

Custom Go daemon for peer discovery, key distribution, and config synchronization in the BGP overlay network.

## Features

**Sprint 1 (Current):**
- etcd integration (watch `/peers/` for changes)
- mDNS peer discovery skeleton (over TINC interface)
- Basic logging and signal handling

**Sprint 2 (Planned):**
- Full mDNS service discovery and advertisement
- Automatic TINC key distribution
- Config sync (bird.conf, tinc.conf)
- Health monitoring and metrics

**Sprint 3+:**
- Chaos testing support
- Advanced metrics (Prometheus exporter)
- Automated failover logic

## Build

```bash
# Get dependencies
go mod download

# Build binary
go build -o bgp-daemon ./cmd/bgp-daemon

# Build with optimizations
go build -ldflags="-s -w" -o bgp-daemon ./cmd/bgp-daemon
```

## Run

```bash
# Default (etcd on localhost:2379, interface tinc0)
./bgp-daemon

# Custom etcd endpoint
./bgp-daemon -etcd etcd1:2379

# Custom TINC interface
./bgp-daemon -iface tun0

# Verbose logging
./bgp-daemon -v

# All options
./bgp-daemon -etcd etcd1:2379,etcd2:2379 -iface tinc0 -v
```

## Flags

- `-etcd`: etcd endpoints (comma-separated), default: `localhost:2379`
- `-iface`: TINC interface name, default: `tinc0`
- `-v`: Enable verbose logging

## Development

```bash
# Run tests
go test ./...

# Run with race detector
go test -race ./...

# Format code
go fmt ./...

# Lint (requires golangci-lint)
golangci-lint run

# View coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Docker Integration

In Sprint 1, the daemon runs on the host (not containerized) and connects to Docker containers:

```bash
# Run daemon connecting to Docker's etcd
./bgp-daemon -etcd localhost:2379
```

## Architecture

```
┌─────────────┐
│   Daemon    │
├─────────────┤
│ Discovery   │ ← mDNS over TINC
│ (mdns.go)   │
├─────────────┤
│ Sync Logic  │ ← etcd watch
│ (main.go)   │
├─────────────┤
│ Types       │ ← Peer struct
│ (types.go)  │
└─────────────┘
      ↓
  ┌───┴───┐
  ↓       ↓
etcd    TINC
cluster  mesh
```

## Sprint 1 Limitations

- mDNS discovery returns empty list (no services advertising yet)
- Key distribution not implemented (manual in Sprint 1)
- Config sync not implemented (using Docker volumes)
- Health monitoring basic (etcd watch only)

These will be implemented in Sprint 2.

## Dependencies

- `github.com/hashicorp/mdns v1.0.5` - mDNS service discovery
- `go.etcd.io/etcd/client/v3 v3.5.14` - etcd client library

## License

TBD
