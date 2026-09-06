<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Sim24Regular } from '@vicons/fluent'
import type { CardPolicy } from '../types/api'
import { useCardPolicyToggles, type PolicyMirror } from '../composables/useCardPolicyToggles'
import { devicesService } from '../services/devices'

const props = defineProps<{
  deviceId: string
  iccid?: string
  policy: CardPolicy | null
  deviceOnline: boolean
  isPCSC?: boolean
}>()

const emit = defineEmits<{
  policyChanged: []
}>()

const ipVersion = ref<'v4' | 'v6' | 'v4v6'>('v4')
const apn = ref('')

const canToggle = computed(() => props.deviceOnline && !!props.iccid)

const mirror = computed<PolicyMirror | null>(() =>
  props.policy
    ? {
        network_enabled: props.policy.network_enabled,
        vowifi_enabled: props.policy.vowifi_enabled,
        airplane_enabled: props.policy.airplane_enabled,
        volte_enabled: false
      }
    : null
)

watch(
  () => props.policy,
  (p) => {
    if (!p) return
    ipVersion.value = p.ip_version || 'v4'
    apn.value = p.apn || ''
  },
  { immediate: true }
)

const {
  local,
  networkPending,
  networkFailed,
  vowifiPending,
  vowifiFailed,
  airplanePending,
  airplaneFailed,
  onNetworkToggle,
  onVoWiFiToggle,
  onAirplaneToggle
} = useCardPolicyToggles(mirror, {
  async applyNetwork(enabled, _next, _prev) {
    if (!props.deviceId) return { ok: false }
    const r = enabled
      ? await devicesService.startNetwork(props.deviceId, { ip_version: ipVersion.value, apn: apn.value })
      : await devicesService.stopNetwork(props.deviceId)
    return { ok: r.ok }
  },
  async applyVoWiFi(enabled, _next, _prev) {
    if (!props.deviceId) return { ok: false }
    const r = enabled
      ? await devicesService.enableVoWiFi(props.deviceId)
      : await devicesService.disableVoWiFi(props.deviceId)
    return { ok: r.ok }
  },
  async applyAirplane(enabled, _next, _prev) {
    if (!props.deviceId) return { ok: false }
    const r = await devicesService.setFlightMode(props.deviceId, enabled)
    return { ok: r.ok }
  },
  onChanged() {
    emit('policyChanged')
  }
})
</script>

<template>
  <div class="module-card-policy">
    <!-- 头部 -->
    <div class="terminal-card-header">
      <div class="terminal-icon-box">
        <el-icon size="14"><Sim24Regular /></el-icon>
      </div>
      <div class="terminal-header-title">系统控制</div>
    </div>

    <!-- 内容区 -->
    <div class="policy-body">
      <div v-if="!canToggle" class="policy-unavailable">
        设备离线或无 ICCID，控制暂不可用
      </div>
      <template v-else>
        <div class="form-grid">
          <!-- VoWiFi -->
          <div class="field col-span-2 form-switch-row">
            <div>
              <div class="switch-title">WiFi 通话</div>
              <div class="switch-desc">通过 WiFi 网络进行语音通话</div>
            </div>
            <el-switch :model-value="local.vowifi_enabled" :loading="vowifiPending" :disabled="!canToggle || vowifiPending" :class="{ 'is-failed': vowifiFailed }" @update:model-value="onVoWiFiToggle" />
          </div>
          <!-- 飞行模式 -->
          <div class="field col-span-2 form-switch-row" :class="{ 'is-unsupported': isPCSC }">
            <div>
              <div class="switch-title">飞行模式</div>
              <div class="switch-desc">断开所有无线连接</div>
            </div>
            <el-switch :model-value="local.airplane_enabled" :loading="airplanePending" :disabled="!canToggle || local.vowifi_enabled || airplanePending || isPCSC" :class="{ 'is-failed': airplaneFailed }" @update:model-value="onAirplaneToggle" />
          </div>
          <!-- 移动数据（底部） -->
          <div class="field col-span-2 form-switch-row" :class="{ 'is-unsupported': isPCSC }">
            <div>
              <div class="switch-title">移动数据</div>
              <div class="switch-desc">启用蜂窝数据连接</div>
            </div>
            <el-switch :model-value="local.network_enabled" :loading="networkPending" :disabled="!canToggle || local.vowifi_enabled || local.airplane_enabled || networkPending || isPCSC" :class="{ 'is-failed': networkFailed }" @update:model-value="onNetworkToggle" />
          </div>
          <div class="field" :class="{ 'is-unsupported': isPCSC }">
            <label class="form-label">IP 版本</label>
            <el-select v-model="ipVersion" class="!w-full" :disabled="isPCSC">
              <el-option label="IPv4" value="v4" />
              <el-option label="IPv6" value="v6" />
              <el-option label="IPv4/IPv6" value="v4v6" />
            </el-select>
          </div>
          <div class="field" :class="{ 'is-unsupported': isPCSC }">
            <label class="form-label">APN</label>
            <el-input v-model="apn" placeholder="留空=运营商默认" :disabled="isPCSC" />
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.module-card-policy {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.terminal-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.terminal-icon-box {
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

.terminal-header-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}

.policy-body {
  padding: 12px;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}

.policy-unavailable {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 120px;
  color: var(--muted-foreground);
  font-size: 13px;
  font-family: var(--oomol-font-sans);
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.field.col-span-2 {
  grid-column: 1 / -1;
}

.form-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.form-switch-row {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  background: var(--muted);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 10px 12px;
}

.switch-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.switch-desc {
  font-size: 11px;
  color: var(--muted-foreground);
  margin-top: 1px;
}

.is-failed :deep(.el-switch__core) {
  border-color: var(--destructive) !important;
}

/* PC/SC 设备不支持的功能灰度显示 */
.form-switch-row.is-unsupported .switch-title,
.form-switch-row.is-unsupported .switch-desc,
.field.is-unsupported .form-label {
  opacity: 0.4;
}
</style>
