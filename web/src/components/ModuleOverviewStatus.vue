<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { DeviceOverviewItem } from '../types/api'
import { isControlOnline, isRadioRegistered, isRecoveryPhase, lifecycleStatusLabel } from '../utils/deviceLifecycle'
import { Pulse24Regular, Settings24Regular, WifiWarning24Filled } from '@vicons/fluent'
import { WifiCalling3Twotone, WifiProtectedSetupRound, RunningWithErrorsFilled, CellTowerRound, Md3GMobiledataTwotone, Md4GMobiledataTwotone, Md4GPlusMobiledataTwotone, Md5GRound } from '@vicons/material'
import OperatorSelectionDialog from './ModuleOperatorSelectionDialog.vue'
import ModuleOverviewActivity from './ModuleOverviewActivity.vue'
import ModuleNetworkOverview from './ModuleNetworkOverview.vue'
import ModuleSimManagement from './ModuleSimManagement.vue'
import { useDevicesStore } from '../stores/devices'
import { copyToClipboard } from '../utils/clipboard'
import ChinaMobileIcon from '../assets/svgs/china-mobile.svg'
import ChinaTelecomIcon from '../assets/svgs/china-telecom.svg'
import ChinaUnicomIcon from '../assets/svgs/china-unicom.svg'
import confetti from 'canvas-confetti'

const props = defineProps<{
  device: DeviceOverviewItem | null
  reconnectingVoWiFi?: boolean
  rotating?: boolean
  trafficSpeedRx?: string
  trafficSpeedTx?: string
  trafficMinuteRx?: string
  trafficMinuteTx?: string
}>()

const emit = defineEmits<{
  'reconnect-vowifi': []
  'rotate-ip': []
}>()

const store = useDevicesStore()
const showOperatorSelection = ref(false)

function copyVal(val: string | undefined) {
  if (!val || val === '--') return
  void copyToClipboard(val)
}

// PC/SC 读卡器设备：无 modem，始终以 VoWiFi 模式展示
const isPCSC = computed(() => props.device?.esim_transport === 'pcsc')

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

// VoWiFi 已开启但未就绪时的实时状态文本
const vowifiStartingText = computed(() => {
  const rt = props.device?.vowifi_runtime
  if (!rt) return ''
  // recover_failed 状态显示固定标题，错误原因在 hero-sub 展示
  if (rt.phase === 'recover_failed') return 'VoWiFi 启动失败'
  // 优先显示 stage_label（实时进度）
  if (rt.stage_label) return rt.stage_label
  // 其次显示 last_reason
  if (rt.last_reason) return rt.last_reason
  // 默认
  return '等待启动...'
})

// 恢复倒计时文本（UIM 门控和普通恢复失败都走这里）
const recoverRetryText = computed(() => {
  const rt = props.device?.vowifi_runtime
  if (!rt) return ''
  if (rt.retry_in_seconds && rt.retry_in_seconds > 0) {
    return `${rt.retry_in_seconds}s 后重试`
  }
  return ''
})

// VoWiFi 启动过程追踪：只在同一设备启动过程中变为 ok 才触发礼花
const isStarting = ref(false)
const lastDeviceId = ref<string | undefined>()

