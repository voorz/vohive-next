<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import type { DeviceOverviewItem } from '../types/api'

const props = defineProps<{
  device: DeviceOverviewItem | null
}>()

// ---- 阶段标签映射 ----
const stageLabels: Record<string, string> = {
  sim_init: 'SIM 初始化',
  epdg_dns: 'ePDG DNS 解析',
  tunnel_connect: '隧道连接中',
  tunnel_ready: '隧道已建立',
  ims_register: 'IMS 注册中',
  ims_challenge: 'AKA 挑战',
  ims_protected: '受保护通道',
  ims_ready: 'IMS 就绪',
  sms_ready: 'SMS 就绪',
  call_ready: '通话就绪',
  failed: '失败',
}

// ---- 实时计时 ----
const now = ref(Date.now())
let timer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  timer = setInterval(() => { now.value = Date.now() }, 1000)
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})

// ---- 计算属性 ----
const rt = computed(() => props.device?.vowifi_runtime)

const hasActivity = computed(() => {
  const r = rt.value
  if (!r) return false
  return !!(r.stage || r.stage_label || r.generation || (r.attempt_index ?? 0) > 0 || (r.register_round ?? 0) > 0)
})

const stageDisplay = computed(() => {
  const r = rt.value
  if (!r) return ''
  return r.stage_label || (r.stage ? (stageLabels[r.stage] ?? r.stage) : '')
})

const elapsedSec = computed(() => {
  const r = rt.value
  if (!r?.stage_started_at) return 0
  const start = new Date(r.stage_started_at).getTime()
  if (isNaN(start)) return 0
  return Math.max(0, Math.floor((now.value - start) / 1000))
})

const elapsedDisplay = computed(() => {
  const s = elapsedSec.value
  if (s < 60) return `${s}s`
  return `${Math.floor(s / 60)}m${s % 60}s`
})

const showRetry = computed(() => {
  const r = rt.value
  if (!r) return false
  return (r.attempt_index ?? 0) > 0 || (r.max_attempts ?? 0) > 0
})

const retryDisplay = computed(() => {
  const r = rt.value
  if (!r) return ''
  const cur = r.attempt_index ?? 0
  const max = r.max_attempts ?? 0
  if (max > 0) return `${cur + 1}/${max}`
  if (cur > 0) return `${cur + 1}`
  return ''
})

const retryDanger = computed(() =>
  (rt.value?.attempt_index ?? 0) >= Math.max(1, (rt.value?.max_attempts ?? 1) - 1)
)

const showSIPStatus = computed(() => (rt.value?.last_sip_status ?? 0) > 0)
</script>

<template>
  <div v-if="hasActivity" class="activity-collapse">
    <div class="activity-header">
      <span class="activity-title">实时活动</span>
      <span v-if="(rt?.generation ?? 0) > 0" class="activity-gen">Gen {{ rt!.generation }}</span>
    </div>
    <div class="activity-rows">
      <div class="activity-row">
        <span class="activity-row-label">当前阶段</span>
        <span class="activity-row-value" :class="{ fail: rt?.stage === 'failed' }">
          {{ stageDisplay || '--' }}
          <span v-if="elapsedSec > 0" class="activity-elapsed">{{ elapsedDisplay }}</span>
        </span>
      </div>
      <div v-if="showRetry" class="activity-row">
        <span class="activity-row-label">重试次数</span>
        <span class="activity-row-value" :class="{ danger: retryDanger }">{{ retryDisplay }}</span>
      </div>
      <div v-if="(rt?.register_round ?? 0) > 0 || (rt?.max_challenge_rounds ?? 0) > 0" class="activity-row">
        <span class="activity-row-label">挑战轮次</span>
        <span class="activity-row-value">{{ rt?.register_round ?? 0 }}{{ (rt?.max_challenge_rounds ?? 0) > 0 ? `/${rt?.max_challenge_rounds}` : '' }}</span>
      </div>
      <div v-if="(rt?.register_variant_total ?? 0) > 0" class="activity-row">
        <span class="activity-row-label">注册变体</span>
        <span class="activity-row-value">{{ (rt?.register_variant_index ?? 0) + 1 }}/{{ rt?.register_variant_total }}</span>
      </div>
      <div v-if="showSIPStatus" class="activity-row">
        <span class="activity-row-label">SIP 状态</span>
        <span class="activity-row-value" :class="{ danger: (rt?.last_sip_status ?? 0) >= 400 }">{{ rt?.last_sip_status }}</span>
      </div>
      <div v-if="showSIPStatus && rt?.last_sip_reason" class="activity-row">
        <span class="activity-row-label">SIP Reason</span>
        <span class="activity-row-value reason-text">{{ rt?.last_sip_reason }}</span>
      </div>
      <div class="activity-row">
        <span class="activity-row-label">语音 Agent</span>
        <span class="activity-row-value" :class="{ ok: rt?.call_ready, danger: !rt?.call_ready }">
          {{ rt?.call_ready ? '已注册' : '未注册' }}
        </span>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 与父组件 .vowifi-detail-collapse 保持一致的视觉风格 */
.activity-collapse {
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}
.activity-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  background: var(--muted);
  border-bottom: 1px solid var(--border);
}
.activity-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.activity-gen {
  font-size: 10px;
  font-weight: 700;
  font-family: var(--oomol-font-mono);
  padding: 1px 7px;
  border-radius: 999px;
  background: color-mix(in oklab, var(--brand) 10%, var(--muted));
  color: var(--brand);
  border: 1px solid color-mix(in oklab, var(--brand) 20%, var(--border));
}

/* 与父组件 .detail-row 保持一致的视觉风格 */
.activity-rows {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px;
}
.activity-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 12px;
}
.activity-row-label {
  color: var(--muted-foreground);
  flex-shrink: 0;
}
.activity-row-value {
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.activity-row-value.fail { color: var(--destructive); }
.activity-row-value.danger { color: var(--destructive); }
.activity-row-value.ok { color: var(--brand); }
.activity-elapsed {
  font-size: 11px;
  color: var(--muted-foreground);
  margin-left: 6px;
}
.reason-text {
  max-width: 200px;
}
</style>
