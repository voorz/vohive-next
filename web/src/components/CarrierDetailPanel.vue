<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { ElMessage } from 'element-plus'
import { useCarrierStore } from '../stores/carrier'
import CarrierConfigCard from './CarrierConfigCard.vue'
import CarrierEditArea from './CarrierEditArea.vue'
import ListSkeleton from './ListSkeleton.vue'
import EmptyState from './EmptyState.vue'
import { Sim24Regular, Edit24Regular } from '@vicons/fluent'
import CarrierIcon from './CarrierIcon.vue'
import { downloadIcon } from '../composables/useOperatorIcon'

const store = useCarrierStore()
const { detail, selectedCarrier, detailLoading, previewTarget, carriers, selectedMcc, selectedMnc } = storeToRefs(store)

const hasUserConfig = computed(() => !!detail.value?.user_config)
const hasSystemDefault = computed(() => !!detail.value?.system_default)

// 窄屏下拉选择
function handleSelectChange(value: string) {
  const [mcc, mnc] = value.split('-')
  if (mcc && mnc) store.selectCarrier(mcc, mnc)
}

const selectValue = computed(() => `${selectedMcc.value}-${selectedMnc.value}`)

// 编辑运营商信息对话框
const editDialogOpen = ref(false)
const editForm = ref({ name: '', mcc: '', mnc: '', ike_addr: '', ims_tac: 0, ims_cell_id: 0 })

function openEditDialog() {
  if (!detail.value) return
  editForm.value = {
    name: detail.value.name,
    mcc: selectedMcc.value,
    mnc: selectedMnc.value,
    ike_addr: detail.value.ike_addr,
    ims_tac: detail.value.device_ims_tac,
    ims_cell_id: detail.value.device_ims_cell_id
  }
  editDialogOpen.value = true
}

function saveEdit() {
  if (!detail.value) return
  if (!editForm.value.name.trim()) {
    ElMessage.warning('运营商名称不能为空')
    return
  }
  if (!editForm.value.mcc.trim() || !editForm.value.mnc.trim()) {
    ElMessage.warning('MCC 和 MNC 不能为空')
    return
  }
  const oldMcc = selectedMcc.value
  const oldMnc = selectedMnc.value
  const newMcc = editForm.value.mcc.trim()
  const newMnc = editForm.value.mnc.trim()
  // 更新 store 中的基础信息
  detail.value.name = editForm.value.name.trim()
  detail.value.ike_addr = editForm.value.ike_addr
  detail.value.device_ims_tac = editForm.value.ims_tac
  detail.value.device_ims_cell_id = editForm.value.ims_cell_id
  // 如果 MCC/MNC 变了，需要更新列表项的 key
  if (oldMcc !== newMcc || oldMnc !== newMnc) {
    const item = carriers.value.find(c => c.mcc === oldMcc && c.mnc === oldMnc)
    if (item) {
      item.mcc = newMcc
      item.mnc = newMnc
      item.name = editForm.value.name.trim()
      item.ike_addr = editForm.value.ike_addr
      item.device_ims_tac = editForm.value.ims_tac
      item.device_ims_cell_id = editForm.value.ims_cell_id
    }
    selectedMcc.value = newMcc
    selectedMnc.value = newMnc
    detail.value.mcc = newMcc
    detail.value.mnc = newMnc
  } else {
    // 只更新名称和其他字段
    const item = carriers.value.find(c => c.mcc === newMcc && c.mnc === newMnc)
    if (item) {
      item.name = editForm.value.name.trim()
      item.ike_addr = editForm.value.ike_addr
      item.device_ims_tac = editForm.value.ims_tac
      item.device_ims_cell_id = editForm.value.ims_cell_id
    }
  }
  ElMessage.success('运营商信息已更新')
  editDialogOpen.value = false
  // 如果 MCC/MNC 变了，后台自动下载新图标
  if (oldMcc !== newMcc || oldMnc !== newMnc) {
    downloadIcon(newMcc, newMnc, editForm.value.name).then(() => {
      window.dispatchEvent(new CustomEvent('vohive-icon-updated', { detail: { mcc: newMcc, mnc: newMnc } }))
    })
  }
}
</script>

