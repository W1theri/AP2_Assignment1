FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy ap2-generated module first (local dependency)
COPY ap2-generated/ ./ap2-generated/

# Copy go mod files
COPY payment-service/go.mod payment-service/go.sum ./
RUN go mod download

# Copy source
COPY payment-service/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /payment-service ./cmd/payment-service

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /payment-service /payment-service
ENTRYPOINT ["/payment-service"]
