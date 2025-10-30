# Manual Testing - Guía de Debugging Paso a Paso

Esta guía replica manualmente lo que hace el workflow de CI para debuggear problemas de integración.

**Actualizado para Sprint 1.5** (Enero 2025)

- ✅ 5 nodos en full mesh topology (antes 3)
- ✅ BGP peers generados dinámicamente desde templates (protocols.conf.j2)
- ✅ TINC con Subnet declarations para layer 2 (fix de ARP)
- ✅ Tests escalables con detección automática de node count
- ✅ Pre-commit hooks configurados (gofmt, go vet, tests)

## Arquitectura Sprint 1.5 - Cambios Clave

### Dynamic BGP Peer Configuration

En lugar de hardcodear peers en `protocols.conf`, ahora usamos Jinja2 templates:

- **Template**: `configs/bird/protocols.conf.j2`
- **Variables**: `NODE_IP`, `NODE_ID`, `TOTAL_NODES` (desde docker-compose.yml)
- **Resultado**: Cada nodo genera N-1 peers automáticamente (full mesh)

Ejemplo para node1 (NODE_IP=10.0.0.1, TOTAL_NODES=5):

```conf
protocol bgp peer1 { local 10.0.0.1 as 65000; neighbor 10.0.0.2 as 65000; }
protocol bgp peer2 { local 10.0.0.1 as 65000; neighbor 10.0.0.3 as 65000; }
protocol bgp peer3 { local 10.0.0.1 as 65000; neighbor 10.0.0.4 as 65000; }
protocol bgp peer4 { local 10.0.0.1 as 65000; neighbor 10.0.0.5 as 65000; }
```

### TINC Layer 2 Fix

Agregamos `Subnet = IP/32` en host files para correcta resolución ARP:

- **Sin Subnet**: ARP muestra `<incomplete>`, ping falla
- **Con Subnet**: ARP muestra `REACHABLE`, ping 100% exitoso

### Pre-Commit Hooks

Instalados en `.git/hooks/pre-commit` para prevenir CI failures:

1. ✅ Go formatting (`gofmt -s`)
2. ✅ Go vet (static analysis)
3. ✅ Unit tests

Instalar en nuevos clones: `./scripts/install-hooks.sh`

## Prerequisitos

```bash
# Verificar herramientas
docker --version        # >= 24.0
docker compose version  # v2.x
etcdctl version        # Para queries manuales a etcd
go version             # >= 1.23 (para daemon-go development)

# Si falta etcdctl:
ETCD_VER=v3.5.14
wget https://github.com/etcd-io/etcd/releases/download/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz
tar xzf etcd-${ETCD_VER}-linux-amd64.tar.gz
sudo mv etcd-${ETCD_VER}-linux-amd64/etcdctl /usr/local/bin/

# Instalar pre-commit hooks (opcional pero recomendado)
./scripts/install-hooks.sh
```

## Paso 1: Preparación

```bash
cd /home/pablo/repos/BGP

# Limpiar ejecución anterior
docker compose down -v
docker volume prune -f

# Copiar .env
cp .env.example .env

# Ver configuración
cat .env
```

## Paso 2: Build de Imágenes

```bash
# Build con output detallado
docker compose build --no-cache --progress=plain

# Verificar imágenes creadas
docker images | grep bgp4mesh

# Deberías ver:
# bgp4mesh-bird
# bgp4mesh-tinc
# bgp4mesh-daemon
# bgp4mesh-prometheus
```

## Paso 3: Levantar Servicios

```bash
# Levantar con logs en foreground
docker compose up

# O en background para seguir trabajando:
docker compose up -d

# Ver logs de todos los servicios
docker compose logs -f
```

## Paso 4: Verificar Contenedores

