package api

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"github.com/voorz/vohive/internal/api/docs"
)

const (
	// 顶级路由（不需要 /api 前缀，不需要鉴权）
	docsUIPath      = "/docs"
	openAPIJSONPath = "/openapi.json"
	openAPIYAMLPath = "/openapi.yaml"
)

type apiDocsLinks struct {
	DocsUI      string `json:"docs_ui"`
	OpenAPIYAML string `json:"openapi_yaml"`
	OpenAPIJSON string `json:"openapi_json"`
}

//go:embed all:docs_assets/scalar
var scalarAssetEmbedFS embed.FS

var (
	openAPISpecJSONOnce sync.Once
	openAPISpecJSON     []byte
	openAPISpecJSONErr  error

	openAPISpecYAMLOnce sync.Once
	openAPISpecYAML     []byte
	openAPISpecYAMLErr  error
)

func currentAPIDocsLinks() apiDocsLinks {
	return apiDocsLinks{
		DocsUI:      docsUIPath,
		OpenAPIYAML: openAPIYAMLPath,
		OpenAPIJSON: openAPIJSONPath,
	}
}

// loadOpenAPISpecJSON 从 swag 生成的 docs 包读取 OpenAPI JSON spec。
// 首次调用时通过 docs.SwaggerInfo.ReadDoc() 渲染模板并缓存结果。
func loadOpenAPISpecJSON() ([]byte, error) {
	openAPISpecJSONOnce.Do(func() {
	// docs 包 init() 已自动注册 SwaggerInfo，直接 ReadDoc 即可
	openAPISpecJSON = []byte(docs.SwaggerInfo.ReadDoc())
	})
	if openAPISpecJSONErr != nil {
		return nil, openAPISpecJSONErr
	}
	return openAPISpecJSON, nil
}

// loadOpenAPISpecYAML 从 swag 生成的 JSON spec 转换为 YAML 格式。
func loadOpenAPISpecYAML() ([]byte, error) {
	openAPISpecYAMLOnce.Do(func() {
		jsonBytes, err := loadOpenAPISpecJSON()
		if err != nil {
			openAPISpecYAMLErr = fmt.Errorf("加载 OpenAPI JSON 用于 YAML 转换: %w", err)
			return
		}
		var raw any
		if err := json.Unmarshal(jsonBytes, &raw); err != nil {
			openAPISpecYAMLErr = fmt.Errorf("解析 OpenAPI JSON: %w", err)
			return
		}
		normalized := normalizeYAMLValue(raw)
		yamlBytes, err := yaml.Marshal(normalized)
		if err != nil {
			openAPISpecYAMLErr = fmt.Errorf("编码 OpenAPI YAML: %w", err)
			return
		}
		openAPISpecYAML = yamlBytes
	})
	if openAPISpecYAMLErr != nil {
		return nil, openAPISpecYAMLErr
	}
	return openAPISpecYAML, nil
}


func normalizeYAMLValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, inner := range typed {
			out[k] = normalizeYAMLValue(inner)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for k, inner := range typed {
			out[fmt.Sprint(k)] = normalizeYAMLValue(inner)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, inner := range typed {
			out[i] = normalizeYAMLValue(inner)
		}
		return out
	default:
		return v
	}
}

// handleOpenAPIJSON serves the OpenAPI spec as JSON at /openapi.json (no auth).
func (s *Server) handleOpenAPIJSON(c *gin.Context) {
	payload, err := loadOpenAPISpecJSON()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "生成 OpenAPI JSON 失败: " + err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
}

// handleOpenAPIYAML serves the OpenAPI spec as YAML at /openapi.yaml (no auth).
func (s *Server) handleOpenAPIYAML(c *gin.Context) {
	payload, err := loadOpenAPISpecYAML()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "生成 OpenAPI YAML 失败: " + err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", payload)
}

// handleDocsAsset serves Scalar JS assets at /docs/assets/*filepath (no auth).
func (s *Server) handleDocsAsset(c *gin.Context) {
	assetPath := strings.TrimPrefix(c.Param("filepath"), "/")
	if assetPath == "" {
		c.String(http.StatusNotFound, "Not Found")
		return
	}
	assetPath = path.Clean(assetPath)
	if assetPath == "." || strings.HasPrefix(assetPath, "../") {
		c.String(http.StatusNotFound, "Not Found")
		return
	}

	content, err := fs.ReadFile(scalarAssetEmbedFS, path.Join("docs_assets/scalar", assetPath))
	if err != nil {
		c.String(http.StatusNotFound, "Not Found")
		return
	}
	c.Data(http.StatusOK, docsAssetContentType(assetPath), content)
}

func docsAssetContentType(assetPath string) string {
	switch strings.ToLower(path.Ext(assetPath)) {
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".png":
		return "image/png"
	case ".svg":
		return "image/svg+xml"
	case ".txt", ".map":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// handleDocs serves the Scalar API reference page at /docs (no auth required for the page itself).
func (s *Server) handleDocs(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(scalarDocsHTML))
}

// handleAPIDocs is a legacy compat alias for /api/docs → redirect to /docs.
func (s *Server) handleAPIDocs(c *gin.Context) {
	c.Redirect(http.StatusFound, docsUIPath)
}

const scalarDocsHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <title>VoHive API Docs</title>
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
  <style>
    :root {
      --brand: #00BC7D;
      --bg: #0a0a0a;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
    }
    #scalar-api-reference {
      min-height: 100vh;
    }
    /* Scalar 主题覆盖 */
    .scalar-app {
      --scalar-color-accent: var(--brand);
      --scalar-background-accent: rgba(0, 188, 125, 0.12);
    }
  </style>
</head>
<body>
  <div id="scalar-api-reference"></div>
  <script src="/docs/assets/scalar-api-reference.js"></script>
  <script>
    (function () {
      var OPENAPI_JSON_URL = '/openapi.json';
      var token = '';

      try {
        token = String(localStorage.getItem('token') || '').trim();
      } catch (err) {
        token = '';
      }

      var config = {
        spec: { url: OPENAPI_JSON_URL },
        theme: 'default',
        darkMode: false,
        forceDarkModeState: 'light',
        hideClientButton: false,
        hideModels: false,
        defaultOpenAllTags: false,
        layout: 'modern',
        customCss: ':root{--scalar-color-accent:#00BC7D;--scalar-background-accent:rgba(0,188,125,0.12);}',
      };

      // If we have a token, inject it as a proxy auth header so "Try it out" works.
      if (token) {
        config.auth = {
          bearer: { token: token }
        };
      }

      window.Scalar.createApiReference('#scalar-api-reference', config);
    })();
  </script>
</body>
</html>`
