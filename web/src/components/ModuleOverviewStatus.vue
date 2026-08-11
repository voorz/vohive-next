<script setup lang="ts">
import { computed, ref } from 'vue'
import type { DeviceOverviewItem } from '../types/api'
import { isControlOnline, isRadioRegistered, isRecoveryPhase, lifecycleStatusLabel } from '../utils/deviceLifecycle'
import { Pulse24Regular, Settings24Regular, ChevronDown24Regular } from '@vicons/fluent'
import OperatorSelectionDialog from './ModuleOperatorSelectionDialog.vue'
import ModuleOverviewActivity from './ModuleOverviewActivity.vue'
import { useDevicesStore } from '../stores/devices'

const props = defineProps<{
  device: DeviceOverviewItem | null
}>()

const store = useDevicesStore()
const showOperatorSelection = ref(false)

// VoWiFi 详情折叠
const showVowifiDetail = ref(false)
const hasError = computed(() =>
  !!(props.device?.vowifi_runtime?.last_error_class || props.device?.vowifi_runtime?.last_error)
)

// ---- VoWiFi 状态 ----
const vowifiEnabled = computed(() => !!props.device?.vowifi_enabled)

const readinessItems = computed(() => {
  const rt = props.device?.vowifi_runtime
  return [
    { key: 'SIM',    ready: rt?.sim_ready },
    { key: 'Access', ready: rt?.access_ready },
    { key: 'Tunnel', ready: rt?.tunnel_ready },
    { key: 'IMS',    ready: rt?.ims_ready },
    { key: 'SMS',    ready: rt?.sms_ready },
    { key: 'Call',   ready: rt?.call_ready },
  ]
})

const vowifiStatus = computed<'ok' | 'partial' | 'off'>(() => {
  const rt = props.device?.vowifi_runtime
  if (!rt) return 'off'
  const all = [rt.sim_ready, rt.access_ready, rt.tunnel_ready, rt.ims_ready, rt.sms_ready, rt.call_ready]
  if (all.every(Boolean)) return 'ok'
  if (all.some(Boolean)) return 'partial'
  return 'off'
})

const notReadyNames = computed(() =>
  readinessItems.value.filter(i => !i.ready).map(i => i.key)
)

// ---- 蜂窝状态 ----
const controlOnline = computed(() => isControlOnline(props.device))
const isRegistered = computed(() => isRadioRegistered(props.device))
const inRecovery = computed(() => isRecoveryPhase(props.device?.lifecycle_phase))
const deviceRunning = computed(() => !!props.device?.running)

function hasValidSignalDbm(dbm: number | null | undefined): dbm is number {
  return typeof dbm === 'number' && Number.isFinite(dbm) && dbm !== 0 && dbm !== -999
}

const signalLevel = computed<number>(() => {
  const dbm = props.device?.modem?.signal_dbm
  if (!hasValidSignalDbm(dbm)) return 0
  if (dbm >= -75)  return 5
  if (dbm >= -85)  return 4
  if (dbm >= -95)  return 3
  if (dbm >= -105) return 2
  return 1
})

const signalTone = computed<'good' | 'fair' | 'poor'>(() => {
  const dbm = props.device?.modem?.signal_dbm
  if (!hasValidSignalDbm(dbm)) return 'poor'
  if (dbm >= -85) return 'good'
  if (dbm >= -100) return 'fair'
  return 'poor'
})

// 蜂窝 hero 状态文本（含生命周期阶段）
const cellularStatusText = computed(() => {
  const phaseText = lifecycleStatusLabel(props.device?.lifecycle_phase)
  if (phaseText && props.device?.lifecycle_phase !== 'online' && props.device?.lifecycle_phase !== 'offline') return phaseText
  if (!controlOnline.value) return deviceRunning.value ? '控制面恢复中' : '离线'
  if (isRegistered.value) return ''
  if (props.device?.registration_state_label === 'searching') return '搜索网络中'
  if (props.device?.registration_state_label === 'denied') return '驻网被拒'
  return '未驻网'
})

// 蜂窝 hero 色调
const cellularHeroTone = computed<'ok' | 'warning' | 'off'>(() => {
  if (inRecovery.value) return 'warning'
  if (!controlOnline.value) return 'off'
  return isRegistered.value ? 'ok' : 'warning'
})

const networkModeDisplay = computed(() =>
  [props.device?.modem?.network_duplex, props.device?.modem?.network_mode].filter(Boolean).join(' ') || '--'
)
</script>

