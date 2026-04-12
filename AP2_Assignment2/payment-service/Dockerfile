FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o payment-server ./cmd/payment-service

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/payment-server .
EXPOSE 8081 50051
CMD ["./payment-server"]
