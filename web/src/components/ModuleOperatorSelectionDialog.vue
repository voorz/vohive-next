<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { devicesService } from '../services/devices'
import type { OperatorCandidate, OperatorSelection, OperatorSelectionRAT } from '../types/api'
import { useDevicesStore } from '../stores/devices'
import { ElMessage } from 'element-plus'
import { Settings24Regular } from '@vicons/fluent'

const props = defineProps<{
  modelValue: boolean
  deviceId: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'updated': []
}>()

const devicesStore = useDevicesStore()
const loading = ref(false)
const currentSelection = ref<OperatorSelection | null>(null)
const lastNotifiedScanKey = ref('')

const scanState = computed(() => {
  if (!props.deviceId) return null
  return devicesStore.getOperatorScan(props.deviceId)
})
const scanning = computed(() => scanState.value?.status === 'running')
const candidates = computed<OperatorCandidate[]>(() => scanState.value?.candidates || [])
const scanMessage = computed(() => scanState.value?.message || '')
const scanError = computed(() => (scanState.value?.retryable ? '' : (scanState.value?.error || '')))
const scanRetryable = computed(() => !!scanState.value?.retryable)

// 开关状态：后端 mode=manual → 开
const isManualMode = computed(() => currentSelection.value?.mode === 'manual')
// 展示扫描区域：手动模式 或 正在扫描 或 有结果
const showScanArea = computed(() => isManualMode.value || scanning.value || candidates.value.length > 0)

function handleDialogModelUpdate(value: boolean) {
  emit('update:modelValue', value)
}

function firstCandidateRAT(candidate: OperatorCandidate): OperatorSelectionRAT | undefined {
  return candidate.rats?.find(rat => !!rat)
}

function ratDisplay(candidate: OperatorCandidate) {
  const rats = candidate.rats?.filter(Boolean) || []
  return rats.length > 0 ? rats.map(rat => rat.toUpperCase()).join(' / ') : '--'
}

const loadCurrent = async () => {
  if (!props.deviceId) return
  loading.value = true
  try {
    const res = await devicesService.getOperatorSelection(props.deviceId)
    if (!res.ok) throw new Error(res.error.message)
    currentSelection.value = res.data
  } catch (e: any) {
    ElMessage.error(e.message || '加载当前配置失败')
  } finally {
    loading.value = false
  }
}

const doScan = async () => {
  if (!props.deviceId || scanning.value) return
  try {
    await devicesStore.startOperatorScan(props.deviceId)
  } catch (e: any) {
    ElMessage.error(e.message || '扫描网络失败')
  }
}

// 开关关闭 → 恢复自动选网
const onToggleOff = async () => {
  loading.value = true
  try {
    const res = await devicesService.setOperatorSelection(props.deviceId, { mode: 'automatic' })
    if (!res.ok) throw new Error(res.error.message)
    ElMessage.success('已恢复自动选网')
    currentSelection.value = res.data
    emit('updated')
  } catch (e: any) {
    ElMessage.error(e.message || '设置失败')
  } finally {
    loading.value = false
  }
}

// 自动模式下点击开关 → 提示需要先扫描并选择运营商
const onSwitchClick = () => {
  if (loading.value) return
  if (isManualMode.value) {
    // 手动模式 → 关闭（恢复自动）
    onToggleOff()
  } else {
    // 自动模式 → 触发扫描（不切后端模式，选了运营商才变 manual）
    doScan()
  }
}

const setModeManual = async (candidate: OperatorCandidate) => {
  loading.value = true
  try {
    const rat = firstCandidateRAT(candidate)
    const res = await devicesService.setOperatorSelection(props.deviceId, {
      mode: 'manual',
      plmn: candidate.plmn,
      includes_pcs_digit: candidate.includes_pcs_digit,
      rat
    })
    if (!res.ok) throw new Error(res.error.message)
    ElMessage.success(`已锁定网络 ${candidate.operator_name || candidate.plmn}`)
    currentSelection.value = res.data
    emit('updated')
  } catch (e: any) {
    ElMessage.error(e.message || '设置失败')
  } finally {
    loading.value = false
  }
}

watch(() => props.modelValue, (val) => {
  if (val) {
    loadCurrent()
    if (props.deviceId) {
      void devicesStore.resumeOperatorScan(props.deviceId)
    }
  }
})

