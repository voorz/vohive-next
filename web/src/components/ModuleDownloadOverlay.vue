<script setup lang="ts">
import { computed, ref } from 'vue'
import { CheckmarkCircle24Filled, DismissCircle24Filled, SpinnerIos20Regular, ArrowDownload24Regular } from '@vicons/fluent'
import CarrierIcon from './CarrierIcon.vue'
import { useProviderLogo, autoDownloadIcon } from '../composables/useProviderLogo'
import { watch } from 'vue'

export interface PreviewData {
  metadata: {
    iccid: string
    profile_name: string
    service_provider_name: string
    profile_class?: string
    icon_base64?: string
    profile_owner_mcc?: string
    profile_owner_mnc?: string
    estimated_profile_size?: number
  }
  euicc_info2: {
    free_non_volatile_memory: number
    free_volatile_memory?: number
    installed_application?: number
  }
  cc_required: boolean
  euicc_cert_der?: string
  eum_cert_der?: string
}

export interface DoneData {
  iccid?: string
  profile_name?: string
  service_provider_name?: string
  free_non_volatile_memory?: number
}

const props = defineProps<{
  visible: boolean
  progress: number
  message: string
  error: string
  errorCode?: string
  errorDetails?: string
  subjectCode?: string
  reasonCode?: string
  subjectIdentifier?: string
  batchCurrent?: number
  batchTotal?: number
  phase?: 'preview-loading' | 'preview-ready' | 'downloading' | 'done' | 'error' | 'idle'
  previewData?: PreviewData | null
  doneData?: DoneData | null
}>()

const emit = defineEmits<{
  'close': []
  'cancel': []
  'confirm-download': [confirmationCode: string]
}>()

// 兼容旧逻辑：如果未传 phase，则从 progress/error 推断
const status = computed(() => {
  if (props.phase) return props.phase
  if (props.error) return 'error' as const
  if (props.progress >= 100) return 'done' as const
  return 'downloading' as const
})

const safeProgress = computed(() => Math.max(0, Math.min(props.progress, 100)))

const canClose = computed(() => status.value !== 'downloading' && status.value !== 'preview-loading')

// SM-DP+ 错误码行：[SM-DP+ Error (subjectCode, reasonCode)]
const smdpErrorCodeLine = computed(() => {
  if (!props.subjectCode && !props.reasonCode) return ''
  return `[SM-DP+ Error (${props.subjectCode || 'N/A'}, ${props.reasonCode || 'N/A'})]`
})

// SM-DP+ Identifier 行：[Identifier: xxx]
const smdpIdentifierLine = computed(() => {
  if (!props.subjectIdentifier) return ''
  return `[Identifier: ${props.subjectIdentifier}]`
})

// 弹窗标题
const titleText = computed(() => {
  switch (status.value) {
    case 'preview-loading': return '查询 Profile 信息'
    case 'preview-ready': return '确认下载 Profile'
    default: return '下载 eSIM Profile'
  }
})

// 格式化 ICCID（直接显示，不分段）
function formatIccid(iccid: string): string {
  if (!iccid) return '--'
  return iccid
}

// 格式化字节数
function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '--'
  const kb = 1024
  const mb = kb * 1024
  if (bytes >= mb) return `${(bytes / mb).toFixed(1)} MB`
  if (bytes >= kb) return `${(bytes / kb).toFixed(1)} KB`
  return `${bytes} B`
}

// 运营商 Logo
const { ready: logoReady } = useProviderLogo()
const previewPlmn = computed(() => {
  if (!logoReady.value || !props.previewData) return null
  const mcc = props.previewData.metadata.profile_owner_mcc
  const mnc = props.previewData.metadata.profile_owner_mnc
  if (!mcc || !mnc) return null
  return { mcc, mnc }
})
watch(previewPlmn, (val) => {
  if (val && props.previewData) {
    autoDownloadIcon(val.mcc, val.mnc, props.previewData.metadata.service_provider_name)
  }
})

// 预估大小
const estimatedSize = computed(() => {
  return props.previewData?.metadata.estimated_profile_size ?? 0
})

// 下载占用 = 下载前剩余(预览阶段) - 下载后剩余(done事件)
const doneConsumedBytes = computed(() => {
  const before = props.previewData?.euicc_info2.free_non_volatile_memory ?? 0
  const after = props.doneData?.free_non_volatile_memory ?? 0
  if (before <= 0 || after <= 0) return 0
  return Math.max(0, before - after)
})

// 空间不足警告：剩余空间 < 预估大小 * 1.25
const spaceWarning = computed(() => {
  if (!props.previewData) return null
  const free = props.previewData.euicc_info2.free_non_volatile_memory
  if (free <= 0) return null
  if (estimatedSize.value > 0 && free < estimatedSize.value * 1.25) {
    return `剩余空间不足（${formatBytes(free)}），预估需要 ${formatBytes(estimatedSize.value)}，可能导致安装失败`
  }
  if (free < 102400) {
    return `剩余空间不足（${formatBytes(free)}），可能导致安装失败`
  }
  return null
})

