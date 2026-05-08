FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files first for layer caching
COPY notification-service/go.mod notification-service/go.sum ./
RUN go mod download

# Copy source
COPY notification-service/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /notification-service ./cmd/notification-service

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /notification-service /notification-service
ENTRYPOINT ["/notification-service"]