<template>
  <div class="ov-card">
    <div class="ov-card-head">
      <div class="ov-icon-box">
        <el-icon size="14"><Pulse24Regular /></el-icon>
      </div>
      <span class="ov-card-title">运行状态</span>
    </div>
    <div class="ov-card-body">

      <!-- VoWiFi 模式 -->
      <template v-if="vowifiEnabled">
        <div class="status-hero" :class="vowifiStatus">
          <div class="status-pulse" :class="vowifiStatus"></div>
          <div class="status-hero-text">
            <div class="status-hero-title">
              <template v-if="vowifiStatus === 'ok'">WiFi-Calling · 全部就绪</template>
              <template v-else-if="vowifiStatus === 'partial'">{{ notReadyNames.join(' · ') }} 未就绪</template>
              <template v-else>VoWiFi 未连接</template>
            </div>
            <div v-if="vowifiStatus === 'ok'" class="status-hero-sub">通过 ePDG 隧道连接 IMS 核心网</div>
            <div v-else-if="vowifiStatus === 'partial' && device?.vowifi_runtime?.last_reason" class="status-hero-sub">
              {{ device.vowifi_runtime.last_reason }}
            </div>
          </div>
        </div>

        <div class="readiness-chain">
          <div
            v-for="item in readinessItems"
            :key="item.key"
            class="readiness-segment"
            :class="{ ready: item.ready === true, 'not-ready': item.ready === false }"
          />
        </div>
        <div class="readiness-labels">
          <span
            v-for="item in readinessItems"
            :key="item.key"
            class="readiness-label"
            :class="{ fail: item.ready === false }"
          >{{ item.key }}</span>
        </div>

        <!-- 实时活动（嵌入运行状态卡片内部） -->
        <ModuleOverviewActivity :device="device" />

        <div class="vowifi-detail-collapse">
          <button class="vowifi-detail-header" @click="showVowifiDetail = !showVowifiDetail">
            <el-icon size="14" class="vowifi-detail-arrow" :class="{ expanded: showVowifiDetail || hasError }">
              <ChevronDown24Regular />
            </el-icon>
            <span class="vowifi-detail-title">详情</span>
          </button>
          <div v-show="showVowifiDetail || hasError" class="status-detail-rows">
            <div class="detail-row">
              <span class="detail-row-label">数据平面</span>
              <span class="detail-row-value">{{ device?.vowifi_runtime?.dataplane_mode || '--' }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-row-label">最后原因</span>
              <span class="detail-row-value">{{ device?.vowifi_runtime?.last_reason || '--' }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-row-label">错误分类</span>
              <span class="detail-row-value">{{ device?.vowifi_runtime?.last_error_class || '--' }}</span>
            </div>
          </div>
        </div>
      </template>

      <!-- 蜂窝模式 -->
      <template v-else>
        <div class="cellular-hero" :class="cellularHeroTone">
          <div class="status-pulse" :class="cellularHeroTone"></div>
          <div class="cellular-hero-info">
            <div class="cellular-hero-name">
              <template v-if="isRegistered">{{ device?.modem?.operator || '--' }}</template>
              <template v-else>{{ cellularStatusText }}</template>
            </div>
            <div class="cellular-hero-meta">
              <template v-if="isRegistered">{{ networkModeDisplay }}</template>
              <template v-else-if="!deviceRunning">设备未运行</template>
              <template v-else>未驻网</template>
            </div>
          </div>
          <button
            v-if="device?.id"
            class="ov-icon-btn"
            title="网络选择设置"
            @click="showOperatorSelection = true"
          >
            <el-icon size="16"><Settings24Regular /></el-icon>
          </button>
        </div>

        <div v-if="hasValidSignalDbm(device?.modem?.signal_dbm)" class="signal-box">
          <div>
            <div class="signal-dbz">
              <span class="signal-dbz-value" :class="signalTone">{{ device?.modem?.signal_dbm }}</span>
              <span class="signal-dbz-unit">dBm</span>
            </div>
            <div class="signal-detail">
              <span>RSRP {{ device?.modem?.signal_rsrp ?? '--' }}</span>
              <span>RSRQ {{ device?.modem?.signal_rsrq ?? '--' }}</span>
              <span>SINR {{ device?.modem?.signal_sinr ?? '--' }}</span>
              <template v-if="device?.modem?.nr5g_signal_sinr !== undefined">
                <span>NR5G SINR {{ device?.modem?.nr5g_signal_sinr }}</span>
              </template>
            </div>
          </div>
          <div class="signal-bars">
            <div
              v-for="i in 5"
              :key="i"
              class="signal-bar"
              :class="i <= signalLevel ? ['active', signalTone] : 'inactive'"
            />
          </div>
        </div>

        <div class="status-detail-rows">
          <div class="detail-row">
            <span class="detail-row-label">网络模式</span>
            <span class="detail-row-value">{{ networkModeDisplay }}</span>
          </div>
          <div class="detail-row">
            <span class="detail-row-label">频段 / 信道</span>
            <span class="detail-row-value">{{ device?.modem?.radio_band || '--' }} / {{ device?.modem?.radio_channel ?? '--' }}</span>
          </div>
          <div class="detail-row">
            <span class="detail-row-label">注册状态</span>
            <span class="detail-row-value">{{ device?.modem?.reg_status_text || '--' }}</span>
          </div>
        </div>
      </template>

    </div>
  </div>

  <!-- 运营商选择弹窗 -->
  <OperatorSelectionDialog
    v-if="device?.id"
    v-model="showOperatorSelection"
    :device-id="device.id"
    @updated="store.fetchDetail(device!.id)"
  />
</template>

<style scoped>
.ov-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
}
.ov-card-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
}
.ov-icon-box {
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
.ov-card-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}
.ov-card-head-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}
.ov-icon-btn {
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.12s;
  flex-shrink: 0;
}
.ov-icon-btn:hover { background: var(--accent); color: var(--foreground); }
.ov-card-body { padding: 14px; }

