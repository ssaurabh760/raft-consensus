#!/bin/bash
# Demo script showing Raft consensus features:
# 1. Leader election
# 2. KV store operations
# 3. Log replication
# 4. Fault tolerance
set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

BASE_URL="http://localhost"

echo -e "${GREEN}=== Raft Consensus Demo ===${NC}"
echo ""

# Step 1: Check cluster status
echo -e "${YELLOW}--- Step 1: Check cluster status ---${NC}"
for port in 8001 8002 8003 8004 8005; do
    echo -n "  Node on port $port: "
    curl -s "$BASE_URL:$port/status" | python3 -m json.tool 2>/dev/null || echo "not available"
done
echo ""

# Find the leader
LEADER_PORT=""
for port in 8001 8002 8003 8004 8005; do
    ROLE=$(curl -s "$BASE_URL:$port/status" 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('role',''))" 2>/dev/null)
    if [ "$ROLE" = "Leader" ]; then
        LEADER_PORT=$port
        break
    fi
done

if [ -z "$LEADER_PORT" ]; then
    echo -e "${RED}No leader found! Is the cluster running?${NC}"
    echo "Start with: scripts/cluster-start.sh"
    exit 1
fi
echo -e "${GREEN}Leader found on port $LEADER_PORT${NC}"
echo ""

# Step 2: Write key-value pairs
echo -e "${YELLOW}--- Step 2: Write KV pairs via leader ---${NC}"
echo -n "  PUT name=alice: "
curl -s -X PUT "$BASE_URL:$LEADER_PORT/kv/name" -d '{"value":"alice"}'
echo ""
echo -n "  PUT age=30: "
curl -s -X PUT "$BASE_URL:$LEADER_PORT/kv/age" -d '{"value":"30"}'
echo ""
echo -n "  PUT city=sf: "
curl -s -X PUT "$BASE_URL:$LEADER_PORT/kv/city" -d '{"value":"sf"}'
echo ""

sleep 1

# Step 3: Read from different nodes (shows replication)
echo ""
echo -e "${YELLOW}--- Step 3: Read from all nodes (shows replication) ---${NC}"
for port in 8001 8002 8003 8004 8005; do
    echo -n "  GET name from port $port: "
    curl -s "$BASE_URL:$port/kv/name" 2>/dev/null || echo "not available"
    echo ""
done
echo ""

# Step 4: Delete a key
echo -e "${YELLOW}--- Step 4: Delete a key ---${NC}"
echo -n "  DELETE city: "
curl -s -X DELETE "$BASE_URL:$LEADER_PORT/kv/city"
echo ""
sleep 1
echo -n "  GET city (should be 404): "
curl -s "$BASE_URL:$LEADER_PORT/kv/city"
echo ""

# Step 5: Fault tolerance demo
echo ""
echo -e "${YELLOW}--- Step 5: Fault tolerance ---${NC}"
echo "  Stopping leader container (raft-node-${LEADER_PORT: -1})..."
docker stop "raft-node-${LEADER_PORT: -1}" 2>/dev/null || true
sleep 3

echo "  Checking for new leader..."
NEW_LEADER=""
for port in 8001 8002 8003 8004 8005; do
    if [ "$port" = "$LEADER_PORT" ]; then continue; fi
    ROLE=$(curl -s "$BASE_URL:$port/status" 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('role',''))" 2>/dev/null)
    if [ "$ROLE" = "Leader" ]; then
        NEW_LEADER=$port
        break
    fi
done

if [ -n "$NEW_LEADER" ]; then
    echo -e "  ${GREEN}New leader elected on port $NEW_LEADER!${NC}"
    echo -n "  Data still accessible - GET name: "
    curl -s "$BASE_URL:$NEW_LEADER/kv/name"
    echo ""
else
    echo -e "  ${RED}No new leader yet (may need more time)${NC}"
fi

# Restart the stopped node
echo "  Restarting stopped node..."
docker start "raft-node-${LEADER_PORT: -1}" 2>/dev/null || true
sleep 2

echo ""
echo -e "${GREEN}=== Demo complete! ===${NC}"
echo "Cluster is still running. Stop with: scripts/cluster-stop.sh"
