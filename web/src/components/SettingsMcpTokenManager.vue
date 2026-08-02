<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ChevronDown20Regular, Delete20Regular, Add20Regular } from '@vicons/fluent'
import { systemService, type APITokenInfo } from '../services/system'

const expanded = ref(true)
const loading = ref(false)
const creating = ref(false)
const tokens = ref<APITokenInfo[]>([])

// 创建表单
const newName = ref('')
const newExpiryDays = ref(30)

// 新 Token 展示
const showResult = ref(false)
const newToken = ref('')

const hasTokens = computed(() => tokens.value.length > 0)

onMounted(async () => {
  await loadTokens()
})

async function loadTokens() {
  loading.value = true
  try {
    const res = await systemService.listAPITokens()
    if (res.ok) {
      tokens.value = res.data.tokens || []
    }
  } catch {
    // keep defaults
  } finally {
    loading.value = false
  }
}

async function create() {
  const name = newName.value.trim()
  if (!name) {
    ElMessage.error('名称不能为空')
    return
  }
  creating.value = true
  try {
    const res = await systemService.createAPIToken(name, newExpiryDays.value)
    if (!res.ok) throw new Error(res.error.message || '创建失败')
    newToken.value = res.data.token
    showResult.value = true
    newName.value = ''
    await loadTokens()
  } catch (e: any) {
    ElMessage.error(e.message || '创建失败')
  } finally {
    creating.value = false
  }
}

async function remove(id: number, name: string) {
  try {
    await ElMessageBox.confirm(`确定要删除 Token「${name}」吗？删除后使用该 Token 的 API 请求将失效。`, '删除确认', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch { return }
  try {
    const res = await systemService.deleteAPIToken(id)
    if (!res.ok) throw new Error(res.error.message || '删除失败')
    ElMessage.success('Token 已删除')
    await loadTokens()
  } catch (e: any) {
    ElMessage.error(e.message || '删除失败')
  }
}

function formatExpiry(expiry: number): string {
  if (expiry === 0) return '永不过期'
  const date = new Date(expiry * 1000)
  const now = new Date()
  if (date < now) return '已过期'
  return date.toLocaleString('zh-CN')
}

function formatLastUsed(lastUsed: string | null): string {
  if (!lastUsed) return '—'
  return new Date(lastUsed).toLocaleString('zh-CN')
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString('zh-CN')
}

async function copy(text: string) {
  let ok = false
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      ok = true
    } catch { /* fall through */ }
  }
  if (!ok) {
    try {
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.left = '-9999px'
      document.body.appendChild(ta)
      ta.select()
      ok = document.execCommand('copy')
      document.body.removeChild(ta)
    } catch { /* give up */ }
  }
  if (ok) {
    ElMessage.success('已复制到剪贴板')
  } else {
    ElMessage.error('复制失败')
  }
}
</script>

<template>
  <div class="faq-card mt-4">
    <div class="faq-header" @click="expanded = !expanded">
      <span class="faq-title">Token 管理</span>
      <el-icon class="faq-arrow" :class="{ expanded }" size="16">
        <ChevronDown20Regular />
      </el-icon>
    </div>
    <div v-show="expanded" class="faq-body">
      <div class="mb-4">
        <div class="faq-item-title">API 访问 Token</div>
        <div class="faq-item-desc">创建后可使用 Token 代替用户名密码进行 API 调用（Bearer 认证）</div>
      </div>

      <!-- 创建表单 -->
      <div class="token-create-form">
        <div class="flex items-center gap-3 flex-wrap">
          <div class="flex items-center gap-2">
            <label class="settings-form-label whitespace-nowrap">名称</label>
            <el-input v-model="newName" placeholder="例如 MCP Client" class="!w-[180px]" @keyup.enter="create" />
          </div>
          <div class="flex items-center gap-2">
            <label class="settings-form-label whitespace-nowrap">有效期</label>
            <el-select v-model="newExpiryDays" class="!w-[120px]">
              <el-option label="7 天" :value="7" />
              <el-option label="30 天" :value="30" />
              <el-option label="365 天" :value="365" />
              <el-option label="永不过期" :value="0" />
            </el-select>
          </div>
          <el-button type="primary" :loading="creating" @click="create" class="!border-0 ml-auto">
            <el-icon><Add20Regular /></el-icon>
            <span class="ml-1">创建 Token</span>
          </el-button>
        </div>
      </div>

      <!-- Token 列表 -->
      <div v-loading="loading" class="mt-4">
        <el-table
          v-if="hasTokens"
          :data="tokens"
          class="token-table"
          size="small"
          :border="false"
        >
          <el-table-column prop="name" label="名称" min-width="120" />
          <el-table-column prop="prefix" label="前缀" width="100">
            <template #default="{ row }">
              <code class="token-prefix">{{ row.prefix }}…</code>
            </template>
          </el-table-column>
          <el-table-column label="创建时间" width="170">
            <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="过期时间" width="170">
            <template #default="{ row }">{{ formatExpiry(row.expiry) }}</template>
          </el-table-column>
          <el-table-column label="最近使用" width="170">
            <template #default="{ row }">{{ formatLastUsed(row.last_used_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="80" fixed="right">
            <template #default="{ row }">
              <el-button
                type="danger"
                text
                size="small"
                @click="remove(row.id, row.name)"
              >
                <el-icon><Delete20Regular /></el-icon>
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <div v-else-if="!loading" class="token-empty-hint">
          尚未创建任何 API Token
        </div>
      </div>

      <!-- 新 Token 展示 -->
      <el-dialog v-model="showResult" title="API Token 已创建" width="500px" :close-on-click-modal="false">
        <div class="space-y-3">
          <div class="text-sm" style="color: var(--warning);">请立即复制保存，此 Token 仅显示一次！</div>
          <div class="flex items-center gap-2">
            <el-input :model-value="newToken" readonly class="flex-1 font-mono" />
            <el-button @click="copy(newToken)">复制</el-button>
          </div>
        </div>
        <template #footer>
          <el-button type="primary" @click="showResult = false" class="!border-0">我已保存</el-button>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<style scoped>
.token-create-form {
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: color-mix(in oklab, var(--muted) 50%, transparent);
}

.token-table {
  --el-table-border-color: var(--border);
  --el-table-header-bg-color: color-mix(in oklab, var(--muted) 50%, transparent);
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
  --el-table-row-hover-bg-color: var(--accent);
  --el-table-text-color: var(--foreground);
  --el-table-header-text-color: var(--muted-foreground);
}

.token-table :deep(.el-table__cell) {
  border-bottom: 1px solid var(--border);
}

.token-prefix {
  font-family: ui-monospace, monospace;
  font-size: 12px;
  color: var(--muted-foreground);
}

.token-empty-hint {
  font-size: 13px;
  color: var(--muted-foreground);
  text-align: center;
  padding: 24px 0;
}

.faq-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
}

.faq-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
}

.faq-header:hover {
  background: var(--accent);
}

.faq-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.faq-arrow {
  transition: transform 0.2s;
  color: var(--muted-foreground);
}

.faq-arrow.expanded {
  transform: rotate(180deg);
}

.faq-body {
  padding: 16px;
  border-top: 1px solid var(--border);
}

.faq-item-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.faq-item-desc {
  font-size: 12px;
  color: var(--muted-foreground);
  margin-top: 2px;
}

.settings-form-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--foreground);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
</style>
