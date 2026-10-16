#!/bin/bash
set -e

# Default ports
export GO_PORT=${GO_PORT:-8081}
export PORT=${PORT:-8000}
export GO_SERVICE_URL="http://127.0.0.1:${GO_PORT}"

# Resolve INTERNAL_API_SECRET from environment or container .env.
# If unset, generate an in-memory 256-bit cryptographic hash with low memory footprint.
if [ -z "$INTERNAL_API_SECRET" ]; then
    if [ -f "/app/.env" ]; then
        INTERNAL_API_SECRET=$(grep -E '^INTERNAL_API_SECRET=' /app/.env | cut -d '=' -f2- | tr -d '"'\'' ')
    fi
fi

if [ -z "$INTERNAL_API_SECRET" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
        export INTERNAL_API_SECRET=$(head -c 32 /dev/urandom | sha256sum | awk '{print $1}')
    else
        export INTERNAL_API_SECRET=$(python3 -c "import secrets; print(secrets.token_hex(32))")
    fi
    echo "[SECURITY] Generated ephemeral 256-bit INTERNAL_API_SECRET for container processes."
else
    export INTERNAL_API_SECRET
fi

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

