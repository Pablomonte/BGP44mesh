# Prompt para Claude/Grok: Proyecto BGP Network Architecture

## 🎯 Objetivo del Proyecto

Diseñar e implementar **BGP Network System** - un framework de red distribuida con lógica BGP que integra:
- **TINC v1.0**: Mesh VPN Layer 2 (switch mode) para conectividad overlay segura
- **BIRD 3.x**: Routing daemon con soporte BGPv4 + IPv6 (MP-BGP)
- **Ansible**: Orquestación para provisioning y config management continuo
- **Sistema Custom de Propagación**: Discovery automático, key distribution, config sync, health monitoring
- **CI/CD Automático**: Per-commit deployment con rolling updates

---

## 🏗️ Arquitectura Confirmada (según análisis Grok)

```
┌─────────────────┐    Git Push    ┌─────────────────┐    Rolling Deploy    ┌─────────────────┐
│   BGP Config    │ ─────────────► │   GitLab CI/CD  │ ──────────────────► │   BGP Edge Node │
│   Repository    │                │   + Ansible     │                     │  (TINC + BIRD)  │
└─────────────────┘                └─────────────────┘                     └─────────────────┘
        │                                  │                                        │
        ▼                                  ▼                                        ▼
┌─────────────────┐              ┌─────────────────┐                     ┌─────────────────┐
│ etcd Storage    │              │  Prometheus     │                     │  TINC Mesh      │
│ - Configs       │              │  - Telemetry    │                     │  - Layer 2 VPN  │
│ - Keys          │              │  - Metrics      │                     │  - BGP Peering  │
└─────────────────┘              └─────────────────┘                     └─────────────────┘
```

---

## 📋 Decisiones Técnicas Confirmadas

### 1. BIRD Specifications ✅

**Decisión de Grok:**
- **Versión**: BIRD 3.x (3.1.4+) - current, soporte MP-BGP maduro
- **Protocolos**: BGPv4 + IPv6 (multiprotocol via RFC 4760)
- **Deployment**:
  - **Debian/Servidores**: Docker containers con systemd orchestration
  - **OpenWrt/Gateways**: Native via opkg (custom feeds para 3.x)
- **Features**: RPKI validation (RFC 6811), BFD integration, route reflectors

**Justificación técnica:**
- BIRD 3.x reduce config boilerplate 30-40% vs 1.6
- MP-BGP unificado elimina necesidad de daemons separados
- Overhead: ~100MB por 10k routes (optimizable con `channel bgp limit`)
- Performance: Maneja 1M routes con <5% CPU en hardware moderno
- Trade-off: Mayor consumo memoria vs 1.6, pero mejor reconvergencia (<30s con BFD)

**Alternativas descartadas:**
- BIRD 1.6: EOL, syntax obsoleto
- FRR: 2x overhead en memoria en hardware embebido
- Quagga: Obsoleto (forked a FRR)

---

### 2. Ansible Alcance ✅

**Decisión de Grok:**
- **Provisioning inicial**: Instalación de BIRD/TINC via roles customizados
- **Config management continuo**: `ansible-pull` cada 5 min desde repo Git
- **Integración CI/CD**: GitHub Actions/GitLab CI con pipelines automáticos

**Workflow:**
```bash
# Provisioning inicial
ansible-playbook -i inventory site.yml --tags provision

# Config management continuo (ejecutado por cron en cada nodo)
ansible-pull -U git@repo:playbooks.git -i localhost local.yml

# CI/CD pipeline
git commit → webhook → ansible lint → dry-run → manual approval → deploy
```

**Justificación técnica:**
- Agentless (SSH-based): Ideal para OpenWrt con dropbear
- Idempotencia: Evita config drifts en producción
- Ansible-pull: Mitiga issues de push en nodos detrás de firewalls
- Performance: <1min por nodo en deploys
- Trade-off: No realtime, pero suficiente para config changes

**Roles principales:**
- `role/bird`: Instalación, configs, filters BGP
- `role/tinc`: Setup mesh, key management, tinc-up scripts
- `role/monitoring`: Prometheus exporters, Grafana dashboards

---

### 3. Stack Kafka + IPFS: **NO NECESARIOS** ✅

