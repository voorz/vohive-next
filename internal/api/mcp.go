package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/internal/config"
	"github.com/voorz/vohive/pkg/logger"
)

// ── JSON-RPC 类型 ──

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ── MCP 工具定义 ──

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpToolResult struct {
	Content []mcpContent `json:"content"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ── Streamable HTTP Transport ──
// 单 POST 端点，无 SSE 连接，无 session 管理，请求-响应模式

// handleMcpRequest 
//
// @Summary      McpRequest
// @Tags         mcp
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "成功"
// @Failure      400  {object}  map[string]interface{}  "参数错误"
// @Failure      401  {object}  map[string]interface{}  "未授权"
// @Router       /mcp [post]
// @Security     BearerAuth
func (s *Server) handleMcpRequest(c *gin.Context) {
	var req jsonRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("MCP 请求解析失败", "err", err, "ip", c.ClientIP())
		c.JSON(http.StatusBadRequest, jsonRPCResponse{
			JSONRPC: "2.0",
			Error:  &jsonRPCError{Code: -32700, Message: "parse error"},
		})
		return
	}

	// notification（无 ID）→ 202 Accepted
	if req.ID == nil {
		logger.Debug("MCP notification", "method", req.Method)
		c.Status(http.StatusAccepted)
		return
	}

	logger.Info("MCP request", "method", req.Method, "id", req.ID, "ip", c.ClientIP())
	resp := s.dispatchMCP(req)
	c.JSON(http.StatusOK, resp)
}

// ── JSON-RPC 分发 ──

func (s *Server) dispatchMCP(req jsonRPCRequest) jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: gin.H{
				"protocolVersion": "2025-03-26",
				"capabilities":    gin.H{"tools": gin.H{}},
				"serverInfo":       gin.H{"name": "vohive", "version": "0.0.1"},
			},
		}

	case "tools/list":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: gin.H{
				"tools": s.mcpTools(),
			},
		}

	case "tools/call":
		return s.handleMcpToolCall(req)

	default:
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32601,
				Message: "method not found: " + req.Method,
			},
		}
	}
}

// ── 工具列表 ──

func (s *Server) mcpTools() []mcpTool {
	return []mcpTool{
		{
			Name:        "get_device_status",
			Description: "获取设备列表和 VoWiFi 运行状态。",
			InputSchema: gin.H{
				"type": "object",
				"properties": gin.H{
					"device_id": gin.H{"type": "string", "description": "设备 ID（可选，不传返回全部）"},
				},
			},
		},
		{
			Name:        "get_config",
			Description: "读取 VoHive 配置。section: server/web/security/site/all",
			InputSchema: gin.H{
				"type": "object",
				"properties": gin.H{
					"section": gin.H{"type": "string", "description": "配置节名（默认 all）"},
				},
			},
		},
		{
			Name:        "tail_logs",
			Description: "读取最近的日志行。支持按级别和关键词过滤。",
			InputSchema: gin.H{
				"type": "object",
				"properties": gin.H{
					"lines":   gin.H{"type": "integer", "description": "行数（默认 100）", "default": 100},
					"level":   gin.H{"type": "string", "description": "日志级别：debug/info/warn/error"},
					"keyword": gin.H{"type": "string", "description": "关键词过滤"},
				},
			},
		},
	}
}

// ── 工具调用 ──

func (s *Server) handleMcpToolCall(req jsonRPCRequest) jsonRPCResponse {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		logger.Warn("MCP tool call 参数解析失败", "err", err)
		return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32602, Message: "invalid params"}}
	}

	logger.Info("MCP tool call", "tool", params.Name, "args", params.Arguments)

	var result string

	switch params.Name {
	case "get_device_status":
		result = s.mcpGetDeviceStatus(params.Arguments)
	case "get_config":
		result = s.mcpGetConfig(params.Arguments)
	case "tail_logs":
		result = s.mcpTailLogs(params.Arguments)
	default:
		return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32602, Message: "unknown tool: " + params.Name}}
	}

	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: mcpToolResult{
			Content: []mcpContent{{Type: "text", Text: result}},
		},
	}
}

// ── 工具实现 ──

func (s *Server) mcpGetDeviceStatus(args map[string]any) string {
	if s.pool == nil {
		return "设备池未初始化"
	}
	workers := s.pool.GetAllWorkers()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("设备数量: %d\n\n", len(workers)))
	for _, w := range workers {
		status := w.GetCachedDeviceStatus()
		healthy := w.GetCachedHealthy()
		ip := w.GetCachedIP()
		sb.WriteString(fmt.Sprintf("─ %s (%s)\n", w.ID, w.Config.Name))
		sb.WriteString(fmt.Sprintf("  健康: %v | IP: %s | 接口: %s\n", healthy, ip, w.Config.Interface))
		sb.WriteString(fmt.Sprintf("  状态: %s\n", status))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (s *Server) mcpGetConfig(args map[string]any) string {
	cfg := config.GetConfig()
	if cfg == nil {
		return "配置未初始化"
	}
	section, _ := args["section"].(string)

	switch section {
	case "", "all":
		b, _ := json.MarshalIndent(map[string]any{
			"server":   cfg.Server,
			"web":      gin.H{"username": cfg.Web.Username},
			"security": cfg.Security,
			"site":     cfg.Site,
		}, "", "  ")
		return string(b)
	case "server":
		b, _ := json.MarshalIndent(cfg.Server, "", "  ")
		return string(b)
	case "web":
		b, _ := json.MarshalIndent(gin.H{"username": cfg.Web.Username}, "", "  ")
		return string(b)
	case "security":
		b, _ := json.MarshalIndent(cfg.Security, "", "  ")
		return string(b)
	case "site":
		b, _ := json.MarshalIndent(cfg.Site, "", "  ")
		return string(b)
	default:
		return "未知配置节: " + section
	}
}

func (s *Server) mcpTailLogs(args map[string]any) string {
	lines := 100
	if v, ok := args["lines"].(float64); ok && v > 0 {
		lines = int(v)
	}
	level, _ := args["level"].(string)
	keyword, _ := args["keyword"].(string)

	logPath := "logs/app.log"
	data, err := readFileLines(logPath, lines, level, keyword)
	if err != nil {
		return fmt.Sprintf("读取日志失败: %v", err)
	}
	if len(data) == 0 {
		return "无匹配日志"
	}
	return strings.Join(data, "\n")
}

// ── 辅助函数 ──

func (s *Server) initMCP() {
	logger.Info("MCP server 已初始化 (Streamable HTTP)")
}

// readFileLines 读取日志文件最后 N 行，支持级别和关键词过滤
func readFileLines(path string, maxLines int, level, keyword string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if level != "" {
			if !strings.Contains(strings.ToUpper(line), strings.ToUpper(level)) {
				continue
			}
		}
		if keyword != "" {
			if !strings.Contains(line, keyword) {
				continue
			}
		}
		all = append(all, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	start := len(all) - maxLines
	if start < 0 {
		start = 0
	}
	return all[start:], nil
}