watch(scanState, (next) => {
  if (!next || !props.modelValue) return
  const notifyKey = `${next.scan_id}:${next.status}`
  if (notifyKey === lastNotifiedScanKey.value) return
  if (next.status === 'complete') {
    ElMessage.success('运营商扫描完成')
  } else if (next.status === 'failed' && next.retryable) {
    ElMessage.warning(next.message || '扫描超时或模组忙，请稍后重试')
  }
  lastNotifiedScanKey.value = notifyKey
})
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    @update:model-value="handleDialogModelUpdate"
    width="min(500px, 92vw)"
    class="op-sel-dialog"
  >
    <template #header>
      <div class="op-sel-header">
        <div class="op-sel-icon-box">
          <el-icon size="14"><Settings24Regular /></el-icon>
        </div>
        <span class="op-sel-title">运营商网络选择</span>
      </div>
    </template>

    <div class="op-sel-body">
      <!-- 手动模式开关 -->
      <div class="op-sel-toggle-row">
        <div class="op-sel-toggle-info">
          <div class="op-sel-toggle-label">手动选择网络</div>
          <div class="op-sel-toggle-desc">开启后扫描可用网络，选择运营商锁定；关闭恢复自动</div>
        </div>
        <div
          class="op-sel-switch"
          :class="{ on: isManualMode || scanning, loading: loading }"
          @click="onSwitchClick"
        >
          <span class="op-sel-switch-dot" />
        </div>
      </div>

      <!-- 自动模式提示 -->
      <div v-if="!showScanArea" class="op-sel-auto-hint">
        自动模式：设备将自动选择最佳可用运营商网络
      </div>

      <!-- 当前锁定信息 -->
      <div v-if="isManualMode && currentSelection?.plmn" class="op-sel-locked">
        <span class="op-sel-locked-label">已锁定</span>
        <span class="op-sel-locked-value">{{ currentSelection.plmn }}</span>
      </div>

      <!-- 扫描区域 -->
      <template v-if="showScanArea">
        <!-- 重新扫描按钮 -->
        <div v-if="!scanning && candidates.length > 0" class="op-sel-rescan">
          <el-button @click="doScan" size="small" plain>重新扫描</el-button>
        </div>

        <!-- 扫描消息 -->
        <div v-if="scanMessage || scanError" class="op-sel-scan-msg" :class="{ error: !!scanError }">
          {{ scanError || scanMessage }}
        </div>

        <!-- 扫描结果列表 -->
        <div v-if="candidates.length > 0" class="op-sel-list">
          <div
            v-for="c in candidates"
            :key="`${c.plmn}-${ratDisplay(c)}`"
            class="op-sel-candidate"
            :class="{ current: c.status === 'current', disabled: loading }"
            @click="!loading && setModeManual(c)"
          >
            <div class="op-sel-candidate-info">
              <div class="op-sel-candidate-name">
                {{ c.operator_name || c.short_name || '未知网络' }}
                <span v-if="c.status === 'current'" class="op-sel-tag current">当前</span>
                <span v-else-if="c.status === 'forbidden'" class="op-sel-tag forbidden">禁用</span>
              </div>
              <div class="op-sel-candidate-meta">{{ c.plmn }} · {{ ratDisplay(c) }}</div>
            </div>
            <span class="op-sel-lock-btn">锁定</span>
          </div>
        </div>

        <!-- 空状态 -->
        <div v-else-if="scanning" class="op-sel-empty">
          正在搜索周围网络，这可能需要 1-3 分钟...
        </div>
        <div v-else-if="scanRetryable" class="op-sel-empty warning">
          {{ scanMessage || '扫描超时或模组忙，请稍后重试' }}
        </div>
        <div v-else class="op-sel-empty">
          暂无可用网络
          <el-button @click="doScan" :loading="scanning" size="small" plain>重新扫描</el-button>
        </div>
      </template>
    </div>
  </el-dialog>
</template>