**Decisión de Grok: Usar alternativas más simples**

#### Reemplazos propuestos:

**Para Telemetría (reemplaza Kafka):**
- **Prometheus**: Push metrics via HTTP, blackbox exporters
- **Ventajas**: 50-70% menor latency vs Kafka, sin Zookeeper dependency
- **Overhead**: 50MB/nodo vs 200MB con Kafka
- **Métricas**: BGP flaps, TINC peer status, route counts

**Para Config Storage (reemplaza IPFS):**
- **etcd**: Key-value store con watch API, HA con raft
- **Ventajas**: Realtime sync, integración con Ansible (etcd3 module)
- **Storage**: <100MB/nodo vs IPFS overhead de 10% bandwidth
- **Data**: bird.conf, tinc.conf, RSA keys (encrypted)

**Justificación técnica:**
- Sistema custom de propagación ya provee low-latency sync
- Kafka overkill para escala objetivo (50 nodos iniciales)
- IPFS slow en cold starts, problemas en OpenWrt con low storage
- Trade-off: etcd centralizado pero HA, vs IPFS fully distributed

**Alternativas descartadas:**
- Consul: Más pesado que etcd
- Git para configs: No realtime
- MQTT: Considerado, pero etcd mejor integración con Ansible

---

### 4. CI/CD Strategy ✅

**Decisión de Grok:**

#### Triggers (ambos):
1. **Cambios en configs**: git diff en `bird.conf` / `tinc.conf`
2. **Nuevos peers**: hooks en directorio `tinc/hosts/`

#### Alcance:
- **Rolling update de toda la red** (no single-node)
- **Batches**: `serial: 20%` en Ansible playbook
- **Zero-downtime**: <1min de outage por nodo durante roll

#### Pipeline stages:
```yaml
stages:
  - test       # ansible-lint, molecule para roles
  - validate   # dry-run con --check, syntax bird.conf
  - canary     # deploy a 2 nodos test
  - deploy     # rolling update (manual approval para prod)
  - rollback   # automatic si flap masivo detectado
```

**Justificación técnica:**
- Per-commit asegura rapid feedback
- Rolling update: Consistencia en topología, evita blackholing
- Canary nodes: Early detection de errores
- Performance: <10min full rollout en red de 50 nodos
- Trade-off: Más lento que single-node, pero sin outages

**Health checks:**
- BGP sessions: `birdc show protocols all | grep Established`
- TINC peers: `tinc dump reachable`
- Route propagation: Test de conectividad end-to-end

---

### 5. Sistema Custom de Propagación ✅

**Decisión de Grok: Orden de prioridad**

#### Priority 1: **Discovery Automático** 🥇
- **Método**: mDNS over TINC para peer detection
- **Por qué primero**: Habilita auto-scaling sin registry central
- **Overhead**: <1% bandwidth
- **Implementación**: Daemon en Go parseando `tinc dump`

#### Priority 2: **Key Distribution** 🥈
- **Método**: Secure SCP con pre-shared keys
- **Por qué**: Mitiga manualidad de TINC 1.0 (no tiene invitaciones de 1.1)
- **Rotación**: Cron job mensual vía Ansible
- **Storage**: etcd con encryption at rest

#### Priority 3: **Config Sync** 🥉
- **Método**: rsync con inotify para realtime propagation
- **Por qué**: Asegura consistency de bird.conf en toda la red
- **Latency**: <5s para sync completo
- **Fallback**: Ansible-pull cada 5min

#### Priority 4: **Health Monitoring** 🏅
- **Método**: SNMP traps + Prometheus alerts
- **Por qué**: Último porque discovery/keys son blockers
- **Métricas**: Flap counts, peer uptime, route convergence time

**Información propagada (del reporte Grok):**
```json
{
  "peers": {
    "tinc_ips": ["10.0.0.1", "10.0.0.2"],
    "rsa_keys": ["base64_key1", "base64_key2"],
    "endpoints": ["203.0.113.1:655", "203.0.113.2:655"]
  },
  "bgp_config": {
    "as_numbers": [65001, 65002],
    "prefixes": ["2001:db8:1::/48", "2001:db8:2::/48"],
    "policies": ["export_to_peers", "import_from_transit"]
  },
  "topology": {
    "adjacencies": [["node1", "node2"], ["node2", "node3"]],
    "graph": "networkx serialized format"
  },
  "metrics": {
    "rtt": "45ms",
    "loss": "0.5%",
    "flap_count": 3,
    "uptime": "99.95%"
  }
}
```

