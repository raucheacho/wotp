# ─── Stage 1: Build Dashboard ──────────────────────────────────────
FROM oven/bun:alpine AS dashboard-builder

ARG VERSION=dev

WORKDIR /app/dashboard

# Install dependencies first (cache layer)
COPY dashboard/package.json dashboard/bun.lockb* ./
RUN bun install

# Build static assets
COPY dashboard/ ./
ENV VITE_APP_VERSION=${VERSION}

RUN bun run build

# ─── Stage 2: Build Go Binary ─────────────────────────────────────
FROM golang:alpine AS go-builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Download Go modules first (cache layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy dashboard build output for embedding
COPY --from=dashboard-builder /app/dashboard/dist ./dashboard/dist

# Build the binary with embedded dashboard
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w -extldflags '-static'" \
    -tags netgo \
    -o /wotp-core \
    ./cmd/wotp-core

# ─── Stage 3: Runtime ─────────────────────────────────────────────
FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S wotp \
    && adduser -S wotp -G wotp

WORKDIR /app

# Copy the compiled binary
COPY --from=go-builder /wotp-core /app/wotp-core

# Create data directories
RUN mkdir -p /data/session /data/db \
    && chown -R wotp:wotp /data /app

USER wotp

EXPOSE 54321

VOLUME ["/data/session", "/data/db"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:54321/health || exit 1

ENTRYPOINT ["/app/wotp-core", "-data=/data"]
