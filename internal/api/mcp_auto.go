package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/voorz/vohive/pkg/logger"
)

// mcpAutoTag 是 swag 注解中用于标记自动暴露为 MCP 工具的 tag 值。
// 在 handler 函数注解中添加 @Tags <原有tag>, mcp-auto 即可自动暴露。
// 注意：swag 的 @Tags 使用逗号分隔多个 tag。
const mcpAutoTag = "mcp-auto"

// autoMCPTool 描述一个从 OpenAPI spec 自动生成的 MCP 工具。
type autoMCPTool struct {
	// 原始 HTTP method 和 path，用于构造转发请求
	method string
	path   string // OpenAPI path，如 /sms/send
}

var (
	autoToolsOnce   sync.Once
	autoTools       []mcpTool
	autoToolsMap    map[string]autoMCPTool // tool name → 元信息
	autoToolsLoaded bool
)

// loadAutoMCPTools 从 swagger spec 解析标记了 mcp-auto tag 的端点，
// 生成 MCP 工具定义。只在首次调用时执行一次。
func loadAutoMCPTools() {
	autoToolsOnce.Do(func() {
		autoToolsMap = make(map[string]autoMCPTool)

		specJSON, err := loadOpenAPISpecJSON()
		if err != nil {
			logger.Error("MCP 自动工具：加载 swagger spec 失败", "err", err)
			return
		}

		var spec struct {
			BasePath string `json:"basePath"` // 如 "/api"
			Paths    map[string]map[string]struct {
				Summary    string `json:"summary"`
				Tags       []string `json:"tags"`
				Parameters []struct {
					Name     string `json:"name"`
					In       string `json:"in"`       // path / query / body
					Required bool   `json:"required"`
					Type     string `json:"type"`      // string/integer/...
					Schema  struct {
						Ref string `json:"$ref"` // body 参数可能有 $ref
					} `json:"schema"`
				} `json:"parameters"`
			} `json:"paths"`
		}

		if err := json.Unmarshal(specJSON, &spec); err != nil {
			logger.Error("MCP 自动工具：解析 swagger spec 失败", "err", err)
			return
		}

		basePath := spec.BasePath // 通常 "/api"

		for path, methods := range spec.Paths {
			for method, op := range methods {
				// 检查是否有 mcp-auto tag
				found := false
				for _, t := range op.Tags {
					if t == mcpAutoTag {
						found = true
						break
					}
				}
				if !found {
					continue
				}

				// 跳过 SSE stream 端点（不适合 MCP 同步调用）
				if strings.HasSuffix(path, "/stream") || strings.Contains(path, "/stream/") {
					continue
				}

				toolName := buildMCPToolName(method, path)
				tool := mcpTool{
					Name:        toolName,
					Description: op.Summary,
					InputSchema: buildMCPInputSchema(op.Parameters),
				}

				autoTools = append(autoTools, tool)
				autoToolsMap[toolName] = autoMCPTool{
					method: strings.ToUpper(method),
					path:   basePath + path, // 完整路径如 /api/sms/send
				}
			}
		}

		autoToolsLoaded = true
		logger.Info("MCP 自动工具加载完成", "count", len(autoTools))
	})
}

// buildMCPToolName 从 HTTP method + path 生成 MCP 工具名。
// 例：POST /sms/send → send_sms
//     POST /devices/{device_id}/actions/reboot → reboot_device
func buildMCPToolName(method, path string) string {
	// 去掉 path variables，提取有意义的部分
	segments := strings.Split(strings.Trim(path, "/"), "/")
	var meaningful []string
	for _, seg := range segments {
		// 跳过 path parameters 如 {device_id}
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			continue
		}
		meaningful = append(meaningful, seg)
	}

	// 取最后 1-2 个有意义的段作为工具名
	var parts []string
	if len(meaningful) >= 2 {
		// 倒数第二段 + 最后一段，但如果倒数第二段是 "actions" 则只取最后一段
		secondLast := meaningful[len(meaningful)-2]
		last := meaningful[len(meaningful)-1]
		if secondLast == "actions" {
			parts = []string{last}
		} else {
			parts = []string{last, secondLast}
			// 让最后一段在前（动词在前）
			// reboot_device 而不是 device_reboot
			parts[0], parts[1] = parts[1], parts[0]
		}
	} else if len(meaningful) == 1 {
		parts = []string{meaningful[0]}
	}

	// 用 method 前缀消歧义（如果有同名工具）
	name := strings.Join(parts, "_")
	// 替换连字符为下划线
	name = strings.ReplaceAll(name, "-", "_")
	return name
}

// buildMCPInputSchema 从 OpenAPI parameters 构造 JSON Schema（MCP inputSchema）。
func buildMCPInputSchema(params []struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
	Type     string `json:"type"`
	Schema  struct {
		Ref string `json:"$ref"`
	} `json:"schema"`
}) map[string]any {
	properties := make(map[string]any)
	var required []string

	for _, p := range params {
		if p.In == "body" {
			// body 参数 → 作为 object 类型属性
			properties[p.Name] = gin.H{
				"type":        "object",
				"description": p.Name,
			}
			if p.Required {
				required = append(required, p.Name)
			}
		} else if p.In == "path" || p.In == "query" {
			prop := gin.H{
				"type":        p.Type,
				"description": p.Name,
			}
			if p.Type == "" {
				prop["type"] = "string"
			}
			properties[p.Name] = prop
			if p.Required {
				required = append(required, p.Name)
			}
		}
	}

	schema := gin.H{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// callAutoMCPTool 执行自动工具调用：构造 HTTP 请求转发到自身 API handler。
// 返回 MCP 格式的文本结果。
func (s *Server) callAutoMCPTool(toolName string, args map[string]any, token string) (string, error) {
	info, ok := autoToolsMap[toolName]
	if !ok {
		return "", fmt.Errorf("unknown auto tool: %s", toolName)
	}

	// 构造完整 URL：替换 path variables
	urlPath := info.path
	for key, val := range args {
		strVal := fmt.Sprintf("%v", val)
		placeholder := "{" + key + "}"
		if strings.Contains(urlPath, placeholder) {
			urlPath = strings.ReplaceAll(urlPath, placeholder, strVal)
			delete(args, key) // 已使用的参数从 args 中删除
		}
	}

	// 构造 HTTP 请求
	baseURL := "http://127.0.0.1" + s.cfg.Port
	targetURL := baseURL + urlPath

	var bodyReader io.Reader
	if info.method == http.MethodPost || info.method == http.MethodPut || info.method == http.MethodPatch || info.method == http.MethodDelete {
		// 剩余的 args 作为 JSON body
		if len(args) > 0 {
			bodyBytes, err := json.Marshal(args)
			if err != nil {
				return "", fmt.Errorf("序列化请求体失败: %w", err)
			}
			bodyReader = strings.NewReader(string(bodyBytes))
		}
	} else {
		// GET：剩余 args 作为 query params
		if len(args) > 0 {
			q := make([]string, 0, len(args))
			for k, v := range args {
				q = append(q, fmt.Sprintf("%s=%v", k, v))
			}
			targetURL += "?" + strings.Join(q, "&")
		}
	}

	req, err := http.NewRequest(info.method, targetURL, bodyReader)
	if err != nil {
		return "", fmt.Errorf("构造请求失败: %w", err)
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 返回 HTTP 状态码 + 响应体
	return fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body)), nil
}