```bash
# Listar todos los contenedores
docker compose ps

# Deberías ver 21 contenedores "Up":
# - tinc1, tinc2, tinc3, tinc4, tinc5 (5)
# - bird1, bird2, bird3, bird4, bird5 (5)
# - daemon1, daemon2, daemon3, daemon4, daemon5 (5)
# - etcd1, etcd2, etcd3, etcd4, etcd5 (5)
# - prometheus (1)

# Verificar que estén healthy
docker compose ps | grep healthy

# Si alguno no está Up, ver logs:
docker logs tinc1
docker logs bird1
docker logs daemon1
```

## Paso 5: Verificar etcd (Clave!)

### 5.1 Verificar Cluster

```bash
# Ver miembros del cluster
docker exec etcd1 etcdctl member list

# Verificar salud
docker exec etcd1 etcdctl endpoint health

# Ver estado detallado
docker exec etcd1 etcdctl endpoint status --write-out=table
```

### 5.2 Verificar Registro de Peers

```bash
# Listar todas las keys de peers
docker exec etcd1 etcdctl get /peers --prefix --keys-only

# Deberías ver:
# /peers/node1
# /peers/node2
# /peers/node3
# /peers/node4
# /peers/node5

# Ver contenido completo de un peer
docker exec etcd1 etcdctl get /peers/node1

# Debería mostrar JSON con:
# {"ip":"10.0.0.1","endpoint":"node1:655","key":"-----BEGIN RSA PUBLIC KEY-----\n..."}
```

### 5.3 Verificar Timing de Registro

```bash
# Ver logs de TINC para ver cuándo se registró
docker logs tinc1 | grep -E "interface configured|Waiting for"

# Ver logs de daemon para ver cuándo sincronizó
docker logs daemon1 | grep -E "Stored own key|Synced host file"
```

## Paso 6: Verificar TINC (Debugging Detallado)

### 6.1 Verificar Configuración de TINC

```bash
# Ver tinc.conf de node1
docker exec tinc1 cat /var/run/tinc/bgpmesh/tinc.conf

# Debería mostrar:
# Name = node1
# Mode = switch
# Port = 655
# ConnectTo = node2  # Bootstrap topology - hardcoded in entrypoint.sh
# ConnectTo = node3  # Node1 and node2 use hardcoded ConnectTo for initial mesh

# Ver tinc.conf de node2
docker exec tinc2 cat /var/run/tinc/bgpmesh/tinc.conf

# Debería mostrar:
# Name = node2
# Mode = switch
# Port = 655
# ConnectTo = node1  # Bootstrap node

# Nota: A partir de node3, no hay ConnectTo hardcodeados
# Los nodos 3-5 se conectan dinámicamente vía peer discovery

# Verificar configuración en todos los nodos
for i in {1..5}; do
  echo "=== Node $i ==="
  docker exec tinc$i cat /var/run/tinc/bgpmesh/tinc.conf | grep -E "Name|Mode|Port|ConnectTo"
done
```

### 6.2 Verificar Host Files (CRÍTICO!)

```bash
# Ver host files en tinc1
docker exec tinc1 ls -la /var/run/tinc/bgpmesh/hosts/

# Deberían existir 5 archivos: node1, node2, node3, node4, node5

# Si faltan archivos, el problema está aquí!
# Verificar contenido de un host file
docker exec tinc1 cat /var/run/tinc/bgpmesh/hosts/node2

# Debería mostrar (Sprint 1.5 - con Subnet declaration):
# # Host configuration for node2
# Address = node2
# Port = 655
# Subnet = 10.0.0.2/32
#
# -----BEGIN RSA PUBLIC KEY-----
# MIIBCgKCAQEA...
# -----END RSA PUBLIC KEY-----

# IMPORTANTE: La línea "Subnet = 10.0.0.X/32" es CRÍTICA para layer 2
# Sin ella, ARP resolution falla y el ping no funciona

# Verificar en todos los nodos
for i in {1..5}; do
  echo "=== Tinc $i - Host Files ==="
  docker exec tinc$i ls /var/run/tinc/bgpmesh/hosts/ | wc -l
  docker exec tinc$i ls /var/run/tinc/bgpmesh/hosts/
done
```

