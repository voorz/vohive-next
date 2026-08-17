package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vohive/internal/voice"
)

// handleVoiceStream 通话状态 SSE 端点
//
// GET /devices/:device_id/voice/stream
//
// 事件格式：
//
//	event: call
//	data: {"state":"ringing","direction":"incoming","number":"+8613...","call_id":"..."}
//
// 事件类型：
//   - dialing: 拨号中（呼出）
//   - ringing: 振铃中（来电）
//   - connected: 通话中
//   - ended: 通话结束
// handleVoiceStream 
//
// @Summary      VoiceStream
// @Tags         voice
// @Produce      json
// @Param        device_id  path      string  true  "device_id"
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /devices/{device_id}/voice/stream [get]
// @Security     BearerAuth
func (s *Server) handleVoiceStream(c *gin.Context) {
	deviceID := deviceIDParam(c)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 device_id"})
		return
	}

	if s.voiceBus == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "通话服务未启用"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "流式输出不支持"})
		return
	}

	// 事件管道
	eventCh := make(chan voice.CallEvent, 32)
	done := make(chan struct{})

	// 订阅通话事件
	handler := func(event voice.CallEvent) {
		// 只推送当前设备的事件
		if event.DeviceID != deviceID {
			return
		}
		select {
		case eventCh <- event:
		case <-done:
		}
	}
	s.voiceBus.OnEvent(handler)
	defer func() {
		close(done)
		// 无法取消订阅 OnEvent（简化实现，处理器会在 done 后自动丢弃）
	}()

	ctx := c.Request.Context()

	for {
		select {
		case event := <-eventCh:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(c.Writer, "event: call\ndata: %s\n\n", string(data))
			flusher.Flush()
		case <-ctx.Done():
			return
		case <-s.shutdownCh:
			return
		}
	}
}

// handleGetVoiceHistory 获取通话记录列表
//
// GET /devices/:device_id/voice/history?limit=50&before_ts=&before_peer=
// handleGetVoiceHistory 
//
// @Summary      GetVoiceHistory
// @Tags         voice
// @Produce      json
// @Param        device_id  path      string  true  "device_id"
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /devices/{device_id}/voice/history [get]
// @Security     BearerAuth
func (s *Server) handleGetVoiceHistory(c *gin.Context) {
	deviceID := deviceIDParam(c)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 device_id"})
		return
	}

	limit := 50
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}

	// 从设备 ID 查找 IMSI（简化：用 device_id 作为查询条件）
	records, err := db.GetVoiceHistory(deviceID, limit, nil, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询通话记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"records": records,
	})
}

// handleDeleteVoiceHistory 删除单条通话记录
//
// DELETE /devices/:device_id/voice/history/:id
// handleDeleteVoiceHistory 
//
// @Summary      DeleteVoiceHistory
// @Tags         voice
// @Produce      json
// @Param        device_id  path      string  true  "device_id"
// @Param        id  path      string  true  "id"
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /devices/{device_id}/voice/history/{id} [delete]
// @Security     BearerAuth
func (s *Server) handleDeleteVoiceHistory(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少记录 ID"})
		return
	}

	var id uint
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的记录 ID"})
		return
	}

	if err := db.DeleteVoiceHistory(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除通话记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleDeleteAllVoiceHistory 删除某设备的所有通话记录
//
// DELETE /devices/:device_id/voice/history
// handleDeleteAllVoiceHistory 
//
// @Summary      DeleteAllVoiceHistory
// @Tags         voice
// @Produce      json
// @Param        device_id  path      string  true  "device_id"
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /devices/{device_id}/voice/history [delete]
// @Security     BearerAuth
func (s *Server) handleDeleteAllVoiceHistory(c *gin.Context) {
	deviceID := deviceIDParam(c)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 device_id"})
		return
	}

	if err := db.DeleteVoiceHistoryByIMSI(deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除通话记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
