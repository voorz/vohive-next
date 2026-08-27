<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
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
import CountryFlag from './CountryFlag.vue'
import { downloadIcon, getCachedIcon } from '../composables/useOperatorIcon'
import { mccToIso, loadPlmnInfo, getPlmnInfo } from '../composables/plmn-info'

const emit = defineEmits<{
  'open-search': []
}>()

const store = useCarrierStore()
const { carriers, selectedKey, loading } = storeToRefs(store)

const searchText = ref('')

// PLMN 索引加载（用于 mccToIso 内部查询）
onMounted(async () => {
  await loadPlmnInfo()
})

// 国旗 ISO — 直接用 MCC+MNC 查询，与 CarrierIcon 使用同一套数据源
function getCountryIso(mcc: string, mnc: string): string {
  return mccToIso(mcc, mnc)
}

// 国家码 — 从 PLMN 信息获取
function getCountryCode(mcc: string, mnc: string): string {
  const plmn = mnc ? `${mcc}-${mnc}` : mcc
  const info = getPlmnInfo(plmn)
  return info?.country?.code || ''
}

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

function handleSelect(key: string) {
  store.selectCarrier(key)
}

async function handleAddCarriers(plmns: string[]) {
  const ok = await store.addCarriersFromIndex(plmns)
  if (ok) {
    ElMessage.success(`已添加 ${plmns.length} 个运营商`)
    // 后台批量下载图标
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

async function handleDelete(key: string, name: string) {
  const confirmed = await ElMessageBox.confirm(
    `确定移除运营商「${name}」(${key})？\n该操作仅从列表移除，不删除模板，可重新添加。`,
    '确认移除运营商',
    { confirmButtonText: '移除', cancelButtonText: '取消', type: 'warning' }
  ).then(() => true).catch(() => false)
  if (!confirmed) return
  const ok = await store.removeCarrier(key)
  if (ok) ElMessage.success('已移除')
}
</script>

<template>
  <div class="carrier-list-panel">
    <!-- 搜索栏 + 添加按钮 -->
    <div class="list-search">
      <el-input
        v-model="searchText"
        placeholder="搜索运营商 / MCC:MNC"
        clearable
      >
        <template #prefix>
          <el-icon><Search24Regular /></el-icon>
        </template>
      </el-input>
      <el-button size="small" type="primary" @click="emit('open-search')" class="!border-0 add-btn">
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
        subtitle="点击「添加」搜索并添加运营商"
      />

      <div v-else class="carrier-cards">
        <div
          v-for="item in filteredCarriers"
          :key="item.key"
          class="carrier-card"
          :class="{ selected: item.key === selectedKey }"
          @click="handleSelect(item.key)"
        >
          <CarrierIcon :mcc="item.mcc" :mnc="item.mnc" :name="item.name" :carrier-key="item.key" :size="38" />
          <div class="carrier-card-info">
            <div class="carrier-card-name">{{ item.name }}</div>
            <div class="carrier-card-meta">
              <span class="carrier-card-plmn">{{ item.mcc }}:{{ item.mnc }}</span>
              <span v-if="getCountryCode(item.mcc, item.mnc)" class="carrier-card-code">+{{ getCountryCode(item.mcc, item.mnc) }}</span>
              <CountryFlag v-if="getCountryIso(item.mcc, item.mnc)" :iso="getCountryIso(item.mcc, item.mnc)" :size="14" class="carrier-card-flag" />
              <span v-if="getCountryIso(item.mcc, item.mnc)" class="carrier-card-iso">{{ getCountryIso(item.mcc, item.mnc) }}</span>
            </div>
          </div>
          <div class="carrier-card-badges">
            <span v-if="item.active" class="badge-dot active" title="用户自定义模板生效" />
            <span v-else-if="item.has_system_default" class="badge-dot system" title="系统默认模板生效" />
            <span v-else class="badge-dot none" title="无模板配置" />
          </div>
          <div class="carrier-card-actions">
            <button
              class="carrier-card-delete"
              title="移除运营商"
              @click.stop="handleDelete(item.key, item.name)"
            >
              <el-icon size="14"><Delete24Regular /></el-icon>
            </button>
          </div>
        </div>
        <!-- 虚线框添加卡片 -->
        <button class="carrier-add-card" @click="emit('open-search')">
          <el-icon size="20"><Add24Regular /></el-icon>
          <span>添加运营商</span>
        </button>
      </div>
    </div>

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
  display: flex;
  align-items: center;
  gap: 8px;
  height: 60px;
  padding: 0 12px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.list-search .el-input {
  flex: 1;
}

.add-btn {
  flex-shrink: 0;
}

.list-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 12px;
}

.carrier-cards {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.carrier-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid var(--border);
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
  box-shadow: 0 0 0 1px color-mix(in oklab, var(--brand) 20%, transparent);
}

.carrier-add-card {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 62px;
  padding: 10px 12px;
  border: 1px dashed var(--border);
  border-radius: 6px;
  background: transparent;
  color: var(--muted-foreground);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s, background 0.15s;
}

.carrier-add-card:hover {
  border-color: var(--brand);
  color: var(--brand);
  background: color-mix(in oklab, var(--brand) 5%, transparent);
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
}

.carrier-card-meta {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 1px;
}

.carrier-card-code {
  font-size: 11px;
  font-family: var(--oomol-font-mono);
  color: var(--brand);
  opacity: 0.8;
}

.carrier-card-flag {
  opacity: 0.9;
}

.carrier-card-iso {
  font-size: 11px;
  color: var(--muted-foreground);
  opacity: 0.7;
  font-family: var(--oomol-font-mono);
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
  background: var(--brand);
}

.badge-dot.system {
  background: var(--warning);
}

.badge-dot.none {
  background: var(--muted-foreground);
  opacity: 0.3;
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
