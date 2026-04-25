// Version: 1.1.0 - MongoDB integration
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"streaming-ingest/internal/adapters"
	"streaming-ingest/internal/events"
	"streaming-ingest/internal/rabbitmq"
	"streaming-ingest/internal/videos"
	"streaming-ingest/internal/webhooks"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017/streaming"
	}

	// Connect to MongoDB
	mongoClient, err := connectMongo(mongoURI)
	if err != nil {
		log.Fatalf("Could not connect to MongoDB: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := mongoClient.Disconnect(ctx); err != nil {
			log.Printf("MongoDB disconnect error: %v", err)
		}
	}()
	log.Println("Connected to MongoDB successfully.")

	// Retry loop for RabbitMQ connection (since it may start slower in compose)
	var pub *rabbitmq.Publisher
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

	// Instantiate Storage Adapters
	storageAdapters := map[string]adapters.StorageAdapter{
		"minio":  adapters.NewMinioAdapter(),
		"aws-s3": adapters.NewS3Adapter(),
	}

	// Instantiate Repository
	videoRepo := videos.NewMongoRepository(mongoClient, "streaming", "videos")

	// Instantiate Services & Handlers
	eventsService := events.NewService(pub)
	eventsHandler := events.NewHandler(eventsService)

	webhookService := webhooks.NewService(pub, storageAdapters, videoRepo)
	webhookHandler := webhooks.NewHandler(webhookService)

	videosService := videos.NewService(storageAdapters, videoRepo)
	videosHandler := videos.NewHandler(videosService)

	// Routing setup
	v1 := app.Group("/api/v1")
	v1.Post("/events", eventsHandler.ReceiveEvent)
	v1.Post("/webhooks/storage/:provider", webhookHandler.HandleProviderWebhook)
	v1.Get("/videos", videosHandler.ListVideos)

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

func connectMongo(uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, err
	}

	// Verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return client, nil
}
