#!/bin/bash
set -e

echo "=================================="
echo "dotidx Docker Quick Start"
echo "=================================="
echo ""

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed. Please install Docker first."
    exit 1
fi

# Check if docker-compose is installed
if command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
elif docker compose version &> /dev/null; then
    COMPOSE_CMD="docker compose"
else
    echo "Warning: docker-compose not found, using plain docker commands"
    COMPOSE_CMD=""
fi

# Determine project root
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"
cd "$PROJECT_ROOT"

echo "Project directory: $PROJECT_ROOT"
echo ""

# Ask for Sidecar configuration
echo "Enter external Sidecar configuration (press Enter for defaults):"
read -p "Sidecar host [host.docker.internal]: " SIDECAR_HOST
SIDECAR_HOST=${SIDECAR_HOST:-host.docker.internal}

read -p "Sidecar port [10800]: " SIDECAR_PORT
SIDECAR_PORT=${SIDECAR_PORT:-10800}

echo ""
echo "Configuration:"
echo "  Sidecar: http://$SIDECAR_HOST:$SIDECAR_PORT"
echo ""

# Update config file
CONFIG_FILE="conf/conf-docker-test.toml"
if [ -f "$CONFIG_FILE" ]; then
    echo "Updating $CONFIG_FILE..."
    # Simple sed replacements (works on both macOS and Linux)
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s|relay_ip = \".*\"|relay_ip = \"$SIDECAR_HOST\"|g" "$CONFIG_FILE"
        sed -i '' "s|chainreader_ip = \".*\"|chainreader_ip = \"$SIDECAR_HOST\"|g" "$CONFIG_FILE"
        sed -i '' "s|sidecar_ip = \".*\"|sidecar_ip = \"$SIDECAR_HOST\"|g" "$CONFIG_FILE"
        sed -i '' "s|chainreader_port = [0-9]*|chainreader_port = $SIDECAR_PORT|g" "$CONFIG_FILE"
        sed -i '' "s|sidecar_port = [0-9]*|sidecar_port = $SIDECAR_PORT|g" "$CONFIG_FILE"
    else
        sed -i "s|relay_ip = \".*\"|relay_ip = \"$SIDECAR_HOST\"|g" "$CONFIG_FILE"
        sed -i "s|chainreader_ip = \".*\"|chainreader_ip = \"$SIDECAR_HOST\"|g" "$CONFIG_FILE"
        sed -i "s|sidecar_ip = \".*\"|sidecar_ip = \"$SIDECAR_HOST\"|g" "$CONFIG_FILE"
        sed -i "s|chainreader_port = [0-9]*|chainreader_port = $SIDECAR_PORT|g" "$CONFIG_FILE"
        sed -i "s|sidecar_port = [0-9]*|sidecar_port = $SIDECAR_PORT|g" "$CONFIG_FILE"
    fi
fi

echo ""
echo "Building Docker image..."
echo "This may take 10-15 minutes on first run..."
echo ""

if [ -n "$COMPOSE_CMD" ]; then
    $COMPOSE_CMD build
    echo ""
    echo "Starting container with docker-compose..."
    $COMPOSE_CMD up -d
    CONTAINER_NAME="dotidx-test"
else
    docker build -t dotidx:latest .
    echo ""
    echo "Starting container..."
    docker run -d \
        --name dotidx-test \
        -p 8080:8080 \
        dotidx:latest
    CONTAINER_NAME="dotidx-test"
fi

echo ""
echo "Waiting for container to start..."
sleep 5

echo ""
echo "=================================="
echo "dotidx is starting!"
echo "=================================="
echo ""
echo "Container logs:"
docker logs $CONTAINER_NAME
echo ""
echo "=================================="
echo ""
echo "Access the frontend at: http://localhost:8080"
echo ""
echo "To view logs:"
echo "  docker logs -f $CONTAINER_NAME"
echo ""
echo "To index blocks:"
echo "  docker exec -it $CONTAINER_NAME su - dotidx -c \\"
echo "    \"/dotidx/bin/dixbatch \\"
echo "      -database postgres://dotidx:testpassword@localhost:5432/dotidx \\"
echo "      -chainreader http://$SIDECAR_HOST:$SIDECAR_PORT \\"
echo "      -relaychain polkadot \\"
echo "      -chain polkadot \\"
echo "      -start 1 -end 1000\""
echo ""
echo "To stop:"
if [ -n "$COMPOSE_CMD" ]; then
    echo "  $COMPOSE_CMD down"
else
    echo "  docker stop $CONTAINER_NAME"
    echo "  docker rm $CONTAINER_NAME"
fi
echo ""
