<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { ElMessage } from 'element-plus'
import { useCarrierStore } from '../stores/carrier'
import CarrierListPanel from '../components/CarrierListPanel.vue'
import CarrierDetailPanel from '../components/CarrierDetailPanel.vue'
import CarrierPreviewPanel from '../components/CarrierPreviewPanel.vue'
import CarrierSearchDialog from '../components/CarrierSearchDialog.vue'
import { Eye24Regular } from '@vicons/fluent'
import { downloadIcon, getCachedIcon } from '../composables/useOperatorIcon'
import { loadPlmnCatalog } from '../composables/plmn-catalog'

const store = useCarrierStore()
const { detail, carriers } = storeToRefs(store)

// 预览抽屉（中窄屏）
const previewDrawerOpen = ref(false)

// 搜索弹窗（页面级，宽屏和窄屏共用）
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

// 是否显示三栏布局
const showThreeColumn = computed(() => isWide.value)

// 是否显示预览抽屉按钮
const showPreviewButton = computed(() => !isWide.value)

// 屏幕变宽时自动关闭预览抽屉，避免右栏和抽屉双重渲染
watch(isWide, (wide) => {
  if (wide) previewDrawerOpen.value = false
})

onMounted(async () => {
  syncWidth()
  window.addEventListener('resize', syncWidth, { passive: true })
  await store.fetchCarriers()
  // 确保 PLMN catalog 已加载（首次打开会 fetch all.json，之后从缓存读）
  await loadPlmnCatalog()
  // 页面加载时批量下载缺失的运营商图标（仅一次，失败不重试）
  for (const c of store.carriers) {
    if (!(await getCachedIcon(c.mcc, c.mnc, c.name, c.key))) {
      downloadIcon(c.mcc, c.mnc, c.name, c.key).then(result => {
        if (result) {
          window.dispatchEvent(new CustomEvent('vohive-icon-updated', { detail: { mcc: c.mcc, mnc: c.mnc } }))
        }
      })
    }
  }
})

async function handleAddCarriers(plmns: string[]) {
  const ok = await store.addCarriersFromIndex(plmns)
  if (ok) {
    ElMessage.success(`已添加 ${plmns.length} 个运营商`)
    for (const c of store.carriers) {
      if (plmns.includes(c.key) || plmns.includes(`${c.mcc}-${c.mnc}`)) {
        if (!(await getCachedIcon(c.mcc, c.mnc, c.name, c.key))) {
          downloadIcon(c.mcc, c.mnc, c.name, c.key).then(result => {
            if (result) {
              window.dispatchEvent(new CustomEvent('vohive-icon-updated', { detail: { mcc: c.mcc, mnc: c.mnc } }))
            }
          })
        }
      }
    }
  }
}
</script>

<template>
  <div class="carrier-page">
    <!-- 三栏布局 -->
    <div class="carrier-grid" :class="{ 'three-col': showThreeColumn, 'two-col': isMedium, 'one-col': isNarrow }">
      <!-- 左栏：运营商列表（窄屏隐藏，由详情页下拉替代） -->
      <div v-show="!isNarrow" class="carrier-col-list">
        <CarrierListPanel @open-search="searchDialogOpen = true" />
      </div>

      <!-- 中栏：详情页 -->
      <div class="carrier-col-detail">
        <CarrierDetailPanel @open-search="searchDialogOpen = true" />

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

    <!-- 搜索添加运营商弹窗（页面级） -->
    <CarrierSearchDialog
      v-model="searchDialogOpen"
      :existing-plmns="carriers.map(c => c.key)"
      @add="handleAddCarriers"
    />
  </div>
</template>

<style scoped>
.carrier-page {
  height: calc(100svh - 56px - 24px);
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
position: fixed;
bottom: 10px;
right: 10px;
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
:deep(.el-drawer__header) {
margin-bottom: 0;
padding: 8px 12px;
}

:deep(.el-drawer__body) {
padding: 12px;
display: flex;
flex-direction: column;
position: relative;
}

:deep(.el-drawer__body > *) {
  flex: 1;
  min-height: 0;
}

/* 响应式 */
@media (max-width: 768px) {
  .carrier-page {
    height: calc(100svh - 56px - 24px);
  }
}
</style>
