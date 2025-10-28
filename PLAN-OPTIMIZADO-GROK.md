# Plan Optimizado BGP - Análisis Grok

**Fecha**: 2025-10-27
**Objetivo**: Reducir de 42-45 archivos a ~28-32, priorizar funcional sobre documentación

---

## Respuestas a Preguntas Específicas

### 1. CONTRIBUTING.md y CHANGELOG.md
**Decisión**: **POSTERGAR hasta Sprint 2**
- CONTRIBUTING innecesario sin colaboración externa
- CHANGELOG irrelevante sin releases, usar `git log --oneline`
- Agregar cuando el equipo crezca o se publique repo

### 2. LICENSE
**Decisión**: **POSTERGAR hasta Sprint 2**
- No afecta ejecución ni testing en dev local
- Permite revisar dependencies primero (BIRD BSD, TINC GPL2)
- Agregar al publicar en repo público
- Considerar MIT para compatibilidad Go daemon

### 3. Docs (7 archivos)
**Decisión**: **Solo QUICKSTART.md + architecture/decisions.md**
- **Mantener**:
  - `QUICKSTART.md`: Steps para `make deploy-local`, verificaciones
  - `architecture/decisions.md`: ADRs (BIRD 3.x, TINC 1.0, etcd)
- **Postergar**:
  - README duplicado en docs/
  - overview.md
  - runbooks/ (deployment, troubleshooting) → para producción
  - api/daemon-api.md → cuando daemon madure

### 4. Docker entrypoints
**Decisión**: **MANTENER separados bird y tinc, UNIFICAR monitoring**
- Bird y TINC separados para:
  - Restarts independientes (`docker restart bird1`)
  - Debugging granular (logs por servicio)
  - Healthchecks específicos
- Monitoring unificado (Prometheus + Grafana en un solo Dockerfile)
- Evitar supervisord (viola single-responsibility)

### 5. Molecule testing
**Decisión**: **Solo integration tests básicos en Sprint 1**
- Molecule overhead en setup (molecule.yml, custom images)
- Priorizar `test_bgp_peering.sh` simple:
  - Spin compose
  - Assert BGP sessions established (`birdc show protocols`)
  - Verificar propagación etcd
- Molecule para Sprint 2 cuando roles sean maduros

---

## Plan Optimizado por Categorías

### MANTENER (~20 archivos) ✅

**Root (5)**:
- `.gitignore` - Go builds, .env
- `README.md` - Overview + QUICKSTART mergeado
- `Makefile` - Targets: deploy-local, test, monitor, clean, validate, help
- `docker-compose.yml` - 3 bird + 3 tinc + 3 etcd + prometheus + grafana
- `.env.example` - ETCD_INITIAL_CLUSTER, BIRD_PASSWORD
- `.editorconfig` - Code style desde día 1

**Docs (2)**:
- `QUICKSTART.md` - git clone, make deploy-local, verificaciones
- `architecture/decisions.md` - ADRs técnicos

**Docker (6)**:
- `bird/Dockerfile` + `bird/entrypoint.sh`
- `tinc/Dockerfile` + `tinc/entrypoint.sh`
- `monitoring/Dockerfile` + `monitoring/entrypoint.sh` (unificado)

**Configs (8)**:
- `bird/bird.conf.j2` - Router ID, protocols BGP
- `bird/filters.conf` - Route-maps
- `bird/protocols.conf` - Peers over TINC IPs
- `tinc/tinc.conf.j2` - Mode=switch, Cipher=AES-256
- `tinc/tinc-up.j2` - ip link set, etcd put /peers
- `tinc/tinc-down.j2` - Cleanup
- `etcd/init.sh` - Cluster bootstrap
- `prometheus/prometheus.yml` - Scrape configs

**Ansible (5)**:
- `ansible.cfg`
- `site.yml`
- `inventory/hosts.ini`
- `group_vars/all.yml`
- `roles/bird/tasks/main.yml`
- `roles/tinc/tasks/main.yml`

