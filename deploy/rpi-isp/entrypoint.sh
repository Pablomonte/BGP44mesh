#!/bin/bash
set -euo pipefail

echo "=== BIRD BGP Daemon (Mock ISP) ==="
echo "Router ID: 172.30.0.1"
echo "BGP AS: 65001"

mkdir -p /run/bird
chown bird:bird /run/bird

# Start BIRD in foreground
exec bird -f -c /etc/bird/bird.conf

