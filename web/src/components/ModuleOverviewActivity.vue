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
  recover_failed: 'VoWiFi 启动失败',
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

const stageDisplay = computed(() => {
  const r = rt.value
  if (!r) return ''
  return r.stage_label || (r.stage ? (stageLabels[r.stage] ?? r.stage) : '') || (r.phase ? (stageLabels[r.phase] ?? r.phase) : '')
})

const elapsedSec = computed(() => {
  const r = rt.value
  if (!r?.stage_started_at) return 0
  const start = new Date(r.stage_started_at).getTime()
  if (isNaN(start) || start <= 0) return 0
  return Math.max(0, Math.floor((now.value - start) / 1000))
})

const elapsedDisplay = computed(() => {
  const s = elapsedSec.value
  if (s < 60) return `${s}s`
  return `${Math.floor(s / 60)}m${s % 60}s`
})

// ---- 位置2：SIP状态 / 重试次数 / 挑战轮次 轮换 ----
const hasRetry = computed(() => {
  const r = rt.value
  return (r?.max_attempts ?? 0) > 0 && (r?.attempt_index ?? 0) > 0
})

const hasChallengeRound = computed(() =>
  (rt.value?.register_round ?? 0) > 0 || (rt.value?.max_challenge_rounds ?? 0) > 0
)

// 位置2 优先级：挑战轮次 > 重试次数 > SIP状态
const slot2 = computed(() => {
  if (hasChallengeRound.value) {
    const r = rt.value!
    const round = r.register_round ?? 0
    const max = r.max_challenge_rounds ?? 0
    return {
      label: '挑战轮次',
      value: max > 0 ? `${round}/${max}` : `${round}`,
      tone: 'normal' as const,
    }
  }
  if (hasRetry.value) {
    const r = rt.value!
    const cur = r.attempt_index ?? 0
    const max = r.max_attempts ?? 0
    return {
      label: '重试次数',
      value: max > 0 ? `${cur + 1}/${max}` : `${cur + 1}`,
      tone: (cur >= Math.max(1, max - 1)) ? 'danger' as const : 'normal' as const,
    }
  }
  // 默认显示 SIP 状态
  const status = rt.value?.last_sip_status ?? 0
  return {
    label: 'SIP 状态',
    value: status > 0 ? `${status}` : '--',
    tone: status >= 400 ? 'danger' as const : 'normal' as const,
  }
})

// ---- 位置3：SIP Reason / 注册变体 轮换 ----
const hasVariant = computed(() =>
  (rt.value?.register_variant_total ?? 0) > 0
)

// 位置3 优先级：注册变体 > SIP Reason
const slot3 = computed(() => {
  if (hasVariant.value) {
    const r = rt.value!
    const idx = (r.register_variant_index ?? 0) + 1
    const total = r.register_variant_total ?? 0
    return {
      label: '注册变体',
      value: `${idx}/${total}`,
      tone: 'normal' as const,
    }
  }
  // 默认显示 SIP Reason
  const reason = rt.value?.last_sip_reason || '--'
  const isError = (rt.value?.last_sip_status ?? 0) >= 400
  return {
    label: 'SIP Reason',
    value: reason,
    tone: isError ? 'danger' as const : 'normal' as const,
  }
})

</script>

<template>
  <div class="activity-collapse">
    <div class="activity-header">
      <span class="activity-title">实时活动</span>
      <span class="activity-gen">Gen {{ rt?.generation ?? '--' }}</span>
    </div>
    <div class="activity-rows">
      <!-- 位置1：当前阶段（固定） -->
      <div class="activity-row">
        <span class="activity-row-label">当前阶段</span>
        <span class="activity-row-value" :class="{ fail: rt?.stage === 'failed' || rt?.phase === 'recover_failed' }">
          {{ stageDisplay || '--' }}
          <span v-if="elapsedSec > 0" class="activity-elapsed">{{ elapsedDisplay }}</span>
        </span>
      </div>
      <!-- 位置2：SIP状态 / 重试次数 / 挑战轮次（轮换） -->
      <div class="activity-row">
        <span class="activity-row-label">{{ slot2.label }}</span>
        <span class="activity-row-value" :class="{ danger: slot2.tone === 'danger' }">{{ slot2.value }}</span>
      </div>
      <!-- 位置3：SIP Reason / 注册变体（轮换） -->
      <div class="activity-row">
        <span class="activity-row-label">{{ slot3.label }}</span>
        <span class="activity-row-value reason-text" :class="{ danger: slot3.tone === 'danger' }">{{ slot3.value }}</span>
      </div>
      <!-- 位置4：语音 Agent（固定） -->
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
  flex: 1;
  min-width: 0;
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
  line-height: 1;
  padding: 0 6px;
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
