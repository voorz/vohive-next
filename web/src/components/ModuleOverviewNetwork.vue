<script setup lang="ts">
import { computed } from 'vue'
import type { DeviceOverviewItem } from '../types/api'
import { Globe24Regular } from '@vicons/fluent'

const props = defineProps<{
  device: DeviceOverviewItem | null
  trafficSpeedRx?: string
  trafficSpeedTx?: string
  trafficMinuteRx?: string
  trafficMinuteTx?: string
}>()

const networkPanelMessage = computed(() => {
  if (!props.device?.network_enabled) return '数据未开启'
  if (!props.device?.network_connected) return '数据网络未连接'
  return ''
})

const trafficStateLabel = computed(() => {
  const status = props.device?.traffic_meta?.status
  if (status === 'waiting_sample') return '等待采样'
  if (status === 'stale') return '采样中断'
  return ''
})

const trafficRxDisplay = computed(() => props.trafficMinuteRx || props.device?.traffic?.rx || trafficStateLabel.value || '--')
const trafficTxDisplay = computed(() => props.trafficMinuteTx || props.device?.traffic?.tx || trafficStateLabel.value || '--')
</script>

<template>
  <div class="ov-card">
    <div class="ov-card-head">
      <div class="ov-icon-box">
        <el-icon size="14"><Globe24Regular /></el-icon>
      </div>
      <span class="ov-card-title">网络信息</span>
    </div>
    <div class="ov-card-body">

      <div v-if="networkPanelMessage" class="net-empty">
        {{ networkPanelMessage }}
      </div>

      <template v-else>
        <!-- 连接路径 -->
        <div class="net-path">
          <div class="net-node" :class="{ active: !!device?.private_ip }">
            <span class="net-node-label">设备</span>
            <span class="net-node-value">{{ device?.private_ip || '--' }}</span>
          </div>
          <div class="net-line" :class="{ active: !!device?.private_ip && !!device?.public_ip }"></div>
          <div class="net-node" :class="{ active: !!device?.public_ip }">
            <span class="net-node-label">外网</span>
            <span class="net-node-value">{{ device?.public_ip || '--' }}</span>
          </div>
        </div>

        <!-- 近1分钟流量 -->
        <div class="traffic-stats">
          <div class="traffic-stat">
            <div class="traffic-stat-label">近1分钟下载</div>
            <div class="traffic-stat-value">{{ trafficRxDisplay }}</div>
          </div>
          <div class="traffic-stat">
            <div class="traffic-stat-label">近1分钟上传</div>
            <div class="traffic-stat-value">{{ trafficTxDisplay }}</div>
          </div>
        </div>
      </template>

    </div>
  </div>
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
}
.ov-icon-btn:hover { background: var(--accent); color: var(--foreground); }
.ov-card-body { padding: 14px; }

.net-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 12px;
  font-size: 13px;
  color: var(--muted-foreground);
}

/* 连接路径 */
.net-path {
  display: flex;
  align-items: center;
  gap: 0;
  margin-bottom: 14px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--muted);
}
.net-node {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  min-width: 0;
}
.net-node-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.net-node-value {
  font-size: 12px;
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}
.net-node.active .net-node-value { color: var(--brand); }
.net-line {
  flex-shrink: 0;
  width: 24px;
  height: 2px;
  background: var(--border);
  position: relative;
}
.net-line.active { background: var(--brand); }
.net-line.active::after {
  content: '';
  position: absolute;
  right: -3px;
  top: -2px;
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--brand);
}

/* 流量统计 */
.traffic-stats { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.traffic-stat {
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--card);
}
.traffic-stat-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.traffic-stat-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--foreground);
  font-variant-numeric: tabular-nums;
  margin-top: 2px;
}
</style>
