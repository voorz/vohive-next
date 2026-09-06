package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
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
	token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	resp := s.dispatchMCP(req, token)
	c.JSON(http.StatusOK, resp)
}

// ── JSON-RPC 分发 ──

func (s *Server) dispatchMCP(req jsonRPCRequest, token string) jsonRPCResponse {
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
		return s.handleMcpToolCall(req, token)

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
	// 先加载自动生成的工具
	loadAutoMCPTools()

	tools := []mcpTool{
		{
			Name:        "login",
			Description: "使用用户名密码登录，返回 Bearer token。获取 token 后可用于调用其他需要认证的 API 端点。",
			InputSchema: gin.H{
				"type": "object",
				"properties": gin.H{
					"username": gin.H{"type": "string", "description": "用户名（默认 admin）"},
					"password": gin.H{"type": "string", "description": "密码（默认 admin）"},
				},
				"required": []string{"username", "password"},
			},
		},
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

	// 合并自动生成的工具
	tools = append(tools, autoTools...)
	return tools
}

// ── 工具调用 ──

func (s *Server) handleMcpToolCall(req jsonRPCRequest, token string) jsonRPCResponse {
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

	// 先检查手动工具
	switch params.Name {
	case "login":
		result = s.mcpLogin(params.Arguments)
	case "get_device_status":
		result = s.mcpGetDeviceStatus(params.Arguments)
	case "get_config":
		result = s.mcpGetConfig(params.Arguments)
	case "tail_logs":
		result = s.mcpTailLogs(params.Arguments)
	default:
		// 检查是否是自动生成的工具
		if _, ok := autoToolsMap[params.Name]; ok {
			res, err := s.callAutoMCPTool(params.Name, params.Arguments, token)
			if err != nil {
				return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32603, Message: err.Error()}}
			}
			result = res
		} else {
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32602, Message: "unknown tool: " + params.Name}}
		}
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

func (s *Server) mcpLogin(args map[string]any) string {
	username, _ := args["username"].(string)
	password, _ := args["password"].(string)

	if username == "" || password == "" {
		return "错误：需要提供 username 和 password"
	}

	// 构造登录请求
	loginURL := "http://127.0.0.1" + s.cfg.Port + "/api/auth/login"
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password)

	resp, err := http.DefaultClient.Post(loginURL, "application/json", strings.NewReader(body))
	if err != nil {
		return fmt.Sprintf("登录请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("读取响应失败: %v", err)
	}

	return fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(respBody))
}

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
	loadAutoMCPTools()
	logger.Info("MCP server 已初始化 (Streamable HTTP)", "auto_tools", len(autoTools))
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
