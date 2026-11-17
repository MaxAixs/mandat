FROM golang:1.27-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd

FROM alpine:3.21

RUN adduser -D -u 10001 mandat

COPY --from=builder /out/server /app/server

# The binary reads config.yaml relative to its working directory.
WORKDIR /app

# The single tracked config; it is secret-free and targets the
# Docker Compose service names.
COPY config.yaml /app/config.yaml

USER mandat

EXPOSE 8080

# Liveness only: readiness (DB) is checked by /health/ready from the orchestrator.
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/health/live || exit 1

ENTRYPOINT ["/app/server"]
