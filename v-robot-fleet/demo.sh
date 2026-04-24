#!/bin/bash

# Blackbox Fleet Demo - One-Click Startup Script
set -e

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RELAY_DIR="$PROJECT_DIR/relay"
SIMULATOR_DIR="$PROJECT_DIR/simulator"
FRONTEND_DIR="$PROJECT_DIR/frontend"

echo "🚀 Starting Blackbox Fleet Demo..."
echo ""

# Kill any existing processes on exit
cleanup() {
    echo ""
    echo "🛑 Shutting down all services..."
    kill $(jobs -p) 2>/dev/null || true
}
trap cleanup EXIT

# Start the Relay Server
echo "🔄 Starting Relay Server..."
cd "$RELAY_DIR"
go run relay.go &
RELAY_PID=$!
echo "   Relay PID: $RELAY_PID"

# Wait for relay to start
sleep 1

# Start the Robot Fleet Simulator
echo "🤖 Starting Robot Fleet Simulator..."
cd "$SIMULATOR_DIR"
go run main.go &
SIMULATOR_PID=$!
echo "   Simulator PID: $SIMULATOR_PID"

# Start the React Frontend
echo "🎨 Cleaning and reinstalling frontend dependencies..."
cd "$FRONTEND_DIR"
#rm -rf node_modules package-lock.json
#npm install --legacy-peer-deps --no-audit --no-fund

echo "🌐 Starting React Dashboard..."
echo "   Dashboard URL: http://localhost:3000"
echo ""
echo "✨ All services started! Press Ctrl+C to stop everything."
echo ""

npm start -- --no-audit

wait

