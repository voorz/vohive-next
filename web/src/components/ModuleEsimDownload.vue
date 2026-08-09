<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { devicesService } from '../services/devices'
import { errorMessage } from '../services/http'
import { api } from '../stores/auth'
import { pickNextDownloadAid } from './deviceEsimOverviewRefresh'
import { describeDownloadTerminalNotice } from './deviceEsimOperationNotice'
import type { EsimChipInfo, EsimSpaceDelta } from '../types/api'
import { ArrowDownload24Regular, Dismiss24Regular } from '@vicons/fluent'

const props = defineProps<{
  deviceId: string
  chipInfo: EsimChipInfo | null
  deviceImei?: string
}>()

const emit = defineEmits<{
  downloaded: []
}>()

const downloadForm = ref({
  smdp: '',
  matchingId: '',
  confirmationCode: '',
  aidHex: '',
  imei: ''
})
const downloading = ref(false)
const downloadProgress = ref(0)
const downloadMsg = ref('')
const downloadError = ref('')
const downloadSessionId = ref(0)

let lastDeviceImeiDefault = ''

function applyDeviceImeiDefault(force = false) {
  const next = (props.deviceImei || '').trim()
  if (force || !downloadForm.value.imei || downloadForm.value.imei === lastDeviceImeiDefault) {
    downloadForm.value.imei = next
  }
  lastDeviceImeiDefault = next
}

