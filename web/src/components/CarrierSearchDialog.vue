<script setup lang="ts">
import { computed, ref, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Search24Regular, Add24Regular } from '@vicons/fluent'
import CarrierIcon from './CarrierIcon.vue'

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

    // 匹配：PLMN(234-30) / MCC:MNC(234:30) / brand / operator / country / iso
    const haystackParts = [
      plmn.toLowerCase(),
      `${entry.mcc}:${entry.mnc}`,
      `${entry.mcc}${entry.mnc}`,
      entry.mcc, entry.mnc,
      countryName.toLowerCase(),
      countryIso.toLowerCase(),
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

    const subs: SubBrand[] = (firstOp?.subs || [])
      .filter(s => s.brand)
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

// 搜索结果变化时清空选中
watch(searchResults, () => {
  selectedCount.value = 0
  nextTick(() => treeRef.value?.setCheckedKeys([]))
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

    <!-- 搜索结果 -->
    <div class="search-results">
      <div v-if="searching" class="search-loading">
        <span>正在加载运营商索引...</span>
      </div>

      <div v-else-if="!searchQuery.trim()" class="search-hint">
        <el-icon size="28"><Search24Regular /></el-icon>
        <span>输入关键词搜索运营商</span>
        <span class="search-hint-sub">支持 PLMN、运营商名称、国家名称</span>
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
              :size="data.isSub ? 24 : 32"
            />
            <div class="node-info">
              <div class="node-brand" :class="{ 'sub-brand': data.isSub }">{{ data.brand }}</div>
              <div class="node-meta">
                <span v-if="!data.isSub" class="node-plmn">{{ data.mcc }}:{{ data.mnc }}</span>
                <span v-if="data.country && !data.isSub" class="node-country">{{ data.country }}</span>
                <span v-if="data.hasSubs" class="node-subs-count">{{ data.subCount }} 个子品牌</span>
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

/* el-tree 自定义样式 */
.result-tree {
  padding: 4px;
  user-select: none;
  -webkit-user-select: none;
}

:deep(.el-tree-node__content) {
  height: auto !important;
  min-height: 48px;
  padding: 4px 6px;
  border-radius: 6px;
  transition: background 0.12s;
}

:deep(.el-tree-node__content:hover) {
  background: var(--accent);
}

:deep(.el-checkbox__inner) {
  border-color: var(--border);
}

:deep(.el-checkbox__input.is-checked .el-checkbox__inner) {
  background-color: var(--brand);
  border-color: var(--brand);
}

:deep(.el-tree-node__expand-icon) {
  color: var(--muted-foreground);
  font-size: 14px;
}

:deep(.el-tree-node__expand-icon.expanded) {
  color: var(--brand);
}

/* 节点内容布局 */
.tree-node-content {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

.node-info {
  flex: 1;
  min-width: 0;
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

.node-country {
  opacity: 0.7;
}

.node-subs-count {
  color: var(--brand);
  opacity: 0.7;
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