### 6.3 Verificar Proceso tincd

```bash
# Ver procesos tincd corriendo
for i in {1..5}; do
  echo "=== Tinc $i ==="
  docker exec tinc$i ps aux | grep tincd | grep -v grep
done

# Ver si tincd está en foreground (-D) o background
docker exec tinc1 ps aux | grep "tincd -D"
```

### 6.4 Verificar Interfaces de Red

```bash
# Ver interface tinc0 en cada nodo
for i in {1..5}; do
  echo "=== Tinc $i Interface ==="
  docker exec tinc$i ip addr show tinc0
done

# Cada uno debería mostrar:
# tinc0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1400
#     inet 10.0.0.X/24
#     inet6 2001:db8::X/64

# Ver rutas
docker exec tinc1 ip route
```

### 6.5 Verificar ARP Resolution (Sprint 1.5 - Crítico)

```bash
# Verificar tabla ARP en tinc1
docker exec tinc1 ip neigh show dev tinc0

# Debería mostrar REACHABLE para todos los peers:
# 10.0.0.2 lladdr XX:XX:XX:XX:XX:XX REACHABLE
# 10.0.0.3 lladdr XX:XX:XX:XX:XX:XX REACHABLE
# 10.0.0.4 lladdr XX:XX:XX:XX:XX:XX REACHABLE
# 10.0.0.5 lladdr XX:XX:XX:XX:XX:XX REACHABLE

# Si muestra "<incomplete>", falta la declaración Subnet en host files!
# Verificar que TODOS los host files tienen Subnet:
for i in {1..5}; do
  echo "=== node$i host file ==="
  docker exec tinc1 grep "Subnet" /var/run/tinc/bgpmesh/hosts/node$i
done
```

### 6.6 Test de Conectividad TINC (LA PRUEBA DEFINITIVA)

```bash
# Desde tinc1, ping a todos los demás
echo "=== Ping from tinc1 to all nodes ==="
for i in {2..5}; do
  echo -n "tinc1 -> 10.0.0.$i: "
  docker exec tinc1 ping -c 3 -W 2 10.0.0.$i >/dev/null 2>&1 && echo "✓ OK" || echo "✗ FAIL"
done

# Si FALLA el ping, AQUÍ está el problema!
# Diagnosticar:

# 1. Ver si tincd está intentando conectar
docker logs tinc1 2>&1 | tail -50

# 2. Ver conexiones de red activas
docker exec tinc1 netstat -tupn | grep tincd

# 3. Intentar conexión manual (si tinc CLI disponible)
docker exec tinc1 sh -c 'tinc -n bgpmesh dump nodes 2>/dev/null' || echo "tinc CLI not available"

# 4. Verificar que existe el host file del peer
docker exec tinc1 ls -la /var/run/tinc/bgpmesh/hosts/node2

# 5. Ver si hay mensajes de error en logs
docker logs tinc1 2>&1 | grep -i "error\|fail\|timeout\|refused"
```

### 6.7 Timing Analysis

```bash
# Ver el orden temporal de eventos
echo "=== TINC1 Timeline ==="
docker logs tinc1 2>&1 | grep -E "Generating|rendered|ConnectTo|Waiting for host|Starting TINC|configured"

# Esto debería mostrar:
# 1. Generating RSA keys
# 2. tinc.conf rendered
# 3. Bootstrap topology configured (ConnectTo directives added)
# 4. Waiting for host file propagation (10s)
# 5. Starting TINC daemon
# 6. interface configured
```

### 6.8 Full Mesh Ping Validation (Sprint 1.5)

