<script setup lang="ts">
/**
 * ModuleConfigCards — 配置 Tab 下的网络设置子卡片
 * IP/APN 走 cardsService.putPolicy（PUT /cards/:iccid/policy）
 */
import { computed, ref, watch } from 'vue'
import { NetworkCheck24Regular } from '@vicons/fluent'
import type { CardPolicy } from '../types/api'
import { cardsService } from '../services/cards'
import { ElMessage } from 'element-plus'

const props = defineProps<{
  iccid?: string
  policy: CardPolicy | null
  deviceOnline: boolean
  isPCSC?: boolean
}>()

const emit = defineEmits<{
  policyChanged: []
}>()

/* ───────── 网络设置区（IP 版本 / APN） ───────── */

const canEdit = computed(() => props.deviceOnline && !!props.iccid)

const ipVersion = ref<'v4' | 'v6' | 'v4v6'>('v4')
const apn = ref('')
const netSaving = ref(false)
const netDirty = ref(false)

watch(
  () => props.policy,
  (p) => {
    if (!p) return
    ipVersion.value = p.ip_version || 'v4'
    apn.value = p.apn || ''
    netDirty.value = false
  },
  { immediate: true }
)

watch([ipVersion, apn], () => { netDirty.value = true })

async function saveNetworkSettings() {
  if (!props.iccid) return
  netSaving.value = true
  try {
    const result = await cardsService.putPolicy(props.iccid, {
      ip_version: ipVersion.value,
      apn: apn.value,
    })
    if (!result.ok) throw new Error(result.error.message || '保存失败')
    ElMessage.success('网络设置已保存')
    netDirty.value = false
    emit('policyChanged')
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '保存失败')
  } finally {
    netSaving.value = false
  }
}
</script>

<template>
  <div class="cc-card" :class="{ 'is-unsupported': isPCSC }">
    <div class="terminal-card-header">
      <div class="terminal-icon-box">
        <el-icon size="14"><NetworkCheck24Regular /></el-icon>
      </div>
      <div class="terminal-header-title">网络设置</div>
    </div>
    <div class="cc-body">
      <div v-if="!canEdit" class="cc-unavailable">
        设备离线或无 ICCID，网络设置暂不可用
      </div>
      <template v-else>
        <div class="form-grid">
          <div class="field">
            <label class="form-label">IP 版本</label>
            <el-select v-model="ipVersion" class="!w-full" :disabled="isPCSC || netSaving">
              <el-option label="IPv4" value="v4" />
              <el-option label="IPv6" value="v6" />
              <el-option label="IPv4/IPv6" value="v4v6" />
            </el-select>
          </div>
          <div class="field">
            <label class="form-label">APN</label>
            <el-input v-model="apn" placeholder="留空=运营商默认" :disabled="isPCSC || netSaving" />
          </div>
        </div>
        <div class="cc-footer">
          <button
            class="cc-btn cc-btn-save"
            :disabled="!netDirty || netSaving || isPCSC"
            @click="saveNetworkSettings"
          >
            {{ netSaving ? '保存中...' : '保存' }}
          </button>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.cc-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  overflow: hidden;
}

.terminal-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
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

.cc-body {
  padding: 16px;
}

.cc-unavailable {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 80px;
  color: var(--muted-foreground);
  font-size: 13px;
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

.form-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.cc-card.is-unsupported .form-label {
  opacity: 0.4;
}

.cc-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

.cc-btn {
  padding: 5px 16px;
  font-size: 12px;
  font-weight: 600;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s;
  border: 1px solid var(--border);
  background: var(--background);
  color: var(--foreground);
  white-space: nowrap;
}

.cc-btn:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.cc-btn-save:not(:disabled):hover {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
}
</style>
