FROM golang:1.21-alpine AS builder
WORKDIR /src
ENV GOPROXY=https://goproxy.io,direct
RUN apk add --no-cache git ca-certificates
COPY ap2-generated ./ap2-generated
COPY order-service ./order-service
WORKDIR /src/order-service
RUN go mod download
RUN go build -o /out/order-server ./cmd/order-service

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /out/order-server .
EXPOSE 8080 50052
CMD ["./order-server"]
