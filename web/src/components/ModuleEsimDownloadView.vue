<script setup lang="ts">
import type { EsimChipInfo } from '../types/api'
import { ArrowLeft24Regular } from '@vicons/fluent'
import ModuleEsimDownload from './ModuleEsimDownload.vue'

const _props = defineProps<{
  deviceId: string
  chipInfo: EsimChipInfo | null
  deviceImei?: string
}>()

const emit = defineEmits<{
  'back': []
  'downloaded': []
}>()
</script>

<template>
  <div class="download-view">
    <!-- 头部 (60px) — 参照 preview-header -->
    <div class="download-header">
      <div class="download-header-left">
        <button class="download-header-back" title="返回" @click="emit('back')">
          <el-icon size="20"><ArrowLeft24Regular /></el-icon>
        </button>
        <div class="download-title">下载 eSIM</div>
      </div>
    </div>

    <!-- 下载内容 — 参照 preview-download-scroll -->
    <div class="download-body">
      <div v-if="!chipInfo" class="download-empty">
        <el-empty description="未检测到 eUICC，无法下载" :image-size="60" />
      </div>
      <ModuleEsimDownload
        v-else
        :device-id="deviceId"
        :chip-info="chipInfo"
        :device-imei="deviceImei"
        @downloaded="emit('downloaded')"
      />
    </div>

    <!-- 底部预留栏 — 参照 preview-footer -->
    <div class="download-footer" />
  </div>
</template>

<style scoped>
/* 容器 — 参照 preview-panel */
.download-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  flex: 1;
}

/* 头部 — 参照 preview-header (60px) */
.download-header {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 12px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.download-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

/* 返回按钮 — 参照 preview-header-icon (38x38, 圆角6px, 边框) */
.download-header-back {
  width: 38px;
  height: 38px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--background);
  color: var(--foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.12s;
}
.download-header-back:hover {
  border-color: var(--brand);
  color: var(--brand);
}

.download-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--foreground);
}

/* 下载内容 — 参照 preview-download-scroll */
.download-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 12px;
}

.download-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  flex: 1;
  min-height: 200px;
  color: var(--muted-foreground);
  font-size: 13px;
}

/* 底部预留栏 — 参照 preview-footer */
.download-footer {
  flex-shrink: 0;
  border-top: 1px solid var(--border);
  padding: 8px 12px;
  min-height: 40px;
}
</style>
