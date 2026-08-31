<script setup lang="ts">
import { computed, ref } from 'vue'
import type { DeviceOverviewItem, CardPolicy } from '../types/api'
import { useCardPolicyToggles, type PolicyMirror } from '../composables/useCardPolicyToggles'
import { devicesService } from '../services/devices'
import {
  WifiCalling3Twotone,
  AirplanemodeActiveRound,
  SignalCellularAltRound,
  PhoneInTalkRound,
} from '@vicons/material'

const props = defineProps<{
  device: DeviceOverviewItem | null
  policy: CardPolicy | null
  deviceOnline: boolean
  isPCSC?: boolean
}>()

const emit = defineEmits<{
  changed: []
}>()

const canToggle = computed(() => props.deviceOnline && !!props.device?.id)

const mirror = computed<PolicyMirror | null>(() =>
  props.policy
    ? {
        network_enabled: props.policy.network_enabled,
        vowifi_enabled: props.policy.vowifi_enabled,
        airplane_enabled: props.policy.airplane_enabled,
        volte_enabled: false, // VoLTE 后端未实现，恒 false
      }
    : null
)

const ipVersion = ref<'v4' | 'v6' | 'v4v6'>('v4')
const apn = ref('')

const {
  local,
  networkPending,
  networkFailed,
  vowifiPending,
  vowifiFailed,
  airplanePending,
  airplaneFailed,
  voltePending,
  volteFailed,
  onNetworkToggle,
  onVoWiFiToggle,
  onAirplaneToggle,
  onVolteToggle,
} = useCardPolicyToggles(mirror, {
  async applyNetwork(enabled, _next, prev) {
    if (!props.device?.id) return { ok: false }
    // 先关闭互斥项（用 prev 判断之前是否开着）
    if (enabled) {
      if (prev.vowifi_enabled) {
        await devicesService.disableVoWiFi(props.device.id).catch(() => {})
      }
      if (prev.airplane_enabled) {
        await devicesService.setFlightMode(props.device.id, false).catch(() => {})
      }
    }
    const r = enabled
      ? await devicesService.startNetwork(props.device.id, { ip_version: ipVersion.value, apn: apn.value })
      : await devicesService.stopNetwork(props.device.id)
    return { ok: r.ok }
  },
  async applyVoWiFi(enabled, _next, prev) {
    if (!props.device?.id) return { ok: false }
    // 先关闭互斥项
    if (enabled && prev.network_enabled) {
      await devicesService.stopNetwork(props.device.id).catch(() => {})
    }
    const r = enabled
      ? await devicesService.enableVoWiFi(props.device.id)
      : await devicesService.disableVoWiFi(props.device.id)
    return { ok: r.ok }
  },
  async applyAirplane(enabled, _next, prev) {
    if (!props.device?.id) return { ok: false }
    // 先关闭互斥项
    if (enabled && prev.network_enabled) {
      await devicesService.stopNetwork(props.device.id).catch(() => {})
    }
    const r = await devicesService.setFlightMode(props.device.id, enabled)
    return { ok: r.ok }
  },
  async applyVolte(enabled, _next, prev) {
    // VoLTE 后端未实现，mock 逻辑：开启时关 vowifi + 关飞行模式
    if (enabled) {
      if (props.device?.id && prev.vowifi_enabled) {
        await devicesService.disableVoWiFi(props.device.id).catch(() => {})
      }
      if (props.device?.id && prev.airplane_enabled) {
        await devicesService.setFlightMode(props.device.id, false).catch(() => {})
      }
    }
    return { ok: true }
  },
  onChanged() {
    emit('changed')
  },
})

// 描边颜色：正常驻网（蜂窝数据开）⇒ 品牌色；飞行模式或 VoWiFi ⇒ 普通色
const gridVariant = computed(() => {
  if (local.value.airplane_enabled || local.value.vowifi_enabled) return 'idle'
  if (local.value.network_enabled) return 'active'
  return 'idle'
})
</script>

