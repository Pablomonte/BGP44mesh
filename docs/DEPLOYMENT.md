# Deployment Guide

## Docker Compose (Development/Testing)

### Requirements

- Docker 24+ with Compose v2
- >8GB RAM (>16GB for 5-node)
- Linux kernel with TUN/TAP support

### Local Deployment (Sprint 1.5 - 5 nodes)

```bash
git clone https://github.com/pablomonte/bgp-network.git
cd bgp-network
cp .env.example .env
make deploy-local
```

Wait ~90-120s for convergence (5-node mesh).

**Architecture**:

`docker-compose.yml` deploys a full mesh with **21 containers**:
- 5x BIRD (BGP routing with dynamic peer configuration)
- 5x TINC (VPN mesh with Subnet declarations for layer 2)
- 5x etcd (distributed storage, 5-node cluster)
- 5x daemon (Go automation for peer propagation)
- 1x prometheus + grafana (monitoring)

Each BIRD node automatically configures **N-1 peers** (4 peers for 5 nodes) using the `protocols.conf.j2` template with environment variables (`NODE_IP`, `NODE_ID`, `TOTAL_NODES`).

### Verify Deployment

```bash
docker ps                                    # Should show 21 running
docker exec bird1 birdc show protocols       # BGP sessions (expect 4/4 Established)
docker exec tinc1 ip addr show tinc0         # TINC interface
docker exec etcd1 etcdctl endpoint health    # etcd cluster
curl http://localhost:2112/metrics           # Daemon metrics (via tinc1)

# Verify all nodes have correct peer counts (Sprint 1.5)
for i in {1..5}; do
  echo "bird$i: $(docker exec bird$i birdc show protocols | grep -c Established) peers"
done
# Should show "4 peers" for each node
```

**Automated Key Distribution**:

TINC keys are automatically distributed via etcd:
1. Each node generates RSA-2048 keys on startup
2. tinc-up script stores keys in etcd at `/peers/<node>`
3. Go daemon syncs keys and updates ConnectTo directives
4. TINC daemon reloads with new peers

No manual bootstrap required. Nodes auto-discover and connect.

### Configuration

Edit `.env`:

```bash
BGP_AS=65000
TINC_NETNAME=bgpmesh
TINC_PORT=655
```

Restart affected services (all 5 nodes):

```bash
docker restart bird1 bird2 bird3 bird4 bird5
# Or restart all bird containers at once
docker restart $(docker ps -q -f name=bird)
```

### Monitoring

- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin/admin)

Dashboard: BGP Daemon Overview

### Cleanup

```bash
make clean                      # Stop containers
docker compose down -v          # Remove volumes (deletes data)
```

---

## Ansible (Production)

### Requirements

**Control node**:
- Ansible 2.16+
- Python 3.8+
- SSH access to targets

**Target nodes** (each):
- Ubuntu 22.04+ or Debian 12+
- >2GB RAM, >10GB disk
- Root or sudo access

### Setup

#### 1. Inventory

```bash
cd ansible
cp inventory/hosts.ini.example inventory/hosts.ini
```

Edit `inventory/hosts.ini`:

```ini
[bgp_nodes]
node1 ansible_host=192.168.1.101 node_ip=10.0.0.1 router_id=192.0.2.1
node2 ansible_host=192.168.1.102 node_ip=10.0.0.2 router_id=192.0.2.2
node3 ansible_host=192.168.1.103 node_ip=10.0.0.3 router_id=192.0.2.3
node4 ansible_host=192.168.1.104 node_ip=10.0.0.4 router_id=192.0.2.4
node5 ansible_host=192.168.1.105 node_ip=10.0.0.5 router_id=192.0.2.5

[bgp_nodes:vars]
ansible_user=root
```

#### 2. Variables

Edit `group_vars/all.yml`:

```yaml
bgp_as: 65000
tinc_netname: "bgpmesh"
etcd_version: "3.5.14"
```

#### 3. Build Daemon

```bash
cd daemon-go
make build
# Binary at bin/bgp-daemon
```

#### 4. Test Connectivity

```bash
cd ../ansible
ansible -i inventory/hosts.ini all -m ping
```

### Deployment

Full deploy (all roles, all nodes):

```bash
ansible-playbook -i inventory/hosts.ini playbook.yml
```

Duration: ~10-15 min for 5 nodes

### Partial Deployment

Specific roles:

```bash
ansible-playbook -i inventory/hosts.ini playbook.yml --tags tinc,bird
ansible-playbook -i inventory/hosts.ini playbook.yml --tags daemon
```

Specific host:

```bash
ansible-playbook -i inventory/hosts.ini playbook.yml --limit node1
```

### Verification

#### All Services

```bash
ansible -i inventory/hosts.ini all -m shell -a "systemctl status etcd bird tinc@bgpmesh bgp-daemon --no-pager"
```

#### etcd Cluster

```bash
ssh root@node1
etcdctl member list
etcdctl endpoint health
```

Expected:
```
http://10.0.0.1:2379 is healthy: successfully committed proposal: took = 3.2ms
http://10.0.0.2:2379 is healthy: successfully committed proposal: took = 3.8ms
...
```

#### TINC Mesh

```bash
ssh root@node1
ip addr show tinc0
ping -c 3 10.0.0.2
tinc -n bgpmesh info
```

#### BGP Sessions

```bash
ssh root@node1
birdc show protocols
birdc show route
```

Expected:
```
Name       Proto      Table      State
peer2      BGP        ---        up      Established
peer3      BGP        ---        up      Established
...
```