/* Hero */
.status-hero {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 14px;
  border-radius: 8px;
  margin-bottom: 14px;
  position: relative;
  overflow: hidden;
}
.status-hero.ok {
  background: linear-gradient(135deg, color-mix(in oklab, var(--brand) 12%, var(--card)), color-mix(in oklab, var(--brand) 4%, var(--card)));
  border: 1px solid color-mix(in oklab, var(--brand) 25%, var(--border));
}
.status-hero.partial {
  background: linear-gradient(135deg, color-mix(in oklab, var(--warning) 12%, var(--card)), color-mix(in oklab, var(--warning) 4%, var(--card)));
  border: 1px solid color-mix(in oklab, var(--warning) 25%, var(--border));
}
.status-hero.off { background: var(--muted); border: 1px solid var(--border); }

.status-pulse {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  flex-shrink: 0;
  position: relative;
}
.status-pulse.ok { background: var(--brand); }
.status-pulse.partial { background: var(--warning); }
.status-pulse.warning { background: var(--warning); }
.status-pulse.off { background: var(--muted-foreground); opacity: 0.4; }
.status-pulse.ok::after, .status-pulse.partial::after, .status-pulse.warning::after {
  content: '';
  position: absolute;
  inset: -3px;
  border-radius: 999px;
  border: 2px solid currentColor;
  animation: ov-pulse-ring 2s ease-out infinite;
}
.status-pulse.ok::after { color: var(--brand); }
.status-pulse.partial::after { color: var(--warning); }
.status-pulse.warning::after { color: var(--warning); }
@keyframes ov-pulse-ring {
  0% { transform: scale(0.8); opacity: 0.8; }
  100% { transform: scale(2.2); opacity: 0; }
}

.status-hero-text { flex: 1; min-width: 0; }
.status-hero-title { font-size: 15px; font-weight: 700; line-height: 1.3; }
.status-hero.ok .status-hero-title { color: color-mix(in oklab, var(--brand) 85%, var(--foreground)); }
.status-hero.partial .status-hero-title { color: color-mix(in oklab, var(--warning) 85%, var(--foreground)); }
.status-hero.off .status-hero-title { color: var(--muted-foreground); }
.status-hero-sub { font-size: 12px; margin-top: 2px; color: var(--muted-foreground); }

/* Readiness */
.readiness-chain { display: flex; gap: 6px; margin-bottom: 6px; }
.readiness-segment { flex: 1; height: 12px; border-radius: 999px; background: var(--muted); position: relative; overflow: hidden; }
.readiness-segment.ready { background: var(--brand); }
.readiness-segment.not-ready { background: color-mix(in oklab, var(--destructive) 60%, var(--muted)); }
.readiness-segment.ready::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, transparent, color-mix(in oklab, white 30%, transparent), transparent);
  animation: ov-shimmer 2s ease-in-out infinite;
}
@keyframes ov-shimmer {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}
.readiness-labels { display: flex; gap: 6px; margin-bottom: 14px; }
.readiness-label { flex: 1; text-align: center; font-size: 10px; font-weight: 600; color: var(--muted-foreground); text-transform: uppercase; letter-spacing: 0.03em; }
.readiness-label.fail { color: var(--destructive); }

