package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"streaming-ingest/internal/events"
	"streaming-ingest/internal/rabbitmq"
	"streaming-ingest/internal/webhooks"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	// Initialize Fiber
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(logger.New())

	// Read Configs
	rabbitMQURL := os.Getenv("RABBITMQ_URL")
	if rabbitMQURL == "" {
		rabbitMQURL = "amqp://guest:guest@localhost:5672/"
	}

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	// Retry loop for RabbitMQ connection (since it may start slower in compose)
	var pub *rabbitmq.Publisher
	var err error
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		pub, err = rabbitmq.NewPublisher(rabbitMQURL)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to RabbitMQ, retrying in 2 seconds... (%d/%d): %v", i+1, maxRetries, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Could not connect to RabbitMQ after retries: %v", err)
	}
	defer pub.Close()
	log.Println("Connected to RabbitMQ successfully.")

	// Instantiate Services & Handlers
	eventsService := events.NewService(pub)
	eventsHandler := events.NewHandler(eventsService)

	webhookService := webhooks.NewService(pub)
	webhookHandler := webhooks.NewHandler(webhookService)

	// Routing setup
	v1 := app.Group("/api/v1")
	v1.Post("/events", eventsHandler.ReceiveEvent)
	v1.Post("/webhooks/storage/:provider", webhookHandler.HandleProviderWebhook)

	// Graceful Shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit
		log.Println("Gracefully shutting down server...")
		if err := app.Shutdown(); err != nil {
			log.Fatalf("Server shutdown error: %v", err)
		}
	}()

	log.Printf("Event Gateway starting on port %s...", serverPort)
	if err := app.Listen(":" + serverPort); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
