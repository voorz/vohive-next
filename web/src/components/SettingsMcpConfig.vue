<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { Copy24Regular, CheckmarkCircle24Regular, Document24Regular, Link24Regular, Open20Regular } from '@vicons/fluent'

const endpointCopied = ref(false)
const configCopied = ref(false)

// 状态检测
const statusOk = ref<boolean | null>(null) // null = 未检测, true = 正常, false = 异常
const statusChecking = ref(false)
let statusTimer: ReturnType<typeof setInterval> | null = null

const endpoint = computed(() => `${window.location.origin}/api/mcp`)
const openapiUrl = computed(() => `${window.location.origin}/openapi.json`)

const configJson = computed(() => JSON.stringify({
  mcpServers: {
    vohive: {
      url: endpoint.value,
      headers: { Authorization: 'Bearer <API_TOKEN>' },
    },
  },
}, null, 2))

async function checkStatus() {
  statusChecking.value = true
  try {
    const res = await fetch(`${window.location.origin}/ping`, { method: 'GET' })
    statusOk.value = res.ok
  } catch {
    statusOk.value = false
  } finally {
    statusChecking.value = false
  }
}

onMounted(() => {
  checkStatus()
  statusTimer = setInterval(checkStatus, 30000)
})

onBeforeUnmount(() => {
  if (statusTimer) clearInterval(statusTimer)
})

// 复制到剪贴板，带 fallback
async function copyToClipboard(text: string): Promise<boolean> {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // fall through to fallback
    }
  }
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return ok
  } catch {
    return false
  }
}

async function copyEndpoint() {
  const ok = await copyToClipboard(endpoint.value)
  if (ok) {
    endpointCopied.value = true
    ElMessage.success('已复制端点地址')
    setTimeout(() => { endpointCopied.value = false }, 2000)
  } else {
    ElMessage.error('复制失败')
  }
}

async function copyConfig() {
  const ok = await copyToClipboard(configJson.value)
  if (ok) {
    configCopied.value = true
    ElMessage.success('已复制配置')
    setTimeout(() => { configCopied.value = false }, 2000)
  } else {
    ElMessage.error('复制失败')
  }
}
</script>

<template>
  <div class="mcp-config-card">
    <div class="mcp-config-summary">
      <!-- 标题 -->
      <div>
        <h2 class="mcp-config-title">MCP Server</h2>
        <p class="mcp-config-desc">通过 MCP 协议让 AI 助手直接调试和操作 VoHive</p>
      </div>

      <!-- 端点地址 + 状态 -->
      <div class="mcp-config-field">
        <div class="mcp-field-row">
          <span class="mcp-field-label">端点地址</span>
          <span class="mcp-status" :class="{ ok: statusOk === true, err: statusOk === false, checking: statusChecking }">
            <span class="mcp-status-dot"></span>
            {{ statusChecking ? '检测中…' : statusOk === true ? '服务正常' : statusOk === false ? '服务异常' : '未检测' }}
          </span>
        </div>
        <div class="mcp-endpoint">
          <code>{{ endpoint }}</code>
          <el-button text size="default" @click="copyEndpoint" class="mcp-copy-btn">
            <el-icon size="20"><component :is="endpointCopied ? CheckmarkCircle24Regular : Copy24Regular" /></el-icon>
          </el-button>
        </div>
      </div>

      <!-- 文档入口 -->
      <div class="mcp-docs-inline">
        <a class="mcp-doc-card" href="/docs" target="_blank" rel="noreferrer">
          <div class="settings-icon-box">
            <el-icon size="20"><Document24Regular /></el-icon>
          </div>
          <div class="mcp-doc-info">
            <strong class="mcp-doc-title">API 文档</strong>
            <p class="mcp-doc-desc">Scalar API Reference 交互式文档</p>
          </div>
          <el-icon class="mcp-doc-external" size="16"><Open20Regular /></el-icon>
        </a>

        <a class="mcp-doc-card" :href="openapiUrl" target="_blank" rel="noreferrer">
          <div class="settings-icon-box">
            <el-icon size="20"><Link24Regular /></el-icon>
          </div>
          <div class="mcp-doc-info">
            <strong class="mcp-doc-title">OpenAPI Schema</strong>
            <p class="mcp-doc-desc">OpenAPI 3.0 规范 JSON</p>
          </div>
          <el-icon class="mcp-doc-external" size="16"><Open20Regular /></el-icon>
        </a>
      </div>
    </div>

    <!-- 客户端配置代码 -->
    <div class="mcp-config-code">
      <div class="mcp-config-code-header">
        <strong>客户端配置</strong>
        <el-button text size="default" @click="copyConfig" class="mcp-copy-btn">
          <el-icon size="20"><component :is="configCopied ? CheckmarkCircle24Regular : Copy24Regular" /></el-icon>
        </el-button>
      </div>
      <pre>{{ configJson }}</pre>
    </div>
  </div>