```bash
# Test completo de conectividad full mesh (N×(N-1) pairs)
# Para 5 nodos = 20 pings totales

echo "=== Full Mesh Connectivity Test ==="
TOTAL=0
SUCCESS=0

for src in {1..5}; do
  for dst in {1..5}; do
    if [ "$src" != "$dst" ]; then
      TOTAL=$((TOTAL + 1))
      if docker exec tinc$src ping -c 1 -W 2 10.0.0.$dst >/dev/null 2>&1; then
        SUCCESS=$((SUCCESS + 1))
        echo "✓ tinc$src -> 10.0.0.$dst"
      else
        echo "✗ tinc$src -> 10.0.0.$dst FAILED"
      fi
    fi
  done
done

echo ""
echo "Result: $SUCCESS/$TOTAL pings successful"

# Para 5 nodos, deberías ver: 20/20 pings successful
```

## Paso 7: Verificar Daemon Go

```bash
# Ver logs del daemon1
docker logs daemon1 | tail -80

# Buscar mensajes clave en orden:
docker logs daemon1 | grep -E "Connecting to etcd|Connected to etcd|Stored own key|Synced host file|etcd PUT"

# Debería mostrar:
# 1. ✓ Connected to etcd
# 2. ✓ TINC manager initialized
# 3. ✓ Read local public key (XXX bytes)
# 4. ✓ Stored own key in etcd at /peers/node1
# 5. ✓ Synced host file for peer (para cada peer)
# 6. 📥 etcd PUT: /peers/nodeX (cuando otros nodos se registran)

# Ver si el daemon está recibiendo eventos de etcd
docker logs daemon1 | grep "📥 etcd PUT"

# Si NO hay mensajes "etcd PUT", el daemon no está viendo los otros nodos!
# Verificar conexión a etcd:
docker exec daemon1 nc -zv etcd1 2379
```

## Paso 8: Verificar BIRD (BGP)

### 8.1 Verificar Configuración

```bash
# Ver configuración principal de BIRD1
docker exec bird1 cat /etc/bird/bird.conf | head -30

# Ver configuración de peers (generada dinámicamente desde template)
docker exec bird1 cat /var/run/bird/protocols.conf

# Deberías ver N-1 peers (para 5 nodos, 4 peers):
# protocol bgp peer1 {
#     description "BGP peer at 10.0.0.2";
#     local 10.0.0.1 as 65000;
#     neighbor 10.0.0.2 as 65000;
#     ...
# }
# protocol bgp peer2 { ... }
# protocol bgp peer3 { ... }
# protocol bgp peer4 { ... }

# IMPORTANTE: protocols.conf se genera desde protocols.conf.j2
# usando variables de entorno NODE_IP, NODE_ID, TOTAL_NODES

# Verificar que cada nodo tiene su propia configuración única
for i in {1..5}; do
  echo "=== Bird $i - Local IP ==="
  docker exec bird$i grep "local 10.0.0" /var/run/bird/protocols.conf | head -1
done

# Ver estado general
docker exec bird1 birdc show status
```

### 8.2 Verificar Template Rendering (Sprint 1.5)

```bash
# Ver variables de entorno usadas para rendering
docker exec bird1 env | grep -E "NODE_IP|NODE_ID|TOTAL_NODES|BGP_AS"

# Deberías ver:
# NODE_IP=10.0.0.1
# NODE_ID=1
# TOTAL_NODES=5
# BGP_AS=65000

# Verificar que el template se renderizó correctamente
docker exec bird1 ls -la /var/run/bird/protocols.conf

# Ver el contenido generado
docker exec bird1 cat /var/run/bird/protocols.conf | head -50

# Contar cuántos peers se generaron
docker exec bird1 grep -c "protocol bgp peer" /var/run/bird/protocols.conf

# Debería ser N-1 (para 5 nodos = 4 peers)

# Verificar que cada nodo tiene diferentes IPs locales
for i in {1..5}; do
  echo -n "bird$i local IP: "
  docker exec bird$i grep "local 10.0.0" /var/run/bird/protocols.conf | head -1 | awk '{print $2}'
done

# Deberías ver:
# bird1 local IP: 10.0.0.1
# bird2 local IP: 10.0.0.2
# bird3 local IP: 10.0.0.3
# bird4 local IP: 10.0.0.4
# bird5 local IP: 10.0.0.5
```

