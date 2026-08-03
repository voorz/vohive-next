<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useCarrierStore } from '../stores/carrier'
import ListSkeleton from './ListSkeleton.vue'
import EmptyState from './EmptyState.vue'
import {
  Add24Regular,
  Delete24Regular,
  Search24Regular
} from '@vicons/fluent'
import CarrierIcon from './CarrierIcon.vue'
import { downloadIcon } from '../composables/useOperatorIcon'
const store = useCarrierStore()
const { carriers, selectedMcc, selectedMnc, loading } = storeToRefs(store)

const searchText = ref('')
const addDialogOpen = ref(false)

const addForm = ref({ name: '', mcc: '', mnc: '', ike_addr: '', device_ims_tac: 0, device_ims_cell_id: 0 })

const filteredCarriers = computed(() => {
  const q = searchText.value.trim().toLowerCase()
  if (!q) return carriers.value
  return carriers.value.filter(c =>
    c.name.toLowerCase().includes(q) ||
    c.mcc.includes(q) ||
    c.mnc.includes(q) ||
    `${c.mcc}:${c.mnc}`.includes(q)
  )
})

function handleSelect(mcc: string, mnc: string) {
  store.selectCarrier(mcc, mnc)
}

async function handleAdd() {
  const form = addForm.value
  if (!form.name.trim() || !form.mcc.trim() || !form.mnc.trim()) {
    ElMessage.warning('请填写运营商名称、MCC 和 MNC')
    return
  }
  const ok = await store.addCarrier(
    form.name.trim(),
    form.mcc.trim(),
    form.mnc.trim(),
    form.ike_addr.trim(),
    form.device_ims_tac,
    form.device_ims_cell_id
  )
if (ok) {
ElMessage.success('运营商已添加')
addDialogOpen.value = false
// 后台自动下载运营商图标
const mcc = form.mcc.trim(), mnc = form.mnc.trim()
downloadIcon(mcc, mnc).then(() => {
window.dispatchEvent(new CustomEvent('vohive-icon-updated', { detail: { mcc, mnc } }))
})
addForm.value = { name: '', mcc: '', mnc: '', ike_addr: '', device_ims_tac: 0, device_ims_cell_id: 0 }
}
}

async function handleDelete(mcc: string, mnc: string, name: string) {
  const confirmed = await ElMessageBox.confirm(
    `确定移除运营商「${name}」(${mcc}:${mnc})？\n该操作会同时删除其用户配置模板。`,
    '确认移除运营商',
    { confirmButtonText: '移除', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)
  if (!confirmed) return
  const ok = await store.removeCarrier(mcc, mnc)
  if (ok) ElMessage.success('已移除')
}
</script>

<template>
  <div class="carrier-list-panel">
    <!-- 搜索栏 -->
    <div class="list-search">
      <el-input
        v-model="searchText"
        placeholder="搜索运营商 / MCC:MNC"
        size="small"
        clearable
      >
        <template #prefix>
          <el-icon><Search24Regular /></el-icon>
        </template>
      </el-input>
    </div>

    <!-- 操作按钮 -->
    <div class="list-actions">
      <el-button size="small" type="primary" @click="addDialogOpen = true" class="!border-0">
        <el-icon class="mr-1"><Add24Regular /></el-icon>
        <span>添加</span>
      </el-button>
    </div>

    <!-- 运营商列表 -->
    <div class="list-scroll">
      <ListSkeleton v-if="loading && carriers.length === 0" :rows="4" />

      <EmptyState
        v-else-if="filteredCarriers.length === 0"
        title="暂无运营商"
        subtitle="点击「添加」创建运营商配置"
      />

      <div v-else class="carrier-cards">
        <div
          v-for="item in filteredCarriers"
          :key="`${item.mcc}-${item.mnc}`"
          class="carrier-card"
          :class="{ selected: item.mcc === selectedMcc && item.mnc === selectedMnc }"
          @click="handleSelect(item.mcc, item.mnc)"
        >
          <CarrierIcon :mcc="item.mcc" :mnc="item.mnc" :size="28" />
          <div class="carrier-card-info">
            <div class="carrier-card-name">{{ item.name }}</div>
            <div class="carrier-card-plmn">{{ item.mcc }}:{{ item.mnc }}</div>
          </div>
          <div class="carrier-card-badges">
            <span v-if="item.active" class="badge-dot active" title="用户配置启用中" />
            <span v-else-if="item.has_user_config" class="badge-dot idle" title="有用户配置（未启用）" />
            <span v-if="item.has_system_default" class="badge-dot system" title="有系统默认" />
          </div>
          <div class="carrier-card-actions">
            <button
              v-if="!item.has_system_default"
              class="carrier-card-delete"
              title="移除运营商"
              @click.stop="handleDelete(item.mcc, item.mnc, item.name)"
            >
              <el-icon size="14"><Delete24Regular /></el-icon>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 添加运营商对话框 -->
    <el-dialog v-model="addDialogOpen" title="添加运营商" width="460px" :close-on-click-modal="false">
      <div class="space-y-4">
        <div class="space-y-1">
          <label class="carrier-form-label">运营商名称</label>
          <el-input v-model="addForm.name" placeholder="例如 giffgaff UK" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="carrier-form-label">MCC</label>
            <el-input v-model="addForm.mcc" placeholder="例如 234" />
          </div>
          <div class="space-y-1">
            <label class="carrier-form-label">MNC</label>
            <el-input v-model="addForm.mnc" placeholder="例如 10" />
          </div>
        </div>
        <div class="space-y-1">
          <label class="carrier-form-label">ePDG 地址（可选）</label>
          <el-input v-model="addForm.ike_addr" placeholder="空则自动生成 3GPP FQDN" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="carrier-form-label">LTE TAC</label>
            <el-input-number v-model="addForm.device_ims_tac" :min="0" class="!w-full" />
          </div>
          <div class="space-y-1">
            <label class="carrier-form-label">LTE Cell ID</label>
            <el-input-number v-model="addForm.device_ims_cell_id" :min="0" class="!w-full" />
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="addDialogOpen = false">取消</el-button>
        <el-button type="primary" @click="handleAdd" class="!border-0">添加</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.carrier-list-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  overflow: hidden;
}

.list-search {
  height: 60px;
  display: flex;
  align-items: center;
  padding: 0 12px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.list-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 44px;
  padding: 0 12px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.list-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 6px;
}

.carrier-cards {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.carrier-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid transparent;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
}

.carrier-card:hover {
  background: var(--accent);
}

.carrier-card.selected {
  background: color-mix(in oklab, var(--brand) 8%, var(--card));
  border-color: color-mix(in oklab, var(--brand) 30%, var(--border));
}

.carrier-card-info {
  flex: 1;
  min-width: 0;
}

.carrier-card-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.carrier-card-plmn {
  font-size: 11px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  margin-top: 1px;
}

.carrier-card-badges {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.badge-dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
}

.badge-dot.active {
  background: var(--success);
}

.badge-dot.idle {
  background: var(--warning);
  opacity: 0.6;
}

.badge-dot.system {
  background: var(--info);
  opacity: 0.5;
}

.carrier-card-actions {
  width: 24px;
  height: 24px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.carrier-card-delete {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.12s, background 0.12s, color 0.12s;
}

.carrier-card:hover .carrier-card-delete {
  opacity: 0.7;
}

.carrier-card-delete:hover {
  background: var(--destructive);
  color: white;
  opacity: 1;
}

.carrier-form-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--foreground);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.carrier-form-hint {
  font-size: 12px;
  color: var(--muted-foreground);
  margin-top: 4px;
}
</style>
