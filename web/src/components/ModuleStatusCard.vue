<script setup lang="ts">
/**
 * ModuleStatusCard — VoWiFi Toggle 控制卡片
 * 包含：状态文本 + VoWiFi Toggle 开关
 */
import { computed } from 'vue'
import type { DeviceOverviewItem } from '../types/api'
import { WifiOutlined, WifiOffFilled } from '@vicons/material'

const props = defineProps<{
  device: DeviceOverviewItem | null
  togglingVoWiFi?: boolean
}>()

const emit = defineEmits<{
  'toggle-vowifi': [enabled: boolean]
}>()

const vowifiEnabled = computed(() => !!props.device?.vowifi_enabled)

const allReady = computed(() => {
  const rt = props.device?.vowifi_runtime
  if (!rt) return false
  return [rt.sim_ready, rt.access_ready, rt.tunnel_ready, rt.ims_ready, rt.sms_ready, rt.call_ready].every(Boolean)
})

const isPCSC = computed(() => props.device?.esim_transport === 'pcsc')

const statusText = computed(() => {
  if (!vowifiEnabled.value && !isPCSC.value) return 'VoWiFi OFF'
  const rt = props.device?.vowifi_runtime
  if (!rt) return 'VoWiFi OFF'
  if (allReady.value) return 'VoWiFi Running'
  return 'VoWiFi OFF'
})

function onToggle(val: boolean) {
  emit('toggle-vowifi', val)
}
</script>

<template>
  <div class="msc-card">
    <div class="msc-body">
      <!-- 左侧：图标盒子 + 状态文本 + 指示灯 -->
      <div class="msc-left">
        <div class="msc-icon-box">
          <el-icon size="14">
            <WifiOutlined v-if="vowifiEnabled" />
            <WifiOffFilled v-else />
          </el-icon>
        </div>
        <div class="msc-indicator" :class="{ on: allReady }"></div>
        <span class="msc-status-text">{{ statusText }}</span>
      </div>

      <!-- 右侧：Toggle switch -->
      <label class="ux-vault-toggle" :class="{ 'is-loading': togglingVoWiFi }">
        <input
          type="checkbox"
          class="ux-vault-toggle__input"
          :checked="vowifiEnabled"
          :disabled="togglingVoWiFi"
          @change="onToggle(($event.target as HTMLInputElement).checked)"
        />
        <div class="ux-vault-toggle__wrapper">
          <svg class="ux-vault-toggle__filter" width="0" height="0">
            <defs>
              <filter id="ux-metal-noise">
                <feTurbulence type="fractalNoise" baseFrequency="0.75" numOctaves="3" result="noise" />
                <feColorMatrix type="matrix" values="1 0 0 0 0 0 1 0 0 0 0 0 1 0 0 0 0 0 0.12 0" in="noise" result="coloredNoise" />
                <feComposite operator="in" in="coloredNoise" in2="SourceGraphic" result="composite" />
                <feBlend mode="multiply" in="composite" in2="SourceGraphic" />
              </filter>
            </defs>
          </svg>
          <div class="ux-vault-toggle__track">
            <div class="ux-vault-toggle__texture"></div>
            <svg class="ux-vault-toggle__circuit" viewBox="0 0 140 60">
              <path d="M 35 30 L 60 30 L 75 18 L 115 18" class="ux-circuit-path ux-circuit--off" />
              <path d="M 25 42 L 65 42 L 80 30 L 105 30" class="ux-circuit-path ux-circuit--on" />
            </svg>
            <div class="ux-vault-toggle__status ux-vault-toggle__status--off">
              <span style="--i:1">O</span><span style="--i:2">F</span><span style="--i:3">F</span>
            </div>
            <div class="ux-vault-toggle__status ux-vault-toggle__status--on">
              <span style="--i:1">O</span><span style="--i:2">N</span>
            </div>
          </div>
          <div class="ux-vault-toggle__thumb">
            <div class="ux-vault-toggle__thumb-ring"></div>
            <div class="ux-vault-toggle__thumb-core">
              <svg class="ux-thumb-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="5" />
                <path d="M12 2 L12 5 M12 19 L12 22 M2 12 L5 12 M19 12 L22 12" />
              </svg>
            </div>
            <div class="ux-vault-toggle__thumb-glare"></div>
          </div>
        </div>
      </label>
    </div>
  </div>
</template>

<style scoped>
@import '../assets/toggle-switches/silly-insect-64.css';

.msc-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
}

.msc-body {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px;
}

/* 左侧 */
.msc-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.msc-icon-box {
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

.msc-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--muted-foreground);
  opacity: 0.4;
  flex-shrink: 0;
  transition: background 0.3s, opacity 0.3s, box-shadow 0.3s;
}
.msc-indicator.on {
  background: var(--brand);
  opacity: 1;
  box-shadow: 0 0 8px color-mix(in oklab, var(--brand) 50%, transparent);
}

.msc-status-text {
  font-size: 13px;
  font-weight: 700;
  color: var(--foreground);
  white-space: nowrap;
}

/* Toggle loading 状态 */
.ux-vault-toggle.is-loading {
  opacity: 0.6;
  pointer-events: none;
}

/* 缩放 Toggle 适配卡片尺寸 */
.ux-vault-toggle {
  --track-w: 130px;
  --track-h: 44px;
  --thumb-size: 36px;
  flex-shrink: 0;
}
.ux-vault-toggle__input:checked ~ .ux-vault-toggle__wrapper .ux-vault-toggle__thumb {
  transform: translateX(84px);
}
.ux-vault-toggle__input:checked:active ~ .ux-vault-toggle__wrapper .ux-vault-toggle__thumb {
  transform: translateX(56px) scale(0.95);
}
</style>
