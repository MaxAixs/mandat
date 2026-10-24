FROM golang:1.27-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd

FROM alpine:3.21

RUN adduser -D -u 10001 mandat

COPY --from=builder /out/server /app/server

# The example is a complete, secret-free config; it serves as the
# built-in default and can be overridden with a mounted config.yaml.
COPY config.example.yaml /app/config.yaml

USER mandat

EXPOSE 8080

ENTRYPOINT ["/app/server"]
