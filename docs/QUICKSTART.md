# Quickstart Guide

## Prerequisites

- **Docker 24+** with Compose v2
- **Go 1.21+** for daemon development
- **Ansible 2.16+** for production deployment
- **>8GB RAM** (>16GB recommended for parallel builds)
- **Linux host** (Debian stable recommended, macOS with caveats)

## Setup

### 1. Clone and Configure

```bash
cd ~/repos/BGP
cp .env.example .env
```

Edit `.env` to customize (optional):
```bash
vim .env
# Change BGP_AS=65001 if needed
# Change BIRD_PASSWORD for BGP sessions
```

### 2. Deploy Local 5-Node Mesh (Sprint 1.5)

```bash
make deploy-local
```

This will:
- Build 4 Docker images (BIRD, TINC, daemon, prometheus)
- Start 21 containers (5 bird + 5 tinc + 5 daemon + 5 etcd + 1 prometheus)
- Bootstrap etcd cluster (5 nodes)
- Generate TINC keys with Subnet declarations (layer 2 fix)
- Configure BGP sessions dynamically (N-1 peers per node via templates)

**Wait ~90-120 seconds** for convergence (5 nodes take longer than 3).

### 3. Verify

```bash
# Check all containers are running
docker ps

# Check BGP sessions (Sprint 1.5: dynamic peers)
docker exec bird1 birdc show protocols all
# Should show 4 peers (peer1, peer2, peer3, peer4) as "Established"
# Each node has N-1 peers (5 nodes = 4 peers per node)

# Verify all 5 nodes have correct peer counts
for i in {1..5}; do
  echo "bird$i:"
  docker exec bird$i birdc show protocols | grep -c "Established"
done
# Should show "4" for each node

# Check TINC mesh
docker exec tinc1 tinc -n bgpmesh info
# Should show connected peers

# Check etcd cluster
docker exec etcd1 etcdctl endpoint health
# Should show all endpoints healthy

# Check Prometheus
curl -s http://localhost:9090/-/healthy
# Should return "Prometheus is Healthy."
```

### 4. Monitor

```bash
make monitor
```

Opens:
- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090

### 5. Run Tests

```bash
# Run all tests
make test-all

# Or individual test suites
make test-env          # Environment variables check
make test-configs      # Configuration template validation
make test-builds       # Docker builds
make test-integration  # BGP peering, TINC connectivity, etcd propagation
make test-e2e          # Full stack workflow with timing
```

## Troubleshooting

### TINC Not Connecting

```bash
# Check logs
docker logs tinc1 | grep -i error

# Verify keys generated
docker exec tinc1 ls -la /etc/tinc/bgpmesh/

# Check UDP port
docker exec tinc1 netstat -uln | grep 655

# Verify interface
docker exec tinc1 ip addr show tinc0
```

### BGP Sessions Flapping

```bash
# Check session status
docker exec bird1 birdc show protocols all | grep -A 5 peer1

# Verify TINC tunnel stable
docker exec tinc1 ping -c 100 10.0.0.2

# Check BIRD config
docker exec bird1 cat /etc/bird/bird.conf

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

# If issues, recreate cluster
make clean
make deploy-local
```

### Container Crashes

```bash
# Check status
docker ps -a

# View logs
docker logs bird1
docker logs tinc1
docker logs etcd1

# Restart individual service
docker restart bird1

# Full restart
docker-compose restart
```

### Port Conflicts

If ports 179, 655, 2379, or 9090 are already in use:

```bash
# Find process using port
sudo lsof -i :179

# Kill process or edit docker-compose.yml to use different ports
vim docker-compose.yml
# Change ports section, e.g., "10179:179" for BIRD
```

### Slow Convergence

If deployment takes >2min:

```bash
# Check system resources
docker stats

# Check host specs
free -h
df -h

# If low RAM (<8GB), consider:
# - Closing other applications
# - Building images sequentially instead of parallel
# - Increasing Docker memory limit
```

## Development Workflow

### Modify Configuration

```bash
# Edit BIRD config template
vim configs/bird/bird.conf.j2
# Or edit dynamic peer template (Sprint 1.5)
vim configs/bird/protocols.conf.j2

# Validate
make test-configs

# Apply changes (restart containers - all 5 nodes)
docker restart bird1 bird2 bird3 bird4 bird5

# Verify
docker exec bird1 birdc show protocols
# Should see 4/4 peers established
```

### Modify Docker Image

```bash
# Edit Dockerfile
vim docker/bird/Dockerfile

# Rebuild
make clean
make deploy-local

# Or rebuild single service
docker-compose up -d --build bird1
```

### View Real-time Logs

```bash
# Follow logs for all services
docker-compose logs -f

# Follow specific service
docker logs -f bird1

# Last 50 lines
docker logs --tail 50 bird1
```

## Teardown

```bash
# Stop and remove all containers, networks, and volumes
make clean

# Verify cleanup
docker ps -a | grep bgp
docker volume ls | grep bgp
```

## Next Steps

After successful Sprint 1.5 deployment (5-node full mesh):

1. **Explore monitoring**: Check Grafana dashboards (http://localhost:3000)
2. **Experiment with configs**: Modify BGP policies in `configs/bird/filters.conf`
3. **Test dynamic scaling**: Add protocols.conf.j2 supports any N nodes
4. **Run chaos tests**: Kill containers and observe reconvergence
5. **Develop Go daemon**: See `daemon-go/README.md` - daemon handles peer propagation
6. **Review manual testing**: See `docs/MANUAL_TESTING.md` for detailed debugging guide
7. **Prepare for Sprint 2**: Review Ansible roles for production deployment

**Sprint 1.5 Features:**
- ✅ Dynamic BGP peer configuration (N-1 peers auto-generated)
- ✅ TINC layer 2 fix (Subnet declarations for ARP resolution)
- ✅ Pre-commit hooks (gofmt, go vet, tests) - run `./scripts/install-hooks.sh`
- ✅ Scalable full mesh (tested with 5 nodes, 20 BGP sessions, 20 ping paths)

## Useful Commands

```bash
# BIRD commands
docker exec bird1 birdc show protocols all
docker exec bird1 birdc show route all
docker exec bird1 birdc show status
docker exec bird1 birdc configure check

# TINC commands
docker exec tinc1 tinc -n bgpmesh info
docker exec tinc1 tinc -n bgpmesh dump nodes
docker exec tinc1 tinc -n bgpmesh dump edges
docker exec tinc1 tinc -n bgpmesh dump subnets

# etcd commands
docker exec etcd1 etcdctl get /peers/ --prefix
docker exec etcd1 etcdctl member list
docker exec etcd1 etcdctl endpoint health
docker exec etcd1 etcdctl endpoint status --write-out=table

# Network debugging
docker exec tinc1 ping -c 3 10.0.0.2
docker exec bird1 ip route
docker exec bird1 ip addr
```

## Support

- Check [CLAUDE.md](../CLAUDE.md) for development guidelines
- Review [architecture decisions](architecture/decisions.md)
- See [main README](../README.md) for project overview
