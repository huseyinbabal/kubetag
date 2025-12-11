package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/huseyinbabal/kubetag/internal/handler"
	"github.com/huseyinbabal/kubetag/internal/repository"
	"github.com/huseyinbabal/kubetag/internal/service"
	"github.com/huseyinbabal/kubetag/pkg/database"
	"github.com/huseyinbabal/kubetag/pkg/k8s"
)

func main() {
	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database connection
	dbConfig := database.NewConfigFromEnv()
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations
	if err := database.Migrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Initialize repository
	imageRepo := repository.NewImageRepository(db)

	// Initialize service layer
	var imageService *service.ImageService

	// Create informer manager with event handler
	informerManager := k8s.NewInformerManager(k8sClient.GetClientset(), func(event k8s.ImageEvent) {
		if imageService != nil {
			imageService.HandleImageEvent(event)
		}
	})

	// Create service with repository and informer
	imageService = service.NewImageService(imageRepo, informerManager)

	// Start informers
	log.Println("Starting Kubernetes informers...")
	if err := informerManager.Start(ctx); err != nil {
		log.Fatalf("Failed to start informers: %v", err)
	}

	// Initialize handlers
	imageHandler := handler.NewImageHandler(imageService)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:               "KubeTag v2.0.0",
		ReadBufferSize:        16384, // 16KB (default is 4KB)
		DisableStartupMessage: false,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// Serve static files
	app.Static("/", "./web/static")

	// API routes
	api := app.Group("/api")
	api.Get("/images", imageHandler.GetImages)
	api.Get("/images/:name/history", imageHandler.GetImageHistory)
	api.Get("/health", imageHandler.HealthCheck)

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Setup graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down gracefully...")
		cancel() // Cancel context to stop informers

		if err := app.Shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	log.Printf("Starting server on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
