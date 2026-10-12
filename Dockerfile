# ==============================================================================
# STAGE 1: Build the Go Microservice
# ==============================================================================
FROM golang:1.24-alpine AS go-builder

WORKDIR /build

# Copy Go files and download dependencies
COPY go-service/ ./
RUN go mod download

# Compile statically linked standalone Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /go-server ./cmd/server/main.go

# ==============================================================================
# STAGE 2: Python FastAPI Runtime & Unified Deployment Container
# ==============================================================================
FROM python:3.10-slim

WORKDIR /app

# Install system utilities needed for runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies
COPY backend/requirements.txt ./
RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir -r requirements.txt

# Copy Python backend application code
COPY backend/ ./

# Copy compiled Go binary from Stage 1
COPY --from=go-builder /go-server /app/go-server

# Copy startup script
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh /app/go-server

# Default environment configuration
ENV PYTHONUNBUFFERED=1 \
    PORT=8000 \
    GO_PORT=8081 \
    GO_SERVICE_URL=http://127.0.0.1:8081

EXPOSE 8000 8081

ENTRYPOINT ["/app/entrypoint.sh"]

