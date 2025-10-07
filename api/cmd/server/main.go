package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
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
			c.JSON(200, gin.H{"status": "healthy"})
		})

		// Workspaces
		workspaces := v1.Group("/workspaces")
		{
			workspaces.GET("", listWorkspaces)
			workspaces.POST("", createWorkspace)
			workspaces.GET("/:id", getWorkspace)
			workspaces.PUT("/:id", updateWorkspace)
			workspaces.DELETE("/:id", deleteWorkspace)
			workspaces.POST("/:id/start", startWorkspace)
			workspaces.POST("/:id/stop", stopWorkspace)
		}

		// Templates
		templates := v1.Group("/templates")
		{
			templates.GET("", listTemplates)
			templates.POST("", createTemplate)
			templates.GET("/:id", getTemplate)
			templates.DELETE("/:id", deleteTemplate)
		}

		// Credentials
		credentials := v1.Group("/credentials")
		{
			credentials.GET("", listCredentials)
			credentials.POST("", createCredential)
			credentials.GET("/:id", getCredential)
			credentials.PUT("/:id", updateCredential)
			credentials.DELETE("/:id", deleteCredential)
		}

		// Cluster
		cluster := v1.Group("/cluster")
		{
			cluster.GET("/status", getClusterStatus)
			cluster.POST("/install", installCluster)
		}
	}

	// Get port from environment or default to 8090
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	log.Printf("Starting API server on port %s...", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// Placeholder handlers
func listWorkspaces(c *gin.Context) {
	c.JSON(200, gin.H{"workspaces": []interface{}{}})
}

func createWorkspace(c *gin.Context) {
	c.JSON(201, gin.H{"message": "Workspace created"})
}

func getWorkspace(c *gin.Context) {
	c.JSON(200, gin.H{"id": c.Param("id")})
}

func updateWorkspace(c *gin.Context) {
	c.JSON(200, gin.H{"message": "Workspace updated"})
}

func deleteWorkspace(c *gin.Context) {
	c.JSON(204, nil)
}

func startWorkspace(c *gin.Context) {
	c.JSON(200, gin.H{"message": "Workspace starting"})
}

func stopWorkspace(c *gin.Context) {
	c.JSON(200, gin.H{"message": "Workspace stopping"})
}

func listTemplates(c *gin.Context) {
	c.JSON(200, gin.H{"templates": []interface{}{}})
}

func createTemplate(c *gin.Context) {
	c.JSON(201, gin.H{"message": "Template created"})
}

func getTemplate(c *gin.Context) {
	c.JSON(200, gin.H{"id": c.Param("id")})
}

func deleteTemplate(c *gin.Context) {
	c.JSON(204, nil)
}

func listCredentials(c *gin.Context) {
	c.JSON(200, gin.H{"credentials": []interface{}{}})
}

func createCredential(c *gin.Context) {
	c.JSON(201, gin.H{"message": "Credential created"})
}

func getCredential(c *gin.Context) {
	c.JSON(200, gin.H{"id": c.Param("id")})
}

func updateCredential(c *gin.Context) {
	c.JSON(200, gin.H{"message": "Credential updated"})
}

func deleteCredential(c *gin.Context) {
	c.JSON(204, nil)
}

func getClusterStatus(c *gin.Context) {
	c.JSON(200, gin.H{"status": "unknown", "installed": false})
}

func installCluster(c *gin.Context) {
	c.JSON(202, gin.H{"message": "Installation started"})
}
