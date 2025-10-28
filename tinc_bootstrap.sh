#!/bin/bash
# TINC Mesh Bootstrap Script
# Distributes host files and configures ConnectTo directives for full-mesh connectivity

set -euo pipefail

# Configuration
NODES=(1 2 3)
TINC_NETNAME="bgpmesh"
TEMP_DIR=$(mktemp -d)

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Cleanup on exit
trap "rm -rf $TEMP_DIR" EXIT

log_info "TINC Bootstrap Script - Starting"
log_info "Working directory: $TEMP_DIR"
echo ""

# Step 1: Extract host files from each container
log_info "Step 1: Extracting host files from containers..."

for i in "${NODES[@]}"; do
    container="tinc$i"
    host_file="$TEMP_DIR/node$i"

    log_info "  Extracting from $container..."

    # Check if host file exists
    if docker exec "$container" test -f "/var/run/tinc/$TINC_NETNAME/hosts/node$i"; then
        docker cp "$container:/var/run/tinc/$TINC_NETNAME/hosts/node$i" "$host_file" 2>/dev/null || {
            log_error "Failed to copy host file from $container"
            exit 1
        }

        # Verify file is not empty
        if [ ! -s "$host_file" ]; then
            log_error "Host file for node$i is empty"
            exit 1
        fi

        # Calculate checksum
        md5sum "$host_file" >> "$TEMP_DIR/checksums.txt"
        log_info "    ✓ Extracted node$i host file ($(wc -l < "$host_file") lines)"
    else
        log_error "Host file not found in $container"
        exit 1
    fi
done

echo ""
log_info "Step 2: Distributing host files to all nodes..."

# Step 2: Distribute host files cross-node
for i in "${NODES[@]}"; do
    container="tinc$i"

    log_info "  Configuring $container..."

    for j in "${NODES[@]}"; do
        if [[ $i != $j ]]; then
            host_file="$TEMP_DIR/node$j"

            # Copy host file to container
            docker cp "$host_file" "$container:/var/run/tinc/$TINC_NETNAME/hosts/node$j" 2>/dev/null || {
                log_error "Failed to copy node$j to $container"
                exit 1
            }

            # Set correct permissions
            docker exec "$container" chmod 644 "/var/run/tinc/$TINC_NETNAME/hosts/node$j" || {
                log_warn "Could not set permissions for node$j in $container"
            }

            log_info "    ✓ Added node$j host file"
        fi
    done
done

echo ""
log_info "Step 3: Adding ConnectTo directives..."

# Step 3: Add ConnectTo directives to tinc.conf
for i in "${NODES[@]}"; do
    container="tinc$i"

    log_info "  Updating $container tinc.conf..."

    # Remove existing ConnectTo lines
    docker exec "$container" sed -i '/^ConnectTo/d' "/var/run/tinc/$TINC_NETNAME/tinc.conf" || {
        log_warn "Could not remove old ConnectTo in $container"
    }

    # Add ConnectTo for each peer
    for j in "${NODES[@]}"; do
        if [[ $i != $j ]]; then
            docker exec "$container" bash -c "echo 'ConnectTo = node$j' >> /var/run/tinc/$TINC_NETNAME/tinc.conf"
            log_info "    ✓ Added ConnectTo = node$j"
        fi
    done
done

echo ""
log_info "Step 4: Reloading TINC daemons..."

# Step 4: Reload tincd on all nodes
for i in "${NODES[@]}"; do
    container="tinc$i"

    log_info "  Reloading $container..."

    # Try graceful reload first
    if docker exec "$container" tincd -n "$TINC_NETNAME" -kHUP 2>/dev/null; then
        log_info "    ✓ Graceful reload successful"
    else
        # Fallback to container restart
        log_warn "    Graceful reload failed, restarting container..."
        docker restart "$container" >/dev/null 2>&1
        log_info "    ✓ Container restarted"
    fi
done

# Wait for convergence
log_info "Waiting 15 seconds for mesh convergence..."
sleep 15

echo ""
log_info "Step 5: Verifying connectivity..."

# Step 5: Verify connections
all_connected=true

for i in "${NODES[@]}"; do
    container="tinc$i"

    log_info "  Checking $container connections..."

    # Get list of reachable nodes
    reachable=$(docker exec "$container" tinc -n "$TINC_NETNAME" dump reachable 2>/dev/null | grep -c "node" || echo "0")
    expected=$((${#NODES[@]} - 1))  # Should connect to n-1 peers

    if [ "$reachable" -ge "$expected" ]; then
        log_info "    ✓ Connected to $reachable peers (expected $expected)"
    else
        log_warn "    ⚠ Only connected to $reachable peers (expected $expected)"
        all_connected=false
    fi

    # Verify interface is up
    if docker exec "$container" ip link show tinc0 2>/dev/null | grep -q "state UP"; then
        log_info "    ✓ Interface tinc0 is UP"
    else
        log_error "    ✗ Interface tinc0 is DOWN"
        all_connected=false
    fi
done

echo ""
echo "======================================"

if [ "$all_connected" = true ]; then
    log_info "TINC mesh bootstrap completed successfully!"
    echo ""
    log_info "Next steps:"
    echo "  1. Verify BGP sessions: docker exec bird1 birdc show protocols"
    echo "  2. Test connectivity: docker exec tinc1 ping -c3 10.0.0.2"
    echo "  3. Run integration tests: make test-integration"
    exit 0
else
    log_error "TINC mesh bootstrap completed with warnings"
    log_warn "Check container logs for details: docker logs tinc1"
    exit 1
fi
