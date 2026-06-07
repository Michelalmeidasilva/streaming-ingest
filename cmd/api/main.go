// Version: 1.1.0 - MongoDB integration
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"streaming-ingest/internal/adapters"
	"streaming-ingest/internal/events"
	"streaming-ingest/internal/mongoindex"
	"streaming-ingest/internal/rabbitmq"
	"streaming-ingest/internal/telemetry"
	"streaming-ingest/internal/uploadstate"
	"streaming-ingest/internal/videos"
	"streaming-ingest/internal/webhooks"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type shutdowner interface {
	Shutdown() error
}

func main() {
	if err := loadDotEnv(".env"); err != nil {
		log.Printf("WARNING: could not load .env: %v", err)
	}

	app := newApp()

	rabbitMQURL := requireEnv("RABBITMQ_URL")
	mongoURI := requireEnv("MONGODB_URI")

	mongoClient, err := retry(10, 2*time.Second, func() (*mongo.Client, error) {
		return connectMongo(mongoURI)
	}, func(attempt, max int, err error) {
		log.Printf("Failed to connect to MongoDB, retrying in 2 seconds... (%d/%d): %v", attempt, max, err)
	})
	if err != nil {
		log.Fatalf("Could not connect to MongoDB at %s after retries. Start the infra stack with `cd ../infra && make up`: %v", redactMongoURI(mongoURI), err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := mongoClient.Disconnect(ctx); err != nil {
			log.Printf("MongoDB disconnect error: %v", err)
		}
	}()
	log.Println("Connected to MongoDB successfully.")
	ensureMongoIndexes(mongoClient, "streaming")
	// The videos service (events `upload.started`, storage webhooks, catalog
	// handler) owns its own collection. The upload-state store keeps the canonical
	// per-video lifecycle document (thumbnail/transcode/playback) in `videos`.
	// Sharing one collection produced duplicate documents per video_id: events did
	// an unconditional InsertOne alongside the upload-state upsert.
	videoRepo := videos.NewMongoRepository(mongoClient, "streaming", "videos_catalog")
	eventRepo := events.NewMongoRepository(mongoClient, "streaming", "events")
	uploadStateRepo := uploadstate.NewMongoRepository(mongoClient, "streaming", "upload_sessions", "videos")

	pub, err := retry(10, 2*time.Second, func() (*rabbitmq.Publisher, error) {
		return rabbitmq.NewPublisher(rabbitMQURL)
	}, func(attempt, max int, err error) {
		log.Printf("Failed to connect to RabbitMQ, retrying in 2 seconds... (%d/%d): %v", attempt, max, err)
	})
	if err != nil {
		log.Fatalf("Could not connect to RabbitMQ after retries: %v", err)
	}
	defer pub.Close()
	log.Println("Connected to RabbitMQ successfully.")

	storageAdapters := createStorageAdapters()

	// Instantiate Services & Handlers
	eventsService := events.NewService(pub, eventRepo, videoRepo)
	eventsHandler := events.NewHandler(eventsService)

	uploadStateService := uploadstate.NewService(uploadStateRepo)
	uploadStateHandler := uploadstate.NewHandler(uploadStateService)

	webhookService := webhooks.NewService(pub, storageAdapters, videoRepo, uploadStateService)
	webhookHandler := webhooks.NewHandler(webhookService)

	videosService := videos.NewService(storageAdapters, videoRepo)
	videosHandler := videos.NewHandler(videosService)

	registerRoutes(app, eventsHandler, webhookHandler, videosHandler, uploadStateHandler)

	installGracefulShutdown(app, nil)

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Event Gateway starting on port %s...", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func newApp() *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})
	app.Use(logger.New())
	app.Use(telemetry.New("streaming-ingest").Middleware())
	return app
}

func requireEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s is required", key)
	}
	return value
}

func registerRoutes(app *fiber.App, eventsHandler *events.Handler, webhookHandler *webhooks.Handler, videosHandler *videos.Handler, uploadStateHandler *uploadstate.Handler) {
	v1 := app.Group("/api/v1")
	v1.Post("/events", eventsHandler.ReceiveEvent)
	v1.Post("/webhooks/storage/:provider", webhookHandler.HandleProviderWebhook)
	v1.Get("/videos", videosHandler.ListVideos)
	v1.Get("/videos/database", videosHandler.ListDatabaseVideos)
	v1.Get("/videos/search", videosHandler.SearchVideos)
	v1.Put("/upload-state/sessions/:sessionId", uploadStateHandler.SaveState)
	v1.Get("/upload-state/sessions/:sessionId", uploadStateHandler.GetState)
	v1.Delete("/upload-state/sessions/:sessionId", uploadStateHandler.DeleteSession)
	v1.Put("/upload-state/videos/:videoId", uploadStateHandler.SaveVideo)
	v1.Get("/upload-state/videos", uploadStateHandler.ListVideos)
	v1.Get("/upload-state/videos/:videoId", uploadStateHandler.GetVideo)
	v1.Patch("/upload-state/videos/:videoId", uploadStateHandler.PatchVideo)
	v1.Delete("/upload-state/videos/:videoId", uploadStateHandler.DeleteVideo)
}

func createStorageAdapters() map[string]adapters.StorageAdapter {
	return map[string]adapters.StorageAdapter{
		"minio":  adapters.NewMinioAdapter(),
		"aws-s3": adapters.NewS3Adapter(),
	}
}

func installGracefulShutdown(app shutdowner, signals <-chan os.Signal) chan os.Signal {
	quit := make(chan os.Signal, 1)
	if signals == nil {
		signal.Notify(quit, os.Interrupt)
		signals = quit
	}

	go func() {
		<-signals
		log.Println("Gracefully shutting down server...")
		if err := app.Shutdown(); err != nil {
			log.Fatalf("Server shutdown error: %v", err)
		}
	}()

	return quit
}

func retry[T any](attempts int, delay time.Duration, operation func() (T, error), onRetry func(attempt, max int, err error)) (T, error) {
	var zero T
	if attempts <= 0 {
		return zero, errors.New("attempts must be greater than zero")
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		value, err := operation()
		if err == nil {
			return value, nil
		}

		lastErr = err
		if attempt < attempts {
			if onRetry != nil {
				onRetry(attempt, attempts, err)
			}
			time.Sleep(delay)
		}
	}

	return zero, lastErr
}

// ensureMongoIndexes creates the indexes the gateway relies on. Failures are
// logged but non-fatal: a missing index degrades to a slower scan, not an
// outage (and a unique-index conflict from pre-existing duplicates must not
// block startup).
func ensureMongoIndexes(client *mongo.Client, dbName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := client.Database(dbName)
	err := mongoindex.EnsureIndexes(ctx, func(name string) mongoindex.IndexManager {
		return db.Collection(name).Indexes()
	})
	if err != nil {
		log.Printf("WARNING: could not ensure MongoDB indexes (continuing): %v", err)
		return
	}
	log.Println("MongoDB indexes ensured.")
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

func isMongoAuthError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "authentication failed") ||
		strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "sasl conversation error") ||
		strings.Contains(msg, "atlaserror") && strings.Contains(msg, "bad auth")
}

func redactMongoURI(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "<redacted-mongodb-uri>"
	}

	if parsed.User != nil {
		username := parsed.User.Username()
		if username != "" {
			parsed.User = url.User(username)
		} else {
			parsed.User = nil
		}
	}

	return parsed.String()
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
