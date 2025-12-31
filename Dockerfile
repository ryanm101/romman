# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev make

WORKDIR /src
COPY . .

# Build all binaries using Makefile
RUN make build-all

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache sqlite-libs ca-certificates

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /src/bin/romman /app/
COPY --from=builder /src/bin/romman-tui /app/
COPY --from=builder /src/bin/romman-web /app/

# Create data directories
RUN mkdir -p /data /dats

# Environment
ENV ROMMAN_DB=/data/romman.db
ENV ROMMAN_DAT_DIR=/dats
ENV ROMMAN_PORT=8080

EXPOSE 8080

# Default to web server
CMD ["/app/romman-web"]