watch([vowifiStatus, () => props.device?.id], ([status, id]) => {
  // 设备切换时重置启动状态
  if (id !== lastDeviceId.value) {
    isStarting.value = false
    lastDeviceId.value = id
    return
  }
  // 同一设备：进入启动中状态
  if (status === 'partial' || (status === 'off' && vowifiEnabled.value)) {
    isStarting.value = true
    return
  }
  // 同一设备：从启动中变为 ok → 触发礼花
  if (status === 'ok' && isStarting.value) {
    isStarting.value = false
    // 左侧发射
    confetti({
      particleCount: 80,
      spread: 70,
      origin: { x: 0.2, y: 0.6 },
      colors: ['#10b981', '#34d399', '#6ee7b7', '#fbbf24', '#60a5fa'],
    })
    // 右侧发射
    setTimeout(() => {
      confetti({
        particleCount: 80,
        spread: 70,
        origin: { x: 0.8, y: 0.6 },
        colors: ['#10b981', '#34d399', '#6ee7b7', '#fbbf24', '#60a5fa'],
      })
    }, 150)
    // 中间补射
    setTimeout(() => {
      confetti({
        particleCount: 50,
        spread: 100,
        origin: { x: 0.5, y: 0.5 },
        colors: ['#10b981', '#34d399', '#6ee7b7', '#fbbf24', '#60a5fa'],
      })
    }, 300)
  }
}, { immediate: true })

const notReadyNames = computed(() =>
  readinessItems.value.filter(i => !i.ready).map(i => i.key)
)

// VoWiFi 状态图标：全部就绪→WifiCalling3Twotone，其余→WifiWarning24Filled
const vowifiIcon = computed(() => {
  if (vowifiStatus.value === 'ok') return 'success'
  return 'error'
})

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

// dBm 文字颜色（纯信号强度）
const signalTone = computed<'good' | 'fair' | 'poor'>(() => {
  const dbm = props.device?.modem?.signal_dbm
  if (!hasValidSignalDbm(dbm)) return 'poor'
  if (dbm >= -85) return 'good'
  if (dbm >= -100) return 'fair'
  return 'poor'
})