<template>
  <div class="detail-panel">
    <!-- 详情页头部 (60px) -->
    <div class="detail-header">
      <!-- 窄屏下拉选择器 -->
      <div class="detail-header-narrow">
        <el-select
          :model-value="selectValue"
          @change="handleSelectChange"
          placeholder="选择运营商"
          class="!w-full"
        >
          <el-option
            v-for="c in carriers"
            :key="`${c.mcc}-${c.mnc}`"
            :label="`${c.name}  ${c.mcc}:${c.mnc}`"
            :value="`${c.mcc}-${c.mnc}`"
          />
        </el-select>
      </div>
      <!-- 宽屏：图标盒子 + 运营商名 + PLMN + 编辑按钮 -->
      <div class="detail-header-wide">
        <CarrierIcon :mcc="selectedMcc" :mnc="selectedMnc" :name="selectedCarrier?.name" :size="38" />
        <div class="detail-header-info">
          <div class="detail-header-name">{{ selectedCarrier?.name || '未选择' }}</div>
          <div class="detail-header-plmn">{{ selectedCarrier?.mcc }}:{{ selectedCarrier?.mnc }}</div>
        </div>
        <el-button
          v-if="detail"
          size="small"
          @click="openEditDialog"
        >
          <el-icon class="mr-1"><Edit24Regular /></el-icon>
          <span>编辑运营商</span>
        </el-button>
      </div>
    </div>

    <ListSkeleton v-if="detailLoading && !detail" :rows="3" />

    <EmptyState
      v-else-if="!detail"
      title="选择一个运营商"
      subtitle="从左侧列表选择运营商查看配置详情"
    />

    <div v-else class="detail-content">
      <!-- 配置卡片区域 -->
      <div class="config-cards-row">
        <CarrierConfigCard
          type="system"
          :active="!detail.active || !hasUserConfig"
          :has-config="hasSystemDefault"
          :selected="previewTarget === 'system'"
          @click="store.setPreviewTarget('system')"
        />
        <CarrierConfigCard
          type="user"
          :active="detail.active"
          :has-config="hasUserConfig"
          :selected="previewTarget === 'user'"
          @click="store.setPreviewTarget('user')"
        />
      </div>

      <!-- 编辑区 -->
      <div class="edit-area-wrap">
        <CarrierEditArea />
      </div>
    </div>

    <!-- 编辑运营商信息对话框 -->
    <el-dialog v-model="editDialogOpen" title="编辑运营商信息" width="460px" :close-on-click-modal="false">
      <div class="space-y-4">
        <div class="space-y-1">
          <label class="carrier-form-label">运营商名称</label>
          <el-input v-model="editForm.name" placeholder="例如 giffgaff UK" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="carrier-form-label">MCC</label>
            <el-input v-model="editForm.mcc" placeholder="例如 234" />
          </div>
          <div class="space-y-1">
            <label class="carrier-form-label">MNC</label>
            <el-input v-model="editForm.mnc" placeholder="例如 10" />
          </div>
        </div>
        <div class="space-y-1">
          <label class="carrier-form-label">ePDG 地址（可选）</label>
          <el-input v-model="editForm.ike_addr" placeholder="空则自动生成 3GPP FQDN" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="carrier-form-label">LTE TAC</label>
            <el-input-number v-model="editForm.ims_tac" :min="0" class="!w-full" />
          </div>
          <div class="space-y-1">
            <label class="carrier-form-label">LTE Cell ID</label>
            <el-input-number v-model="editForm.ims_cell_id" :min="0" class="!w-full" />
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="editDialogOpen = false">取消</el-button>
        <el-button type="primary" @click="saveEdit" class="!border-0">保存</el-button>
      </template>
    </el-dialog>
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

/* 配置卡片行 — 有 padding */
.config-cards-row {
  display: flex;
  gap: 10px;
  padding: 10px;
  flex-shrink: 0;
}

/* 编辑区 — 填充剩余空间 */
.edit-area-wrap {
  flex: 1;
  min-height: 0;
  padding: 0 10px 10px;
}

/* 响应式：窄屏显示下拉选择器 */
@media (max-width: 768px) {
  .detail-header-narrow {
    display: block;
  }
  .detail-header-wide {
    display: none;
  }
}

.carrier-form-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--foreground);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
</style>