**Daemon custom specs:**
- **Lenguaje**: Go (cross-platform, bajo overhead)
- **Protocolo**: UDP multicast (239.255.0.1:9999) over TINC
- **Fanout limit**: 100 peers sin degradación
- **Edge cases**: Key compromise → rotate via cron

---

## 🔧 Stack Tecnológico Final

**Networking:**
- BGP: BIRD 3.x (MP-BGP, RPKI, BFD)
- VPN: TINC 1.0 (switch mode, RSA-2048, AES-256)
- Orchestration: Docker (Debian) + systemd (ambos)

**Storage & Telemetry:**
- Config: etcd cluster (3 nodes HA, raft consensus)
- Metrics: Prometheus + Grafana + Alertmanager
- Logs: syslog-ng → centralized (no ELK, too heavy)

**Automation:**
- CI/CD: GitLab CI / GitHub Actions
- Config Management: Ansible (push + pull hybrid)
- Testing: Molecule para roles, Mininet para network simulation

**Monitoring:**
- Prometheus (metrics): bird_exporter, tinc_exporter
- Grafana (dashboards): BGP sessions, route counts, peer status
- BFD: Liveness detection (<30s reconvergencia)

---

## 🎯 Tareas de Implementación

### Sprint 1: Fundamentos (Semana 1-2) - ALTA PRIORIDAD

#### 1.1 Setup inicial del repositorio BGP/
- Estructura de directorios
- Docker compose para dev local (BIRD + TINC + etcd)
- Makefile para automatización (`make deploy-local`, `make test`)

#### 1.2 BIRD 3.x deployment básico
- Container image con BIRD 3.1.4
- bird.conf template con BGPv4 + IPv6
- systemd unit file para orchestration
- Test: 2 nodos BGP peer via TINC

#### 1.3 TINC 1.0 mesh básico (3 nodos)
- Configuración switch mode
- RSA key generation automático
- tinc-up scripts para BIRD integration
- Connectivity tests (ping over tunnel)

#### 1.4 etcd cluster setup
- 3 nodes HA con raft
- Storage de configs (bird.conf, tinc.conf)
- Ansible integration (etcd3 module)

---

### Sprint 2: Automation & Propagación (Semana 3-4) - MEDIA-ALTA

#### 2.1 Sistema custom de propagación - Phase 1: Discovery
- Daemon Go para mDNS over TINC
- Parsing de `tinc dump` para peer detection
- Tests: Auto-discovery de 5 nodos en <10s

#### 2.2 Ansible roles completos
- `role/bird`: Install, config, filters
- `role/tinc`: Mesh setup, key rotation
- `role/monitoring`: Prometheus exporters
- Playbook: `site.yml` con idempotencia

#### 2.3 CI/CD pipeline básico
- GitHub Actions workflow
- Stages: lint → validate → dry-run
- Config validation (bird --parse-only)

#### 2.4 Prometheus + Grafana
- bird_exporter deployment
- Dashboards: BGP sessions, route counts
- Alerting rules (flap >5/min)

---

### Sprint 3: Production Hardening (Semana 5-6) - MEDIA

#### 3.1 Sistema custom - Phase 2: Key Distribution
- Secure SCP con pre-shared keys
- Integration con etcd para key storage
- Rotación mensual via Ansible cron

#### 3.2 CI/CD completo
- Deploy automático post-merge (con approval)
- Rolling update con canary nodes
- Rollback automático en failures
- Health checks: birdc status, tinc connectivity

#### 3.3 Config Sync realtime
- rsync + inotify para bird.conf changes
- Fallback: ansible-pull cada 5min
- Validation: Config consistency checks

#### 3.4 Testing automation
- Mininet para network topology simulation
- Chaos engineering (node failures, partition tolerance)
- Performance tests: iperf3, mtr

---

### Sprint 4: Escalabilidad & Security (Semana 7+) - BAJA

