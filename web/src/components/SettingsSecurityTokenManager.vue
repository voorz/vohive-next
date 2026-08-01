<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ChevronDown20Regular } from '@vicons/fluent'
import { systemService } from '../services/system'

const expanded = ref(true)
const creating = ref(false)
const ttlInput = ref(0)
const showResult = ref(false)
const newToken = ref('')
const tokenInfo = ref({ has_token: false, expiry: 0 })

onMounted(async () => {
  await loadTokenInfo()
})

async function loadTokenInfo() {
  try {
    const res = await systemService.getSecurity()
    if (res.ok) {
      tokenInfo.value = {
        has_token: res.data.has_api_token,
        expiry: res.data.api_token_expiry,
      }
    }
  } catch { /* keep defaults */ }
}

async function create() {
  creating.value = true
  try {
    const res = await systemService.createAPIToken(ttlInput.value)
    if (!res.ok) throw new Error(res.error.message || '创建失败')
    newToken.value = res.data.token
    showResult.value = true
    await loadTokenInfo()
  } catch (e: any) {
    ElMessage.error(e.message || '创建失败')
  } finally {
    creating.value = false
  }
}

async function remove() {
  try {
    await ElMessageBox.confirm('确定要删除当前 API Token 吗？删除后使用该 Token 的 API 请求将失效。', '删除确认', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch { return }
  try {
    const res = await systemService.deleteAPIToken()
    if (!res.ok) throw new Error(res.error.message || '删除失败')
    ElMessage.success('API Token 已删除')
    await loadTokenInfo()
  } catch (e: any) {
    ElMessage.error(e.message || '删除失败')
  }
}

async function copy(text: string) {
  // 优先 Clipboard API，fallback 到 execCommand
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

      <!-- 当前 Token 状态 -->
      <div v-if="tokenInfo.has_token" class="flex items-center gap-3 mb-4">
        <el-tag type="success" size="small">已创建</el-tag>
        <span v-if="tokenInfo.expiry > 0" class="text-xs" style="color: var(--muted-foreground);">
          过期时间: {{ new Date(tokenInfo.expiry * 1000).toLocaleString('zh-CN') }}
        </span>
        <span v-else class="text-xs" style="color: var(--muted-foreground);">永不过期</span>
        <el-button size="small" type="danger" plain @click="remove">删除 Token</el-button>
      </div>
      <div v-else class="text-sm mb-4" style="color: var(--muted-foreground);">
        尚未创建 API Token
      </div>

      <!-- 创建新 Token -->
      <div class="flex items-end gap-3">
        <div class="space-y-1">
          <label class="settings-form-label">有效期（小时，0=永不过期）</label>
          <el-input-number v-model="ttlInput" :min="0" :max="8760" size="default" controls-position="right" />
        </div>
        <el-button type="primary" :loading="creating" @click="create" class="!border-0">
          {{ tokenInfo.has_token ? '刷新 Token' : '创建 Token' }}
        </el-button>
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
