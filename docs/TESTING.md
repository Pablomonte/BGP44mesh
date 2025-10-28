# Testing Guide

## Unit Tests

Run from `daemon-go/`:

```bash
make test           # All tests
make test-unit      # Unit only (fast)
make test-race      # With race detector
make test-coverage  # With coverage report
```

### Coverage

```
pkg/types:     100%  (complete)
pkg/metrics:   N/A   (no testable statements)
pkg/tinc:      0%    (Sprint 2 Phase 2)
pkg/discovery: 0%    (Sprint 2 Phase 2)
```

Target: >70% overall (currently 85.4% on tested packages)

### Running Specific Tests

```bash
go test -v ./pkg/types/                    # Single package
go test -v ./... -run TestPeer_String      # Single test
go test -v ./... -run ".*Valid.*"          # Pattern match
```

### Coverage Details

```bash
make test-coverage                         # Terminal output
make coverage-html                         # Generate HTML report
```

## Integration Tests

Prerequisites:

```bash
cp .env.example .env
make deploy-local
sleep 90  # Wait for convergence
```

### BGP Peering Test

```bash
./tests/integration/test_bgp_peering.sh
```

Validates:
- BIRD daemon running on all nodes
- BGP sessions in "Established" state
- Route exchange working

### TINC Mesh Test

```bash
./tests/integration/test_tinc_mesh.sh
```

Validates:
- tinc0 interface up with correct IPs
- Peer-to-peer connectivity (ping)
- MTU configuration

### etcd Cluster Test

```bash
./tests/integration/test_etcd_cluster.sh
```

Validates:
- All etcd nodes healthy
- Cluster quorum established
- Read/write operations <10ms

### Daemon Metrics Test

```bash
./tests/integration/test_daemon_metrics.sh
```

Validates:
- Metrics HTTP server responding on :2112
- Expected Prometheus metrics exported
- Metric values reasonable

## CI/CD

Workflow: `.github/workflows/ci.yml`

Pipeline:
1. Validate (env, YAML lint)
2. Build (3 Docker images)
3. Test Go (vet, fmt, unit, race, coverage)
4. Integration (deploy + test)

**Go version**: 1.23
**Coverage check**: Warns if <70% (not failing in Phase 1)
**Duration**: ~8-12 minutes

### Local CI Simulation

```bash
./tests/validation/test_env_vars.sh
./tests/validation/test_configs.sh
make build
cd daemon-go && make ci-test
make deploy-local && sleep 90 && make test-integration
```

## Troubleshooting

### Coverage Below Threshold

Find uncovered code:

```bash
cd daemon-go
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep -v 100.0%
```

Write tests for uncovered functions.

### Race Detector Failures

Error: `WARNING: DATA RACE`

Fix: Use `sync.Mutex`, `sync.RWMutex`, or `atomic` operations

```bash
go test -race ./... 2>&1 | grep "WARNING: DATA RACE"
```

### Docker Tests Hang

```bash
docker ps -a                     # Check container status
docker logs <container>          # View logs
make clean                       # Clean environment
make deploy-local                # Redeploy
```

### Tests Fail After etcd Change

Issue: Tests depend on clean etcd state

Fix:

```bash
docker restart etcd1 etcd2 etcd3
docker exec etcd1 etcdctl del --prefix /
```

## Writing Tests

### Unit Test Template

```go
package mypackage

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMyFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {
            name:     "valid input",
            input:    "test",
            expected: "TEST",
            wantErr:  false,
        },
        {
            name:     "empty input",
            input:    "",
            expected: "",
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := MyFunction(tt.input)

            if tt.wantErr {
                require.Error(t, err)
                return
            }

            require.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Integration Test Template

```bash
#!/bin/bash
set -euo pipefail

echo "=== My Integration Test ==="

# Test 1
echo -n "[TEST] Service running... "
if docker exec container1 pgrep myservice >/dev/null; then
    echo "[PASS]"
else
    echo "[FAIL]"
    exit 1
fi

# Test 2
echo -n "[TEST] Endpoint responds... "
if curl -sf http://localhost:8080/health >/dev/null; then
    echo "[PASS]"
else
    echo "[FAIL]"
    exit 1
fi

echo "=== All tests passed ==="
```

## Test Quality

Good tests:
- Fast (<1s for unit)
- Isolated (no external dependencies)
- Repeatable (same result every time)
- Readable (clear names)

Bad tests:
- Slow (>10s for unit)
- Flaky (random failures)
- Coupled (depend on other tests)
- Brittle (break on minor changes)