<template>
  <div class="ctrl-grid" :class="`is-${gridVariant}`">
    <!-- WiFi 通话 -->
    <div class="ctrl-cell" :class="{ 'is-failed': vowifiFailed }">
      <div class="ctrl-icon-box" :class="{ active: local.vowifi_enabled }">
        <el-icon size="16"><WifiCalling3Twotone /></el-icon>
      </div>
      <div class="ctrl-info">
        <span class="ctrl-label">WiFi 通话</span>
        <span class="ctrl-state" :class="{ on: local.vowifi_enabled }">{{ local.vowifi_enabled ? '开启' : '关闭' }}</span>
      </div>
      <el-switch
        :model-value="local.vowifi_enabled"
        :loading="vowifiPending"
        :disabled="!canToggle || vowifiPending || (isPCSC && !local.vowifi_enabled)"
        @update:model-value="onVoWiFiToggle"
      />
    </div>

    <!-- 飞行模式 -->
    <div class="ctrl-cell" :class="{ 'is-failed': airplaneFailed, 'is-unsupported': isPCSC }">
      <div class="ctrl-icon-box" :class="{ active: local.airplane_enabled }">
        <el-icon size="16"><AirplanemodeActiveRound /></el-icon>
      </div>
      <div class="ctrl-info">
        <span class="ctrl-label">飞行模式</span>
        <span class="ctrl-state" :class="{ on: local.airplane_enabled }">{{ local.airplane_enabled ? '开启' : '关闭' }}</span>
      </div>
      <el-switch
        :model-value="local.airplane_enabled"
        :loading="airplanePending"
        :disabled="!canToggle || local.vowifi_enabled || airplanePending || isPCSC"
        @update:model-value="onAirplaneToggle"
      />
    </div>

    <!-- 蜂窝数据 -->
    <div class="ctrl-cell" :class="{ 'is-failed': networkFailed, 'is-unsupported': isPCSC }">
      <div class="ctrl-icon-box" :class="{ active: local.network_enabled }">
        <el-icon size="16"><SignalCellularAltRound /></el-icon>
      </div>
      <div class="ctrl-info">
        <span class="ctrl-label">蜂窝数据</span>
        <span class="ctrl-state" :class="{ on: local.network_enabled }">{{ local.network_enabled ? '开启' : '关闭' }}</span>
      </div>
      <el-switch
        :model-value="local.network_enabled"
        :loading="networkPending"
        :disabled="!canToggle || local.vowifi_enabled || local.airplane_enabled || networkPending || isPCSC"
        @update:model-value="onNetworkToggle"
      />
    </div>

    <!-- VoLTE -->
    <div class="ctrl-cell" :class="{ 'is-failed': volteFailed, 'is-unsupported': isPCSC }">
      <div class="ctrl-icon-box" :class="{ active: local.volte_enabled }">
        <el-icon size="16"><PhoneInTalkRound /></el-icon>
      </div>
      <div class="ctrl-info">
        <span class="ctrl-label">VoLTE</span>
        <span class="ctrl-state" :class="{ on: local.volte_enabled }">{{ local.volte_enabled ? '开启' : '关闭' }}</span>
      </div>
      <el-switch
        :model-value="local.volte_enabled"
        :loading="voltePending"
        :disabled="!canToggle || local.vowifi_enabled || local.airplane_enabled || voltePending || isPCSC"
        @update:model-value="onVolteToggle"
      />
    </div>
  </div>
</template>

<style scoped>
.ctrl-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
  background: var(--background);
  transition: border-color 0.2s;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12);
}

/* 正常驻网（蜂窝数据开）⇒ 品牌色描边 */
.ctrl-grid.is-active {
  border-color: color-mix(in oklab, var(--brand) 50%, var(--border));
}
/* 飞行模式或 VoWiFi ⇒ 普通色 */
.ctrl-grid.is-idle {
  border-color: var(--border);
}

.ctrl-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px;
  border-right: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
}
/* 右列去掉右边框 */
.ctrl-cell:nth-child(2n) {
  border-right: none;
}
/* 最后一行去掉下边框 */
.ctrl-cell:nth-last-child(-n+2) {
  border-bottom: none;
}

.ctrl-icon-box {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  flex-shrink: 0;
  border: 1px solid var(--border);
  background: var(--muted);
  color: var(--muted-foreground);
  transition: all 0.12s;
}
.ctrl-icon-box.active {
  background: color-mix(in oklab, var(--brand) 15%, var(--card));
  border-color: color-mix(in oklab, var(--brand) 30%, var(--border));
  color: var(--brand);
}

.ctrl-info {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
  flex: 1;
}
.ctrl-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--foreground);
}
.ctrl-state {
  font-size: 11px;
  color: var(--muted-foreground);
}
.ctrl-state.on {
  color: var(--brand);
}

.ctrl-cell.is-unsupported {
  opacity: 0.45;
}
.ctrl-cell.is-failed .ctrl-icon-box {
  border-color: var(--destructive);
  color: var(--destructive);
}

@media (max-width: 480px) {
  .ctrl-grid {
    grid-template-columns: 1fr;
  }
  .ctrl-cell {
    border-right: none;
  }
  .ctrl-cell:nth-last-child(2) {
    border-bottom: 1px solid var(--border);
  }
}
</style>
