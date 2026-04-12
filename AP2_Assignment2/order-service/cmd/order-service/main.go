package main

import (
	"log"
	"net"
	"os"

	"google.golang.org/grpc"

	orderpb "github.com/YOURUSERNAME/ap2-generated/order"

	"order-service/internal/app"
	"order-service/internal/client"
	"order-service/internal/repository"
	grpchandler "order-service/internal/transport/grpc"
	httphandler "order-service/internal/transport/http"
	"order-service/internal/usecase"

	"github.com/gin-gonic/gin"
)

// main — Composition Root.
//
// Assignment 2 изменения (только здесь и в client/):
//   - client.HTTPPaymentClient → client.GRPCPaymentClient
//   - добавлен gRPC сервер для streaming (GRPC_STREAM_PORT)
//
// UseCase, Repository, HTTP handlers — ИДЕНТИЧНЫ Assignment 1.
func main() {
	cfg := app.Config{
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "order_user"),
		DBPassword:     getEnv("DB_PASSWORD", "order_pass"),
		DBName:         getEnv("DB_NAME", "order_db"),
		ServerPort:     getEnv("SERVER_PORT", "8080"),
		PaymentBaseURL: getEnv("PAYMENT_BASE_URL", "http://localhost:8081"),
	}

	paymentGRPCAddr := getEnv("PAYMENT_GRPC_ADDR", "localhost:50051")
	orderGRPCAddr   := getEnv("GRPC_STREAM_PORT", ":50052")

	// 1. DB (не изменилось)
	db, err := app.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 2. Repository (не изменилось)
	orderRepo := repository.NewPostgresOrderRepository(db)

	// 3. Payment Client — ИЗМЕНЕНИЕ: gRPC вместо HTTP
	paymentClient, err := client.NewGRPCPaymentClient(paymentGRPCAddr)
	if err != nil {
		log.Fatalf("Failed to create gRPC payment client: %v", err)
	}

	// 4. Use Case (не изменилось)
	orderUC := usecase.NewOrderUseCase(orderRepo, paymentClient)

	// 5a. REST (не изменилось — все эндпоинты Assignment 1 работают)
	go func() {
		handler := httphandler.NewOrderHandler(orderUC)
		router := gin.Default()
		handler.RegisterRoutes(router)
		log.Printf("[order-service] REST listening on port %s\n", cfg.ServerPort)
		if err := router.Run(":" + cfg.ServerPort); err != nil {
			log.Fatalf("REST server failed: %v", err)
		}
	}()

	// 5b. gRPC Streaming (НОВОЕ)
	streamHandler := grpchandler.NewOrderGRPCServer(orderRepo)
	grpcServer := grpc.NewServer()
	orderpb.RegisterOrderServiceServer(grpcServer, streamHandler)

	lis, err := net.Listen("tcp", orderGRPCAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", orderGRPCAddr, err)
	}

	log.Printf("[order-service] gRPC streaming listening on %s\n", orderGRPCAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC serve error: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
