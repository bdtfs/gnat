#!/bin/bash

trap 'kill 0' SIGINT

echo "Starting gnat services..."
echo ""

APPLICATION_PORT=8778 go run ./cmd/gnat-backend &
BACKEND_PID=$!

sleep 2

FRONTEND_PORT=3000 API_URL=http://localhost:8778 go run ./cmd/gnat-frontend &
FRONTEND_PID=$!

echo ""
echo "✓ Backend running on http://localhost:8778"
echo "✓ Frontend running on http://localhost:3000"
echo ""
echo "Press Ctrl+C to stop both services"

wait