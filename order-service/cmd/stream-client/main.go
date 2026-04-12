// CLI-клиент для тестирования Server-side Streaming.
//
// Запуск:
//   cd order-service
//   ORDER_ID=<uuid> go run ./cmd/stream-client/main.go
//
// В другом терминале меняй статус через REST:
//   curl -X PATCH http://localhost:8080/orders/<uuid>/cancel
//
// Ты увидишь обновление мгновенно в первом терминале.

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/YOURUSERNAME/ap2-generated/order"
)

func main() {
	addr    := getEnv("GRPC_STREAM_PORT", "localhost:50052")
	orderID := mustEnv("ORDER_ID")

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("dial %s: %v", addr, err)
	}
	defer conn.Close()

	c := pb.NewOrderServiceClient(conn)

	// Таймаут 5 минут — достаточно для демонстрации на защите
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	stream, err := c.SubscribeToOrderUpdates(ctx, &pb.OrderRequest{OrderId: orderID})
	if err != nil {
		log.Fatalf("SubscribeToOrderUpdates: %v", err)
	}

	fmt.Printf("✓ Subscribed to order %s\n", orderID)
	fmt.Println("Waiting for real-time status updates from DB...\n")

	for {
		update, err := stream.Recv()
		if err == io.EOF {
			fmt.Println("\nStream closed by server.")
			break
		}
		if err != nil {
			log.Fatalf("stream error: %v", err)
		}

		fmt.Printf("[%s]  order_id=%-36s  status=%s\n",
			update.UpdatedAt.AsTime().Format("15:04:05.000"),
			update.OrderId,
			update.Status,
		)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Required env variable %q is not set", key)
	}
	return v
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