#### Go Daemon

```bash
ssh root@node1
systemctl status bgp-daemon
curl http://localhost:2112/metrics
```

---

## Configuration

### BIRD

Edit `ansible/roles/bird/templates/bird.conf.j2`

Redeploy:

```bash
ansible-playbook -i inventory/hosts.ini playbook.yml --tags bird
```

### TINC

Edit `ansible/roles/tinc/templates/tinc.conf.j2`

Redeploy:

```bash
ansible-playbook -i inventory/hosts.ini playbook.yml --tags tinc
```

### Go Daemon

Rebuild binary:

```bash
cd daemon-go && make build
```

Redeploy:

```bash
cd ../ansible
ansible-playbook -i inventory/hosts.ini playbook.yml --tags daemon
```

---

## Scaling

### Add Node

#### 1. Update Inventory

Add to `inventory/hosts.ini`:

```ini
node6 ansible_host=192.168.1.106 node_ip=10.0.0.6 router_id=192.0.2.6
```

#### 2. Add to etcd Cluster

From existing node:

```bash
ssh root@node1
etcdctl member add etcd6 --peer-urls=http://10.0.0.6:2380
```

Update `etcd_cluster_members` in group_vars to include node6.

#### 3. Deploy

```bash
ansible-playbook -i inventory/hosts.ini playbook.yml --limit node6
```

#### 4. Verify

```bash
etcdctl member list
ssh root@node6 ip addr show tinc0
ssh root@node6 birdc show protocols
```

### Remove Node

```bash
ssh root@node6
systemctl stop bgp-daemon bird tinc@bgpmesh etcd
```

From other node:

```bash
etcdctl member remove <member-id>
```

Remove from inventory.

---

## Backup/Restore

### etcd Backup

Manual:

```bash
ssh root@node1
etcdctl snapshot save /backup/etcd-$(date +%Y%m%d).db
etcdctl snapshot status /backup/etcd-$(date +%Y%m%d).db
```

Automated (cron):

```bash
# /etc/cron.daily/etcd-backup
#!/bin/bash
etcdctl snapshot save /backup/etcd-$(date +%Y%m%d).db
find /backup -name "etcd-*.db" -mtime +7 -delete
```

### etcd Restore

Stop all etcd nodes:

```bash
ansible -i inventory/hosts.ini all -m systemd -a "name=etcd state=stopped"
```

Restore on each node:

```bash
ssh root@node1
etcdctl snapshot restore /backup/etcd-20251028.db \
  --name node1 \
  --initial-cluster "node1=http://10.0.0.1:2380,node2=http://10.0.0.2:2380,..." \
  --initial-advertise-peer-urls http://10.0.0.1:2380 \
  --data-dir /var/lib/etcd-restore

mv /var/lib/etcd /var/lib/etcd-old
mv /var/lib/etcd-restore /var/lib/etcd
```

Start all:

```bash
ansible -i inventory/hosts.ini all -m systemd -a "name=etcd state=started"
```

### TINC Keys Backup

```bash
tar czf tinc-keys-$(date +%Y%m%d).tar.gz /etc/tinc/bgpmesh/rsa_key.priv /etc/tinc/bgpmesh/rsa_key.pub
```

Restore:

```bash
tar xzf tinc-keys-20251028.tar.gz -C /
systemctl restart tinc@bgpmesh
```

---

## Troubleshooting

### etcd Won't Start

Check logs:

```bash
systemctl status etcd
journalctl -xe -u etcd
```

Common causes:
- Port 2379/2380 in use: `netstat -tlpn | grep -E "2379|2380"`
- Permissions: `chown -R etcd:etcd /var/lib/etcd`
- Bad config: `cat /etc/etcd/etcd.conf`

### TINC Not Connecting

Check status:

```bash
tinc -n bgpmesh info
tinc -n bgpmesh dump nodes
journalctl -u tinc@bgpmesh
```

Common causes:
- Missing peer host files: `ls /etc/tinc/bgpmesh/hosts/`
- Firewall blocking UDP 655: `ufw allow 655/udp`
- Wrong ConnectTo: check `/etc/tinc/bgpmesh/tinc.conf`

Debug:

```bash
tinc -n bgpmesh --debug=5
```

### BGP Sessions Not Establishing

Check:

```bash
birdc show protocols all peer1
journalctl -u bird
```

Common causes:
- TINC mesh not connected: `ping 10.0.0.2`
- Wrong neighbor IP: check `/etc/bird/protocols.conf`
- Firewall blocking TCP 179: `iptables -L -n | grep 179`

Validate config:

```bash
bird -p -c /etc/bird/bird.conf
```

### Go Daemon Crashes

Check logs:

```bash
systemctl status bgp-daemon
journalctl -u bgp-daemon -f
```

Common causes:
- etcd unreachable: `etcdctl endpoint health`
- TINC permissions: `chown -R bgp-daemon:bgp-daemon /var/run/tinc`
- Binary version mismatch: `/opt/bgp-daemon/bgp-daemon -version`

### High etcd Latency

Check performance:

```bash
etcdctl check perf
```

Fixes:
- Use SSD for `/var/lib/etcd`
- Reduce network latency between nodes
- Increase resources (CPU, RAM)

### BGP Slow Convergence

Fixes:
- Enable BFD: set `bgp_bfd_enabled: true` in `group_vars/all.yml`
- Reduce BGP timers in BIRD config
- Check TINC overhead: `ping -c 100 10.0.0.2`
