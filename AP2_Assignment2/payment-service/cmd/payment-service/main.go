package main

import (
	"log"
	"net"
	"os"

	"google.golang.org/grpc"

	pb "github.com/YOURUSERNAME/ap2-generated/payment"

	"payment-service/internal/app"
	"payment-service/internal/interceptor"
	"payment-service/internal/repository"
	grpchandler "payment-service/internal/transport/grpc"
	httphandler "payment-service/internal/transport/http"
	"payment-service/internal/usecase"

	"github.com/gin-gonic/gin"
)

// main — Composition Root.
// Теперь запускает gRPC сервер для inter-service коммуникации.
// HTTP сервер сохранён для обратной совместимости.
func main() {
	cfg := app.Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5433"),
		DBUser:     getEnv("DB_USER", "payment_user"),
		DBPassword: getEnv("DB_PASSWORD", "payment_pass"),
		DBName:     getEnv("DB_NAME", "payment_db"),
		ServerPort: getEnv("SERVER_PORT", "8081"),
	}
	grpcAddr := getEnv("GRPC_PORT", ":50051")

	// 1. Infrastructure: DB (не изменилось)
	db, err := app.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 2. Repository (не изменилось)
	paymentRepo := repository.NewPostgresPaymentRepository(db)

	// 3. Use Case (не изменилось — та же бизнес-логика из Assignment 1)
	paymentUC := usecase.NewPaymentUseCase(paymentRepo)

	// 4a. gRPC Delivery (НОВОЕ — заменяет HTTP для inter-service вызовов)
	grpcHandler := grpchandler.NewPaymentGRPCServer(paymentUC)

	grpcServer := grpc.NewServer(
		// БОНУС: logging interceptor (+10%)
		grpc.UnaryInterceptor(interceptor.LoggingUnaryInterceptor),
	)
	pb.RegisterPaymentServiceServer(grpcServer, grpcHandler)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", grpcAddr, err)
	}

	// 4b. HTTP Delivery (сохранён из Assignment 1)
	go func() {
		httpHandler := httphandler.NewPaymentHandler(paymentUC)
		router := gin.Default()
		httpHandler.RegisterRoutes(router)
		log.Printf("[payment-service] HTTP still available on port %s (legacy)\n", cfg.ServerPort)
		if err := router.Run(":" + cfg.ServerPort); err != nil {
			log.Printf("[payment-service] HTTP server stopped: %v", err)
		}
	}()

	log.Printf("[payment-service] gRPC server listening on %s\n", grpcAddr)
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