**Daemon Go (5)**:
- `go.mod`
- `cmd/bgp-daemon/main.go`
- `pkg/discovery/mdns.go` - mDNS over TINC
- `pkg/types/types.go`
- `README.md` - Build/run instructions

**CI/CD (1)**:
- `.github/workflows/ci.yml` - Lint Go, run make test

**Tests (1)**:
- `integration/test_bgp_peering.sh`

**Total**: ~25-28 archivos

---

### POSTERGAR (Sprint 2) 📅

**Root**:
- `LICENSE` - Cuando se publique repo
- `CONTRIBUTING.md` - Multi-dev collaboration
- `CHANGELOG.md` - Auto-gen en releases

**Docs**:
- `docs/README.md` - Redundante
- `architecture/overview.md` - Expandir decisions.md después
- `runbooks/deployment.md` - Para producción
- `runbooks/troubleshooting.md` - Basado en issues reales
- `api/daemon-api.md` - OpenAPI cuando daemon sea estable

**Tests**:
- `molecule/default/molecule.yml` - Testing avanzado Ansible

**Docker** (si aplica):
- Separar monitoring si crece (alertmanager)

---

### ELIMINAR ❌

- Cualquier README duplicado si QUICKSTART cubre todo
- Docs no mencionadas en "Mantener" o "Postergar"
- Configs innecesarias (si no se usan en Sprint 1)

---

## Justificaciones Técnicas Clave

### Por qué reducir docs:
- **Acelera iteraciones**: Cambios en `tinc-up.j2` no requieren updates masivos
- **Reduce cognitive load**: Foco en código funcional vs polish
- **Mitiga con**: Inline comments + godoc en Go

### Por qué separar entrypoints Docker:
- **Modularidad**: Restart independiente crítico en dev
- **Debugging**: `docker logs bird1` específico
- **Healthchecks**: Por servicio (`birdc show status`)
- **SRP**: Single Responsibility Principle

### Por qué postergar Molecule:
- **Overhead**: Setup toma tiempo (custom images, drivers)
- **MVP approach**: Scripts bash suficientes para validación rápida
- **Integración temprana**: Mejor iterar rápido en daemon Go

### Por qué mantener .editorconfig:
- **Consistencia desde día 1**: Go/Ansible/YAML
- **Evita drifts**: Tabs vs spaces, line endings

---

## Flujo de Trabajo Optimizado

```bash
# Sprint 1 - Setup (~1-2 horas)
git clone <repo>
cd BGP
cp .env.example .env
make deploy-local     # docker-compose up -d

# Validación
make test             # integration/test_bgp_peering.sh
make monitor          # Abre Grafana localhost:3000

# Iteración
vim configs/bird/bird.conf.j2
make validate         # ansible-playbook --syntax-check
docker restart bird1

# Cleanup
make clean            # docker-compose down -v
```

---

## Métricas de Éxito Sprint 1

- [ ] `make deploy-local` funciona en <5min
- [ ] BGP sessions established (birdc show protocols)
- [ ] TINC mesh up (tinc dump reachable)
- [ ] etcd propagation working (etcdctl get /peers)
- [ ] Prometheus scraping metrics
- [ ] Integration test pasa

---

## Trade-offs Aceptados

1. **Menos docs → Mayor reliance en code comments**
   - Mitigation: Godoc + inline comments exhaustivos

2. **Sin Molecule → Menos coverage en edge cases**
   - Mitigation: Integration tests cubren happy path

3. **Sin LICENSE → All rights reserved default**
   - Mitigation: Placeholder UNLICENSED, agregar en Sprint 2

4. **Monitoring unificado → Un container más pesado**
   - Mitigation: Multi-stage build, aceptable para dev local

---

## Próximos Pasos

1. **Crear estructura** con archivos "Mantener"
2. **Makefile funcional** con todos los targets
3. **docker-compose.yml** completo (9 services)
4. **Test smoke** del stack
5. **Commit inicial**: `feat: initial optimized structure (28 files)`

---

**Recomendación Final de Grok**:
Este setup reduce tiempo de bootstrap de horas a 1-2 horas, con dependencies claras. Escalable a producción con adiciones mínimas. Git branches para features pospuestas.