#### 4.1 Route Reflectors
- BIRD config para RR en servidores Debian
- Reduce sessions de O(n²) a O(n)
- Testing con 50+ nodos

#### 4.2 Sistema custom - Phase 3: Health Monitoring
- SNMP traps para alerts
- Integration con Prometheus
- Dashboards: Uptime, flap rates, latency

#### 4.3 Security hardening
- BGP MD5 authentication
- TINC key rotation automation
- etcd encryption at rest
- RPKI validation

#### 4.4 High availability
- etcd multi-region replication
- BGP multi-path para redundancy
- Automated failover tests

---

## 🤖 Protocolo de Colaboración Claude ↔ Grok

### Workflow:

**Claude (arquitecto/implementador)**:
1. Diseña estructura inicial de código
2. Implementa Ansible roles y configs
3. Crea tests y validaciones
4. Documenta decisiones técnicas

**Grok (revisor/consultor)** - vía Playwright MCP:
1. Revisa arquitectura propuesta
2. Sugiere optimizaciones técnicas
3. Identifica edge cases y trade-offs
4. Propone mejoras de performance/security

### Formato de Consulta:

```bash
# Claude exporta contexto
mkdir -p ~/repos/BGP/.context-for-grok/
cp README.md ~/repos/BGP/.context-for-grok/
cp architecture.md ~/repos/BGP/.context-for-grok/
cp ansible/roles/bird/tasks/main.yml ~/repos/BGP/.context-for-grok/

# Claude usa Playwright MCP para Grok
# (desde interfaz de Claude Code)
# → Upload context files
# → Ask: "Review this BIRD config for production edge cases"
# → Extract response y apply feedback
```

---

## 📊 Métricas de Éxito

### Funcionales:
- [ ] 3+ nodos BGP en TINC mesh functioning
- [ ] Route announcements propagated en <30s
- [ ] CI deploy completo en <10min
- [ ] Configs persistidas en etcd con <5s sync

### Performance:
- [ ] Latency overlay: <50ms adicional vs direct
- [ ] BGP convergence: <30s con BFD
- [ ] Prometheus scrape: >10 metrics/sec sustained
- [ ] etcd latency: <2s para read/write

### Operacionales:
- [ ] Zero-downtime deploys (rolling update)
- [ ] Automated rollback funcional (<2min)
- [ ] Monitoring lag: <1min
- [ ] Documentation: 100% cobertura

---

## 🔐 Consideraciones de Seguridad

**BGP:**
- MD5 authentication en sessions (RFC 5925)
- Prefix filtering (max-prefix limits)
- RPKI validation contra hijacking
- Route-map policies estrictas

**TINC:**
- RSA-2048 keys (upgrade path a 4096)
- AES-256 cipher (validar vs ChaCha20 en ARM)
- Key rotation mensual automatizada
- Firewall: Solo UDP 655 desde IPs conocidas

**etcd:**
- TLS para client-server communication
- Encryption at rest para secrets
- RBAC para access control
- Regular backups con etcdctl snapshot

**CI/CD:**
- Signed commits (GPG)
- Ansible Vault para secrets
- Manual approval para production deploys
- Audit logs completos

---

## 🚀 Entregables Esperados

### 1. Repositorio BGP/ funcional:
```
BGP/
├── docker-compose.yml          # Stack completo (BIRD, TINC, etcd, Prometheus)
├── Makefile                    # Comandos: deploy-local, test, monitor
├── ansible/
│   ├── roles/
│   │   ├── bird/               # BIRD 3.x deployment
│   │   ├── tinc/               # TINC 1.0 mesh
│   │   └── monitoring/         # Prometheus exporters
│   ├── inventory/
│   │   ├── hosts               # Static inventory
│   │   └── dynamic_tinc.py     # Dynamic discovery via TINC
│   ├── site.yml                # Main playbook
│   └── group_vars/all.yml      # Configs globales
├── daemon/                     # Sistema custom propagación (Go)
│   ├── main.go
│   ├── discovery.go            # mDNS
│   ├── keydist.go              # Key distribution
│   └── sync.go                 # Config sync
├── configs/
│   ├── bird/                   # Templates bird.conf
│   └── tinc/                   # Templates tinc.conf
├── .github/workflows/
│   └── deploy.yml              # CI/CD pipeline
├── docs/
│   ├── architecture.md         # Diagramas UML
│   ├── runbooks/               # Ops procedures
│   └── api.md                  # API del daemon custom
└── tests/
    ├── mininet/                # Network simulation
    └── molecule/               # Ansible role tests
```

