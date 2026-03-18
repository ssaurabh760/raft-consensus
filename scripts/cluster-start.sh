#!/bin/bash
# Start a 5-node Raft cluster using Docker Compose.
set -e

echo "=== Starting 5-node Raft cluster ==="

docker compose up -d --build

echo ""
echo "Waiting for cluster to start..."
sleep 3

echo ""
echo "=== Checking node status ==="
for port in 8001 8002 8003 8004 8005; do
    echo -n "Node on port $port: "
    curl -s "http://localhost:$port/status" 2>/dev/null || echo "not ready"
done

echo ""
echo "=== Cluster started ==="
echo "HTTP APIs: http://localhost:8001 through http://localhost:8005"
echo "Use 'docker compose logs -f' to follow logs"
echo "Use 'scripts/cluster-stop.sh' to stop"
