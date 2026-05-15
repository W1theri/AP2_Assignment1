package main

import (
	"log"
	"net"
	"os"
	"strconv"

	"google.golang.org/grpc"

	_ "github.com/W1theri/ap2-generated/codec/jsoncodec"
	orderpb "github.com/W1theri/ap2-generated/order"

	"order-service/internal/app"
	"order-service/internal/cache"
	"order-service/internal/client"
	"order-service/internal/middleware"
	"order-service/internal/repository"
	grpchandler "order-service/internal/transport/grpc"
	httphandler "order-service/internal/transport/http"
	"order-service/internal/usecase"

	"github.com/gin-gonic/gin"
)

// main — Composition Root.
//
// Assignment 4 changes:
//   - Added Redis cache for cache-aside pattern
//   - Added rate limiter middleware
//   - Added cache invalidation on order status changes
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
	orderGRPCAddr := getEnv("GRPC_STREAM_PORT", ":50052")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	cacheTTLSeconds := getEnvInt("CACHE_TTL", 300)
	rateLimitRequests := getEnvInt("RATE_LIMIT_REQUESTS", 10)
	rateLimitWindow := getEnvInt("RATE_LIMIT_WINDOW", 60)

	// 1. DB (не изменилось)
	db, err := app.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 2. Redis Cache (НОВОЕ для Assignment 4)
	var cacheService cache.Cache
	redisCache, err := cache.NewRedisCache(redisURL)
	if err != nil {
		log.Printf("Warning: Failed to initialize Redis cache: %v. Using no-op cache.\n", err)
		cacheService = &cache.NoOpCache{}
	} else {
		defer redisCache.Close()
		cacheService = redisCache
	}

	// 3. Repository (now with cache support)
	orderRepo := repository.NewPostgresOrderRepository(db)
	cachedRepo := repository.NewCachedOrderRepository(orderRepo, cacheService, cacheTTLSeconds)

	// 4. Payment Client
	paymentClient, err := client.NewGRPCPaymentClient(paymentGRPCAddr)
	if err != nil {
		log.Fatalf("Failed to create gRPC payment client: %v", err)
	}

	// 5. Use Case
	orderUC := usecase.NewOrderUseCase(cachedRepo, paymentClient)

	// 6a. REST with Rate Limiter (ИЗМЕНЕНО)
	go func() {
		handler := httphandler.NewOrderHandler(orderUC)
		router := gin.Default()

		// Add rate limiter middleware (НОВОЕ)
		rateLimiter := middleware.NewRateLimiter(cacheService, rateLimitRequests, rateLimitWindow)
		router.Use(rateLimiter.Middleware())

		handler.RegisterRoutes(router)
		log.Printf("[order-service] REST listening on port %s\n", cfg.ServerPort)
		if err := router.Run(":" + cfg.ServerPort); err != nil {
			log.Fatalf("REST server failed: %v", err)
		}
	}()

	// 6b. gRPC Streaming (uses DB repo directly — WatchOrderStatus is not cached)
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

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
