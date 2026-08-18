<script setup lang="ts">
import { ref, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox, ElLoading } from 'element-plus'
import { systemService, type ReleaseItem } from '../services/system'
import {
  FolderOpen24Regular,
  ArrowDownload24Regular,
  Cloud24Regular,
  Desktop24Regular,
  Dismiss24Regular,
  CheckmarkCircle24Regular,
} from '@vicons/fluent'

const props = defineProps<{
  repoAvailable: boolean
}>()

// ── 切换 ──
const activeMode = ref<'local' | 'online'>('local')

// ── 本地更新 ──
const selectedFile = ref<File | null>(null)
const dragOver = ref(false)
const installing = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)

function triggerFileInput() {
  fileInputRef.value?.click()
}

function onFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files && input.files.length > 0) {
    selectedFile.value = input.files[0]
  }
}

function onDrop(event: DragEvent) {
  event.preventDefault()
  dragOver.value = false
  if (selectedFile.value) return
  if (event.dataTransfer && event.dataTransfer.files.length > 0) {
    selectedFile.value = event.dataTransfer.files[0]
  }
}

function onDragOver(event: DragEvent) {
  event.preventDefault()
  if (selectedFile.value) return
  dragOver.value = true
}

function onDragLeave(event: DragEvent) {
  event.preventDefault()
  dragOver.value = false
}

function clearFile() {
  selectedFile.value = null
  if (fileInputRef.value) fileInputRef.value.value = ''
}

async function installLocal() {
  if (!selectedFile.value) return

  try {
    await ElMessageBox.confirm(
      '请确认您选择的二进制文件架构与当前系统匹配（如 linux_amd64），错误的架构可能导致系统崩溃无法启动。是否继续安装？',
      '本地更新确认',
      {
        confirmButtonText: '确认安装',
        cancelButtonText: '取消',
        type: 'warning',
        confirmButtonClass: 'el-button--danger',
      }
    )
  } catch {
    return
  }

  installing.value = true
  const file = selectedFile.value

  try {
    const res = await systemService.uploadLocalUpdate(file)
    if (!res.ok) throw new Error(res.error.message || '安装失败')
    await showInstallingOverlay()
  } catch (e: any) {
    ElMessage.error(e.message || '安装失败')
  } finally {
    installing.value = false
  }
}

// ── 在线更新 ──
const releases = ref<ReleaseItem[]>([])
const loadingReleases = ref(false)
const selectedTag = ref('')
const downloading = ref(false)
const downloadProgress = ref(0)
const showProgress = ref(false)

let progressTimer: ReturnType<typeof setInterval> | null = null

function clearProgressTimer() {
  if (progressTimer) {
    clearInterval(progressTimer)
    progressTimer = null
  }
}

async function fetchReleases() {
  if (!props.repoAvailable) return
  loadingReleases.value = true
  try {
    const res = await systemService.listReleases()
    if (res.ok) {
      releases.value = res.data
      if (res.data.length > 0) {
        selectedTag.value = res.data[0].tag_name
      }
    } else {
      throw new Error(res.error.message || '获取版本列表失败')
    }
  } catch (e: any) {
    ElMessage.error(e.message || '获取版本列表失败')
  } finally {
    loadingReleases.value = false
  }
}

function switchToOnline() {
  activeMode.value = 'online'
  if (releases.value.length === 0 && props.repoAvailable) {
    fetchReleases()
  }
}

