# ── Build stage ───────────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o order-service ./cmd/order-service

# ── Run stage ─────────────────────────────────────────────────────────────
FROM alpine:3.18

WORKDIR /app

COPY --from=builder /app/order-service .

EXPOSE 8080

CMD ["./order-service"]