// 信号格颜色（搜索网络中→橙色，驻网被拒→红色，其余按信号强度）
const signalBarTone = computed<'good' | 'fair' | 'poor' | 'warning' | 'danger'>(() => {
  if (props.device?.registration_state_label === 'searching') return 'warning'
  if (props.device?.registration_state_label === 'denied') return 'danger'
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

// ico1 运营商图标：搜索网络中→WifiProtectedSetupRound，驻网失败→RunningWithErrorsFilled，其他→CellTowerRound
const operatorIconType = computed<'searching' | 'failed' | 'default'>(() => {
  if (props.device?.registration_state_label === 'searching') return 'searching'
  if (props.device?.registration_state_label === 'denied') return 'failed'
  return 'default'
})

// 运营商 SVG 图标（中国三大运营商）
const operatorSvg = computed(() => {
  const op = props.device?.modem?.operator || ''
  const spn = props.device?.modem?.native_spn || ''
  const name = op || spn
  if (name.includes('移动') || name.includes('China Mobile') || name.includes('CMCC')) return ChinaMobileIcon
  if (name.includes('电信') || name.includes('China Telecom') || name.includes('CTCC')) return ChinaTelecomIcon
  if (name.includes('联通') || name.includes('China Unicom') || name.includes('CUCC')) return ChinaUnicomIcon
  return null
})

// ico2 信号格旁图标：按网络模式适配 3G/4G/4G+/5G
const networkModeIcon = computed(() => {
  const mode = props.device?.modem?.network_mode?.toUpperCase() || ''
  if (mode.includes('NR5G') || mode.includes('5G')) return '5g'
  if (mode.includes('LTE') && mode.includes('PLUS')) return '4g-plus'
  if (mode.includes('LTE') || mode.includes('4G')) return '4g'
  if (mode.includes('UMTS') || mode.includes('GSM') || mode.includes('CDMA') || mode.includes('3G')) return '3g'
  return null
})
</script>

<template>
  <div class="ov-card">
    <div class="ov-card-head">
      <div class="ov-icon-box">
        <el-icon size="14"><Pulse24Regular /></el-icon>
      </div>
      <span class="ov-card-title">运行状态</span>
      <button
        v-if="vowifiEnabled || isPCSC"
        class="ov-reconnect-btn"
        :disabled="reconnectingVoWiFi || (isPCSC && !vowifiEnabled)"
        @click="emit('reconnect-vowifi')"
      >
        <span>重连</span>
      </button>
      <button
        v-else
        class="ov-reconnect-btn"
        style="margin-left: auto;"
        :disabled="!device?.network_connected || rotating"
        @click="emit('rotate-ip')"
      >
        <span>切换 IP</span>
      </button>
    </div>
    <div class="ov-card-body">

      <!-- VoWiFi 模式（PC/SC 设备始终进入此分支） -->
      <template v-if="vowifiEnabled || isPCSC">
        <!-- 第一行：Hero 大卡片（不变） -->
        <div class="hero-card" :class="vowifiStatus">
          <div class="hero-top">
            <div class="hero-icon-box" :class="vowifiIcon">
                <el-icon size="35">
                  <WifiCalling3Twotone v-if="vowifiIcon === 'success'" />
                  <WifiWarning24Filled v-else />
                </el-icon>
              </div>
            <div class="hero-text">
              <div class="hero-title">
                <template v-if="vowifiStatus === 'ok'">WiFi-Calling · 全部就绪</template>
                <template v-else-if="vowifiStatus === 'partial'">{{ notReadyNames.join(' · ') }} 未就绪</template>
                <template v-else-if="vowifiEnabled && vowifiStartingText">{{ vowifiStartingText }}</template>
                <template v-else>VoWiFi 未连接</template>
              </div>
              <div v-if="vowifiStatus === 'ok'" class="hero-sub">通过 ePDG 隧道连接 IMS 核心网</div>
              <div v-else-if="vowifiStatus === 'partial' && device?.vowifi_runtime?.last_reason" class="hero-sub">
                {{ device.vowifi_runtime.last_reason }}
              </div>
              <div v-else-if="vowifiStatus === 'off' && vowifiEnabled && device?.vowifi_runtime?.last_reason" class="hero-sub">
                <span class="copyable" @click="copyVal(device.vowifi_runtime.last_reason)">{{ device.vowifi_runtime.last_reason }}</span>
                <el-popover
                  v-if="recoverRetryText"
                  placement="bottom"
                  :width="300"
                  trigger="hover"
                >
                  <template #reference>
                    <div class="recover-retry-text">{{ recoverRetryText }}</div>
                  </template>
                  <div class="text-xs leading-relaxed">
                    可在「系统设置」-「全局」设置失败重试间隔时间
                  </div>
                </el-popover>
              </div>
            </div>
          </div>
          <div class="hero-divider" :class="vowifiStatus"></div>
          <div class="hero-bottom">
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
          </div>
        </div>

        <!-- 第二行：Activity + SIM卡管理（并排） -->
        <div class="card-row">
          <ModuleOverviewActivity :device="device" />
          <!-- SIM卡管理卡片（暂时隐藏） -->
          <ModuleSimManagement v-if="false" :device="device" />
        </div>
      </template>

      <!-- 蜂窝模式 -->
      <template v-else>
        <!-- 第一行：Hero + 网络概览（并排） -->
        <div class="card-row">
        <div class="hero-card" :class="cellularHeroTone">
          <div class="hero-top">
            <div class="hero-icon-box" :class="operatorIconType">
                <img v-if="operatorSvg && isRegistered" :src="operatorSvg" alt="operator" class="operator-svg" />
                <el-icon v-else size="35">
                  <WifiProtectedSetupRound v-if="operatorIconType === 'searching'" />
                  <RunningWithErrorsFilled v-else-if="operatorIconType === 'failed'" />
                  <CellTowerRound v-else />
                </el-icon>
              </div>
            <div class="hero-text">
              <div class="hero-title">
                <template v-if="isRegistered">{{ device?.modem?.operator || '--' }}</template>
                <template v-else>{{ cellularStatusText }}</template>
              </div>
              <div class="hero-sub">
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
          <div v-if="hasValidSignalDbm(device?.modem?.signal_dbm)" class="hero-divider" :class="cellularHeroTone"></div>
          <div v-if="hasValidSignalDbm(device?.modem?.signal_dbm)" class="hero-bottom signal-area">
            <div class="signal-left">
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
            <div class="signal-bars-group">
              <div class="signal-bars-wrapper">
                <el-icon v-if="networkModeIcon" size="16" class="network-mode-icon">
                  <Md5GRound v-if="networkModeIcon === '5g'" />
                  <Md4GPlusMobiledataTwotone v-else-if="networkModeIcon === '4g-plus'" />
                  <Md4GMobiledataTwotone v-else-if="networkModeIcon === '4g'" />
                  <Md3GMobiledataTwotone v-else-if="networkModeIcon === '3g'" />
                </el-icon>
                <div class="signal-bars">
                  <div
                    v-for="i in 5"
                    :key="i"
                    class="signal-bar"
                    :class="i <= signalLevel ? ['active', signalBarTone] : 'inactive'"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
        <!-- 第一行右侧：网络概览 -->
        <ModuleNetworkOverview
          :device="device"
          :traffic-speed-rx="trafficSpeedRx"
          :traffic-speed-tx="trafficSpeedTx"
          :traffic-minute-rx="trafficMinuteRx"
          :traffic-minute-tx="trafficMinuteTx"
        />
        </div>
        <!-- 第二行：Activity + SIM卡管理（并排） -->
        <div class="card-row">
        <div class="activity-card">
          <div class="activity-header">
            <span class="activity-title">实时活动</span>
          </div>
          <div class="activity-rows">
            <div class="detail-row">
              <span class="detail-row-label">网络模式</span>
              <span class="detail-row-value">{{ networkModeDisplay }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-row-label">频段</span>
              <span class="detail-row-value">{{ device?.modem?.radio_band || '--' }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-row-label">信道</span>
              <span class="detail-row-value">{{ device?.modem?.radio_channel ?? '--' }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-row-label">注册状态</span>
              <span class="detail-row-value">{{ device?.modem?.reg_status_text || '--' }}</span>
            </div>
          </div>
        </div>
        <!-- 第二行右侧：SIM卡管理（暂时隐藏） -->
        <ModuleSimManagement v-if="false" :device="device" />
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
.ov-reconnect-btn {
margin-left: auto;
display: inline-flex;
align-items: center;
height: 24px;
font-size: 12px;
font-weight: 500;
padding: 0 11px;
border-radius: 4px;
border: 1px solid var(--foreground);
background: var(--foreground);
color: var(--background);
cursor: pointer;
transition: all 0.12s;
flex-shrink: 0;
line-height: 1;
}
.ov-reconnect-btn:not(:disabled):hover {
  background: var(--muted);
  color: var(--foreground);
  border-color: var(--border);
}
.ov-reconnect-btn:disabled {
  cursor: not-allowed;
  opacity: 0.4;
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
.ov-icon-btn:hover { background: var(--foreground); color: var(--background); }
.ov-card-body { padding: 14px; }

/* ===== Hero 大卡片（统一结构） ===== */
.hero-card {
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 12px;
  border: 1px solid var(--border);
  flex: 1;
  min-width: 0;
}
.hero-card.ok {
  background: linear-gradient(135deg, color-mix(in oklab, var(--brand) 10%, var(--card)), color-mix(in oklab, var(--brand) 3%, var(--card)));
  border-color: color-mix(in oklab, var(--brand) 20%, var(--border));
}
.hero-card.partial {
  background: linear-gradient(135deg, color-mix(in oklab, var(--warning) 10%, var(--card)), color-mix(in oklab, var(--warning) 3%, var(--card)));
  border-color: color-mix(in oklab, var(--warning) 20%, var(--border));
}
.hero-card.off {
  background: var(--muted);
}
.hero-card.warning {
  background: linear-gradient(135deg, color-mix(in oklab, var(--warning) 10%, var(--card)), color-mix(in oklab, var(--warning) 3%, var(--card)));
  border-color: color-mix(in oklab, var(--warning) 20%, var(--border));
}

/* Hero 上半部分 */
.hero-top {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
}

/* 图标盒子 */
.hero-icon-box {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  overflow: visible;
  border: 1px solid var(--border);
  background: #000000;
  position: relative;
}
.hero-card.ok .hero-icon-box { border-color: color-mix(in oklab, var(--brand) 20%, var(--border)); }
.hero-card.partial .hero-icon-box { border-color: color-mix(in oklab, var(--warning) 20%, var(--border)); }
.hero-card.warning .hero-icon-box { border-color: color-mix(in oklab, var(--warning) 20%, var(--border)); }
.hero-card.off .hero-icon-box { border-color: var(--border); }
.hero-icon-box.success { color: var(--brand); }
.hero-icon-box.error { color: var(--destructive); }
.hero-card.ok .hero-icon-box { color: var(--brand); }
.hero-card.partial .hero-icon-box { color: var(--warning); }
.hero-card.off .hero-icon-box { color: var(--muted-foreground); opacity: 0.4; }
.hero-icon-box.searching { color: var(--warning); }
.hero-icon-box.failed { color: var(--destructive); }
.hero-icon-box.default { color: var(--brand); }
.hero-card.off .hero-icon-box { color: var(--muted-foreground); opacity: 0.4; }

/* 脉冲扩散环动效（ok / partial / warning / searching / failed / default 状态触发） */
.hero-icon-box::after {
  content: '';
  position: absolute;
  inset: -2px;
  border-radius: 10px;
  border: 2px solid currentColor;
  opacity: 0;
  pointer-events: none;
}
.hero-card.ok .hero-icon-box::after {
  animation: hero-pulse-ring 2.5s ease-out infinite;
  color: var(--brand);
}
.hero-card.partial .hero-icon-box::after {
  animation: hero-pulse-ring 2.5s ease-out infinite;
  color: var(--warning);
}
.hero-card.warning .hero-icon-box::after {
  animation: hero-pulse-ring 2.5s ease-out infinite;
  color: var(--warning);
}
.hero-card.off .hero-icon-box::after { animation: none; }
@keyframes hero-pulse-ring {
  0% { transform: scale(0.95); opacity: 0.7; }
  100% { transform: scale(1.4); opacity: 0; }
}
.operator-svg {
  width: 35px;
  height: 35px;
  object-fit: contain;
}

.hero-text { flex: 1; min-width: 0; }
.hero-title { font-size: 15px; font-weight: 700; line-height: 1.3; }
.hero-card.ok .hero-title { color: color-mix(in oklab, var(--brand) 85%, var(--foreground)); }
.hero-card.partial .hero-title { color: color-mix(in oklab, var(--warning) 85%, var(--foreground)); }
.hero-card.warning .hero-title { color: color-mix(in oklab, var(--warning) 85%, var(--foreground)); }
.hero-card.off .hero-title { color: var(--muted-foreground); }
.hero-sub { font-size: 12px; margin-top: 2px; color: var(--muted-foreground); }
.hero-sub .copyable { cursor: pointer; }
.hero-sub .copyable:hover { color: var(--brand); }

/* 分割线（同步卡片轮廓线色调） */
.hero-divider { border-top: 1px solid var(--border); }
.hero-divider.ok { border-top-color: color-mix(in oklab, var(--brand) 25%, var(--border)); }
.hero-divider.partial { border-top-color: color-mix(in oklab, var(--warning) 25%, var(--border)); }
.hero-divider.warning { border-top-color: color-mix(in oklab, var(--warning) 25%, var(--border)); }
.hero-divider.off { border-top-color: var(--border); }

/* Hero 下半部分 */
.hero-bottom {
  padding: 12px;
}
.hero-bottom:not(.signal-area) {
  min-height: 75px;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

/* Readiness */
.readiness-chain { display: flex; gap: 6px; margin-bottom: 6px; }
.readiness-segment { flex: 1; height: 12px; border-radius: 999px; background: color-mix(in oklab, var(--muted-foreground) 15%, transparent); position: relative; overflow: hidden; }
.readiness-segment.ready { background: var(--brand); }
.readiness-segment.not-ready { background: color-mix(in oklab, var(--destructive) 60%, var(--muted)); }
.readiness-segment.ready::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, transparent 20%, color-mix(in oklab, white 60%, transparent), transparent 80%);
  animation: ov-shimmer 1.5s ease-in-out infinite;
}
@keyframes ov-shimmer {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}
.readiness-labels { display: flex; gap: 6px; }
.readiness-label { flex: 1; text-align: center; font-size: 10px; font-weight: 600; color: var(--muted-foreground); text-transform: uppercase; letter-spacing: 0.03em; }
.readiness-label.fail { color: var(--destructive); }

/* Signal area (蜂窝 hero 下半部分) */
.signal-area { display: flex; align-items: center; gap: 12px; }
.signal-left { flex: 1; min-width: 0; }
.signal-dbz { display: flex; align-items: baseline; gap: 4px; }
.signal-dbz-value { font-size: 28px; font-weight: 800; line-height: 1; font-variant-numeric: tabular-nums; }
.signal-dbz-value.good { color: var(--brand); }
.signal-dbz-value.fair { color: var(--warning); }
.signal-dbz-value.poor { color: var(--destructive); }
.signal-dbz-unit { font-size: 12px; color: var(--muted-foreground); }
.signal-bars-group { display: flex; align-items: flex-end; flex-shrink: 0; }
.signal-bars-wrapper { position: relative; display: flex; align-items: flex-end; }
.signal-bars { display: flex; align-items: flex-end; gap: 3px; height: 32px; }
.network-mode-icon {
  position: absolute;
  top: 0;
  left: 0;
  color: var(--muted-foreground);
  z-index: 1;
  line-height: 0;
  transform: translate(-2px, -4.5px);
}
.network-mode-icon :deep(svg) {
  display: block;
  margin: 0;
  padding: 0;
}
.signal-bar { width: 5px; border-radius: 2px; transition: all 0.3s; }
.signal-bar:nth-child(1) { height: 20%; }
.signal-bar:nth-child(2) { height: 40%; }
.signal-bar:nth-child(3) { height: 60%; }
.signal-bar:nth-child(4) { height: 80%; }
.signal-bar:nth-child(5) { height: 100%; }
.signal-bar.active.good { background: var(--brand); }
.signal-bar.active.fair { background: var(--warning); }
.signal-bar.active.poor { background: var(--destructive); }
.signal-bar.active.warning { background: var(--warning); }
.signal-bar.active.danger { background: var(--destructive); }
.signal-bar.inactive { background: color-mix(in oklab, var(--muted-foreground) 20%, transparent); }
.signal-detail { font-size: 10px; color: var(--muted-foreground); margin-top: 6px; display: flex; gap: 12px; flex-wrap: wrap; }

/* Card row (并排布局) */
.card-row {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
  align-items: stretch;
}
.card-row:last-child {
  margin-bottom: 0;
}
.card-row > .hero-card {
  margin-bottom: 0;
}

/* Placeholder card (占位) */
.placeholder-card {
  flex: 1;
  min-width: 0;
  border: 1px dashed var(--border);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: var(--muted-foreground);
  min-height: 80px;
  background: var(--muted);
}

/* Activity card (子卡片) */
.activity-card {
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
.activity-rows {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px;
}

/* Detail rows */
.detail-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; font-size: 12px; }
.detail-row-label { color: var(--muted-foreground); flex-shrink: 0; }
.detail-row-value { color: var(--foreground); text-align: right; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
