#!/bin/bash
set -euo pipefail

echo "=== BIRD BGP Daemon ==="
echo "Router ID: ${ROUTER_ID:-not set}"
echo "BGP AS: ${BGP_AS:-not set}"
mkdir -p /run/bird
chown bird:bird /run/bird
# Start BIRD in foreground
exec bird -f -c /etc/bird/bird.conf

