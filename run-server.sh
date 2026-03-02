#!/bin/bash
# Quick Start Script for Network Monitoring System

echo "======================================"
echo "Building Backend Server with SQLite"
echo "======================================"

cd backend

# Download dependencies
echo ""
echo "Step 1: Downloading Go dependencies..."
go mod download

# Build server
echo ""
echo "Step 2: Building server..."
go build -o server main.go database.go

if [ $? -ne 0 ]; then
  echo "❌ Build failed!"
  exit 1
fi

echo ""
echo "✅ Build successful!"
echo ""
echo "======================================"
echo "Starting Server"
echo "======================================"
echo ""
./server
