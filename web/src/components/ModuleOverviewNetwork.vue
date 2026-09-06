<script setup lang="ts">
import { computed, ref } from 'vue'
import type { DeviceOverviewItem } from '../types/api'
import { ChevronDown24Regular, Globe24Regular } from '@vicons/fluent'
import { copyToClipboard } from '../utils/clipboard'

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

// 流量折叠
const trafficExpanded = ref(false)

const trafficStateLabel = computed(() => {
  const status = props.device?.traffic_meta?.status
  if (status === 'waiting_sample') return '等待采样'
  if (status === 'stale') return '采样中断'
  return ''
})

const trafficRxDisplay = computed(() => props.trafficMinuteRx || props.device?.traffic?.rx || trafficStateLabel.value || '--')
const trafficTxDisplay = computed(() => props.trafficMinuteTx || props.device?.traffic?.tx || trafficStateLabel.value || '--')
const trafficRateDisplay = computed(() => props.trafficSpeedRx || props.device?.traffic?.rate || trafficStateLabel.value || '--')
const trafficUploadRateDisplay = computed(() => props.trafficSpeedTx || trafficStateLabel.value || '--')

const trafficSummary = computed(() => {
  const rx = props.device?.traffic?.rx
  const tx = props.device?.traffic?.tx
  if (!rx && !tx) return ''
  return `↓ ${rx || '--'} · ↑ ${tx || '--'}`
})

function copyVal(val: string | undefined) {
  if (!val || val === '--') return
  void copyToClipboard(val)
}
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

        <!-- IP 地址 -->
        <div class="ip-rows">
          <div v-if="device?.private_ip" class="ip-row">
            <span class="ip-row-label">内网 <span class="ip-tag">IPv4</span></span>
            <span class="ip-row-value copyable" @click="copyVal(device.private_ip)">{{ device.private_ip }}</span>
          </div>
          <div v-if="device?.public_ip" class="ip-row">
            <span class="ip-row-label">外网 <span class="ip-tag">IPv4</span></span>
            <span class="ip-row-value copyable" @click="copyVal(device.public_ip)">{{ device.public_ip }}</span>
          </div>
          <div v-if="device?.private_ipv6" class="ip-row">
            <span class="ip-row-label">内网 <span class="ip-tag">IPv6</span></span>
            <span class="ip-row-value copyable" @click="copyVal(device.private_ipv6)">{{ device.private_ipv6 }}</span>
          </div>
          <div v-if="device?.public_ipv6" class="ip-row">
            <span class="ip-row-label">外网 <span class="ip-tag">IPv6</span></span>
            <span class="ip-row-value copyable" @click="copyVal(device.public_ipv6)">{{ device.public_ipv6 }}</span>
          </div>
          <div v-if="device?.interface" class="ip-row">
            <span class="ip-row-label">接口</span>
            <span class="ip-row-value">{{ device.interface }}</span>
          </div>
        </div>

        <!-- 流量折叠 -->
        <div class="traffic-collapse">
          <div class="traffic-header" @click="trafficExpanded = !trafficExpanded">
            <div class="traffic-header-left">
              <el-icon size="16" class="traffic-arrow" :class="{ expanded: trafficExpanded }">
                <ChevronDown24Regular />
              </el-icon>
              <span class="traffic-header-title">流量分析</span>
            </div>
            <span v-if="trafficSummary" class="traffic-summary">{{ trafficSummary }}</span>
          </div>
          <div v-show="trafficExpanded" class="traffic-body">
            <div class="traffic-stats">
              <div class="traffic-stat">
                <div class="traffic-stat-label">近1分钟下载</div>
                <div class="traffic-stat-value">{{ trafficRxDisplay }}</div>
              </div>
              <div class="traffic-stat">
                <div class="traffic-stat-label">近1分钟上传</div>
                <div class="traffic-stat-value">{{ trafficTxDisplay }}</div>
              </div>
              <div class="traffic-stat">
                <div class="traffic-stat-label">实时下载速率</div>
                <div class="traffic-stat-value">{{ trafficRateDisplay }}</div>
              </div>
              <div class="traffic-stat">
                <div class="traffic-stat-label">实时上传速率</div>
                <div class="traffic-stat-value">{{ trafficUploadRateDisplay }}</div>
              </div>
            </div>
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

/* IP 行 */
.ip-rows { display: flex; flex-direction: column; gap: 6px; }
.ip-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--card);
}
.ip-row-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-foreground);
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}
.ip-tag {
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 9px;
  font-weight: 700;
  background: var(--muted);
  color: var(--muted-foreground);
}
.ip-row-value {
  font-size: 12px;
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ip-row-value.copyable {
  cursor: pointer;
}
.ip-row-value.copyable:hover {
  color: var(--brand);
}

/* 流量折叠 */
.traffic-collapse {
  margin-top: 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}
.traffic-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: var(--muted);
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
}
.traffic-header:hover { background: var(--accent); }
.traffic-header-left { display: flex; align-items: center; gap: 8px; }
.traffic-header-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.traffic-arrow { color: var(--muted-foreground); transition: transform 0.2s; }
.traffic-arrow.expanded { transform: rotate(180deg); }
.traffic-summary { font-size: 11px; color: var(--muted-foreground); font-family: var(--oomol-font-mono); }

.traffic-body { padding: 14px; border-top: 1px solid var(--border); }
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