/* VoWiFi 详情折叠 */
.vowifi-detail-collapse { border: 1px solid var(--border); border-radius: 8px; overflow: hidden; margin-top: 12px; }
.vowifi-detail-header { display: flex; align-items: center; gap: 8px; width: 100%; padding: 8px 12px; background: var(--muted); border: none; cursor: pointer; user-select: none; transition: background 0.15s; }
.vowifi-detail-header:hover { background: var(--accent); }
.vowifi-detail-title { font-size: 11px; font-weight: 700; color: var(--muted-foreground); text-transform: uppercase; letter-spacing: 0.04em; }
.vowifi-detail-arrow { color: var(--muted-foreground); transition: transform 0.2s; }
.vowifi-detail-arrow.expanded { transform: rotate(180deg); }

/* Detail rows */
.status-detail-rows { display: flex; flex-direction: column; gap: 6px; padding: 12px; border-top: 1px solid var(--border); }
.detail-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; font-size: 12px; }
.detail-row-label { color: var(--muted-foreground); flex-shrink: 0; }
.detail-row-value { color: var(--foreground); font-family: var(--oomol-font-mono); text-align: right; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* Cellular hero */
.cellular-hero {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 14px;
  border-radius: 8px;
  margin-bottom: 14px;
}
.cellular-hero.ok {
  background: linear-gradient(135deg, color-mix(in oklab, var(--brand) 10%, var(--card)), color-mix(in oklab, var(--brand) 3%, var(--card)));
  border: 1px solid color-mix(in oklab, var(--brand) 20%, var(--border));
}
.cellular-hero.warning {
  background: linear-gradient(135deg, color-mix(in oklab, var(--warning) 10%, var(--card)), color-mix(in oklab, var(--warning) 3%, var(--card)));
  border: 1px solid color-mix(in oklab, var(--warning) 20%, var(--border));
}
.cellular-hero.off {
  background: var(--muted);
  border: 1px solid var(--border);
}
.cellular-hero-info { flex: 1; min-width: 0; }
.cellular-hero-name { font-size: 15px; font-weight: 700; color: var(--foreground); }
.cellular-hero.off .cellular-hero-name { color: var(--muted-foreground); }
.cellular-hero-meta { font-size: 12px; color: var(--muted-foreground); margin-top: 2px; font-family: var(--oomol-font-mono); }

/* Signal */
.signal-box { display: flex; align-items: center; gap: 16px; padding: 14px; border: 1px solid var(--border); border-radius: 8px; background: var(--muted); margin-bottom: 14px; }
.signal-dbz { display: flex; align-items: baseline; gap: 4px; }
.signal-dbz-value { font-size: 28px; font-weight: 800; line-height: 1; font-variant-numeric: tabular-nums; }
.signal-dbz-value.good { color: var(--brand); }
.signal-dbz-value.fair { color: var(--warning); }
.signal-dbz-value.poor { color: var(--destructive); }
.signal-dbz-unit { font-size: 12px; color: var(--muted-foreground); }
.signal-bars { display: flex; align-items: flex-end; gap: 3px; height: 32px; margin-left: auto; }
.signal-bar { width: 5px; border-radius: 2px; transition: all 0.3s; }
.signal-bar:nth-child(1) { height: 20%; }
.signal-bar:nth-child(2) { height: 40%; }
.signal-bar:nth-child(3) { height: 60%; }
.signal-bar:nth-child(4) { height: 80%; }
.signal-bar:nth-child(5) { height: 100%; }
.signal-bar.active.good { background: var(--brand); }
.signal-bar.active.fair { background: var(--warning); }
.signal-bar.active.poor { background: var(--destructive); }
.signal-bar.inactive { background: color-mix(in oklab, var(--muted-foreground) 20%, transparent); }
.signal-detail { font-size: 10px; color: var(--muted-foreground); font-family: var(--oomol-font-mono); margin-top: 6px; display: flex; gap: 12px; flex-wrap: wrap; }
</style>
