<script setup lang="ts">
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../stores/auth'
import { pickNextDownloadAid } from './deviceEsimOverviewRefresh'
import type { EsimChipInfo } from '../types/api'
import { Dismiss24Regular, ArrowDownload24Regular } from '@vicons/fluent'

const props = defineProps<{
  deviceId: string
  chipInfo: EsimChipInfo | null
}>()

const emit = defineEmits<{
  downloaded: []
}>()

const lpaList = ref('')
const downloading = ref(false)
const batchCurrent = ref(0)
const batchTotal = ref(0)
const batchProgress = ref(0)
const batchMsg = ref('')
const batchError = ref('')

const parsedLines = computed(() => {
  return lpaList.value
    .split('\n')
    .map(s => s.trim())
    .filter(s => s.length > 0)
})

function onQrScanned(data: string) {
  lpaList.value = lpaList.value
    ? lpaList.value + '\n' + data
    : data
  ElMessage.success('二维码识别成功，已添加到列表')
}

function clearForm() {
  lpaList.value = ''
  batchError.value = ''
  batchProgress.value = 0
  batchMsg.value = ''
  batchCurrent.value = 0
  batchTotal.value = 0
}

function parseLPA(line: string): { smdp: string; matchingId: string } | null {
  const trimmed = line.trim()
  if (trimmed.startsWith('LPA:')) {
    const parts = trimmed.split('$')
    if (parts.length >= 3) {
      return { smdp: parts[1], matchingId: parts[2] }
    }
  }
  return { smdp: trimmed, matchingId: '' }
}

async function downloadBatch() {
  const lines = parsedLines.value
  if (lines.length === 0) {
    ElMessage.warning('请输入至少一行 LPA 激活码')
    return
  }

  downloading.value = true
  batchTotal.value = lines.length
  batchCurrent.value = 0
  batchProgress.value = 0
  batchMsg.value = ''
  batchError.value = ''

  const targetAidHex = pickNextDownloadAid(props.chipInfo, '')
  const base = api.defaults.baseURL || ''
  const token = localStorage.getItem('token') || ''
  let successCount = 0

  for (let i = 0; i < lines.length; i++) {
    batchCurrent.value = i + 1
    const parsed = parseLPA(lines[i])
    if (!parsed || !parsed.smdp) {
      ElMessage.warning(`第 ${i + 1} 行格式无效，跳过`)
      continue
    }

    batchMsg.value = `正在下载 ${parsed.smdp}...`

    const params = new URLSearchParams({ smdp: parsed.smdp })
    if (parsed.matchingId) params.set('matching_id', parsed.matchingId)
    if (targetAidHex) params.set('aid_hex', targetAidHex)
    const url = `${base}/devices/${props.deviceId}/esim/actions/download?${params}`

    try {
      const res = await fetch(url, {
        method: 'GET',
        headers: { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' }
      })
      if (!res.ok || !res.body) continue

      const reader = res.body.getReader()
      const decoder = new TextDecoder('utf-8')
      let buffer = ''

      while (true) {
        const { value, done } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        while (true) {
          const nl = buffer.indexOf('\n')
          if (nl < 0) break
          let lineData = buffer.slice(0, nl)
          buffer = buffer.slice(nl + 1)
          if (lineData.endsWith('\r')) lineData = lineData.slice(0, -1)
          if (!lineData.startsWith('data:')) continue
          const payload = lineData.slice('data:'.length).trim()
          try {
            const evt = JSON.parse(payload) as { step: string; pct: number; msg: string }
            batchProgress.value = Math.round(((i + evt.pct / 100) / lines.length) * 100)
            batchMsg.value = evt.msg
            if (evt.step === 'done') {
              successCount++
              break
            }
            if (evt.step === 'error') {
              batchError.value = `第 ${i + 1} 个下载失败: ${evt.msg}`
              break
            }
          } catch { /* 忽略 */ }
        }
      }
    } catch {
      // 继续下一个
    }
  }

  batchProgress.value = 100
  batchMsg.value = `批量下载完成 (${successCount}/${lines.length})`
  if (successCount > 0) {
    ElMessage.success(`成功下载 ${successCount} 个 Profile`)
    emit('downloaded')
  }
  downloading.value = false
}
</script>

<template>
  <div class="batch-download">
    <!-- 多行激活码输入 -->
    <div class="batch-field">
      <label class="batch-label">完整激活码</label>
      <textarea
        v-model="lpaList"
        class="batch-textarea"
        rows="6"
        placeholder="LPA:1$smdp.example$XXXX-XXXX-XXXX&#10;LPA:1$smdp2.example$YYYY-YYYY-YYYY&#10;..."
      />
    </div>

    <!-- 进度条 -->
    <div v-if="downloading || batchError || batchProgress > 0" class="batch-progress-wrap">
      <div v-if="batchTotal > 0" class="batch-progress-count">
        批量下载中 ({{ batchCurrent }}/{{ batchTotal }})
      </div>
      <div class="batch-progress-track">
        <div
          class="batch-progress-fill"
          :class="{ error: !!batchError }"
          :style="{ width: batchProgress + '%' }"
        />
      </div>
      <div class="batch-progress-text" :class="{ error: !!batchError }">
        {{ batchError || batchMsg || `${batchProgress}%` }}
      </div>
    </div>

    <!-- 操作按钮 -->
    <div class="batch-actions">
      <button class="batch-btn clear" @click="clearForm" :disabled="downloading">
        <el-icon size="14"><Dismiss24Regular /></el-icon>
        清空
      </button>
      <button class="batch-btn primary" @click="downloadBatch" :disabled="downloading">
        <el-icon size="14"><ArrowDownload24Regular /></el-icon>
        {{ downloading ? '下载中...' : '下载' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.batch-download {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.batch-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.batch-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.batch-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
}

.batch-textarea {
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--background);
  color: var(--foreground);
  font-size: 12px;
  font-family: var(--oomol-font-mono);
  outline: none;
  resize: vertical;
  transition: border-color 0.12s;
  line-height: 1.6;
}
.batch-textarea:focus {
  border-color: var(--brand);
}
.batch-textarea::placeholder {
  color: var(--muted-foreground);
  opacity: 0.6;
}

.batch-progress-wrap {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.batch-progress-count {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
}
.batch-progress-track {
  height: 8px;
  border-radius: 999px;
  background: var(--muted);
  overflow: hidden;
}
.batch-progress-fill {
  height: 100%;
  border-radius: 999px;
  background: var(--brand);
  transition: width 0.3s ease;
}
.batch-progress-fill.error {
  background: #ef4444;
}
.batch-progress-text {
  font-size: 11px;
  color: var(--muted-foreground);
}
.batch-progress-text.error {
  color: #ef4444;
}

.batch-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}

.batch-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 14px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: transparent;
  color: var(--muted-foreground);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.12s;
}
.batch-btn:hover {
  background: var(--background);
  color: var(--foreground);
}
.batch-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.batch-btn.primary {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
}
.batch-btn.primary:hover {
  opacity: 0.9;
}
.batch-btn.primary:disabled {
  opacity: 0.5;
}
</style>
