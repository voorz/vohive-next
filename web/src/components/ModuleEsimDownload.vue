<script setup lang="ts">
import { ref } from 'vue'
import type { EsimChipInfo } from '../types/api'
import ModuleEsimDownloadEuiccInfo from './ModuleEsimDownloadEuiccInfo.vue'
import ModuleEsimDownloadSingle from './ModuleEsimDownloadSingle.vue'
import ModuleEsimDownloadBatch from './ModuleEsimDownloadBatch.vue'

const props = defineProps<{
  deviceId: string
  chipInfo: EsimChipInfo | null
  deviceImei?: string
}>()

const emit = defineEmits<{
  downloaded: []
}>()

const mode = ref<'single' | 'batch'>('single')

function onDownloaded() {
  emit('downloaded')
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
        <span class="download-section-title">输入完整激活码/上传或通过相机扫描</span>
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
          @downloaded="onDownloaded"
        />
        <ModuleEsimDownloadBatch
          v-else
          :device-id="deviceId"
          :chip-info="chipInfo"
          @downloaded="onDownloaded"
        />
      </div>
    </div>
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
