package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/pkg/notify"
)

// handleNotificationStream 前端全局通知 SSE 端点
//
// GET /notifications/stream
//
// 事件格式：
//
//	event: notification
//	data: {"level":"low","event":"sms_received","title":"收到新短信","body":"...","device_id":"...","timestamp":"..."}
//
// 事件等级：
//   - high: 来电等需要强提醒的通知（前端用自定义全屏浮窗展示）
//   - low:  短信/设备上下线等一般通知（前端用 ElNotification 右上角气泡）
//
// handleNotificationStream
//
// @Summary      NotificationStream
// @Tags         notifications
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /notifications/stream [get]
// @Security     BearerAuth
func (s *Server) handleNotificationStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "流式输出不支持"})
		return
	}

	// 订阅通知广播器
	ch := notify.GlobalNotificationBroadcaster.Subscribe()
	defer notify.GlobalNotificationBroadcaster.Unsubscribe(ch)

	// 发送初始连接事件
	c.SSEvent("connected", gin.H{"message": "已连接通知流"})
	flusher.Flush()

	ctx := c.Request.Context()

	for {
		select {
		case n := <-ch:
			data, err := json.Marshal(n)
			if err != nil {
				continue
			}
			c.SSEvent("notification", json.RawMessage(data))
			flusher.Flush()
		case <-ctx.Done():
			return
		case <-s.shutdownCh:
			return
		}
	}
}
