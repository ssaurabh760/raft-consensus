# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy dependency files first for better caching.
COPY go.mod ./
RUN go mod download

# Copy source code.
COPY . .

# Build the binary.
RUN CGO_ENABLED=0 GOOS=linux go build -o /raft-node ./cmd/raft/

# Runtime stage — minimal image.
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /raft-node /app/raft-node

# Create data directory for persistence.
RUN mkdir -p /data

EXPOSE 8000 9000

ENTRYPOINT ["/app/raft-node"]