async function downloadAndInstall() {
  if (!selectedTag.value) return

  try {
    await ElMessageBox.confirm(
      `确定要下载并安装版本 ${selectedTag.value} 吗？安装完成后系统将自动重启。`,
      '在线更新确认',
      {
        confirmButtonText: '下载并安装',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
  } catch {
    return
  }

  downloading.value = true
  downloadProgress.value = 0
  showProgress.value = true

  clearProgressTimer()
  progressTimer = setInterval(() => {
    if (downloadProgress.value < 90) {
      downloadProgress.value += Math.random() * 8
      if (downloadProgress.value > 90) downloadProgress.value = 90
    }
  }, 500)

  try {
    const res = await systemService.applyUpdateByTag(selectedTag.value)
    clearProgressTimer()
    downloadProgress.value = 100

    if (!res.ok) throw new Error(res.error.message || '下载安装失败')

    showProgress.value = false
    await showInstallingOverlay()
  } catch (e: any) {
    clearProgressTimer()
    showProgress.value = false
    ElMessage.error(e.message || '下载安装失败')
  } finally {
    downloading.value = false
  }
}

// ── 全屏 Loading 遮罩 ──
async function showInstallingOverlay() {
  const loading = ElLoading.service({
    lock: true,
    text: '正在安装中，安装完成会自动刷新，请勿关闭该页面！',
    background: 'rgba(0, 0, 0, 0.85)',
  })

  let attempts = 0
  const maxAttempts = 60
  const poll = setInterval(async () => {
    attempts++
    if (attempts > maxAttempts) {
      clearInterval(poll)
      loading.close()
      ElMessage.error('等待服务恢复超时，请手动刷新页面')
      return
    }
    try {
      const res = await systemService.getInfo()
      if (res.ok) {
        clearInterval(poll)
        loading.close()
        window.location.reload()
      }
    } catch {
      // 服务还未恢复
    }
  }, 2000)
}

function formatDate(dateStr: string): string {
  if (!dateStr) return '--'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  return d.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

onBeforeUnmount(() => {
  clearProgressTimer()
})
</script>

<template>
  <div class="core-mgmt-card">
    <!-- 头部：标题 + 切换按钮 -->
    <div class="core-mgmt-header">
      <div class="flex items-center gap-2">
        <div class="core-mgmt-icon">
          <el-icon size="18"><Cloud24Regular /></el-icon>
        </div>
        <span class="core-mgmt-title">核心管理</span>
      </div>
      <div class="flex items-center gap-1">
        <button
          class="core-mgmt-tab"
          :class="{ active: activeMode === 'local' }"
          @click="activeMode = 'local'"
        >
          <el-icon size="14" class="mr-1"><Desktop24Regular /></el-icon>
          本地安装
        </button>
        <button
          class="core-mgmt-tab"
          :class="{ active: activeMode === 'online' }"
          @click="switchToOnline"
        >
          <el-icon size="14" class="mr-1"><Cloud24Regular /></el-icon>
          在线更新
        </button>
      </div>
    </div>

    <!-- 本地安装 -->
    <div v-if="activeMode === 'local'" class="core-mgmt-body">
      <div class="core-mgmt-hint">选择本地的 VoHive 核心二进制文件进行上传安装</div>

      <div class="flex items-center gap-2 mt-3">
        <input
          ref="fileInputRef"
          type="file"
          class="hidden"
          @change="onFileSelect"
        />
        <el-button size="small" :disabled="!!selectedFile" @click="triggerFileInput">
          <el-icon class="mr-1"><FolderOpen24Regular /></el-icon>
          选择文件
        </el-button>
        <div v-if="selectedFile" class="local-file-chip">
          <span class="local-file-name" :title="selectedFile.name">{{ selectedFile.name }}</span>
          <button class="local-file-remove" :disabled="installing" @click="clearFile" title="删除">
            <el-icon size="12"><Dismiss24Regular /></el-icon>
          </button>
        </div>
        <span v-else class="text-sm truncate flex-1" style="color: var(--muted-foreground);">
          未选择文件
        </span>
      </div>

      <!-- 拖拽区域 -->
      <div
        class="local-drag-zone mt-3"
        :class="{ 'local-drag-zone--over': dragOver, 'local-drag-zone--uploaded': selectedFile }"
        @drop="onDrop"
        @dragover="onDragOver"
        @dragleave="onDragLeave"
      >
        <template v-if="selectedFile">
          <el-icon size="32" style="color: var(--brand);"><CheckmarkCircle24Regular /></el-icon>
          <div class="local-drag-hint" style="color: var(--brand);">已上传</div>
        </template>
        <template v-else>
          <el-icon size="32" style="color: var(--muted-foreground);"><ArrowDownload24Regular /></el-icon>
          <div class="local-drag-hint">或拖拽上传核心二进制开始安装</div>
        </template>
      </div>

      <!-- 操作按钮 -->
      <div class="flex justify-end items-center gap-2 mt-3">
        <el-button size="small" @click="clearFile" :disabled="!selectedFile || installing">取消</el-button>
        <el-button
          type="primary"
          class="!border-0"
          :disabled="!selectedFile"
          :loading="installing"
          @click="installLocal"
        >
          <el-icon class="mr-1"><ArrowDownload24Regular /></el-icon>
          {{ installing ? '安装中' : '开始安装' }}
        </el-button>
      </div>
    </div>

    <!-- 在线更新 -->
    <div v-else class="core-mgmt-body">
      <!-- 未配置提示 -->
      <div v-if="!repoAvailable" class="core-mgmt-hint" style="padding: 24px 0; text-align: center;">
        仓库地址未配置，请在上方配置完成后再试
      </div>

      <template v-else>
        <div class="core-mgmt-hint">选择对应版本在线安装</div>

        <!-- 版本选择 -->
        <div class="mt-3" v-loading="loadingReleases">
          <el-select
            v-model="selectedTag"
            placeholder="选择版本"
            class="w-full"
            :disabled="downloading"
          >
            <el-option
              v-for="rel in releases"
              :key="rel.tag_name"
              :label="`${rel.tag_name}  ${formatDate(rel.published_at)}`"
              :value="rel.tag_name"
            >
              <div class="flex items-center justify-between w-full">
                <span class="font-mono">{{ rel.tag_name }}</span>
                <span class="text-xs" style="color: var(--muted-foreground);">{{ formatDate(rel.published_at) }}</span>
              </div>
            </el-option>
          </el-select>
        </div>

        <!-- 下载进度条 -->
        <div v-if="showProgress" class="mt-3">
          <el-progress
            :percentage="Math.round(downloadProgress)"
            :status="downloadProgress >= 100 ? 'success' : ''"
          />
        </div>

        <!-- 操作按钮 -->
        <div class="flex justify-end mt-3">
          <el-button
            type="primary"
            class="!border-0"
            :disabled="!selectedTag || loadingReleases"
            :loading="downloading"
            @click="downloadAndInstall"
          >
            <el-icon class="mr-1"><ArrowDownload24Regular /></el-icon>
            {{ downloading ? (downloadProgress < 100 ? '下载中' : '安装中') : '下载安装' }}
          </el-button>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.core-mgmt-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
}

.core-mgmt-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}

.core-mgmt-icon {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: var(--muted);
  border: 1px solid var(--border);
  color: var(--foreground);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.core-mgmt-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.core-mgmt-tab {
  display: flex;
  align-items: center;
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: transparent;
  color: var(--muted-foreground);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
}

.core-mgmt-tab:hover {
  border-color: color-mix(in oklab, var(--brand) 30%, var(--border));
  color: var(--foreground);
}

.core-mgmt-tab.active {
  background: var(--brand);
  border-color: var(--brand);
  color: white;
}

.core-mgmt-body {
  padding: 16px;
}

.core-mgmt-hint {
  font-size: 12px;
  color: var(--muted-foreground);
}

.local-drag-zone {
  border: 2px dashed var(--border);
  border-radius: 8px;
  padding: 24px;
  text-align: center;
  transition: border-color 0.15s, background 0.15s;
  cursor: pointer;
}

.local-drag-zone--over {
  border-color: var(--brand);
  background: color-mix(in oklab, var(--brand) 5%, transparent);
}

.local-drag-zone--uploaded {
  border-color: color-mix(in oklab, var(--brand) 40%, var(--border));
  background: color-mix(in oklab, var(--brand) 8%, transparent);
  cursor: default;
}

.local-file-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  padding: 3px 4px 3px 10px;
  border-radius: 999px;
  background: var(--muted);
  border: 1px solid var(--border);
  flex-shrink: 0;
}

.local-file-name {
  font-size: 12px;
  font-family: var(--oomol-font-mono);
  color: var(--foreground);
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.local-file-remove {
  width: 18px;
  height: 18px;
  border: none;
  border-radius: 999px;
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.12s;
}
.local-file-remove:hover {
  background: var(--destructive);
  color: white;
}
.local-file-remove:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.local-drag-hint {
  margin-top: 8px;
  font-size: 12px;
  color: var(--muted-foreground);
}
</style>