### 8.3 Verificar Protocolos BGP (LA PRUEBA FINAL)

```bash
# Ver todos los protocolos en bird1
docker exec bird1 birdc show protocols

# Debería mostrar:
# Name       Proto      Table      State  Since         Info
# device1    Device     ---        up     HH:MM:SS
# kernel1    Kernel     master4    up     HH:MM:SS
# static1    Static     master4    up     HH:MM:SS
# peer1      BGP        ---        up     HH:MM:SS  Established
# peer2      BGP        ---        up     HH:MM:SS  Established
# peer3      BGP        ---        up     HH:MM:SS  Established
# peer4      BGP        ---        up     HH:MM:SS  Established

# Contar sesiones establecidas
docker exec bird1 birdc show protocols | grep -c "Established"

# Ver detalles de una sesión específica
docker exec bird1 birdc show protocols all peer1

# Si el estado es "start" o "Active" en lugar de "Established":
docker exec bird1 birdc show protocols all peer1 | grep -A 5 "BGP state"

# Los errores comunes:
# - "Socket: No route to host" -> TINC no conectó!
# - "Socket: Connection refused" -> BIRD del peer no está escuchando
# - "Active" -> Intentando conectar (puede ser timing)
```

### 8.4 Full Mesh BGP Validation (Sprint 1.5)

```bash
# Verificar que TODOS los nodos tienen 4/4 sesiones BGP establecidas
echo "=== Full Mesh BGP Session Validation ==="

for i in {1..5}; do
  ESTABLISHED=$(docker exec bird$i birdc show protocols 2>/dev/null | grep -c "Established" || echo "0")
  EXPECTED=4  # N-1 para 5 nodos

  if [ "$ESTABLISHED" -eq "$EXPECTED" ]; then
    echo "✓ bird$i: $ESTABLISHED/$EXPECTED sessions established"
  else
    echo "✗ bird$i: $ESTABLISHED/$EXPECTED sessions (INCOMPLETE)"
    docker exec bird$i birdc show protocols
  fi
done

# Resultado esperado para 5 nodos:
# ✓ bird1: 4/4 sessions established
# ✓ bird2: 4/4 sessions established
# ✓ bird3: 4/4 sessions established
# ✓ bird4: 4/4 sessions established
# ✓ bird5: 4/4 sessions established
#
# Total: 20 sesiones BGP (5 nodos × 4 peers cada uno)
```

### 8.5 Diagnóstico de Problemas BGP

```bash
# Si BGP no establece:

# 1. SIEMPRE verificar TINC primero
docker exec bird1 ping -c 3 10.0.0.2

# 2. Verificar que BIRD está escuchando en puerto 179
docker exec bird1 netstat -tuln | grep 179

# 3. Ver logs de BIRD
docker logs bird1 | grep -i "bgp\|peer\|error"

# 4. Test de conectividad TCP al peer
docker exec bird1 nc -zv 10.0.0.2 179

# 5. Ver configuración de peer en BIRD
docker exec bird1 cat /etc/bird/bird.conf | grep -A 10 "protocol bgp peer1"
```

## Paso 9: Script de Test Completo

