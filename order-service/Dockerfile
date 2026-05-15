FROM golang:1.21-alpine AS builder
WORKDIR /src

# Do not use ",direct" — it falls back to github.com which may be blocked in Docker
ARG GOPROXY=https://goproxy.io,https://proxy.golang.org
ENV GOPROXY=${GOPROXY}
ENV GOSUMDB=sum.golang.org

RUN apk add --no-cache git ca-certificates

COPY ap2-generated ./ap2-generated
COPY order-service/go.mod order-service/go.sum ./order-service/
WORKDIR /src/order-service
RUN go mod download

COPY order-service ./
RUN go build -o /out/order-server ./cmd/order-service

FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/order-server .
EXPOSE 8080 50052
CMD ["./order-server"]
