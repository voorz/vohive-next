<script setup lang="ts">
import { ref } from 'vue'
import type { EsimChipInfo } from '../types/api'
import ModuleEsimDownloadEuiccInfo from './ModuleEsimDownloadEuiccInfo.vue'
import ModuleEsimDownloadSingle from './ModuleEsimDownloadSingle.vue'
import ModuleEsimDownloadBatch from './ModuleEsimDownloadBatch.vue'
import ModuleDownloadOverlay from './ModuleDownloadOverlay.vue'

const _props = defineProps<{
  deviceId: string
  chipInfo: EsimChipInfo | null
  deviceImei?: string
}>()

const emit = defineEmits<{
  downloaded: []
}>()

const mode = ref<'single' | 'batch'>('single')

// ── 全屏遮罩状态 ──
const overlayVisible = ref(false)
const overlayProgress = ref(0)
const overlayMessage = ref('')
const overlayError = ref('')
const overlayErrorCode = ref('')
const overlayErrorDetails = ref('')
const overlaySubjectCode = ref('')
const overlayReasonCode = ref('')
const overlaySubjectIdentifier = ref('')
const overlayBatchCurrent = ref(0)
const overlayBatchTotal = ref(0)

function onProgress(p: { pct: number; msg: string; error?: string; errorCode?: string; errorDetails?: string; subjectCode?: string; reasonCode?: string; subjectIdentifier?: string; batchCurrent?: number; batchTotal?: number }) {
  overlayVisible.value = true
  overlayProgress.value = p.pct
  overlayMessage.value = p.msg
  overlayError.value = p.error || ''
  overlayErrorCode.value = p.errorCode || ''
  overlayErrorDetails.value = p.errorDetails || ''
  overlaySubjectCode.value = p.subjectCode || ''
  overlayReasonCode.value = p.reasonCode || ''
  overlaySubjectIdentifier.value = p.subjectIdentifier || ''
  if (p.batchCurrent !== undefined) overlayBatchCurrent.value = p.batchCurrent
  if (p.batchTotal !== undefined) overlayBatchTotal.value = p.batchTotal
}

function onDownloaded() {
  // 下载完成：保持 overlay 显示最终状态 1.5s 后关闭
  setTimeout(() => {
    overlayVisible.value = false
  }, 1500)
  emit('downloaded')
}

function closeOverlay() {
  overlayVisible.value = false
  overlayProgress.value = 0
  overlayMessage.value = ''
  overlayError.value = ''
  overlayErrorCode.value = ''
  overlayErrorDetails.value = ''
  overlaySubjectCode.value = ''
  overlayReasonCode.value = ''
  overlaySubjectIdentifier.value = ''
  overlayBatchCurrent.value = 0
  overlayBatchTotal.value = 0
}
</script>

<template>
  <div class="download-container">
    <!-- EUICC INFO 折叠区域 -->
    <ModuleEsimDownloadEuiccInfo :chip-info="chipInfo" />

    <!-- 下载区域 -->
    <div class="download-section">
      <!-- 标题 + 模式切换 -->
      <div class="download-section-header">
        <span class="download-section-title">下载eSIM Profile</span>
        <div class="download-mode-switch">
          <button
            class="download-mode-btn"
            :class="{ active: mode === 'single' }"
            @click="mode = 'single'"
          >单个下载</button>
          <button
            class="download-mode-btn"
            :class="{ active: mode === 'batch' }"
            @click="mode = 'batch'"
          >批量下载</button>
        </div>
      </div>

      <!-- 下载表单 -->
      <div class="download-section-body">
        <ModuleEsimDownloadSingle
          v-if="mode === 'single'"
          :device-id="deviceId"
          :chip-info="chipInfo"
          :device-imei="deviceImei"
          @progress="onProgress"
          @downloaded="onDownloaded"
        />
        <ModuleEsimDownloadBatch
          v-else
          :device-id="deviceId"
          :chip-info="chipInfo"
          @progress="onProgress"
          @downloaded="onDownloaded"
        />
      </div>
    </div>

    <!-- 全屏下载遮罩 -->
    <ModuleDownloadOverlay
      :visible="overlayVisible"
      :progress="overlayProgress"
      :message="overlayMessage"
      :error="overlayError"
      :error-code="overlayErrorCode"
      :error-details="overlayErrorDetails"
      :subject-code="overlaySubjectCode"
      :reason-code="overlayReasonCode"
      :subject-identifier="overlaySubjectIdentifier"
      :batch-current="overlayBatchCurrent"
      :batch-total="overlayBatchTotal"
      @close="closeOverlay"
    />
  </div>
</template>

<style scoped>
.download-container {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.download-section {
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: hidden;
}

.download-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  background: var(--muted);
  border-bottom: 1px solid var(--border);
}

.download-section-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--foreground);
}

.download-mode-switch {
  display: flex;
  gap: 2px;
  flex-shrink: 0;
}

.download-mode-btn {
  padding: 3px 10px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: transparent;
  color: var(--muted-foreground);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.12s;
}
.download-mode-btn:hover {
  background: var(--accent);
  color: var(--foreground);
}
.download-mode-btn.active {
  background: var(--background);
  border-color: var(--border);
  color: var(--foreground);
  box-shadow: var(--console-shadow-sm);
}

.download-section-body {
  padding: 10px;
}
</style>
