# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o cdnsbl .

# Final stage - distroless
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/cdnsbl /app/cdnsbl

# Copy .env.example as reference (actual .env should be mounted)
COPY --from=builder /build/.env.example /app/.env.example

# Set data directory for Docker
ENV DATA_DIR=/data

# Create data directory for databases
USER nonroot:nonroot

# Expose DNS port
EXPOSE 53/udp
EXPOSE 5353/udp

# Volume for persistent database storage
VOLUME ["/data"]

ENTRYPOINT ["/app/cdnsbl"]