```bash
# Crear script de test automatizado
cat > /tmp/manual_test.sh << 'TESTSCRIPT'
#!/bin/bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================="
echo "  Manual Integration Test"
echo "========================================="
echo ""

# Test 1: Containers
echo "Test 1: Verificando contenedores..."
RUNNING=$(docker compose ps --format json 2>/dev/null | jq -r 'select(.State == "running") | .Name' | wc -l)
if [ "$RUNNING" -eq 21 ]; then
  echo -e "${GREEN}✓${NC} $RUNNING/21 containers running"
else
  echo -e "${RED}✗${NC} Solo $RUNNING/21 containers running"
  docker compose ps
  exit 1
fi
echo ""

# Test 2: etcd
echo "Test 2: Verificando etcd..."
docker exec etcd1 etcdctl endpoint health >/dev/null 2>&1 || { echo -e "${RED}✗${NC} etcd not healthy"; exit 1; }
PEERS=$(docker exec etcd1 etcdctl get /peers --prefix --keys-only 2>/dev/null | wc -l)
if [ "$PEERS" -eq 5 ]; then
  echo -e "${GREEN}✓${NC} etcd healthy, $PEERS peers registered"
else
  echo -e "${YELLOW}⚠${NC} etcd healthy but only $PEERS/5 peers"
fi
echo ""

# Test 3: TINC Host Files
echo "Test 3: Verificando TINC host files..."
ALL_OK=true
for i in {1..5}; do
  HOSTS=$(docker exec tinc$i ls /var/run/tinc/bgpmesh/hosts/ 2>/dev/null | wc -l)
  if [ "$HOSTS" -eq 5 ]; then
    echo -e "  ${GREEN}✓${NC} tinc$i has 5 host files"
  else
    echo -e "  ${RED}✗${NC} tinc$i has only $HOSTS host files"
    docker exec tinc$i ls /var/run/tinc/bgpmesh/hosts/
    ALL_OK=false
  fi
done
$ALL_OK || { echo "Host files missing!"; exit 1; }
echo ""

# Test 4: TINC Connectivity
echo "Test 4: Verificando conectividad TINC..."
ALL_OK=true
for i in {2..5}; do
  if docker exec tinc1 ping -c 2 -W 2 10.0.0.$i >/dev/null 2>&1; then
    echo -e "  ${GREEN}✓${NC} tinc1 -> 10.0.0.$i"
  else
    echo -e "  ${RED}✗${NC} tinc1 -> 10.0.0.$i FAILED"
    ALL_OK=false
  fi
done
$ALL_OK || { echo "TINC mesh not connected!"; exit 1; }
echo ""

# Test 5: BGP Sessions
echo "Test 5: Verificando sesiones BGP..."
ALL_OK=true
EXPECTED=4  # Para 5 nodos, cada uno tiene 4 peers (full mesh)
for i in {1..5}; do
  ESTABLISHED=$(docker exec bird$i birdc show protocols 2>/dev/null | grep -c "Established" || echo "0")
  if [ "$ESTABLISHED" -eq "$EXPECTED" ]; then
    echo -e "  ${GREEN}✓${NC} bird$i: $ESTABLISHED/$EXPECTED BGP sessions established"
  else
    echo -e "  ${YELLOW}⚠${NC} bird$i: $ESTABLISHED/$EXPECTED BGP sessions (incomplete)"
    docker exec bird$i birdc show protocols
    ALL_OK=false
  fi
done
$ALL_OK || { echo "BGP sessions not fully established"; exit 1; }
echo ""

echo "========================================="
echo -e "${GREEN}✓ All tests passed!${NC}"
echo "========================================="
TESTSCRIPT

chmod +x /tmp/manual_test.sh

# Ejecutar test
/tmp/manual_test.sh
```

## Paso 10: Análisis de Timing

```bash
# Este script muestra el timeline de eventos para diagnosticar timing issues
cat > /tmp/timing_analysis.sh << 'TIMING'
#!/bin/bash

echo "=== TIMING ANALYSIS ==="
echo ""

echo "--- Node1 Timeline ---"
docker logs tinc1 2>&1 | grep -E "Configuration:|Generating|rendered|ConnectTo|Waiting|Starting|configured" | head -20

echo ""
echo "--- Daemon1 Timeline ---"
docker logs daemon1 2>&1 | grep -E "Starting|Connected|Stored own key|Synced|etcd PUT" | head -15

echo ""
echo "--- Bird1 Timeline ---"
docker logs bird1 2>&1 | grep -E "Started|Listening|BGP" | head -10

echo ""
echo "--- etcd Registration Times ---"
for i in {1..5}; do
  echo -n "node$i registered: "
  docker logs daemon$i 2>&1 | grep "Stored own key" | head -1 | cut -d' ' -f1-2
done
TIMING

chmod +x /tmp/timing_analysis.sh
/tmp/timing_analysis.sh
```

