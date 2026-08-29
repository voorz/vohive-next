<script setup lang="ts">
import { computed, ref, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Search24Regular, Add24Regular } from '@vicons/fluent'
import CarrierIcon from './CarrierIcon.vue'
import CountryFlag from './CountryFlag.vue'
import carrierPresets from '../data/carrier-presets.json'
import { loadPlmnCatalog } from '../composables/plmn-catalog'
import { downloadIcon, getCachedIcon } from '../composables/useOperatorIcon'

// plmn-index 条目结构
interface PlmnOperator {
  brand?: string
  operator?: string
  status?: string
  type?: string
  bands?: string
  icon?: string
  icon_scope?: string
  subs?: Array<{
    brand?: string
    names?: string[]
    gid1?: string
    gid2?: string
    profile_names?: string[]
    icon?: string
    icon_scope?: string
  }>
}

interface PlmnEntry {
  mcc: string
  mnc: string
  country: {
    name: string
    iso: string
    code: string
    region: string
  }
  operators: PlmnOperator[]
}

// 子品牌信息
interface SubBrand {
  brand: string
  names: string[]
  gid1: string
  gid2: string
}

// 搜索结果项
interface SearchResult {
  plmn: string           // 234-33
  mcc: string
  mnc: string
  brand: string           // 第一个 operator 的 brand
  operator: string        // 第一个 operator 的 operator
  country: string
  countryIso: string
  countryCode: string     // 国家电话代码，如 44
  region: string
  hasSubs: boolean
  subs: SubBrand[]
}

// el-tree 节点数据
interface TreeNode {
  key: string
  label: string
  disabled?: boolean
  // 自定义渲染数据
  mcc: string
  mnc: string
  brand: string
  country?: string
  countryIso?: string
  countryCode?: string
  operator?: string
  hasSubs?: boolean
  subCount?: number
  isSub?: boolean
  names?: string[]
  gid1?: string
  children?: TreeNode[]
}

const props = defineProps<{
  modelValue: boolean
  existingPlmns: string[]  // 已添加的 PLMN 列表，用于标记
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  add: [plmns: string[]]
}>()

// 响应式宽度：窄屏 90%，宽屏固定 640px
const isNarrow = ref(typeof window !== 'undefined' && window.innerWidth <= 768)
const dialogWidth = computed(() => isNarrow.value ? '90%' : '640px')
function handleResize() {
  isNarrow.value = window.innerWidth <= 768
}
onMounted(() => window.addEventListener('resize', handleResize))
onUnmounted(() => window.removeEventListener('resize', handleResize))

const searchQuery = ref('')
const searchResults = ref<SearchResult[]>([])
const searching = ref(false)
const inputRef = ref<HTMLInputElement | null>(null)

// el-tree ref
const treeRef = ref()
const selectedCount = ref(0)

// 本地缓存的 all.json 数据
let allData: Record<string, PlmnEntry> | null = null
let loadingData: Promise<Record<string, PlmnEntry> | null> | null = null

const ALL_JSON_URL = 'https://raw.githubusercontent.com/voorz/plmn-index/main/plmn/all.json'
const ALL_JSON_MIRROR = 'https://cdn.jsdelivr.net/gh/voorz/plmn-index@main/plmn/all.json'
const CACHE_KEY = 'vohive.plmn-search-cache'
const CACHE_TTL = 7 * 24 * 60 * 60 * 1000 // 7 days

async function loadAllJson(): Promise<Record<string, PlmnEntry> | null> {
  if (allData) return allData
  if (loadingData) return loadingData

  // 尝试 localStorage 缓存
  try {
    const ts = localStorage.getItem(CACHE_KEY + '.ts')
    if (ts) {
      const age = Date.now() - parseInt(ts, 10)
      if (age < CACHE_TTL) {
        const raw = localStorage.getItem(CACHE_KEY)
        if (raw) {
          allData = JSON.parse(raw)
          return allData
        }
      }
    }
  } catch { /* ignore */ }

  loadingData = (async () => {
    for (const url of [ALL_JSON_URL, ALL_JSON_MIRROR]) {
      try {
        const res = await fetch(url)
        if (!res.ok) continue
        const data = await res.json() as Record<string, PlmnEntry>
        allData = data
        try {
          localStorage.setItem(CACHE_KEY, JSON.stringify(data))
          localStorage.setItem(CACHE_KEY + '.ts', String(Date.now()))
        } catch { /* localStorage full */ }
        return data
      } catch { continue }
    }
    return null
  })()

  loadingData.finally(() => { loadingData = null })
  return loadingData
}

