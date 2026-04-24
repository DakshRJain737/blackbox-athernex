#!/bin/bash

# Test script to verify the fleet simulator works
# Run this before demo to ensure everything compiles

echo "🔧 Testing Blackbox Fleet Simulator..."
echo ""

ERRORS=0

# Check Go version
echo "✓ Checking Go installation..."
if ! command -v go &> /dev/null; then
    echo "  ✗ Go not found. Please install Go 1.21+"
    ERRORS=$((ERRORS + 1))
else
    GO_VERSION=$(go version | awk '{print $3}')
    echo "  ✓ Go $GO_VERSION found"
fi

# Check Node version
echo "✓ Checking Node installation..."
if ! command -v node &> /dev/null; then
    echo "  ✗ Node not found. Please install Node 14+"
    ERRORS=$((ERRORS + 1))
else
    NODE_VERSION=$(node --version)
    echo "  ✓ Node $NODE_VERSION found"
fi

# Test Relay compilation
echo "✓ Testing Relay Server..."
cd relay
if go build -o relay relay.go 2>/dev/null; then
    echo "  ✓ Relay compiles successfully"
else
    echo "  ✗ Relay compilation failed"
    ERRORS=$((ERRORS + 1))
fi
cd ..

# Test Simulator compilation
echo "✓ Testing Simulator..."
cd simulator
if go build -o simulator main.go 2>/dev/null; then
    echo "  ✓ Simulator compiles successfully"
else
    echo "  ✗ Simulator compilation failed"
    ERRORS=$((ERRORS + 1))
fi
cd ..

# Test Frontend
echo "✓ Testing Frontend..."
if [ -f "frontend/package.json" ]; then
    echo "  ✓ Frontend package.json exists"
else
    echo "  ✗ Frontend package.json missing"
    ERRORS=$((ERRORS + 1))
fi

echo ""
if [ $ERRORS -eq 0 ]; then
    echo "✨ All checks passed! Ready to run demo."
    echo ""
    echo "Quick start:"
    echo "  ./demo.sh          (Linux/macOS)"
    echo "  demo.bat           (Windows)"
    exit 0
else
    echo "⚠️  $ERRORS issue(s) found. Please fix before running demo."
    exit 1
fi
