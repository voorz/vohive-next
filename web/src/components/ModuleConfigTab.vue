<script setup lang="ts">
/**
 * ModuleConfigTab — 配置 Tab 纯容器
 * 平铺两个独立卡片：网络设置 + 设备配置
 * 滚动由父级 tab-pane 统一控制，内部卡片不自滚动
 */
import type { DeviceOverviewItem, CardPolicy } from '../types/api'
import ModuleConfigCards from './ModuleConfigCards.vue'
import ModuleConfigForm from './ModuleConfigForm.vue'

defineProps<{
  device: DeviceOverviewItem
  policy: CardPolicy | null
  isPCSC?: boolean
}>()

defineEmits<{
  'device-deleted': []
  'policy-changed': []
}>()
</script>

<template>
  <div class="config-tab">
    <ModuleConfigCards
      :iccid="device.modem?.iccid"
      :policy="policy"
      :device-online="device.running"
      :is-p-c-s-c="isPCSC"
      @policy-changed="$emit('policy-changed')"
    />
    <ModuleConfigForm
      :device-id="device.id"
      :device="device"
      @device-deleted="$emit('device-deleted')"
    />
  </div>
</template>

<style scoped>
.config-tab {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
</style>
