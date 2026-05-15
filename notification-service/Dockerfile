FROM golang:1.21-alpine AS builder
WORKDIR /src

ARG GOPROXY=https://goproxy.io,https://proxy.golang.org
ENV GOPROXY=${GOPROXY}
ENV GOSUMDB=sum.golang.org

RUN apk add --no-cache git ca-certificates

COPY notification-service/go.mod notification-service/go.sum ./notification-service/
WORKDIR /src/notification-service
RUN go mod download

COPY notification-service ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/notification-service ./cmd/notification-service

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/notification-service /notification-service
ENTRYPOINT ["/notification-service"]
