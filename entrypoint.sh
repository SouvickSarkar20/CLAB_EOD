#!/bin/bash
set -e

# Default ports
export GO_PORT=${GO_PORT:-8081}
export PORT=${PORT:-8000}
export GO_SERVICE_URL="http://127.0.0.1:${GO_PORT}"
export INTERNAL_API_SECRET="${INTERNAL_API_SECRET:-clab-internal-secret-change-in-production-2026}"

echo "=========================================="
echo " Starting Unified Deployment Container"
echo " - Go DB Engine on port: ${GO_PORT}"
echo " - Python FastAPI on port: ${PORT}"
echo "=========================================="

# 1. Start Go DB microservice in the background
echo "[LAUNCH] Starting Go DB Microservice..."
PORT=${GO_PORT} /app/go-server &
GO_PID=$!

# Trap signals to shut down both processes gracefully
trap 'kill -TERM $GO_PID 2>/dev/null; exit 0' SIGTERM SIGINT

# 2. Wait briefly for Go service to bind
sleep 2

# 3. Start Python FastAPI in the foreground
echo "[LAUNCH] Starting Python FastAPI Gateway..."
exec uvicorn app.main:app --host 0.0.0.0 --port "${PORT}" --access-log

