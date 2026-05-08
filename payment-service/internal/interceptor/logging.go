package interceptor

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
)

// LoggingUnaryInterceptor логирует каждый входящий RPC вызов.
// Бонусное задание (+10%): middleware для Payment Service.
// Выводит: имя метода и длительность выполнения.
func LoggingUnaryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()

	resp, err := handler(ctx, req)

	log.Printf("[gRPC interceptor] method=%s  duration=%s  err=%v",
		info.FullMethod,
		time.Since(start).Round(time.Microsecond),
		err,
	)

	return resp, err
}