function doSearch() {
  const rawQ = searchQuery.value.trim().toLowerCase()
  if (!rawQ || !allData) {
    searchResults.value = []
    return
  }

  // 规范化查询：统一分隔符 : → -
  const q = rawQ.replace(/:/g, '-')

  const results: SearchResult[] = []
  for (const [plmn, entry] of Object.entries(allData)) {
    const firstOp = entry.operators?.[0]
    const brand = firstOp?.brand || ''
    const operator = firstOp?.operator || ''
    const countryName = entry.country?.name || ''
    const countryIso = entry.country?.iso || ''
    const region = entry.country?.region || ''

    // 匹配：PLMN(234-30) / MCC:MNC(234:30) / brand / operator / country / iso / 区号(+44, 44)
    const countryCode = entry.country?.code || ''
    const haystackParts = [
      plmn.toLowerCase(),
      `${entry.mcc}:${entry.mnc}`,
      `${entry.mcc}${entry.mnc}`,
      entry.mcc, entry.mnc,
      countryName.toLowerCase(),
      countryIso.toLowerCase(),
      // 区号两种格式：+44 和 44
      countryCode ? `+${countryCode}` : '',
      countryCode ? countryCode.toLowerCase() : '',
      // 英国别名：标准 ISO 是 GB，但用户常搜 UK
      countryIso.toLowerCase() === 'gb' ? 'uk' : '',
    ]

    for (const op of (entry.operators || [])) {
      if (op.brand) haystackParts.push(op.brand.toLowerCase())
      if (op.operator) haystackParts.push(op.operator.toLowerCase())
      for (const sub of (op.subs || [])) {
        if (sub.brand) haystackParts.push(sub.brand.toLowerCase())
        if (sub.names) {
          for (const name of sub.names) {
            haystackParts.push(name.toLowerCase())
          }
        }
      }
    }

    const haystack = haystackParts.join(' ')
    const normalizedHaystack = haystack.replace(/:/g, '-')

    if (!normalizedHaystack.includes(q)) continue

    // 过滤子品牌：1) 空品牌名 2) 与父品牌同名/包含关系（视为同一运营商别名）
    const parentBrandLower = (brand || operator || '').toLowerCase()
    const subs: SubBrand[] = (firstOp?.subs || [])
      .filter(s => s.brand)
      .filter(s => {
        const subLower = s.brand!.toLowerCase()
        // 子品牌包含父品牌 或 父品牌包含子品牌 → 视为重复，不显示
        return !subLower.includes(parentBrandLower) && !parentBrandLower.includes(subLower)
      })
      .map(s => ({
        brand: s.brand || '',
        names: s.names || [],
        gid1: s.gid1 || '',
        gid2: s.gid2 || '',
      }))

    results.push({
      plmn,
      mcc: entry.mcc,
      mnc: entry.mnc,
      brand: brand || operator || plmn,
      operator,
      country: countryName,
      countryIso,
      countryCode: entry.country?.code || '',
      region,
      hasSubs: subs.length > 0,
      subs,
    })
  }

  searchResults.value = results.slice(0, 200)
}

// 防抖搜索
let searchTimer: ReturnType<typeof setTimeout> | null = null
watch(searchQuery, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(doSearch, 200)
})

// 搜索结果变化时清空选中 + 下载图标
watch(searchResults, async () => {
  selectedCount.value = 0
  nextTick(() => treeRef.value?.setCheckedKeys([]))
  // 下载前 30 个搜索结果的图标
  for (const item of searchResults.value.slice(0, 30)) {
    if (!(await getCachedIcon(item.mcc, item.mnc, item.brand))) {
      downloadIcon(item.mcc, item.mnc, item.brand).then(result => {
        if (result) {
          window.dispatchEvent(new CustomEvent('vohive-icon-updated', { detail: { mcc: item.mcc, mnc: item.mnc } }))
        }
      })
    }
  }
})

// 弹窗打开时加载数据 + 聚焦输入框
watch(() => props.modelValue, async (open) => {
  if (open) {
    searchQuery.value = ''
    searchResults.value = []
    selectedCount.value = 0
    await nextTick()
    inputRef.value?.focus()
    if (!allData) {
      searching.value = true
      await loadAllJson()
      searching.value = false
      if (!allData) {
        ElMessage.error('加载运营商索引失败，请检查网络')
      }
    }
    // 确保 catalog 已加载并下载 preset 图标
    loadPlmnCatalog().then(async () => {
      for (const p of presets) {
        if (!(await getCachedIcon(p.mcc, p.mnc, p.name))) {
          downloadIcon(p.mcc, p.mnc, p.name).then(result => {
            if (result) {
              window.dispatchEvent(new CustomEvent('vohive-icon-updated', { detail: { mcc: p.mcc, mnc: p.mnc } }))
            }
          })
        }
      }
    })
  }
})

