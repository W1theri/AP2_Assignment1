# ── Build stage ───────────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o payment-service ./cmd/payment-service

# ── Run stage ─────────────────────────────────────────────────────────────
FROM alpine:3.18

WORKDIR /app

COPY --from=builder /app/payment-service .

EXPOSE 8081

CMD ["./payment-service"]