### 2. Demo funcional:
```bash
cd ~/repos/BGP

# Levantar stack local (3 nodos)
make deploy-local

# Validar conectividad
make test           # BGP sessions, TINC peers, etcd health

# Monitoreo
make monitor        # Abre Grafana dashboards (localhost:3000)

# Deploy real
ansible-playbook -i inventory/hosts site.yml --check   # Dry-run
ansible-playbook -i inventory/hosts site.yml           # Deploy
```

### 3. CI/CD pipeline ejecuta:
1. **Lint**: ansible-lint, yamllint
2. **Validate**: bird --parse-only, tinc --config-test
3. **Test**: Molecule en Docker, unit tests del daemon Go
4. **Canary Deploy**: 2 nodos test
5. **Production Deploy**: Rolling update (manual approval)
6. **Health Check**: birdc status, tinc dump, etcd health
7. **Rollback**: Automático si >10% nodos fallan

---

## ❓ Preguntas Resueltas (confirmadas por Grok)

1. **BIRD Implementation**: ✅ BIRD 3.x (current, MP-BGP), containers + systemd
2. **TINC version**: ✅ TINC 1.0 (stable, compatible con OpenWrt legacy)
3. **State management**: ✅ etcd para configs, Prometheus para metrics, custom daemon para propagación
4. **Deployment strategy**: ✅ Rolling updates con batches 20%, canary nodes, zero-downtime
5. **Testing strategy**: ✅ Molecule (Ansible), Mininet (network sim), Chaos engineering

---

## 🎯 Contexto de Uso

**User profile**: Desarrollador experimentado con networking en comunidades LibreMesh

**Project philosophy** (del CLAUDE.md):
- **Excellence over speed**: Soluciones correctas, no patches temporales
- **Finish what you start**: No TODOs sin resolver, no planes incompletos
- **Automation-first**: Si se repite 3x, construir herramienta
- **Integration priority**: Soluciones holísticas, no workarounds
- **Low-profile contributions**: El trabajo habla, minimizar autopromoción

**Communication style**:
- Técnico, basado en hechos, sin grandilocuencia
- Commits: `<type>(<scope>): brief description` + "AI-assisted development"
- Documentación: Solo cuando lógica no es obvia o hay context crítico

---

## 📚 Referencias Técnicas

**RFCs:**
- RFC 4271: BGP-4 protocol
- RFC 4760: Multiprotocol Extensions for BGP-4
- RFC 5925: TCP-AO (BGP MD5 successor)
- RFC 6811: RPKI validation

**BIRD Docs:**
- https://bird.network.cz/ (BIRD 3.x manual)
- Migration guide 1.6 → 2.x → 3.x

**TINC Docs:**
- https://www.tinc-vpn.org/documentation/ (TINC 1.0 reference)
- Switch mode vs router mode comparison

**Ansible Best Practices:**
- https://docs.ansible.com/ansible/latest/user_guide/playbooks_best_practices.html
- Ansible Vault, dynamic inventory

**Papers:**
- "Comparison of Routing Protocols for Wireless Mesh Networks" (IEEE 2018)
- "Delay-Based Metric Extension for Babel" (2011)

---

## 🚀 Próximos Pasos

1. **Validar con Pablo**: Confirmar arquitectura y prioridades
2. **Crear estructura inicial**: `mkdir -p` directorios, Makefile básico
3. **Sprint 1 execution**: BIRD + TINC local deployment
4. **Iterar con Grok**: Consultar edge cases durante implementación
5. **Documentar decisiones**: Mantener context transfer documents

---

**Let's build a production-grade BGP network system con automation exhaustiva y resiliencia probada.**

---

*Prompt versión 2.0 - Validado con Grok - 2025-10-27*
*Basado en análisis técnico profundo: BIRD 3.x, TINC 1.0, Ansible, etcd, Prometheus*