// 智能解析 LPA 激活码
watch(() => downloadForm.value.smdp, (newVal) => {
  if (!newVal) return
  if (newVal.startsWith('LPA:')) {
    const parts = newVal.split('$')
    if (parts.length >= 3) {
      downloadForm.value.smdp = parts[1]
      downloadForm.value.matchingId = parts[2]
      ElMessage.success('已自动解析完整的 LPA 激活码')
    }
  } else if (newVal.startsWith('http://') || newVal.startsWith('https://')) {
    downloadForm.value.smdp = newVal.replace(/^https?:\/\//i, '')
  }
})

watch(() => props.deviceImei, () => applyDeviceImeiDefault(false))

watch(() => props.chipInfo, () => {
  downloadForm.value.aidHex = pickNextDownloadAid(props.chipInfo, downloadForm.value.aidHex)
}, { immediate: true })

applyDeviceImeiDefault(true)

function clearForm() {
  downloadForm.value.smdp = ''
  downloadForm.value.matchingId = ''
  downloadForm.value.confirmationCode = ''
  downloadForm.value.imei = lastDeviceImeiDefault
  downloadError.value = ''
  downloadProgress.value = 0
  downloadMsg.value = ''
}

async function downloadProfile() {
  const { smdp, matchingId, confirmationCode, aidHex, imei } = downloadForm.value
  const targetAidHex = aidHex || pickNextDownloadAid(props.chipInfo, '')
  if (!smdp) {
    ElMessage.warning('请输入 SM-DP+ 地址')
    return
  }

  downloadSessionId.value++
  downloading.value = true
  downloadProgress.value = 0
  downloadMsg.value = '正在连接...'
  downloadError.value = ''

  const params = new URLSearchParams({ smdp })
  if (matchingId) params.set('matching_id', matchingId)
  if (confirmationCode) params.set('confirmation_code', confirmationCode)
  if (targetAidHex) params.set('aid_hex', targetAidHex)
  if (imei.trim()) params.set('imei', imei.trim())

  const base = api.defaults.baseURL || ''
  const url = `${base}/devices/${props.deviceId}/esim/actions/download?${params}`
  const token = localStorage.getItem('token') || ''

  try {
    const res = await fetch(url, {
      method: 'GET',
      headers: { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' }
    })
    if (!res.ok) {
      const text = await res.text()
      throw new Error(text || `HTTP ${res.status}`)
    }
    if (!res.body) throw new Error('No stream body')

    const reader = res.body.getReader()
    const decoder = new TextDecoder('utf-8')
    let buffer = ''

    outer: while (true) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      while (true) {
        const nl = buffer.indexOf('\n')
        if (nl < 0) break
        let line = buffer.slice(0, nl)
        buffer = buffer.slice(nl + 1)
        if (line.endsWith('\r')) line = line.slice(0, -1)
        if (!line.startsWith('data:')) continue

        const payload = line.slice('data:'.length).trim()
        try {
          const evt = JSON.parse(payload) as { step: string; msg: string; pct: number; code?: string; warning?: string; space_delta?: EsimSpaceDelta }
          if (evt.step === 'error') {
            downloadError.value = evt.code === 'euicc_insufficient_memory'
              ? 'eUICC 安装 profile 时空间不足，请删除未使用的 profile 后重试。'
              : evt.msg
            break outer
          }
          downloadProgress.value = evt.pct
          downloadMsg.value = evt.msg
          if (evt.step === 'done') {
            const notice = describeDownloadTerminalNotice(evt)
            if (notice.tone === 'warning') {
              ElMessage.warning(notice.message)
            } else {
              ElMessage.success(notice.message)
            }
            downloadForm.value = { smdp: '', matchingId: '', confirmationCode: '', aidHex: targetAidHex, imei }
            emit('downloaded')
            break outer
          }
        } catch { /* 非 JSON 行，忽略 */ }
      }
    }
  } catch (e: unknown) {
    if (!downloadError.value) {
      downloadError.value = errorMessage(e, '下载失败')
    }
  } finally {
    downloading.value = false
  }
}
</script>

<template>
  <div class="download-panel">
    <!-- EUICC 信息（折叠摘要） -->
    <div v-if="chipInfo" class="download-euicc-summary">
      <span class="download-euicc-label">目标 eUICC</span>
      <select v-model="downloadForm.aidHex" class="download-euicc-select">
        <option value="">自动选择</option>
        <option
          v-for="(eid, ei) in (chipInfo?.eids || [])"
          :key="eid.aid"
          :value="eid.aid"
        >
          #{{ Number(ei) + 1 }} (...{{ eid.eid.slice(-4) }}) — {{ eid.free_nvram }}
        </option>
      </select>
    </div>

    <!-- LPA 完整激活码 -->
    <div class="download-field">
      <label class="download-label">完整激活码 / SM-DP+ 地址 *</label>
      <input
        v-model="downloadForm.smdp"
        class="download-input"
        type="text"
        placeholder="LPA:1$smdp.example$XXXX 或 smdp.example"
      />
    </div>

    <!-- Matching ID -->
    <div class="download-field">
      <label class="download-label">Matching ID</label>
      <input
        v-model="downloadForm.matchingId"
        class="download-input"
        type="text"
        placeholder="可选"
      />
    </div>

    <!-- 确认码 -->
    <div class="download-field">
      <label class="download-label">确认码</label>
      <input
        v-model="downloadForm.confirmationCode"
        class="download-input"
        type="text"
        placeholder="可选"
      />
    </div>

    <!-- IMEI -->
    <div class="download-field">
      <label class="download-label">IMEI</label>
      <input
        v-model="downloadForm.imei"
        class="download-input"
        type="text"
        maxlength="15"
        placeholder="默认使用设备 IMEI"
      />
    </div>

    <!-- 进度条 -->
    <div v-if="downloading || downloadError || downloadProgress > 0" class="download-progress-wrap">
      <div class="download-progress-track">
        <div
          class="download-progress-fill"
          :class="{ error: !!downloadError }"
          :style="{ width: downloadProgress + '%' }"
        />
      </div>
      <div class="download-progress-text" :class="{ error: !!downloadError }">
        {{ downloadError || downloadMsg || `${downloadProgress}%` }}
      </div>
    </div>

    <!-- 操作按钮 -->
    <div class="download-actions">
      <button class="download-btn clear" @click="clearForm" :disabled="downloading">
        <el-icon size="14"><Dismiss24Regular /></el-icon>
        清空
      </button>
      <button class="download-btn primary" @click="downloadProfile" :disabled="downloading">
        <el-icon size="14"><ArrowDownload24Regular /></el-icon>
        {{ downloading ? '下载中...' : '下载' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.download-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
}

.download-euicc-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.download-euicc-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
  flex-shrink: 0;
}

.download-euicc-select {
  flex: 1;
  min-width: 0;
  padding: 4px 6px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--background);
  color: var(--foreground);
  font-size: 11px;
  outline: none;
  cursor: pointer;
}
.download-euicc-select:focus {
  border-color: var(--brand);
}

.download-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.download-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
}

.download-input {
  padding: 6px 8px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--background);
  color: var(--foreground);
  font-size: 12px;
  outline: none;
  transition: border-color 0.12s;
}
.download-input:focus {
  border-color: var(--brand);
}
.download-input::placeholder {
  color: var(--muted-foreground);
  opacity: 0.6;
}

.download-progress-wrap {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.download-progress-track {
  height: 8px;
  border-radius: 999px;
  background: var(--muted);
  overflow: hidden;
}

.download-progress-fill {
  height: 100%;
  border-radius: 999px;
  background: var(--brand);
  transition: width 0.3s ease;
}
.download-progress-fill.error {
  background: #ef4444;
}

.download-progress-text {
  font-size: 11px;
  color: var(--muted-foreground);
}
.download-progress-text.error {
  color: #ef4444;
}

.download-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}

.download-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 14px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: transparent;
  color: var(--muted-foreground);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.12s;
}
.download-btn:hover {
  background: var(--background);
  color: var(--foreground);
}
.download-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.download-btn.primary {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
}
.download-btn.primary:hover {
  opacity: 0.9;
}
.download-btn.primary:disabled {
  opacity: 0.5;
}
</style>
