# Phantom C2 — Docker Deployment
# Usage:
#   docker build -t phantom-c2 .
#   docker run -it --rm -p 8080:8080 -p 443:443 -p 3000:3000 phantom-c2

# ── Build Stage ──
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN make server 2>/dev/null || go build -ldflags "-s -w" -o build/phantom-server ./cmd/server

# ── Runtime Stage ──
FROM alpine:3.19

RUN apk add --no-cache ca-certificates openssl

WORKDIR /phantom

# Copy binary
COPY --from=builder /src/build/phantom-server /phantom/phantom-server

# Copy configs and scripts
COPY configs/ /phantom/configs/
COPY scripts/ /phantom/scripts/

# Create data and build directories
RUN mkdir -p data build/agents build/payloads logs reports

# Generate RSA keys on first run (if not mounted)
RUN if [ ! -f configs/server.key ]; then \
      openssl genrsa -out configs/server.key 2048 2>/dev/null && \
      openssl rsa -in configs/server.key -pubout -out configs/server.pub 2>/dev/null; \
    fi

# Generate TLS certs
RUN openssl req -x509 -newkey rsa:2048 \
      -keyout configs/server-tls.key \
      -out configs/server.crt \
      -days 365 -nodes \
      -subj "/C=US/ST=State/L=City/O=Phantom/CN=phantom.local" 2>/dev/null

# Expose ports
EXPOSE 443 8080 53/udp 3000

# Health check
HEALTHCHECK --interval=30s --timeout=5s \
  CMD wget -q --spider http://localhost:8080/api/v1/health || exit 1

ENTRYPOINT ["/phantom/phantom-server"]
CMD ["--config", "configs/server.yaml"]
