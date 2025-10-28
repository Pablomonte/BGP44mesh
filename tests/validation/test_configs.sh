#!/bin/bash
set -euo pipefail

echo "=== Validating Configuration Templates ==="

# Check if jinja2 is available
if ! command -v jinja2 &> /dev/null; then
    echo "⚠ jinja2 not found, installing..."
    sudo apt-get install -y python3-jinja2 >/dev/null 2>&1 || {
        echo "✗ Failed to install jinja2"
        exit 1
    }
fi

# Check if bird is available
if ! command -v bird &> /dev/null; then
    echo "⚠ bird not found, skipping syntax validation (will validate in Docker)"
    echo "✓ Templates will be validated during Docker build"
    exit 0
fi

# Render and validate BIRD config
echo "Validating BIRD config..."
if [ -f configs/bird/bird.conf.j2 ]; then
    jinja2 configs/bird/bird.conf.j2 \
        -D router_id=192.0.2.1 \
        -D bgp_as=65000 \
        > /tmp/bird_test.conf

    if bird --parse-only -c /tmp/bird_test.conf 2>/dev/null; then
        echo "✓ BIRD config valid"
    else
        echo "✗ BIRD config invalid"
        cat /tmp/bird_test.conf
        exit 1
    fi

    rm -f /tmp/bird_test.conf
else
    echo "⚠ bird.conf.j2 not found yet"
fi

echo "=== Configuration validation complete ==="
