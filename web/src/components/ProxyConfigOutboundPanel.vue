<script setup lang="ts">
import EmptyState from './EmptyState.vue'
import ListSkeleton from './ListSkeleton.vue'
import ErrorState from './ErrorState.vue'
import {
  Router24Regular,
  Add24Regular,
  Play24Regular,
  Stop24Regular,
  ArrowSync24Regular,
  Edit24Regular,
  Delete24Regular
} from '@vicons/fluent'
import type { OutboundInstanceWithStatus, ProxyDevice } from '../types/proxy-config'

const props = defineProps<{
  instances: OutboundInstanceWithStatus[]
  devices: ProxyDevice[]
  loading: boolean
  error: { message: string; status?: number } | null
}>()

const emit = defineEmits<{
  'add': []
  'edit': [instance: OutboundInstanceWithStatus]
  'delete': [id: string]
  'start': [id: string]
  'stop': [id: string]
  'restart': [id: string]
}>()

function formatModeLabel(mode: string | undefined) {
  return mode === 'http' ? 'HTTP' : 'SOCKS5'
}

function deviceName(id: string) {
  const d = props.devices.find(d => d.id === id)
  return d ? `${d.name} (${d.interface})` : id
}
</script>

<template>
  <ErrorState
    v-if="error"
    class="mb-3"
    title="加载代理配置失败"
    :message="error.message"
    :status-code="error.status"
    retry-text="重试"
  />

  <div class="px-3 py-3">
    <!-- Section Header -->
    <div class="flex items-center justify-between mb-3">
      <div class="flex items-center gap-3">
        <div class="pc-icon-box">
          <el-icon size="20"><Router24Regular /></el-icon>
        </div>
        <div>
          <div class="pc-section-title">本地出站实例</div>
          <div class="pc-section-desc">每个实例绑定物理网络接口提供出口通道</div>
        </div>
      </div>
      <el-button type="primary" @click="emit('add')" class="!border-0">
        <el-icon class="mr-1.5"><Add24Regular /></el-icon>
        <span>新增实例</span>
      </el-button>
    </div>

    <!-- Loading -->
    <ListSkeleton v-if="loading && instances.length === 0" :rows="2" />

    <!-- Empty -->
    <EmptyState
      v-else-if="instances.length === 0"
      title="暂无代理实例"
      subtitle="点击「新增实例」创建第一个实例"
    />

    <!-- Instance Cards -->
    <div v-else class="space-y-3">
      <div
        v-for="inst in instances"
        :key="inst.id"
        class="pc-outbound-card"
      >
        <!-- Row 1: Status + Name (left) / Tags (right) -->
        <div class="pc-card-row-1">
          <div class="flex items-center gap-2 min-w-0">
            <span
              class="pc-status-dot"
              :style="{ background: inst.running ? 'var(--success)' : 'var(--muted-foreground)' }"
            />
            <span class="pc-card-name">{{ inst.name || inst.id }}</span>
          </div>
          <div class="flex gap-1.5 shrink-0">
            <el-tag size="small" type="info">{{ formatModeLabel(inst.mode) }}</el-tag>
            <el-tag size="small" :type="inst.auth_enabled ? 'warning' : 'info'">
              {{ inst.auth_enabled ? '账号认证' : '免认证' }}
            </el-tag>
          </div>
        </div>

        <!-- Row 2: Listen addr + device -->
        <div class="pc-card-row-2">
          <span class="pc-proto-addr">
            <span class="font-mono">{{ inst.listen_addr }}:{{ inst.listen_port }}</span>
            · 绑定: {{ deviceName(inst.device_id) }}
          </span>
        </div>

        <!-- Row 3: Status tag + Actions -->
        <div class="pc-card-bottom">
          <el-tag
            size="small"
            :type="inst.running ? 'success' : 'danger'"
          >
            {{ inst.running ? '运行中' : '已停止' }}
          </el-tag>
          <div v-if="inst.last_error" class="pc-card-error">{{ inst.last_error }}</div>
          <div class="pc-actions">
            <el-button
              v-if="!inst.running"
              size="small"
              :disabled="!inst.enabled"
              @click="emit('start', inst.id)"
            >
              <el-icon class="mr-1"><Play24Regular /></el-icon>
              <span>启动</span>
            </el-button>
            <el-button
              v-if="inst.running"
              size="small"
              @click="emit('stop', inst.id)"
            >
              <el-icon class="mr-1"><Stop24Regular /></el-icon>
              <span>停止</span>
            </el-button>
            <el-button
              size="small"
              :disabled="!inst.enabled"
              @click="emit('restart', inst.id)"
            >
              <el-icon><ArrowSync24Regular /></el-icon>
            </el-button>
            <el-button size="small" @click="emit('edit', inst)">
              <el-icon class="mr-1"><Edit24Regular /></el-icon>
              <span>编辑</span>
            </el-button>
            <el-button size="small" type="danger" @click="emit('delete', inst.id)">
              <el-icon><Delete24Regular /></el-icon>
            </el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pc-icon-box {
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
}
.pc-section-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--foreground);
}
.pc-section-desc {
  font-size: 12px;
  color: var(--muted-foreground);
  margin-top: 2px;
}

/* ── Outbound card ── */
.pc-outbound-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
  transition: border-color 0.15s;
}
.pc-outbound-card:hover {
  border-color: var(--brand);
}

.pc-card-row-1 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px 4px;
  gap: 8px;
}
.pc-card-name {
  font-weight: 700;
  font-size: 15px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pc-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  flex-shrink: 0;
}

.pc-card-row-2 {
  display: flex;
  align-items: center;
  padding: 0 16px 8px;
  font-size: 12px;
  color: var(--muted-foreground);
}
.pc-proto-addr {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pc-card-bottom {
  border-top: 1px solid var(--border);
  margin-top: 4px;
  padding: 8px 16px;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.pc-card-error {
  font-size: 12px;
  color: var(--destructive);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pc-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  margin-left: auto;
}
</style>
