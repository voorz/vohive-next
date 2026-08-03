<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useCarrierStore } from '../stores/carrier'
import CarrierListPanel from '../components/CarrierListPanel.vue'
import CarrierDetailPanel from '../components/CarrierDetailPanel.vue'
import CarrierPreviewPanel from '../components/CarrierPreviewPanel.vue'
import { Eye24Regular } from '@vicons/fluent'

const store = useCarrierStore()
const { detail, previewTarget } = storeToRefs(store)

// 预览抽屉（中窄屏）
const previewDrawerOpen = ref(false)

// 窗口宽度响应
const isWide = ref(true)
const isMedium = ref(false)
const isNarrow = ref(false)

function syncWidth() {
  const w = window.innerWidth
  isWide.value = w > 1280
  isMedium.value = w > 768 && w <= 1280
  isNarrow.value = w <= 768
}

// 是否显示三栏布局
const showThreeColumn = computed(() => isWide.value)

// 是否显示预览抽屉按钮
const showPreviewButton = computed(() => !isWide.value)

onMounted(() => {
  syncWidth()
  window.addEventListener('resize', syncWidth, { passive: true })
  store.fetchCarriers()
})
</script>

<template>
  <div class="carrier-page">
    <!-- 三栏布局 -->
    <div class="carrier-grid" :class="{ 'three-col': showThreeColumn, 'two-col': isMedium, 'one-col': isNarrow }">
      <!-- 左栏：运营商列表（窄屏隐藏，由详情页下拉替代） -->
      <div v-show="!isNarrow" class="carrier-col-list">
        <CarrierListPanel />
      </div>

      <!-- 中栏：详情页 -->
      <div class="carrier-col-detail">
        <CarrierDetailPanel />

        <!-- 预览按钮（中窄屏浮动） -->
        <button
          v-if="showPreviewButton && detail"
          class="preview-fab"
          title="查看配置预览"
          @click="previewDrawerOpen = true"
        >
          <el-icon size="18"><Eye24Regular /></el-icon>
        </button>
      </div>

      <!-- 右栏：配置预览（宽屏显示） -->
      <div v-if="showThreeColumn" class="carrier-col-preview">
        <CarrierPreviewPanel />
      </div>
    </div>

    <!-- 预览抽屉（中窄屏） -->
    <el-drawer
      v-model="previewDrawerOpen"
      title="配置预览"
      size="400px"
      direction="rtl"
    >
      <CarrierPreviewPanel v-if="detail" />
    </el-drawer>
  </div>
</template>

<style scoped>
.carrier-page {
  height: calc(100svh - 56px - 48px);
  display: flex;
  flex-direction: column;
}

.carrier-grid {
  flex: 1;
  min-height: 0;
  display: grid;
  gap: 12px;
}

.carrier-grid.three-col {
  grid-template-columns: 280px minmax(0, 1fr) 340px;
}

.carrier-grid.two-col {
  grid-template-columns: 260px minmax(0, 1fr);
}

.carrier-grid.one-col {
  grid-template-columns: 1fr;
}

.carrier-col-list {
  min-height: 0;
  overflow: hidden;
}

.carrier-col-detail {
  min-height: 0;
  position: relative;
}

.carrier-col-preview {
  min-height: 0;
}

/* 预览浮动按钮 */
.preview-fab {
  position: absolute;
  bottom: 16px;
  right: 16px;
  z-index: 5;
  width: 42px;
  height: 42px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--card);
  color: var(--foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: var(--console-shadow-md);
  transition: transform 0.15s, background 0.15s;
}

.preview-fab:hover {
  background: var(--accent);
  transform: scale(1.05);
}

/* 抽屉内预览面板铺满高度 */
:deep(.el-drawer__body) {
  padding: 12px;
  display: flex;
  flex-direction: column;
}

:deep(.el-drawer__body > *) {
  flex: 1;
  min-height: 0;
}

/* 响应式 */
@media (max-width: 768px) {
  .carrier-page {
    height: calc(100svh - 56px - 36px);
  }
}
</style>