</template>

<style scoped>
.mcp-config-card {
  display: grid;
  grid-template-columns: minmax(0, 4fr) minmax(0, 6fr);
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
}

.mcp-config-summary {
  display: grid;
  align-content: start;
  justify-items: stretch;
  gap: 12px;
  min-width: 0;
}

.mcp-config-title {
  margin: 1px 0 5px;
  font-size: 16px;
  font-weight: 700;
  color: var(--foreground);
}

.mcp-config-desc {
  margin: 0;
  color: var(--muted-foreground);
  line-height: 1.5;
  font-size: 13px;
}

.mcp-config-field {
  display: grid;
  gap: 7px;
  width: 100%;
  min-width: 0;
}

.mcp-field-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.mcp-field-label {
  color: var(--muted-foreground);
  font-size: 12px;
  font-weight: 600;
}

.mcp-status {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  font-weight: 600;
}

.mcp-status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  display: inline-block;
  background: var(--muted-foreground);
}

.mcp-status.ok .mcp-status-dot {
  background: var(--success);
}

.mcp-status.err .mcp-status-dot {
  background: var(--destructive);
}

.mcp-status.checking .mcp-status-dot {
  background: var(--muted-foreground);
  opacity: 0.5;
}

.mcp-endpoint {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-width: 0;
  padding: 4px 4px 4px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: color-mix(in oklab, var(--muted) 58%, transparent);
}

.mcp-endpoint code {
  flex: 1;
  min-width: 0;
  overflow: auto;
  font-family: ui-monospace, monospace;
  font-size: 12px;
  white-space: nowrap;
}

/* 文档入口 — 行内双卡片 */
.mcp-docs-inline {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.mcp-doc-card {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: color-mix(in oklab, var(--muted) 40%, transparent);
  text-decoration: none;
  transition: border-color 0.15s;
}

.mcp-doc-card:hover {
  border-color: var(--brand);
}

.mcp-doc-info {
  flex: 1;
  min-width: 0;
}

.mcp-doc-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--foreground);
}

.mcp-doc-desc {
  color: var(--muted-foreground);
  font-size: 12px;
  margin: 1px 0 0;
}

.mcp-doc-external {
  color: var(--muted-foreground);
  flex-shrink: 0;
}

.mcp-config-code {
  min-width: 0;
  overflow: hidden;
  border-radius: 6px;
  background: color-mix(in oklab, var(--muted) 58%, transparent);
}

.mcp-config-code-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 42px;
  padding: 4px 6px 4px 14px;
  border-bottom: 1px solid var(--border);
}

.mcp-config-code-header strong {
  font-size: 13px;
  color: var(--foreground);
}

.mcp-config-code pre {
  min-height: 120px;
  margin: 0;
  padding: 14px;
  overflow: auto;
  font-family: ui-monospace, monospace;
  font-size: 12px;
  line-height: 1.55;
  color: var(--foreground);
}

.mcp-copy-btn {
  padding: 4px 8px !important;
  height: auto !important;
}

.mcp-copy-btn:hover {
  background: transparent !important;
}

@media (max-width: 960px) {
  .mcp-config-card {
    display: grid;
    grid-template-columns: 1fr;
  }

  .mcp-docs-inline {
    grid-template-columns: 1fr;
  }
}
</style>
