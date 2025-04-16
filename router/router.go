package router

import (
	"github.com/gin-gonic/gin"
	"org.donghyuns.com/rtsphls/lib"
)

// SetupRoutes configures all routes for the application
func SetupRoutes(router *gin.Engine, streamManager *lib.StreamManager) {
	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// HLS playback routes
	router.GET("/play/hls/:cctvId/index.m3u8", func(c *gin.Context) {
		lib.PlayHLS(c, streamManager)
	})

	router.GET("/play/hls/:cctvId/segment/:seq/file.ts", func(c *gin.Context) {
		lib.PlayHLSTS(c, streamManager)
	})

	// Stream management API routes
	api := router.Group("/api")
	{
		api.GET("/streams", func(c *gin.Context) {
			streams := streamManager.ListStreams()
			c.JSON(200, gin.H{"status": "success", "streams": streams})
		})

		api.POST("/streams/:id", func(c *gin.Context) {
			id := c.Param("id")
			var req struct {
				URL      string `json:"url" binding:"required"`
				OnDemand bool   `json:"on_demand"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"status": "error", "message": err.Error()})
				return
			}

			if streamManager.StreamExists(id) {
				c.JSON(409, gin.H{"status": "error", "message": "Stream ID already exists"})
				return
			}

			streamManager.AddStream(id, req.URL, req.OnDemand)
			c.JSON(201, gin.H{"status": "success", "id": id})
		})

		api.DELETE("/streams/:id", func(c *gin.Context) {
			id := c.Param("id")
			if !streamManager.StreamExists(id) {
				c.JSON(404, gin.H{"status": "error", "message": "Stream not found"})
				return
			}

			streamManager.RemoveStream(id)
			c.JSON(200, gin.H{"status": "success"})
		})
	}
}
