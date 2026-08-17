package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/voorz/vohive/internal/db"

	"github.com/gin-gonic/gin"
)

// handleTrafficAnalysis 
//
// @Summary      TrafficAnalysis
// @Tags         dashboard
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /traffic/analysis [get]
// @Security     BearerAuth
func (s *Server) handleTrafficAnalysis(c *gin.Context) {
	rng := c.Query("range")
	if rng == "" {
		rng = "day"
	}
	deviceID := strings.TrimSpace(c.Query("device_id"))
	now := time.Now()

	buckets, chartData, err := db.GetTrafficAnalysisWithChart(rng, deviceID, now)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"range":   rng,
		"buckets": buckets,
		"chart":   chartData,
	})
}
