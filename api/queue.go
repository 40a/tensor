package api

import (
	"github.com/gin-gonic/gin"
	"github.com/pearsonappeng/tensor/queue"
	"net/http"
)

// QueueStats returns statistics about redis rmq
func QueueStats(c *gin.Context) {
	queues := queue.Queue.GetOpenQueues()
	stats := queue.Queue.CollectStats(queues)

	c.JSON(http.StatusOK, stats)
}
