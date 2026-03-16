# PennyClaw — Multi-stage build for minimal image size
# Target: < 20MB final image

# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w -X main.version=0.1.0 -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o pennyclaw ./cmd/pennyclaw

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates curl sqlite-libs

# Create non-root user
RUN addgroup -S pennyclaw && adduser -S pennyclaw -G pennyclaw

WORKDIR /app
COPY --from=builder /build/pennyclaw .
COPY --from=builder /build/config.example.json ./config.json

RUN mkdir -p /app/data /tmp/pennyclaw-sandbox && \
    chown -R pennyclaw:pennyclaw /app /tmp/pennyclaw-sandbox

USER pennyclaw

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:3000/api/health || exit 1

ENTRYPOINT ["./pennyclaw"]
CMD ["--config", "config.json"]
