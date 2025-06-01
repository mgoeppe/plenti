FROM golang:1.24-alpine as builder

# Install build dependencies for SQLite with CGO
RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY . .

ENV CGO_ENABLED=1
RUN go build -o plenti

FROM alpine:3.17

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/plenti /app/plenti

# Create directories for config and data
RUN mkdir -p /config /data

# Set volumes for configuration and data
VOLUME /config
VOLUME /data

# Set entrypoint to use config from the specified location
ENTRYPOINT ["/bin/sh", "-c", "/app/plenti save --config-path=/config"]