// 转换搜索结果为 el-tree 数据
const treeData = computed<TreeNode[]>(() => {
  return searchResults.value.map(item => {
    const node: TreeNode = {
      key: item.plmn,
      label: item.brand,
      mcc: item.mcc,
      mnc: item.mnc,
      brand: item.brand,
      country: item.country,
      countryIso: item.countryIso,
      countryCode: item.countryCode,
      operator: item.operator,
      hasSubs: item.hasSubs,
      disabled: props.existingPlmns.includes(item.plmn),
    }
    if (item.hasSubs) {
      node.subCount = item.subs.length
      node.children = item.subs.map(sub => ({
        key: `${item.plmn}__${sub.brand}`,
        label: sub.brand,
        mcc: item.mcc,
        mnc: item.mnc,
        brand: sub.brand,
        isSub: true,
        names: sub.names,
        gid1: sub.gid1,
        disabled: props.existingPlmns.includes(`${item.plmn}__${sub.brand}`),
      }))
    }
    return node
  })
})

function handleCheck() {
  const keys = treeRef.value?.getCheckedKeys() || []
  selectedCount.value = keys.length
}

function handleAdd() {
  const keys = treeRef.value?.getCheckedKeys() || []
  if (keys.length === 0) {
    ElMessage.warning('请先选择运营商')
    return
  }
  emit('add', keys as string[])
  treeRef.value?.setCheckedKeys([])
  emit('update:modelValue', false)
}

function handleClose() {
  emit('update:modelValue', false)
}

// 快捷添加胶囊
interface PresetItem {
  key: string
  name: string
  mcc: string
  mnc: string
}
const presets = carrierPresets as PresetItem[]

function isPresetAdded(key: string): boolean {
  return props.existingPlmns.includes(key)
}

function handleQuickAdd(preset: PresetItem) {
  if (isPresetAdded(preset.key)) return
  emit('add', [preset.key])
  emit('update:modelValue', false)
}
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    @update:model-value="handleClose"
    title="添加运营商"
    :width="dialogWidth"
    :close-on-click-modal="false"
    class="carrier-search-dialog"
  >
    <!-- 搜索框 -->
    <div class="search-bar">
      <el-input
        ref="inputRef"
        v-model="searchQuery"
        placeholder="输入 PLMN / 运营商名称 / 国家"
        size="large"
        clearable
      >
        <template #prefix>
          <el-icon><Search24Regular /></el-icon>
        </template>
      </el-input>
    </div>

    <!-- 快捷添加胶囊（搜索时隐藏） -->
    <div v-show="!searchQuery.trim()" class="presets-bar">
      <button
        v-for="preset in presets"
        :key="preset.key"
        class="preset-capsule"
        :class="{ disabled: isPresetAdded(preset.key) }"
        :disabled="isPresetAdded(preset.key)"
        @click="handleQuickAdd(preset)"
      >
        <CarrierIcon :mcc="preset.mcc" :mnc="preset.mnc" :name="preset.name" :size="20" />
        <span class="preset-name">{{ preset.name }}</span>
        <span class="preset-plmn">{{ preset.mcc }}:{{ preset.mnc }}</span>
        <span v-if="isPresetAdded(preset.key)" class="preset-added">已添加</span>
        <span v-else class="preset-add-icon">
          <el-icon size="14"><Add24Regular /></el-icon>
        </span>
      </button>
    </div>

    <!-- 搜索结果 -->
    <div v-show="searching || searchQuery.trim()" class="search-results">
      <div v-if="searching" class="search-loading">
        <span>正在加载运营商索引...</span>
      </div>

      <div v-else-if="searchResults.length === 0" class="search-empty">
        <span>未找到匹配的运营商</span>
      </div>

      <el-tree
        v-else
        ref="treeRef"
        :data="treeData"
        node-key="key"
        show-checkbox
        check-strictly
        :expand-on-click-node="true"
        :props="{ label: 'label', disabled: 'disabled' }"
        @check="handleCheck"
        class="result-tree"
      >
        <template #default="{ data }">
          <div class="tree-node-content">
            <CarrierIcon
              :mcc="data.mcc"
              :mnc="data.mnc"
              :name="data.brand"
              :size="32"
            />
            <div class="node-info">
              <div class="node-brand-row">
                <span class="node-brand" :class="{ 'sub-brand': data.isSub }">{{ data.brand }}</span>
                <span v-if="data.hasSubs" class="node-subs-count">{{ data.subCount }} 个子品牌</span>
              </div>
              <div class="node-meta">
                <span v-if="!data.isSub" class="node-plmn">{{ data.mcc }}:{{ data.mnc }}</span>
                <span v-if="!data.isSub && data.countryCode" class="node-code">+{{ data.countryCode }}</span>
                <CountryFlag v-if="!data.isSub && data.countryIso" :iso="data.countryIso" :size="16" class="node-flag" />
                <span v-if="data.country && !data.isSub" class="node-country">{{ data.country }}</span>
                <span v-if="!data.isSub && data.operator && data.operator !== data.brand" class="node-operator">{{ data.operator }}</span>
                <span v-if="data.names?.length" class="sub-names">{{ data.names.join(', ') }}</span>
                <span v-if="data.gid1" class="sub-gid">GID1: {{ data.gid1 }}</span>
              </div>
            </div>
            <span v-if="data.disabled" class="badge-existing">已添加</span>
          </div>
        </template>
      </el-tree>
    </div>

    <!-- 底部操作栏 -->
    <template #footer>
      <div class="dialog-footer">
        <span class="selected-count" v-if="selectedCount > 0">
          已选 {{ selectedCount }} 个
        </span>
        <span v-else class="selected-count placeholder"></span>
        <div class="footer-actions">
          <el-button @click="handleClose">取消</el-button>
          <el-button
            type="primary"
            :disabled="selectedCount === 0"
            @click="handleAdd"
            class="!border-0"
          >
            <el-icon class="mr-1"><Add24Regular /></el-icon>
            <span>添加{{ selectedCount > 0 ? ` (${selectedCount})` : '' }}</span>
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.search-bar {
  margin-bottom: 12px;
}

