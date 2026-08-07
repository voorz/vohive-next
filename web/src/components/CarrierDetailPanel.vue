<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useCarrierStore } from '../stores/carrier'
import CarrierEditArea from './CarrierEditArea.vue'
import ListSkeleton from './ListSkeleton.vue'
import EmptyState from './EmptyState.vue'
import { Add24Regular } from '@vicons/fluent'
import CarrierIcon from './CarrierIcon.vue'

const emit = defineEmits<{
  'open-search': []
}>()

const store = useCarrierStore()
const { detail, selectedCarrier, detailLoading, carriers, selectedKey } = storeToRefs(store)

// 窄屏下拉选择
function handleSelectChange(value: string) {
  store.selectCarrier(value)
}

const selectValue = computed(() => selectedKey.value)
</script>

<template>
  <div class="detail-panel">
    <!-- 详情页头部 (60px) -->
    <div class="detail-header">
      <!-- 窄屏下拉选择器 + 添加按钮 -->
      <div class="detail-header-narrow">
        <el-select
          :model-value="selectValue"
          @change="handleSelectChange"
          placeholder="选择运营商"
          class="!w-full"
        >
          <el-option
            v-for="c in carriers"
            :key="c.key"
            :label="`${c.name}  ${c.mcc}:${c.mnc}`"
            :value="c.key"
          />
        </el-select>
        <el-button size="small" type="primary" @click="emit('open-search')" class="!border-0 add-btn-narrow">
          <el-icon class="mr-1"><Add24Regular /></el-icon>
          <span>添加</span>
        </el-button>
      </div>
      <!-- 宽屏：图标盒子 + 运营商名 + PLMN -->
      <div class="detail-header-wide">
        <CarrierIcon :mcc="selectedCarrier?.mcc || ''" :mnc="selectedCarrier?.mnc || ''" :name="selectedCarrier?.name" :size="38" />
        <div class="detail-header-info">
          <div class="detail-header-name">{{ selectedCarrier?.name || '未选择' }}</div>
          <div class="detail-header-plmn">{{ selectedCarrier?.mcc }}:{{ selectedCarrier?.mnc }}</div>
        </div>
      </div>
    </div>

    <ListSkeleton v-if="detailLoading && !detail" :rows="3" />

    <EmptyState
      v-else-if="!detail"
      title="选择一个运营商"
      subtitle="从左侧列表选择运营商查看配置详情"
    />

    <div v-else class="detail-content">
      <!-- 编辑区 -->
      <div class="edit-area-wrap">
        <CarrierEditArea />
      </div>
    </div>

  </div>
</template>

<style scoped>
.detail-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  padding: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  overflow: hidden;
}

/* 头部 — 60px 统一高度 */
.detail-header {
  height: 60px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border);
  overflow: hidden;
}

.detail-header-narrow {
  display: none;
  flex: 1;
  align-items: center;
  gap: 8px;
}

.detail-header-narrow .el-select {
  flex: 1;
}

.add-btn-narrow {
  flex-shrink: 0;
}

.detail-header-wide {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

/* 图标盒子 — 38×38 / 6px 圆角 / 主题色 */
.detail-header-icon-box {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  background: var(--background);
  border: 1px solid var(--border);
  color: var(--foreground);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.detail-header-info {
  flex: 1;
  min-width: 0;
}

.detail-header-name {
  font-size: 15px;
  font-weight: 700;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-header-plmn {
  font-size: 12px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  margin-top: 1px;
}

/* 内容区 — 去掉 padding 让分割线贯通 */
.detail-content {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* 编辑区 — 填充全部空间 */
.edit-area-wrap {
  flex: 1;
  min-height: 0;
  padding: 10px;
}

/* 响应式：窄屏显示下拉选择器 */
@media (max-width: 768px) {
  .detail-header-narrow {
    display: flex;
  }
  .detail-header-wide {
    display: none;
  }
}
</style>
