package main

import (
	"log/slog"
	"os"

	"vibespace/pkg/handler"
	"vibespace/pkg/k8s"
	"vibespace/pkg/vibespace"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		slog.Debug("no .env file found, using environment variables")
	}

	slog.Info("initializing vibespaces api server",
		"port", getPort())

	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient()
	if err != nil {
		slog.Warn("kubernetes client initialization failed, running in limited mode",
			"error", err)
		// Don't fatal - allow API to run for development
	} else {
		slog.Info("kubernetes client initialized successfully")
	}

	// Initialize services
	vibespaceService := vibespace.NewService(k8sClient)

	// Initialize handlers
	vibespaceHandler := handler.NewVibespaceHandler(vibespaceService)
	clusterHandler := handler.NewClusterHandler(k8sClient)

	// Initialize Gin router
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// API routes
	v1 := r.Group("/api/v1")
	{
		// Health check
		v1.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status": "healthy",
				"k8s":    k8sClient != nil,
			})
		})

		// Vibespaces
		vibespaces := v1.Group("/vibespaces")
		{
			vibespaces.GET("", vibespaceHandler.List)
			vibespaces.POST("", vibespaceHandler.Create)
			vibespaces.GET("/:id", vibespaceHandler.Get)
			vibespaces.DELETE("/:id", vibespaceHandler.Delete)
			vibespaces.POST("/:id/start", vibespaceHandler.Start)
			vibespaces.POST("/:id/stop", vibespaceHandler.Stop)
			vibespaces.GET("/:id/access", vibespaceHandler.Access)

			// Dynamic service registration (called by port detector in container)
			vibespaces.POST("/:id/services", vibespaceHandler.RegisterService)
			vibespaces.DELETE("/:id/services/:port", vibespaceHandler.UnregisterService)
			vibespaces.GET("/:id/services/:port", vibespaceHandler.GetServiceURL)
		}

		// Cluster
		cluster := v1.Group("/cluster")
		{
			cluster.GET("/status", clusterHandler.GetStatus)
			cluster.GET("/setup", clusterHandler.SetupCluster)  // GET for EventSource compatibility
			cluster.POST("/setup", clusterHandler.SetupCluster) // POST for programmatic access
		}
	}

	// Get port from environment or default to 8090
	port := getPort()

	slog.Info("api server starting",
		"port", port,
		"address", ":"+port,
		"endpoints", []string{"/vibespaces", "/cluster"})

	if err := r.Run(":" + port); err != nil {
		slog.Error("failed to start api server",
			"error", err,
			"port", port)
		os.Exit(1)
	}
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	return port
}
