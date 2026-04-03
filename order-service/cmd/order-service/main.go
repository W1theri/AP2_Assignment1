package main

import (
	"log"
	"os"

	"order-service/internal/app"
	"order-service/internal/client"
	"order-service/internal/repository"
	"order-service/internal/transport/http"
	"order-service/internal/usecase"

	"github.com/gin-gonic/gin"
)

// main is the Composition Root.
// All concrete types are instantiated and wired here — no DI framework needed.
// The use case and handlers only ever see interfaces (Ports).
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

	// 1. Infrastructure: open DB connection
	db, err := app.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 2. Repository Adapter (implements usecase.OrderRepository Port)
	orderRepo := repository.NewPostgresOrderRepository(db)

	// 3. Payment Client Adapter (implements usecase.PaymentClient Port)
	//    A single shared http.Client is created here at the Composition Root.
	//    The 2-second timeout is enforced inside the client adapter.
	paymentClient := client.NewHTTPPaymentClient(cfg.PaymentBaseURL)

	// 4. Use Case (receives Ports, knows nothing about concrete adapters)
	orderUC := usecase.NewOrderUseCase(orderRepo, paymentClient)

	// 5. Delivery Handler (receives Use Case, knows nothing about repositories or HTTP clients)
	handler := http.NewOrderHandler(orderUC)

	// 6. Router
	router := gin.Default()
	handler.RegisterRoutes(router)

	log.Printf("[order-service] Starting on port %s\n", cfg.ServerPort)
	if err := router.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
