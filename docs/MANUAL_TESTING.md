# Manual Testing - Guía de Debugging Paso a Paso

Esta guía replica manualmente lo que hace el workflow de CI para debuggear problemas de integración.

## Prerequisitos

```bash
# Verificar herramientas
docker --version        # >= 24.0
docker compose version  # v2.x
etcdctl version        # Para queries manuales a etcd

# Si falta etcdctl:
ETCD_VER=v3.5.14
wget https://github.com/etcd-io/etcd/releases/download/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz
tar xzf etcd-${ETCD_VER}-linux-amd64.tar.gz
sudo mv etcd-${ETCD_VER}-linux-amd64/etcdctl /usr/local/bin/
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
# ConnectTo = node2
# ConnectTo = node3

# Ver tinc.conf de node2
docker exec tinc2 cat /var/run/tinc/bgpmesh/tinc.conf

# Debería mostrar:
# Name = node2
# Mode = switch
# Port = 655
# ConnectTo = node1

# Verificar que los ConnectTo estén presentes
for i in {1..5}; do
  echo "=== Node $i ==="
  docker exec tinc$i cat /var/run/tinc/bgpmesh/tinc.conf | grep ConnectTo
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

# Debería mostrar:
# Address = tinc2
# -----BEGIN RSA PUBLIC KEY-----
# MIIBCgKCAQEA...
# -----END RSA PUBLIC KEY-----

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

### 6.5 Test de Conectividad TINC (LA PRUEBA DEFINITIVA)

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

### 6.6 Timing Analysis

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
# Ver configuración de BIRD1
docker exec bird1 cat /etc/bird/bird.conf | head -30

# Ver estado general
docker exec bird1 birdc show status
```

### 8.2 Verificar Protocolos BGP (LA PRUEBA FINAL)

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

### 8.3 Diagnóstico de Problemas BGP

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
for i in {1..5}; do
  ESTABLISHED=$(docker exec bird$i birdc show protocols 2>/dev/null | grep -c "Established" || echo "0")
  EXPECTED=$((5-1))  # Cada nodo tiene sesiones con los otros 4
  if [ "$ESTABLISHED" -eq "$ESTABLISHED" ]; then
    echo -e "  ${GREEN}✓${NC} bird$i: $ESTABLISHED BGP sessions established"
  else
    echo -e "  ${YELLOW}⚠${NC} bird$i: $ESTABLISHED BGP sessions (expected $EXPECTED)"
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

## Limpieza

```bash
# Detener todo
docker compose down

# Limpiar volúmenes (borrar todas las claves y datos)
docker compose down -v

# Limpiar todo incluyendo imágenes
docker compose down -v --rmi local
```

## Tips de Debugging

1. **Siempre verificar en orden**: etcd -> host files -> TINC ping -> BGP
2. **Si BGP falla, NUNCA es BGP primero**: Siempre es TINC que no conectó
3. **Timing matters**: Esperar 10-20s después de `docker compose up` antes de testear
4. **Logs son tu amigo**: `docker logs -f` en otra terminal mientras debuggeas
5. **Test incremental**: No testear BGP hasta que TINC ping funcione
