<script setup lang="ts">
import type { DeviceOverviewItem } from '../types/api'
import { ArrowSync24Regular, Power24Regular, Mail24Regular } from '@vicons/fluent'

defineProps<{
  device: DeviceOverviewItem
  rotating: boolean
  rebooting: boolean
  reconnectingVoWiFi: boolean
}>()

const emit = defineEmits<{
  'copy-text': [value: string]
  'rotate-ip': []
  'reboot-modem': []
  'reconnect-vowifi': []
  'open-sms': []
}>()
</script>

<template>
  <div class="device-header">
    <div class="device-header-content">
      <div class="flex items-center gap-3 min-w-0">
        <div class="device-header-icon-box">V</div>
        <div class="min-w-0">
          <div class="device-header-name">{{ device.name || device.id }}</div>
          <div class="device-header-meta">
            <span class="font-mono cursor-pointer hover:underline" @click="emit('copy-text', device.id)">{{ device.id }}</span>
            · 公网 IP:
            <span class="font-mono cursor-pointer hover:underline" @click="emit('copy-text', device.public_ip || '')">{{ device.public_ip || '---' }}</span>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <el-button v-if="device?.vowifi_enabled" :loading="reconnectingVoWiFi" @click="emit('reconnect-vowifi')">
          <el-icon size="20"><ArrowSync24Regular /></el-icon>
          重连 VoWiFi
        </el-button>
        <el-button v-else :loading="rotating" :disabled="!device?.network_connected" @click="emit('rotate-ip')">
          <el-icon size="20"><ArrowSync24Regular /></el-icon>
          切换 IP
        </el-button>
        <el-button :loading="rebooting" @click="emit('reboot-modem')" class="hover:!text-red-600">
          <el-icon size="20"><Power24Regular /></el-icon>
          重启模组
        </el-button>
        <el-button @click="emit('open-sms')">
          <el-icon size="20"><Mail24Regular /></el-icon>
          短信
        </el-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.device-header {
  height: 60px;
  padding: 0 24px;
  display: flex;
  align-items: center;
}

.device-header-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
}

.device-header-icon-box {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  background: var(--muted);
  border: 1px solid var(--border);
  color: var(--foreground);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  font-size: 1rem;
  font-weight: 700;
}

.device-header-name {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.device-header-meta {
  font-size: 12px;
  color: var(--muted-foreground);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
