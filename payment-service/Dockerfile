FROM golang:1.21-alpine AS builder
WORKDIR /src

ARG GOPROXY=https://goproxy.io,https://proxy.golang.org
ENV GOPROXY=${GOPROXY}
ENV GOSUMDB=sum.golang.org

RUN apk add --no-cache git ca-certificates

COPY ap2-generated ./ap2-generated
COPY payment-service/go.mod payment-service/go.sum ./payment-service/
WORKDIR /src/payment-service
RUN go mod download

COPY payment-service ./
RUN go build -o /out/payment-server ./cmd/payment-service

FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/payment-server .
EXPOSE 8081 50051
CMD ["./payment-server"]
