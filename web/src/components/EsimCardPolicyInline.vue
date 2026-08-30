<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import type { CardPolicy } from '../types/api'
import { cardsService } from '../services/cards'
import { useCardPolicyToggles, type PolicyMirror } from '../composables/useCardPolicyToggles'

const props = defineProps<{
  deviceId: string
  iccid: string
  isActiveCard: boolean
  deviceOnline: boolean
  isPCSC?: boolean
}>()

const emit = defineEmits<{
  policyChanged: []
}>()

const policy = ref<CardPolicy | null>(null)
const loadFailed = ref(false)
const loading = ref(false)

// eSIM 设置弹窗始终走 stored 模式：只写 DB，决定卡下次激活时的行为。
// 当前生效的系统级控制由配置 Tab 的卡策略卡片负责。

const mirror = computed<PolicyMirror | null>(() =>
  policy.value
    ? {
        network_enabled: policy.value.network_enabled,
        vowifi_enabled: policy.value.vowifi_enabled,
        airplane_enabled: policy.value.airplane_enabled,
        volte_enabled: false
      }
    : null
)

async function loadPolicy() {
  loading.value = true
  loadFailed.value = false
  const r = await cardsService.getPolicy(props.iccid)
  loading.value = false
  if (r.ok) {
    policy.value = r.data
  } else {
    loadFailed.value = true
  }
}

onMounted(loadPolicy)

// ICCID 变化时重新加载策略（弹窗复用同一组件实例）
watch(() => props.iccid, loadPolicy)

async function putTriple(next: PolicyMirror): Promise<{ ok: boolean }> {
  const r = await cardsService.putPolicy(props.iccid, {
    network_enabled: next.network_enabled,
    vowifi_enabled: next.vowifi_enabled,
    airplane_enabled: next.airplane_enabled
  })
  return { ok: r.ok }
}

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
  async applyNetwork(_enabled, next) {
    return putTriple(next)
  },
  async applyVoWiFi(_enabled, next) {
    return putTriple(next)
  },
  async applyAirplane(_enabled, next) {
    return putTriple(next)
  },
  onChanged() {
    emit('policyChanged')
  }
})
</script>

<template>
  <div class="policy-inline">
    <div v-if="loading" class="policy-loading">
      <el-icon class="animate-spin"><Loading /></el-icon>
      <span>正在加载策略...</span>
    </div>
    <div v-else-if="loadFailed" class="policy-failed">
      <span>策略加载失败</span>
      <el-button size="small" text @click="loadPolicy">重试</el-button>
    </div>
    <template v-else>
      <!-- VoWiFi -->
      <div class="form-switch-row">
        <div>
          <div class="switch-title">VoWiFi</div>
          <div class="switch-desc">通过 WiFi 网络进行语音通话</div>
        </div>
        <div class="switch-action">
          <span v-if="vowifiFailed" class="switch-failed">未生效</span>
          <el-icon v-if="vowifiPending" class="animate-spin switch-pending"><Loading /></el-icon>
          <el-switch
            v-model="local.vowifi_enabled"
            :disabled="vowifiPending"
            :class="{ 'is-failed': vowifiFailed }"
            @change="onVoWiFiToggle"
          />
        </div>
      </div>
      <!-- 网络 -->
      <div class="form-switch-row" :class="{ 'is-unsupported': isPCSC }">
        <div>
          <div class="switch-title">网络</div>
          <div class="switch-desc">启用蜂窝数据连接</div>
        </div>
        <div class="switch-action">
          <span v-if="networkFailed" class="switch-failed">未生效</span>
          <el-icon v-if="networkPending" class="animate-spin switch-pending"><Loading /></el-icon>
          <el-switch
            v-model="local.network_enabled"
            :disabled="local.vowifi_enabled || local.airplane_enabled || networkPending || isPCSC"
            :class="{ 'is-failed': networkFailed }"
            @change="onNetworkToggle"
          />
        </div>
      </div>
      <!-- 飞行模式 -->
      <div class="form-switch-row" :class="{ 'is-unsupported': isPCSC }">
        <div>
          <div class="switch-title">飞行模式</div>
          <div class="switch-desc">断开所有无线连接</div>
        </div>
        <div class="switch-action">
          <span v-if="airplaneFailed" class="switch-failed">未生效</span>
          <el-icon v-if="airplanePending" class="animate-spin switch-pending"><Loading /></el-icon>
          <el-switch
            v-model="local.airplane_enabled"
            :disabled="local.vowifi_enabled || airplanePending || isPCSC"
            :class="{ 'is-failed': airplaneFailed }"
            @change="onAirplaneToggle"
          />
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.policy-inline {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.policy-loading {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 12px;
  font-size: 12px;
  color: var(--muted-foreground);
}

.policy-failed {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  font-size: 12px;
  color: #f59e0b;
}

/* switch-row — 参照 ModuleConfigCards .form-switch-row */
.form-switch-row {
  display: flex;
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

.switch-action {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.switch-failed {
  font-size: 10px;
  color: #f59e0b;
}

.switch-pending {
  color: var(--muted-foreground);
}

.is-failed :deep(.el-switch__core) {
  border-color: var(--destructive) !important;
}

/* 被禁用控制块灰度显示 */
.form-switch-row.is-unsupported .switch-title {
  opacity: 0.4;
}
.form-switch-row.is-unsupported .switch-desc {
  opacity: 0.4;
}
</style>