// 确认码输入
const confirmationCodeInput = ref('')

// 证书导出
function downloadCert(base64Der: string, prefix: string) {
  const binary = atob(base64Der)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  const blob = new Blob([bytes], { type: 'application/pkix-cert' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${prefix}.der`
  a.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="(v: boolean) => { if (!v && canClose) emit('close') }"
    :title="titleText"
    width="min(440px, 90vw)"
    :close-on-click-modal="false"
    :close-on-press-escape="canClose"
    :show-close="canClose"
    :align-center="true"
  >
    <div class="dl-body">
      <!-- ===== 预览加载中 ===== -->
      <template v-if="status === 'preview-loading'">
        <el-icon size="32" class="dl-spinner">
          <SpinnerIos20Regular />
        </el-icon>
        <div class="dl-status-text">{{ message || '正在查询...' }}</div>
      </template>

      <!-- ===== 预览结果（确认下载） ===== -->
      <template v-else-if="status === 'preview-ready' && previewData">
        <!-- 运营商 Logo + 名称 -->
        <div class="dl-preview-header">
          <div class="dl-preview-logo">
            <CarrierIcon v-if="previewPlmn" :mcc="previewPlmn.mcc" :mnc="previewPlmn.mnc" :name="previewData.metadata.service_provider_name" :size="42" />
            <span v-else class="dl-preview-logo-fallback">{{ (previewData.metadata.service_provider_name || '?').charAt(0).toUpperCase() }}</span>
          </div>
          <div class="dl-preview-name">
            {{ previewData.metadata.service_provider_name || '--' }}
          </div>
        </div>

        <!-- 信息卡片 -->
        <div class="dl-preview-card">
          <div class="dl-preview-row">
            <span class="dl-preview-label">ICCID</span>
            <span class="dl-preview-value">{{ formatIccid(previewData.metadata.iccid) }}</span>
          </div>
          <div v-if="previewData.metadata.profile_owner_mcc" class="dl-preview-row">
            <span class="dl-preview-label">PLMN</span>
            <span class="dl-preview-value">{{ previewData.metadata.profile_owner_mcc }} {{ previewData.metadata.profile_owner_mnc }}</span>
          </div>
          <div class="dl-preview-row">
            <span class="dl-preview-label">剩余</span>
            <span class="dl-preview-value">{{ formatBytes(previewData.euicc_info2.free_non_volatile_memory) }}</span>
          </div>
          <div v-if="estimatedSize > 0" class="dl-preview-row">
            <span class="dl-preview-label">预估</span>
            <span class="dl-preview-value accent">{{ formatBytes(estimatedSize) }}</span>
          </div>
        </div>

        <!-- 空间不足警告 -->
        <div v-if="spaceWarning" class="dl-preview-warning">
          {{ spaceWarning }}
        </div>

        <!-- 证书导出（开发者模式） -->
        <div v-if="previewData.euicc_cert_der || previewData.eum_cert_der" class="dl-preview-cert">
          <button v-if="previewData.euicc_cert_der" class="dl-cert-btn" @click="downloadCert(previewData.euicc_cert_der!, 'euiccCertificate')">
            <el-icon size="14"><ArrowDownload24Regular /></el-icon>
            eUICC 证书
          </button>
          <button v-if="previewData.eum_cert_der" class="dl-cert-btn" @click="downloadCert(previewData.eum_cert_der!, 'eumCertificate')">
            <el-icon size="14"><ArrowDownload24Regular /></el-icon>
            EUM 证书
          </button>
        </div>

        <!-- 确认码输入（条件显示） -->
        <div v-if="previewData.cc_required" class="dl-preview-cc-field">
          <label class="dl-preview-cc-label">确认码</label>
          <input
            v-model="confirmationCodeInput"
            class="dl-preview-cc-input"
            type="text"
            placeholder="请输入运营商确认码"
          />
        </div>
      </template>

      <!-- ===== 下载中 ===== -->
      <template v-else-if="status === 'downloading'">
        <el-icon size="32" class="dl-spinner">
          <SpinnerIos20Regular />
        </el-icon>
        <div class="dl-pct">{{ safeProgress }}%</div>
        <el-progress
          :percentage="safeProgress"
          :stroke-width="6"
          :show-text="false"
        />
        <div class="dl-status-text">{{ message }}</div>
        <div v-if="batchTotal && batchTotal > 1" class="dl-batch">
          {{ batchCurrent }} / {{ batchTotal }}
        </div>
        <div class="dl-hint">
          请勿关闭此窗口
        </div>
      </template>

      <!-- ===== 下载完成 ===== -->
      <template v-else-if="status === 'done'">
        <el-icon size="32" class="dl-done-icon">
          <CheckmarkCircle24Filled />
        </el-icon>
        <div class="dl-done-title">安装成功</div>
        <div class="dl-done-subtitle">{{ doneData?.profile_name || 'Profile 已成功安装到您的设备' }}</div>
        <!-- 空间信息卡片 -->
        <div class="dl-done-space">
          <div class="dl-done-space-item">
            <span class="dl-done-space-label">Profile占用</span>
            <span class="dl-done-space-value">{{ formatBytes(doneConsumedBytes) }}</span>
          </div>
          <div class="dl-done-space-item">
            <span class="dl-done-space-label">eUICC剩余</span>
            <span class="dl-done-space-value brand">{{ formatBytes(doneData?.free_non_volatile_memory || 0) }}</span>
          </div>
        </div>
      </template>

      <!-- ===== 错误 ===== -->
      <template v-else-if="status === 'error'">
        <el-icon size="32" class="dl-error-icon">
          <DismissCircle24Filled />
        </el-icon>
        <div class="dl-status-text">下载失败</div>
        <!-- 错误信息卡片 -->
        <div v-if="error" class="dl-error-card">
          <div class="dl-error-card-header">ERROR</div>
          <div v-if="smdpErrorCodeLine" class="dl-error-code-line">{{ smdpErrorCodeLine }}</div>
          <div class="dl-error-msg">{{ error }}</div>
          <div v-if="smdpIdentifierLine" class="dl-error-identifier-line">{{ smdpIdentifierLine }}</div>
        </div>
      </template>
    </div>

    <template #footer>
      <!-- 预览确认阶段：取消 + 下载 -->
      <template v-if="status === 'preview-ready'">
        <el-button size="default" @click="emit('cancel')">取消</el-button>
        <el-button size="default" type="primary" @click="emit('confirm-download', confirmationCodeInput)">
          下载
        </el-button>
      </template>
      <!-- 其他状态：关闭 -->
      <el-button
        v-else-if="canClose"
        size="default"
        @click="emit('close')"
      >
        关闭
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.dl-body {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
}

.dl-spinner {
  color: var(--brand);
  animation: dl-spin 0.8s linear infinite;
}

@keyframes dl-spin {
  to { transform: rotate(360deg); }
}

.dl-done-icon {
  color: var(--brand);
}

.dl-error-icon {
  color: var(--destructive);
}

.dl-pct {
  font-size: 22px;
  font-weight: 700;
  color: var(--foreground);
  line-height: 1;
}

.dl-status-text {
  font-size: 13px;
  color: var(--muted-foreground);
  text-align: center;
}

.dl-batch {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted-foreground);
}

.dl-hint {
  font-size: 11px;
  color: var(--muted-foreground);
  text-align: center;
  opacity: 0.7;
}

/* ===== Preview 状态样式 ===== */
.dl-preview-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.dl-preview-logo {
  width: 48px;
  height: 48px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.dl-preview-logo-fallback {
  font-size: 16px;
  font-weight: 700;
  color: var(--foreground);
}

.dl-preview-name {
  font-size: 18px;
  font-weight: 700;
  color: var(--foreground);
  text-align: center;
}

.dl-preview-card {
  width: 100%;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 10px 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: var(--muted);
}

.dl-preview-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.dl-preview-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
}

.dl-preview-value {
  font-size: 12px;
  color: var(--foreground);
}

.dl-preview-value.accent {
  color: var(--brand);
}


.dl-preview-warning {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid #f59e0b;
  border-radius: 6px;
  background: rgba(245, 158, 11, 0.1);
  color: #d97706;
  font-size: 11px;
  font-weight: 600;
  text-align: center;
}

.dl-preview-cc-field {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.dl-preview-cc-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
}

.dl-preview-cc-input {
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--background);
  color: var(--foreground);
  font-size: 12px;
  outline: none;
  transition: border-color 0.12s;
}
.dl-preview-cc-input:focus {
  border-color: var(--brand);
}

/* ===== 证书导出按钮 ===== */
.dl-preview-cert {
  display: flex;
  gap: 8px;
  width: 100%;
}

.dl-cert-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--background);
  color: var(--muted-foreground);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.12s;
}
.dl-cert-btn:hover {
  border-color: var(--brand);
  color: var(--brand);
}

/* ===== Done 状态样式 ===== */
.dl-done-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--foreground);
}

.dl-done-subtitle {
  font-size: 13px;
  color: var(--muted-foreground);
  text-align: center;
}

.dl-done-space {
  display: flex;
  gap: 24px;
  align-items: center;
  padding: 10px 20px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--muted);
}

.dl-done-space-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.dl-done-space-label {
  font-size: 11px;
  color: var(--muted-foreground);
}

.dl-done-space-value {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.dl-done-space-value.brand {
  color: var(--brand);
}

.dl-done-profile-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted-foreground);
  margin-top: 4px;
  text-align: center;
}

/* ===== Error 样式 ===== */
.dl-error-card {
  width: 100%;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  background: var(--muted);
}

.dl-error-card-header {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 1px;
  color: var(--destructive);
}

.dl-error-msg {
  font-size: 12px;
  color: var(--foreground);
  text-align: center;
  line-height: 1.5;
  word-break: break-word;
}

.dl-error-code-line {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
  text-align: center;
  line-height: 1.5;
}

.dl-error-identifier-line {
  font-size: 11px;
  color: var(--muted-foreground);
  text-align: center;
  line-height: 1.5;
  opacity: 0.8;
}
</style>
