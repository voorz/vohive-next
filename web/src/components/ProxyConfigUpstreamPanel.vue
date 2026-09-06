<script setup lang="ts">
import { computed } from 'vue'
import EmptyState from './EmptyState.vue'
import ListSkeleton from './ListSkeleton.vue'
import ErrorState from './ErrorState.vue'
import CountryFlag from './CountryFlag.vue'
import {
  Earth24Regular,
  Add24Regular,
  Edit24Regular,
  Delete24Regular,
  PlugConnected24Regular,
  Link24Regular
} from '@vicons/fluent'
import type { UpstreamProxyWithMeta } from '../types/proxy-config'

const _props = defineProps<{
  proxies: UpstreamProxyWithMeta[]
  loading: boolean
  error: { message: string; status?: number } | null
}>()

const emit = defineEmits<{
  'batch-import': []
  'edit': [proxy: UpstreamProxyWithMeta]
  'delete': [proxy: UpstreamProxyWithMeta]
  'toggle': [proxy: UpstreamProxyWithMeta]
  'test-latency': [proxy: UpstreamProxyWithMeta]
}>()

const hostPort = computed(() => (addr: string) => {
  // 去掉 socks5:// 协议前缀
  return addr.replace(/^socks5:\/\//i, '')
})
</script>

<template>
  <ErrorState
    v-if="error"
    class="mb-6"
    title="加载前置代理失败"
    :message="error.message"
    :status-code="error.status"
    retry-text="重试"
  />

  <div class="px-6 py-5">
    <!-- Section Header -->
    <div class="flex items-center justify-between mb-5">
      <div class="flex items-center gap-3 min-w-0 overflow-hidden">
        <div class="pc-icon-box">
          <el-icon size="20"><Earth24Regular /></el-icon>
        </div>
        <div class="min-w-0">
          <div class="pc-section-title">VoWiFi 漫游前置代理</div>
          <div class="pc-section-desc">VoWiFi 通过代理穿透连接海外运营商，支持链接串导入与批量管理</div>
        </div>
      </div>
      <div class="flex gap-2 flex-shrink-0">
        <el-button type="primary" @click="emit('batch-import')" class="!border-0">
          <el-icon class="mr-1.5"><Add24Regular /></el-icon>
          <span>新增代理</span>
        </el-button>
      </div>
    </div>

    <!-- Loading -->
    <ListSkeleton v-if="loading && proxies.length === 0" :rows="2" />

    <!-- Empty -->
    <EmptyState
      v-else-if="proxies.length === 0"
      title="暂无前置代理"
      subtitle="点击「新增代理」添加代理节点"
    />

    <!-- Proxy Cards -->
    <div v-else class="space-y-3">
      <div
        v-for="proxy in proxies"
        :key="proxy.id"
        class="pc-upstream-card"
      >
        <!-- Row 1: Status + Name (left) / Toggle (right) -->
        <div class="pc-card-row-1">
          <div class="flex items-center gap-2 min-w-0">
            <span
              class="pc-status-dot"
              :style="{ background: proxy.enabled ? 'var(--success)' : 'var(--muted-foreground)' }"
            />
            <span class="pc-card-name">{{ proxy.name || proxy.id }}</span>
          </div>
          <div class="flex items-center gap-2">
            <el-switch
              :model-value="proxy.enabled"
              size="small"
              @change="emit('toggle', proxy)"
            />
            <el-button size="small" @click="emit('edit', proxy)">
              <el-icon class="mr-1"><Edit24Regular /></el-icon>
              <span>编辑</span>
            </el-button>
            <el-button size="small" type="danger" @click="emit('delete', proxy)">
              <el-icon class="mr-1"><Delete24Regular /></el-icon>
              <span>删除</span>
            </el-button>
          </div>
        </div>

        <!-- Row 2: Protocol + Address (left) / Rule count (right) -->
        <div class="pc-card-row-2">
          <span class="pc-proto-addr">
            Socks5 · <span class="font-mono">{{ hostPort(proxy.addr) }}</span>
          </span>
          <span class="pc-rule-count">
            <el-icon size="14"><Link24Regular /></el-icon>
            <span class="num">{{ proxy.ruleCount }}</span> 个国家规则
          </span>
        </div>

        <!-- Row 3: IP info + Latency -->
        <div class="pc-card-row-3">
          <!-- IP info (单行，超出渐变遮罩) -->
          <div v-if="proxy.lookup && proxy.lookup.ip" class="pc-ip-info">
            <CountryFlag
              v-if="proxy.lookup.country_code"
              :iso="proxy.lookup.country_code"
              :size="20"
            />
            <div v-else class="pc-flag-placeholder">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                <rect x="3" y="5" width="18" height="14" rx="2" stroke="currentColor" stroke-width="1.5" />
                <circle cx="8" cy="10" r="1.5" fill="currentColor" opacity="0.5" />
                <path d="M3 16l4-4 3 3 4-4 7 6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.5" />
              </svg>
            </div>
            <span class="pc-ip-item">
              <span class="pc-ip-label">IP</span>
              <span v-if="proxy.lookup.ip" class="pc-ip-value font-mono">{{ proxy.lookup.ip }}</span>
              <span v-else class="pc-ip-placeholder">—</span>
            </span>
            <span class="pc-ip-item">
              <span class="pc-ip-label">归属</span>
              <span v-if="proxy.lookup.country" class="pc-ip-value">
                {{ proxy.lookup.country }}<template v-if="proxy.lookup.city"> · {{ proxy.lookup.city }}</template>
              </span>
              <span v-else class="pc-ip-placeholder">—</span>
            </span>
            <span class="pc-ip-item">
              <span class="pc-ip-label">类型</span>
              <span v-if="proxy.lookup.organization" class="pc-ip-tag">{{ proxy.lookup.organization }}</span>
              <span v-else class="pc-ip-placeholder">—</span>
            </span>
          </div>
          <!-- 无有效 IP 数据时显示占位 -->
          <div v-else class="pc-ip-info">
            <div class="pc-flag-placeholder">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                <rect x="3" y="5" width="18" height="14" rx="2" stroke="currentColor" stroke-width="1.5" />
                <circle cx="8" cy="10" r="1.5" fill="currentColor" opacity="0.5" />
                <path d="M3 16l4-4 3 3 4-4 7 6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.5" />
              </svg>
            </div>
            <span class="pc-ip-placeholder">节点信息未知</span>
          </div>

          <!-- Latency + Test -->
          <div class="pc-right-group">
            <div class="pc-latency">
              <span v-if="proxy.lookup?.latency_ms != null && proxy.lookup.latency_ms > 0" class="pc-latency-val ok font-mono">
                {{ proxy.lookup.latency_ms }}ms
              </span>
            <span v-else-if="proxy.lookup?.error" class="pc-latency-val fail">超时</span>
            <el-tooltip
                content="SOCKS5 代理连接 1.1.1.1:80 往返延迟"
                placement="top"
              >
                <el-button
                  size="small"
                  :loading="proxy._testing"
                  @click="emit('test-latency', proxy)"
                >
                  <el-icon class="mr-1"><PlugConnected24Regular /></el-icon>
                  <span>测延迟</span>
                </el-button>
              </el-tooltip>
            </div>
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
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  mask-image: linear-gradient(to right, black calc(100% - 20px), transparent 100%);
  -webkit-mask-image: linear-gradient(to right, black calc(100% - 20px), transparent 100%);
}

