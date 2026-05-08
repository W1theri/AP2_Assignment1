package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	_ "github.com/W1theri/ap2-generated/codec/jsoncodec"
	pb "github.com/W1theri/ap2-generated/payment"

	"payment-service/internal/app"
	"payment-service/internal/interceptor"
	"payment-service/internal/messaging"
	"payment-service/internal/repository"
	grpchandler "payment-service/internal/transport/grpc"
	httphandler "payment-service/internal/transport/http"
	"payment-service/internal/usecase"

	"github.com/gin-gonic/gin"
)

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
	amqpURL := getEnv("AMQP_URL", "amqp://guest:guest@localhost:5672/")

	// 1. Infrastructure: DB
	db, err := app.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 2. Repository
	paymentRepo := repository.NewPostgresPaymentRepository(db)

	// 3. RabbitMQ Publisher (with retry)
	var publisher *messaging.RabbitMQPublisher
	for attempt := 1; attempt <= 10; attempt++ {
		publisher, err = messaging.NewRabbitMQPublisher(amqpURL)
		if err == nil {
			break
		}
		log.Printf("[Payment] RabbitMQ not ready (attempt %d/10): %v. Retrying in 3s...", attempt, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("[Payment] Could not connect to RabbitMQ: %v", err)
	}
	defer publisher.Close()

	// 4. Use Case (injected with both repo and publisher)
	paymentUC := usecase.NewPaymentUseCase(paymentRepo, publisher)

	// 5a. gRPC Delivery
	grpcHandler := grpchandler.NewPaymentGRPCServer(paymentUC)
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(interceptor.LoggingUnaryInterceptor),
	)
	pb.RegisterPaymentServiceServer(grpcServer, grpcHandler)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", grpcAddr, err)
	}

	// 5b. HTTP Delivery (legacy)
	router := gin.Default()
	httphandler.NewPaymentHandler(paymentUC).RegisterRoutes(router)
	httpServer := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	// Start gRPC
	go func() {
		log.Printf("[Payment] gRPC server listening on %s", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("[Payment] gRPC server stopped: %v", err)
		}
	}()

	// Start HTTP
	go func() {
		log.Printf("[Payment] HTTP server listening on port %s (legacy)", cfg.ServerPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Payment] HTTP server stopped: %v", err)
		}
	}()

	// --- Graceful Shutdown (Lecture 5: Lifecycle) ---
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("[Payment] Received signal %v — initiating graceful shutdown...", sig)

	grpcServer.GracefulStop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("[Payment] HTTP server shutdown error: %v", err)
	}

	log.Println("[Payment] Service stopped gracefully.")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
