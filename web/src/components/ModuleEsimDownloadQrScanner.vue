<script setup lang="ts">
import { ref, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import jsQR from 'jsqr'
import { Camera24Regular } from '@vicons/fluent'

const emit = defineEmits<{
  scanned: [lpa: string]
}>()

const dragOver = ref(false)
const cameraOpen = ref(false)
const videoRef = ref<HTMLVideoElement | null>(null)
const canvasRef = ref<HTMLCanvasElement | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
let stream: MediaStream | null = null
let scanInterval: ReturnType<typeof setInterval> | null = null

function handleDrop(e: DragEvent) {
  e.preventDefault()
  dragOver.value = false
  const file = e.dataTransfer?.files?.[0]
  if (file) handleFile(file)
}

function handleFileInput(e: Event) {
  const target = e.target as HTMLInputElement
  const file = target.files?.[0]
  if (file) handleFile(file)
  target.value = ''
}

function handleFile(file: File) {
  if (!file.type.startsWith('image/')) {
    ElMessage.warning('请上传图片文件')
    return
  }
  const img = new Image()
  img.onload = () => {
    const canvas = document.createElement('canvas')
    canvas.width = img.width
    canvas.height = img.height
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    ctx.drawImage(img, 0, 0)
    const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height)
    const result = jsQR(imageData.data, imageData.width, imageData.height)
    if (result?.data) {
      emit('scanned', result.data)
    } else {
      ElMessage.warning('未识别到二维码')
    }
  }
  img.src = URL.createObjectURL(file)
}

async function toggleCamera() {
  if (cameraOpen.value) {
    stopCamera()
  } else {
    await startCamera()
  }
}

async function startCamera() {
  if (!navigator.mediaDevices?.getUserMedia) {
    ElMessage.warning('摄像头需要通过 localhost 访问才能使用，或请使用拖拽上传二维码')
    return
  }
  try {
    stream = await navigator.mediaDevices.getUserMedia({ video: true })
    cameraOpen.value = true
    await new Promise(resolve => setTimeout(resolve, 100))
    if (videoRef.value) {
      videoRef.value.srcObject = stream
      videoRef.value.play()
    }
    scanInterval = setInterval(scanFrame, 300)
  } catch (e: unknown) {
    const err = e as DOMException
    if (err.name === 'NotAllowedError') {
      ElMessage.error('摄像头权限被拒绝，请在浏览器设置中允许')
    } else if (err.name === 'NotFoundError') {
      ElMessage.error('未检测到摄像头设备')
    } else {
      ElMessage.error('无法访问摄像头，请使用拖拽上传')
    }
  }
}

function scanFrame() {
  if (!videoRef.value || !canvasRef.value) return
  const video = videoRef.value
  const canvas = canvasRef.value
  if (!video.videoWidth) return
  canvas.width = video.videoWidth
  canvas.height = video.videoHeight
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  ctx.drawImage(video, 0, 0, canvas.width, canvas.height)
  const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height)
  const result = jsQR(imageData.data, imageData.width, imageData.height)
  if (result?.data) {
    emit('scanned', result.data)
    stopCamera()
  }
}

function stopCamera() {
  cameraOpen.value = false
  if (stream) {
    stream.getTracks().forEach(t => t.stop())
    stream = null
  }
  if (scanInterval) {
    clearInterval(scanInterval)
    scanInterval = null
  }
}

onBeforeUnmount(() => stopCamera())
</script>

<template>
  <div class="qr-scanner">
    <!-- 拖拽上传区域 -->
    <div
      class="qr-drop-zone"
      :class="{ active: dragOver }"
      @dragover.prevent="dragOver = true"
      @dragleave.prevent="dragOver = false"
      @drop="handleDrop"
      @click="fileInput?.click()"
    >
      <span class="qr-drop-text">拽上传二维码开始安装<br /><span class="qr-drop-hint">(单个识别)</span></span>
      <input
        ref="fileInput"
        type="file"
        accept="image/*"
        class="qr-file-input"
        @change="handleFileInput"
      />
    </div>

    <!-- 相机按钮 -->
    <button class="qr-camera-btn" :class="{ active: cameraOpen }" @click="toggleCamera">
      <el-icon size="20"><Camera24Regular /></el-icon>
    </button>

    <!-- 摄像头预览 -->
    <div v-if="cameraOpen" class="qr-camera-preview">
      <video ref="videoRef" autoplay playsinline muted />
      <canvas ref="canvasRef" class="qr-canvas-hidden" />
    </div>
  </div>
</template>

<style scoped>
.qr-scanner {
  display: flex;
  gap: 8px;
  align-items: stretch;
}

.qr-drop-zone {
  flex: 1;
  min-width: 0;
  border: 2px dashed var(--border);
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.15s;
  padding: 12px;
  text-align: center;
}
.qr-drop-zone:hover {
  border-color: var(--brand);
  background: var(--accent);
}
.qr-drop-zone.active {
  border-color: var(--brand);
  background: color-mix(in oklab, var(--brand) 8%, transparent);
}

.qr-drop-text {
  font-size: 12px;
  color: var(--muted-foreground);
  line-height: 1.5;
}
.qr-drop-hint {
  font-size: 10px;
  opacity: 0.7;
}

.qr-file-input {
  display: none;
}

.qr-camera-btn {
  width: 48px;
  flex-shrink: 0;
  border: 2px solid var(--border);
  border-radius: 6px;
  background: var(--background);
  color: var(--muted-foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.12s;
}
.qr-camera-btn:hover {
  border-color: var(--brand);
  color: var(--brand);
}
.qr-camera-btn.active {
  border-color: var(--brand);
  color: var(--brand);
  background: color-mix(in oklab, var(--brand) 10%, transparent);
}

.qr-camera-preview {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: 1000;
  border: 2px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.2);
}
.qr-camera-preview video {
  display: block;
  max-width: 400px;
  max-height: 400px;
}

.qr-canvas-hidden {
  display: none;
}
</style>