/* ── Upstream card ── */
.pc-upstream-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
  transition: border-color 0.15s;
}
.pc-upstream-card:hover {
  border-color: var(--brand);
}

.pc-card-row-1 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px 4px;
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
  justify-content: space-between;
  padding: 0 16px 8px;
  font-size: 12px;
  color: var(--muted-foreground);
}
.pc-proto-addr {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pc-rule-count {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding-left: 12px;
}
.pc-rule-count .num {
  color: var(--brand);
  font-weight: 600;
}

/* Row 3: unified IP + latency + actions */
.pc-card-row-3 {
  margin: 0 16px 14px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--muted);
  display: flex;
  align-items: center;
  gap: 8px 12px;
  font-size: 12px;
  min-height: 40px;
}
.pc-ip-info {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  mask-image: linear-gradient(to right, black calc(100% - 24px), transparent 100%);
  -webkit-mask-image: linear-gradient(to right, black calc(100% - 24px), transparent 100%);
}
.pc-flag-placeholder {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
  border-radius: 3px;
  border: 1px solid var(--border);
  background: var(--muted);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--muted-foreground);
  opacity: 0.6;
  overflow: hidden;
}
.pc-ip-item {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.pc-ip-label {
  color: var(--muted-foreground);
  font-weight: 600;
  text-transform: uppercase;
  font-size: 10px;
  letter-spacing: 0.05em;
  flex-shrink: 0;
  white-space: nowrap;
}
.pc-ip-value {
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pc-ip-placeholder {
  color: var(--muted-foreground);
}
.pc-ip-tag {
  color: var(--brand);
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
}
.pc-right-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  margin-left: auto;
}
.pc-latency {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  flex-shrink: 0;
}
.pc-latency-val.ok {
  color: var(--success);
  font-weight: 700;
}
.pc-latency-val.fail {
  color: var(--destructive);
  font-weight: 700;
}
.pc-latency-val.pending {
  color: var(--muted-foreground);
}
.pc-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  flex-wrap: wrap;
}
</style>
