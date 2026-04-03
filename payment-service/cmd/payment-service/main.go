package main

import (
	"log"
	"os"

	"payment-service/internal/app"
	"payment-service/internal/repository"
	"payment-service/internal/transport/http"
	"payment-service/internal/usecase"

	"github.com/gin-gonic/gin"
)

// main is the Composition Root.
// All dependencies are wired manually here — no DI framework.
// This is the only place that knows about concrete types.
func main() {
	cfg := app.Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5433"),
		DBUser:     getEnv("DB_USER", "payment_user"),
		DBPassword: getEnv("DB_PASSWORD", "payment_pass"),
		DBName:     getEnv("DB_NAME", "payment_db"),
		ServerPort: getEnv("SERVER_PORT", "8081"),
	}

	// 1. Infrastructure: open DB connection
	db, err := app.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 2. Repository (Adapter — implements the Port interface)
	paymentRepo := repository.NewPostgresPaymentRepository(db)

	// 3. Use Case (depends only on the Port interface)
	paymentUC := usecase.NewPaymentUseCase(paymentRepo)

	// 4. Delivery (Handler depends on Use Case)
	handler := http.NewPaymentHandler(paymentUC)

	// 5. HTTP Router
	router := gin.Default()
	handler.RegisterRoutes(router)

	log.Printf("[payment-service] Starting on port %s\n", cfg.ServerPort)
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
