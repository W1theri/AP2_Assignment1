FROM golang:1.21-alpine AS builder
WORKDIR /src
ENV GOPROXY=https://goproxy.io,direct
RUN apk add --no-cache git ca-certificates
COPY ap2-generated ./ap2-generated
COPY payment-service ./payment-service
WORKDIR /src/payment-service
RUN go mod download
RUN go build -o /out/payment-server ./cmd/payment-service

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /out/payment-server .
EXPOSE 8081 50051
CMD ["./payment-server"]
