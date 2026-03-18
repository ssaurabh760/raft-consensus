#!/bin/bash
# Stop the Raft cluster and clean up.
set -e

echo "=== Stopping Raft cluster ==="
docker compose down

echo "=== Cluster stopped ==="
echo "To also remove data volumes: docker compose down -v"
