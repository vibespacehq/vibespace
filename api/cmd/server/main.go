package main

import (
	"log/slog"
	"os"

	"vibespace/pkg/handler"
	"vibespace/pkg/k8s"
	"vibespace/pkg/vibespace"

	"github.com/gin-gonic/gin"
)

func main() {
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
	templateHandler := handler.NewTemplateHandler()
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
		}

		// Templates
		templates := v1.Group("/templates")
		{
			templates.GET("", templateHandler.List)
			templates.GET("/:id", templateHandler.Get)
		}

		// Cluster
		cluster := v1.Group("/cluster")
		{
			cluster.GET("/status", clusterHandler.GetStatus)
			cluster.GET("/setup", clusterHandler.SetupCluster)  // GET for EventSource compatibility
			cluster.POST("/setup", clusterHandler.SetupCluster) // POST for programmatic access
			// NOTE: Context routes removed with ADR 0006 (bundled Kubernetes)
			// Previously: GET /contexts, POST /contexts/:name/switch
		}

		// NOTE: Setup routes removed - Harbor replaced with simple Docker Registry
		// Previously: GET /setup/harbor-ca
	}

	// Get port from environment or default to 8090
	port := getPort()

	slog.Info("api server starting",
		"port", port,
		"address", ":"+port,
		"endpoints", []string{"/vibespaces", "/templates", "/cluster"})

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
