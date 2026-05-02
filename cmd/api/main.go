// Version: 1.1.0 - MongoDB integration
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
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
	if err := loadDotEnv(".env"); err != nil {
		log.Printf("WARNING: could not load .env: %v", err)
	}

	// Initialize Fiber
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(logger.New())

	// Read Configs
	rabbitMQURL := os.Getenv("RABBITMQ_URL")
	if rabbitMQURL == "" {
		log.Fatal("RABBITMQ_URL is required")
	}

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		log.Fatal("MONGODB_URI is required")
	}

	var mongoClient *mongo.Client
	var err error
	maxMongoRetries := 10
	for i := 0; i < maxMongoRetries; i++ {
		mongoClient, err = connectMongo(mongoURI)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to MongoDB, retrying in 2 seconds... (%d/%d): %v", i+1, maxMongoRetries, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Could not connect to MongoDB at %s after retries. Start the infra stack with `cd ../infra && make up`: %v", mongoURI, err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := mongoClient.Disconnect(ctx); err != nil {
			log.Printf("MongoDB disconnect error: %v", err)
		}
	}()
	log.Println("Connected to MongoDB successfully.")
	videoRepo := videos.NewMongoRepository(mongoClient, "streaming", "videos")

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
	v1.Get("/videos/search", videosHandler.SearchVideos)

	// Graceful Shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt)
		<-quit
		log.Println("Gracefully shutting down server...")
		if err := app.Shutdown(); err != nil {
			log.Fatalf("Server shutdown error: %v", err)
		}
	}()

	log.Println("Event Gateway starting on port 8080...")
	if err := app.Listen(":8080"); err != nil {
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

func loadDotEnv(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	var lineNum int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return fmt.Errorf("%s:%d: expected KEY=VALUE", filename, lineNum)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return fmt.Errorf("%s:%d: empty key", filename, lineNum)
		}

		value = strings.Trim(value, `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("%s:%d: set %s: %w", filename, lineNum, key, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%s: %w", filename, err)
	}

	return nil
}