## Troubleshooting Común

### Problema: Host files no se sincronizan

```bash
# Verificar que el daemon puede leer la clave pública
docker exec daemon1 cat /var/run/tinc/bgpmesh/rsa_key.pub

# Verificar que puede escribir en hosts/
docker exec daemon1 touch /var/run/tinc/bgpmesh/hosts/test_write
docker exec daemon1 rm /var/run/tinc/bgpmesh/hosts/test_write

# Ver si el daemon recibe eventos de etcd
docker logs daemon1 | grep "etcd PUT" | tail -10

# Ver si hay errores al escribir host files
docker logs daemon1 | grep -i "error\|fail"
```

### Problema: TINC no conecta después de tener host files

```bash
# Ver logs completos de TINC (últimas 100 líneas)
docker logs tinc1 2>&1 | tail -100

# Si no hay output de conexión, TINC puede estar corriendo sin debug
# Verificar cómo se inició tincd:
docker exec tinc1 ps aux | grep tincd

# Si se inició con -D (foreground), los logs deberían estar en docker logs
# Si se inició sin -D, puede estar daemonizado sin logs

# Intentar ver el estado interno de TINC (si tinc CLI está disponible):
docker exec tinc1 sh -c 'command -v tinc && tinc -n bgpmesh dump nodes' || echo "tinc CLI not in PATH"
```

### Problema: Timing - Containers arrancan en orden incorrecto

```bash
# Ver el orden de arranque
docker compose ps --format "{{.Name}} {{.Status}}"

# Verificar depends_on en docker-compose.yml
grep -A 5 "depends_on:" docker-compose.yml

# Si los daemons arrancan antes que TINC genere claves:
docker logs daemon1 | head -20

# Deberías ver: "⚠ Failed to read local key" si arranca muy temprano
# Solución: Agregar health checks o delays
```

### Problema Sprint 1.5: Template rendering falló

```bash
# Verificar que el template existe
docker exec bird1 ls -la /etc/bird/protocols.conf.j2

# Verificar variables de entorno
docker exec bird1 env | grep -E "NODE_|TOTAL_|BGP_"

# Verificar que protocols.conf se generó
docker exec bird1 ls -la /var/run/bird/protocols.conf

# Si no existe, ver logs de entrypoint
docker logs bird1 | grep -i "rendering\|template\|jinja"

# Verificar Python y Jinja2 están instalados
docker exec bird1 which python3
docker exec bird1 python3 -c "import jinja2; print(jinja2.__version__)"

# Re-generar manualmente para debugging
docker exec bird1 python3 << 'EOF'
from jinja2 import Template
import os

with open('/etc/bird/protocols.conf.j2', 'r') as f:
    template = Template(f.read())

output = template.render(
    node_ip=os.environ.get('NODE_IP', '10.0.0.1'),
    node_id=int(os.environ.get('NODE_ID', '1')),
    bgp_as=os.environ.get('BGP_AS', '65000'),
    total_nodes=int(os.environ.get('TOTAL_NODES', '5'))
)
print(output)
EOF
```

### Problema Sprint 1.5: ARP muestra incomplete (falta Subnet)

```bash
# Verificar tabla ARP
docker exec tinc1 ip neigh show dev tinc0

# Si muestra "<incomplete>", verificar host files
for i in {1..5}; do
  echo "=== node$i ==="
  docker exec tinc1 cat /var/run/tinc/bgpmesh/hosts/node$i | grep -E "Address|Port|Subnet"
done

# Debería mostrar para cada host:
# Address = nodeX
# Port = 655
# Subnet = 10.0.0.X/32

# Si falta Subnet, verificar que manager.go tiene la línea:
# content := fmt.Sprintf(`...
# Subnet = %s/32
# ...`, peer.IP.String())

