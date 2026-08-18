<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute } from 'vue-router'
import { useDevicesStore } from '../stores/devices'
import ModuleListPanel from '../components/ModuleListPanel.vue'
import ModuleDetailPanel from '../components/ModuleDetailPanel.vue'
import ModulePreviewPanel from '../components/ModulePreviewPanel.vue'
import ModuleSearchDialog from '../components/ModuleSearchDialog.vue'
import { Eye24Regular } from '@vicons/fluent'

const store = useDevicesStore()
const route = useRoute()
const { list, detail } = storeToRefs(store)

// 选中设备
const selectedId = ref('')

// 预览抽屉（中窄屏）
const previewDrawerOpen = ref(false)

// 搜索弹窗
const searchDialogOpen = ref(false)

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

const showThreeColumn = computed(() => isWide.value)
const showPreviewButton = computed(() => !isWide.value)

onMounted(async () => {
  syncWidth()
  window.addEventListener('resize', syncWidth, { passive: true })
  await store.fetchList()
  // 优先从路由 query 选中指定设备，否则自动选中第一个
  const queryId = String(route.query.device || '').trim()
  if (queryId && list.value.some(d => d.id === queryId)) {
    selectedId.value = queryId
  } else if (list.value.length > 0) {
    selectedId.value = list.value[0].id
  }
})

// 选中设备变化时加载详情
watch(selectedId, async (id) => {
  if (id) {
    await store.fetchDetail(id)
  } else {
    // 清空选中时也需要清空详情
    detail.value = null
  }
})

function handleSelect(id: string) {
  selectedId.value = id
}

async function handleAddDevices(deviceIds: string[]) {
  searchDialogOpen.value = false
  await store.fetchList()
  const id = deviceIds.find(id => id && id.length > 0)
  if (id) {
    selectedId.value = id
  } else if (list.value.length > 0) {
    selectedId.value = list.value[0].id
  }
}

function handleDeviceDeleted() {
  selectedId.value = ''
  void store.fetchList()
}
</script>

<template>
  <div class="module-page">
    <!-- 三栏布局 -->
    <div class="module-grid" :class="{ 'three-col': showThreeColumn, 'two-col': isMedium, 'one-col': isNarrow }">
      <!-- 左栏：模块列表（窄屏隐藏） -->
      <div v-show="!isNarrow" class="module-col-list">
        <ModuleListPanel :selected-id="selectedId" @select="handleSelect" @open-search="searchDialogOpen = true" />
      </div>

      <!-- 中栏：详情页 -->
      <div class="module-col-detail">
        <ModuleDetailPanel :selected-id="selectedId" @device-deleted="handleDeviceDeleted" @open-search="searchDialogOpen = true" />

        <!-- 预览按钮（中窄屏浮动） -->
        <button
          v-if="showPreviewButton && detail"
          class="preview-fab"
          title="查看预览"
          @click="previewDrawerOpen = true"
        >
          <el-icon size="18"><Eye24Regular /></el-icon>
        </button>
      </div>

      <!-- 右栏：预览（宽屏显示） -->
      <div v-if="showThreeColumn" class="module-col-preview">
        <ModulePreviewPanel :device-id="selectedId" :device-imei="detail?.modem?.imei" :device-online="detail?.running" :is-p-c-s-c="detail?.esim_transport === 'pcsc'" />
      </div>
    </div>

    <!-- 预览抽屉（中窄屏） -->
    <el-drawer
      v-model="previewDrawerOpen"
      :with-header="false"
      size="400px"
      direction="rtl"
    >
      <ModulePreviewPanel v-if="detail" :device-id="selectedId" :device-imei="detail?.modem?.imei" :device-online="detail?.running" :is-p-c-s-c="detail?.esim_transport === 'pcsc'" />
    </el-drawer>

    <!-- 搜索添加设备弹窗 -->
    <ModuleSearchDialog
      v-model="searchDialogOpen"
      @add="handleAddDevices"
    />
  </div>
</template>

<style scoped>
.module-page {
  height: calc(100svh - 56px - 48px);
  display: flex;
  flex-direction: column;
}

.module-grid {
  flex: 1;
  min-height: 0;
  display: grid;
  gap: 12px;
}

.module-grid.three-col {
  grid-template-columns: 280px minmax(0, 1fr) 340px;
}

.module-grid.two-col {
  grid-template-columns: 260px minmax(0, 1fr);
}

.module-grid.one-col {
  grid-template-columns: 1fr;
}

.module-col-list {
  min-height: 0;
  overflow: hidden;
}

.module-col-detail {
  min-height: 0;
  position: relative;
}

.module-col-preview {
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
  border: 1px solid var(--brand);
  border-radius: 999px;
  background: var(--brand);
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: var(--console-shadow-md);
  transition: transform 0.15s, background 0.15s, color 0.15s;
}

.preview-fab:hover {
  background: var(--card);
  color: var(--brand);
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
  .module-page {
    height: calc(100svh - 56px - 36px);
  }
}
</style>
