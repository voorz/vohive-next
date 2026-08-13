<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { devicesService } from '../services/devices'
import { errorMessage } from '../services/http'
import { api } from '../stores/auth'
import { pickNextDownloadAid } from './deviceEsimOverviewRefresh'
import { describeDownloadTerminalNotice } from './deviceEsimOperationNotice'
import type { EsimChipInfo, EsimSpaceDelta } from '../types/api'
import { Dismiss24Regular, ArrowDownload24Regular } from '@vicons/fluent'
import ModuleEsimDownloadQrScanner from './ModuleEsimDownloadQrScanner.vue'

const props = defineProps<{
  deviceId: string
  chipInfo: EsimChipInfo | null
  deviceImei?: string
}>()

const emit = defineEmits<{
  downloaded: []
}>()

const lpaCode = ref('')
const smdp = ref('')
const matchingId = ref('')
const confirmationCode = ref('')
const imei = ref(props.deviceImei || '')
const aidHex = ref('')

let lastDeviceImeiDefault = ''

function applyDeviceImeiDefault(force = false) {
  const next = (props.deviceImei || '').trim()
  if (force || !imei.value || imei.value === lastDeviceImeiDefault) {
    imei.value = next
  }
  lastDeviceImeiDefault = next
}

// 智能解析 LPA 激活码 → 自动填入 SM-DP+ 和 Matching ID
watch(lpaCode, (val) => {
  const trimmed = val.trim()
  if (trimmed.startsWith('LPA:')) {
    const parts = trimmed.split('$')
    if (parts.length >= 3) {
      smdp.value = parts[1]
      matchingId.value = parts[2]
      ElMessage.success('已自动解析完整的 LPA 激活码')
    }
  }
})

// QR 扫描结果填入激活码
function onQrScanned(data: string) {
  if (data.startsWith('LPA:')) {
    lpaCode.value = data
    ElMessage.success('二维码识别成功')
  } else {
    lpaCode.value = data
    smdp.value = data
  }
}

watch(() => props.deviceImei, () => applyDeviceImeiDefault(false))
watch(() => props.chipInfo, () => {
  aidHex.value = pickNextDownloadAid(props.chipInfo, aidHex.value)
}, { immediate: true })

applyDeviceImeiDefault(true)

const downloading = ref(false)
const downloadProgress = ref(0)
const downloadMsg = ref('')
const downloadError = ref('')

function clearForm() {
  lpaCode.value = ''
  smdp.value = ''
  matchingId.value = ''
  confirmationCode.value = ''
  imei.value = lastDeviceImeiDefault
  downloadError.value = ''
  downloadProgress.value = 0
  downloadMsg.value = ''
}

async function downloadProfile() {
  const targetAidHex = aidHex.value || pickNextDownloadAid(props.chipInfo, '')
  if (!smdp.value) {
    ElMessage.warning('请输入 SM-DP+ 地址')
    return
  }

  downloading.value = true
  downloadProgress.value = 0
  downloadMsg.value = '正在连接...'
  downloadError.value = ''

  const params = new URLSearchParams({ smdp: smdp.value })
  if (matchingId.value) params.set('matching_id', matchingId.value)
  if (confirmationCode.value) params.set('confirmation_code', confirmationCode.value)
  if (targetAidHex) params.set('aid_hex', targetAidHex)
  if (imei.value.trim()) params.set('imei', imei.value.trim())

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
          const evt = JSON.parse(payload) as { step: string; msg: string; pct: number; code?: string; space_delta?: EsimSpaceDelta }
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
  <div class="single-download">
    <!-- 二维码上传/扫描 -->
    <div class="single-section">
      <label class="single-label">上传或通过相机扫描</label>
      <ModuleEsimDownloadQrScanner @scanned="onQrScanned" />
    </div>

    <!-- 完整激活码 -->
    <div class="single-field">
      <label class="single-label">完整激活码</label>
      <input
        v-model="lpaCode"
        class="single-input"
        type="text"
        placeholder="LPA:1$smdp.example$XXXX-XXXX-XXXX"
      />
    </div>

    <!-- SM-DP+ 地址 -->
    <div class="single-field">
      <label class="single-label">SM-DP+ 地址</label>
      <input
        v-model="smdp"
        class="single-input"
        type="text"
        placeholder="请输入"
      />
    </div>

    <!-- Matching ID -->
    <div class="single-field">
      <label class="single-label">Matching ID</label>
      <input
        v-model="matchingId"
        class="single-input"
        type="text"
        placeholder="请输入"
      />
    </div>

    <!-- 确认码(可选) -->
    <div class="single-field">
      <label class="single-label">确认码<span class="single-optional">(可选)</span></label>
      <input
        v-model="confirmationCode"
        class="single-input"
        type="text"
        placeholder="运营商确认码"
      />
    </div>

    <!-- 进度条 -->
    <div v-if="downloading || downloadError || downloadProgress > 0" class="single-progress-wrap">
      <div class="single-progress-track">
        <div
          class="single-progress-fill"
          :class="{ error: !!downloadError }"
          :style="{ width: downloadProgress + '%' }"
        />
      </div>
      <div class="single-progress-text" :class="{ error: !!downloadError }">
        {{ downloadError || downloadMsg || `${downloadProgress}%` }}
      </div>
    </div>

    <!-- 操作按钮 -->
    <div class="single-actions">
      <button class="single-btn clear" @click="clearForm" :disabled="downloading">
        <el-icon size="14"><Dismiss24Regular /></el-icon>
        清空
      </button>
      <button class="single-btn primary" @click="downloadProfile" :disabled="downloading">
        <el-icon size="14"><ArrowDownload24Regular /></el-icon>
        {{ downloading ? '下载中...' : '下载' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.single-download {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.single-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.single-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.single-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
}
.single-optional {
  opacity: 0.6;
  font-weight: 400;
}

.single-input {
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--background);
  color: var(--foreground);
  font-size: 12px;
  outline: none;
  transition: border-color 0.12s;
}
.single-input:focus {
  border-color: var(--brand);
}
.single-input::placeholder {
  color: var(--muted-foreground);
  opacity: 0.6;
}

.single-progress-wrap {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.single-progress-track {
  height: 8px;
  border-radius: 999px;
  background: var(--muted);
  overflow: hidden;
}
.single-progress-fill {
  height: 100%;
  border-radius: 999px;
  background: var(--brand);
  transition: width 0.3s ease;
}
.single-progress-fill.error {
  background: #ef4444;
}
.single-progress-text {
  font-size: 11px;
  color: var(--muted-foreground);
}
.single-progress-text.error {
  color: #ef4444;
}

.single-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}

.single-btn {
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
.single-btn:hover {
  background: var(--background);
  color: var(--foreground);
}
.single-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.single-btn.primary {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
}
.single-btn.primary:hover {
  opacity: 0.9;
}
.single-btn.primary:disabled {
  opacity: 0.5;
}
</style>
