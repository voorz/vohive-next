<script setup lang="ts">
import { computed, ref, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Search24Regular, Add24Regular, CheckmarkCircle24Regular } from '@vicons/fluent'
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
  subBrands: string[]
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
const selectedPlmns = ref<Set<string>>(new Set())
const inputRef = ref<HTMLInputElement | null>(null)

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
    // 包含所有 operator 和 sub 的名称
    const haystackParts = [
      plmn.toLowerCase(),                          // 234-30
      `${entry.mcc}:${entry.mnc}`,                 // 234:30
      `${entry.mcc}${entry.mnc}`,                  // 23430
      entry.mcc, entry.mnc,
      countryName.toLowerCase(),
      countryIso.toLowerCase(),
    ]

    // 加入所有 operator 的 brand/operator
    for (const op of (entry.operators || [])) {
      if (op.brand) haystackParts.push(op.brand.toLowerCase())
      if (op.operator) haystackParts.push(op.operator.toLowerCase())
      // 加入所有 sub 的 brand 和 names
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

    // 同时规范化 haystack 中的分隔符
    const normalizedHaystack = haystack.replace(/:/g, '-')

    if (!normalizedHaystack.includes(q)) continue

    const subBrands = (firstOp?.subs || []).map(s => s.brand || '').filter(Boolean)

    results.push({
      plmn,
      mcc: entry.mcc,
      mnc: entry.mnc,
      brand: brand || operator || plmn,
      operator,
      country: countryName,
      countryIso,
      region,
      hasSubs: subBrands.length > 0,
      subBrands,
    })
  }

  // 限制结果数量
  searchResults.value = results.slice(0, 200)
}

// 防抖搜索
let searchTimer: ReturnType<typeof setTimeout> | null = null
watch(searchQuery, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(doSearch, 200)
})

// 弹窗打开时加载数据 + 聚焦输入框
watch(() => props.modelValue, async (open) => {
  if (open) {
    selectedPlmns.value.clear()
    searchQuery.value = ''
    searchResults.value = []
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

function toggleSelect(plmn: string) {
  if (selectedPlmns.value.has(plmn)) {
    selectedPlmns.value.delete(plmn)
  } else {
    selectedPlmns.value.add(plmn)
  }
  // 触发响应式更新
  selectedPlmns.value = new Set(selectedPlmns.value)
}

function isSelected(plmn: string) {
  return selectedPlmns.value.has(plmn)
}

function isExisting(plmn: string) {
  return props.existingPlmns.includes(plmn)
}

const selectedCount = computed(() => selectedPlmns.value.size)

function handleAdd() {
  const plmns = Array.from(selectedPlmns.value)
  if (plmns.length === 0) {
    ElMessage.warning('请先选择运营商')
    return
  }
  emit('add', plmns)
  selectedPlmns.value.clear()
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

      <div v-else class="result-list">
        <div
          v-for="item in searchResults"
          :key="item.plmn"
          class="result-item"
          :class="{
            selected: isSelected(item.plmn),
            existing: isExisting(item.plmn)
          }"
          @click="!isExisting(item.plmn) && toggleSelect(item.plmn)"
        >
          <!-- 选中标记 -->
          <div class="result-check">
            <el-icon v-if="isExisting(item.plmn)" size="16" class="check-existing" />
            <el-icon v-else-if="isSelected(item.plmn)" size="16" class="check-selected">
              <CheckmarkCircle24Regular />
            </el-icon>
          </div>

          <!-- 图标 -->
          <CarrierIcon
            :mcc="item.mcc"
            :mnc="item.mnc"
            :name="item.brand"
            :size="32"
          />

          <!-- 信息 -->
          <div class="result-info">
            <div class="result-brand">{{ item.brand }}</div>
            <div class="result-meta">
              <span class="result-plmn">{{ item.mcc }}:{{ item.mnc }}</span>
              <span v-if="item.country" class="result-country">{{ item.country }}</span>
              <span v-if="item.hasSubs" class="result-subs">{{ item.subBrands.join(', ') }}</span>
            </div>
          </div>

          <!-- 已添加标记 -->
          <span v-if="isExisting(item.plmn)" class="badge-existing">已添加</span>
        </div>
      </div>
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

.result-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 4px;
}

.result-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid transparent;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
}

.result-item:hover {
  background: var(--accent);
}

.result-item.selected {
  background: color-mix(in oklab, var(--brand) 8%, var(--card));
  border-color: color-mix(in oklab, var(--brand) 30%, var(--border));
}

.result-item.existing {
  opacity: 0.5;
  cursor: default;
}

.result-item.existing:hover {
  background: transparent;
}

.result-check {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.check-selected {
  color: var(--brand);
}

.check-existing {
  width: 16px;
  height: 16px;
  border-radius: 999px;
  background: var(--muted-foreground);
  opacity: 0.4;
}

.result-info {
  flex: 1;
  min-width: 0;
}

.result-brand {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
  font-size: 11px;
  color: var(--muted-foreground);
}

.result-plmn {
  font-family: var(--oomol-font-mono);
}

.result-subs {
  color: var(--brand);
  opacity: 0.7;
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
