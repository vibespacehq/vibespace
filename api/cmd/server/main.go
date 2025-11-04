package main

import (
	"log/slog"
	"os"

	"workspace/pkg/handler"
	"workspace/pkg/k8s"
	"workspace/pkg/workspace"

	"github.com/gin-gonic/gin"
)

func main() {
	slog.Info("initializing workspaces api server",
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
	workspaceService := workspace.NewService(k8sClient)

	// Initialize handlers
	workspaceHandler := handler.NewWorkspaceHandler(workspaceService)
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

		// Workspaces
		workspaces := v1.Group("/workspaces")
		{
			workspaces.GET("", workspaceHandler.List)
			workspaces.POST("", workspaceHandler.Create)
			workspaces.GET("/:id", workspaceHandler.Get)
			workspaces.DELETE("/:id", workspaceHandler.Delete)
			workspaces.POST("/:id/start", workspaceHandler.Start)
			workspaces.POST("/:id/stop", workspaceHandler.Stop)
			workspaces.GET("/:id/access", workspaceHandler.Access)
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
			cluster.GET("/contexts", clusterHandler.ListContexts)
			cluster.POST("/contexts/:name/switch", clusterHandler.SwitchContext)
		}
	}

	// Get port from environment or default to 8090
	port := getPort()

	slog.Info("api server starting",
		"port", port,
		"address", ":"+port,
		"endpoints", []string{"/workspaces", "/templates", "/cluster"})

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