# Re-sync forzado (si el daemon está corriendo)
docker restart daemon1 daemon2 daemon3 daemon4 daemon5

# Esperar 10s y verificar de nuevo
sleep 10
docker exec tinc1 ip neigh show dev tinc0
```

### Problema Sprint 1.5: BGP tiene menos peers de los esperados

```bash
# Verificar cuántos peers se generaron en el config
docker exec bird1 grep -c "protocol bgp peer" /var/run/bird/protocols.conf

# Debería ser N-1 (para 5 nodos = 4)

# Verificar TOTAL_NODES env var
docker exec bird1 env | grep TOTAL_NODES

# Si no está seteada o es incorrecta, verificar docker-compose.yml:
grep -A 5 "bird1:" docker-compose.yml | grep TOTAL_NODES

# Debería tener:
# - TOTAL_NODES=5

# Si es incorrecto, editar docker-compose.yml y rebuild
docker compose up -d --build bird1 bird2 bird3 bird4 bird5
```

## Limpieza

```bash
# Detener todo
docker compose down

# Limpiar volúmenes (borrar todas las claves y datos)
docker compose down -v

# Limpiar todo incluyendo imágenes
docker compose down -v --rmi local
```

## Paso 11: Interpretar Tests Automatizados (Sprint 1.5)

Los tests automatizados ahora se adaptan dinámicamente al número de nodos:

```bash
# Ver el test automatizado de BGP peering
cat tests/integration/test_bgp_peering.sh | grep -A 20 "NODE_COUNT"

# El test detecta automáticamente cuántos nodos hay:
NODE_COUNT=$(docker compose ps --services 2>/dev/null | grep -c "^bird" || echo 0)
EXPECTED_PEERS=$((NODE_COUNT - 1))

# Para 5 nodos:
# - NODE_COUNT = 5
# - EXPECTED_PEERS = 4
# - Total BGP sessions = 20 (5 × 4)
# - Total TINC pings = 20 (5 × 4)

# Ejecutar el test completo
make test-integration

# O directamente:
./tests/integration/test_bgp_peering.sh

# Resultado esperado:
# ✓ 5/5 containers running
# ✓ etcd healthy, 5 peers registered
# ✓ BGP: 20/20 sessions established
# ✓ TINC: 20/20 pings successful
# ✓ Full mesh connectivity validated
```

### Thresholds Dinámicos

Los tests ahora usan thresholds que escalan con NODE_COUNT:

```bash
# Para N nodos, se verifican:
EXPECTED_CONTAINERS=$((N * 4 + 1))  # N×(tinc+bird+daemon+etcd) + prometheus
EXPECTED_BGP_SESSIONS=$((N * (N-1)))  # Full mesh bidireccional
EXPECTED_PINGS=$((N * (N-1)))  # Full mesh connectivity

# Ejemplos:
# 3 nodos: 13 containers, 6 BGP sessions, 6 pings
# 5 nodos: 21 containers, 20 BGP sessions, 20 pings
# 10 nodos: 41 containers, 90 BGP sessions, 90 pings
```

## Tips de Debugging

1. **Siempre verificar en orden**: etcd -> host files -> TINC ping -> ARP -> BGP
2. **Si BGP falla, NUNCA es BGP primero**: Siempre es TINC que no conectó
3. **Verificar Subnet declarations**: Sin `Subnet = IP/32`, ARP falla y ping no funciona
4. **Timing matters**: Esperar 20-30s después de `docker compose up` antes de testear (para 5 nodos)
5. **Logs son tu amigo**: `docker logs -f` en otra terminal mientras debuggeas
6. **Test incremental**: No testear BGP hasta que TINC ping funcione
7. **Pre-commit hooks**: Usar `./scripts/install-hooks.sh` para prevenir CI failures
8. **Dynamic config**: Verificar que `protocols.conf` se generó con N-1 peers correctos