.search-results {
  max-height: 50vh;
  min-height: 200px;
  overflow-y: auto;
  border: 1px solid var(--border);
  border-radius: 6px;
}

.search-loading,
.search-hint,
.search-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 200px;
  color: var(--muted-foreground);
  font-size: 13px;
}

.search-hint-sub {
  font-size: 12px;
  opacity: 0.7;
}

/* 快捷添加胶囊 */
.presets-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 12px;
}

.preset-capsule {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px 5px 5px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--card);
  cursor: pointer;
  transition: all 0.12s;
  position: relative;
}

.preset-capsule:hover:not(.disabled) {
  border-color: var(--brand);
  background: color-mix(in oklab, var(--brand) 8%, var(--card));
}

.preset-capsule.disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.preset-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--foreground);
  white-space: nowrap;
}

.preset-plmn {
  font-size: 10px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  white-space: nowrap;
}

.preset-added {
  font-size: 10px;
  color: var(--muted-foreground);
  background: var(--muted);
  padding: 1px 5px;
  border-radius: 3px;
  white-space: nowrap;
}

.preset-add-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 999px;
  background: color-mix(in oklab, var(--brand) 12%, transparent);
  color: var(--brand);
  opacity: 0;
  transition: opacity 0.15s;
  flex-shrink: 0;
}

.preset-capsule:hover:not(.disabled) .preset-add-icon {
  opacity: 1;
}

/* el-tree 样式（仅保留必要的最小覆盖） */
.result-tree {
  padding: 4px;
  user-select: none;
  -webkit-user-select: none;
}

:deep(.el-tree-node__content) {
  height: auto !important;
}

:deep(.el-checkbox__input.is-checked .el-checkbox__inner) {
  background-color: var(--brand);
  border-color: var(--brand);
}

/* 节点内容布局 */
.tree-node-content {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
  padding: 2px 0;
}

.node-info {
  flex: 1;
  min-width: 0;
}

.node-brand-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.node-brand {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-brand.sub-brand {
  font-size: 12px;
  font-weight: 500;
}

.node-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
  font-size: 11px;
  color: var(--muted-foreground);
}

.node-plmn {
  font-family: var(--oomol-font-mono);
}

.node-code {
  font-family: var(--oomol-font-mono);
  color: var(--brand);
  opacity: 0.8;
}

.node-flag {
  opacity: 0.9;
}

.node-country {
  opacity: 0.7;
}

.node-operator {
  opacity: 0.6;
  font-style: italic;
}

.node-subs-count {
  color: var(--brand);
  opacity: 0.7;
  flex-shrink: 0;
  white-space: nowrap;
}

.sub-names {
  opacity: 0.7;
}

.sub-gid {
  font-family: var(--oomol-font-mono);
  opacity: 0.5;
}

.badge-existing {
  font-size: 11px;
  color: var(--muted-foreground);
  background: var(--muted);
  padding: 2px 8px;
  border-radius: 4px;
  flex-shrink: 0;
}

.dialog-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.selected-count {
  font-size: 13px;
  font-weight: 600;
  color: var(--brand);
}

.selected-count.placeholder {
  width: 1px;
}

.footer-actions {
  display: flex;
  gap: 8px;
}
</style>