<style scoped>
.op-sel-header {
  display: flex;
  align-items: center;
  gap: 8px;
}
.op-sel-icon-box {
  width: 28px;
  height: 28px;
  border-radius: 5px;
  background: var(--muted);
  border: 1px solid var(--border);
  color: var(--foreground);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.op-sel-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.op-sel-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

/* 开关行 */
.op-sel-toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--muted);
}
.op-sel-toggle-info { flex: 1; min-width: 0; }
.op-sel-toggle-label { font-size: 14px; font-weight: 700; color: var(--foreground); }
.op-sel-toggle-desc { font-size: 11px; color: var(--muted-foreground); margin-top: 2px; }

/* Toggle switch */
.op-sel-switch {
  width: 44px;
  height: 26px;
  border-radius: 999px;
  background: color-mix(in oklab, var(--muted-foreground) 30%, transparent);
  cursor: pointer;
  display: flex;
  align-items: center;
  padding: 2px;
  flex-shrink: 0;
  transition: all 0.2s;
}
.op-sel-switch.on { background: var(--brand); }
.op-sel-switch.loading { opacity: 0.5; cursor: not-allowed; }
.op-sel-switch-dot {
  width: 22px;
  height: 22px;
  border-radius: 999px;
  background: #fff;
  box-shadow: 0 1px 3px rgba(0,0,0,0.2);
  transition: transform 0.2s;
  transform: translateX(0);
}
.op-sel-switch.on .op-sel-switch-dot { transform: translateX(18px); }

/* 重新扫描 */
.op-sel-rescan {
  display: flex;
  justify-content: flex-end;
}

/* 自动模式提示 */
.op-sel-auto-hint {
  padding: 12px 14px;
  border-radius: 8px;
  background: var(--muted);
  border: 1px solid var(--border);
  font-size: 12px;
  color: var(--muted-foreground);
  text-align: center;
}

/* 已锁定信息 */
.op-sel-locked {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-radius: 8px;
  background: color-mix(in oklab, var(--warning) 10%, var(--card));
  border: 1px solid color-mix(in oklab, var(--warning) 20%, var(--border));
}
.op-sel-locked-label { font-size: 12px; color: var(--muted-foreground); }
.op-sel-locked-value { font-size: 14px; font-weight: 700; color: var(--foreground); }

/* 扫描消息 */
.op-sel-scan-msg {
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 12px;
  background: color-mix(in oklab, var(--info, #3b63fb) 8%, var(--muted));
  border: 1px solid color-mix(in oklab, var(--info, #3b63fb) 20%, var(--border));
  color: var(--foreground);
}
.op-sel-scan-msg.error {
  background: color-mix(in oklab, var(--warning) 8%, var(--muted));
  border-color: color-mix(in oklab, var(--warning) 20%, var(--border));
}

/* 扫描结果列表 */
.op-sel-list {
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow-y: auto;
  max-height: 300px;
}
.op-sel-candidate {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  cursor: pointer;
  transition: background 0.12s;
  border-bottom: 1px solid var(--border);
}
.op-sel-candidate:last-child { border-bottom: none; }
.op-sel-candidate:hover { background: var(--accent); }
.op-sel-candidate.current { background: color-mix(in oklab, var(--brand) 5%, transparent); }
.op-sel-candidate.disabled { opacity: 0.5; cursor: not-allowed; }
.op-sel-candidate-info { min-width: 0; }
.op-sel-candidate-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  display: flex;
  align-items: center;
  gap: 6px;
}
.op-sel-candidate-meta {
  font-size: 11px;
  color: var(--muted-foreground);
  margin-top: 2px;
}
.op-sel-tag {
  padding: 1px 6px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 700;
}
.op-sel-tag.current {
  background: color-mix(in oklab, var(--brand) 15%, transparent);
  color: var(--brand);
  border: 1px solid color-mix(in oklab, var(--brand) 30%, transparent);
}
.op-sel-tag.forbidden {
  background: color-mix(in oklab, var(--destructive) 15%, transparent);
  color: var(--destructive);
  border: 1px solid color-mix(in oklab, var(--destructive) 30%, transparent);
}
.op-sel-lock-btn {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted-foreground);
  opacity: 0;
  transition: opacity 0.12s;
}
.op-sel-candidate:hover .op-sel-lock-btn {
  opacity: 1;
  color: var(--brand);
}

/* 空状态 */
.op-sel-empty {
  padding: 32px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--muted-foreground);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}
.op-sel-empty.warning { color: var(--warning); }
</style>
