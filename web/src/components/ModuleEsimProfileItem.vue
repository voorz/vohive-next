<script setup lang="ts">
import { computed, watch } from 'vue'
import type { EsimProfileItem } from '../types/api'
import { Settings24Regular } from '@vicons/fluent'
import CountryFlag from './CountryFlag.vue'
import CarrierIcon from './CarrierIcon.vue'
import { phoneToIso } from '../utils/phone-flag'
import { useProviderLogo, autoDownloadIcon } from '../composables/useProviderLogo'

const props = defineProps<{
  profile: EsimProfileItem
  aidHex: string
  showSensitive: boolean
  switching?: boolean
}>()

const emit = defineEmits<{
  'switch': [iccid: string, state: number, aidHex: string]
  'open-settings': [profile: EsimProfileItem, aidHex: string]
}>()

const flagIso = computed(() => phoneToIso(props.profile.name))
const isActive = computed(() => props.profile.state === 1)

// 运营商 LOGO — 使用 profile 自带的 MCC/MNC（来自 profileOwner），等待 catalog 加载
const { ready: logoReady } = useProviderLogo()
const plmn = computed(() => {
  if (!logoReady.value) return null
  if (!props.profile.mcc || !props.profile.mnc) return null
  return { mcc: props.profile.mcc, mnc: props.profile.mnc }
})

// 有 PLMN 时自动下载图标
watch(plmn, (val) => {
  if (val) {
    autoDownloadIcon(val.mcc, val.mnc, props.profile.service_provider_name)
  }
})
</script>

<template>
  <div class="profile-card">
    <div class="profile-card-inner">
      <!-- 图标盒子 + 内容区 -->
      <div class="profile-card-layout">
        <!-- 运营商图标盒子 -->
        <div class="profile-card-logo">
          <CarrierIcon v-if="plmn" :mcc="plmn.mcc" :mnc="plmn.mnc" :name="profile.service_provider_name" :size="42" />
          <span v-else class="profile-card-logo-fallback">{{ (profile.service_provider_name || profile.name || '?').charAt(0).toUpperCase() }}</span>
        </div>

        <!-- 内容区 -->
        <div class="profile-card-content">
          <!-- 第一行：国旗 + 名称（号码） -->
          <div class="profile-card-line1">
            <CountryFlag v-if="flagIso" :iso="flagIso" :size="18" />
            <span class="profile-card-name" :class="{ masked: !showSensitive }">
              {{ profile.name || profile.service_provider_name || profile.iccid }}
            </span>
          </div>

          <!-- 第二行：ICCID + 切换按钮 + 齿轮 -->
          <div class="profile-card-line2">
            <span class="profile-card-iccid" :class="{ masked: !showSensitive }">
              {{ profile.iccid }}
            </span>
            <button
              class="profile-card-switch"
              :class="{ on: isActive }"
              :disabled="switching"
              :title="isActive ? '点击禁用' : '点击启用'"
              @click.stop="emit('switch', profile.iccid, profile.state, aidHex)"
            >
              <span class="profile-card-switch-dot" />
            </button>
            <button class="profile-card-gear" @click.stop="emit('open-settings', profile, aidHex)">
              <el-icon size="16"><Settings24Regular /></el-icon>
            </button>
          </div>

          <!-- 第三行：运营商信息 -->
          <div class="profile-card-line3">
            <span v-if="profile.service_provider_name">{{ profile.service_provider_name }}</span>
            <span v-if="profile.class_text" class="profile-card-class">{{ profile.class_text }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 外层 — 2px padding 边框效果 + 阴影 */
.profile-card {
  border-radius: 8px;
  padding: 2px;
  background: var(--border);
  box-shadow: 0 1px 3px rgba(0,0,0,0.08);
  cursor: pointer;
  transition: box-shadow 0.15s;
  flex-shrink: 0;
}
.profile-card:hover {
  box-shadow: 0 2px 6px rgba(0,0,0,0.12);
}

/* 内层 — 渐变背景 */
.profile-card-inner {
  background: linear-gradient(0deg, var(--background), var(--card));
  border-radius: 6px;
  padding: 10px;
}

/* 图标盒子 + 内容区 横向布局 */
.profile-card-layout {
  display: flex;
  gap: 10px;
  align-items: center;
}

/* 运营商图标盒子 */
.profile-card-logo {
  width: 48px;
  height: 48px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  overflow: hidden;
}

.profile-card-logo-fallback {
  font-size: 16px;
  font-weight: 700;
  color: var(--foreground);
}

/* 内容区 */
.profile-card-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.profile-card-line1 {
  display: flex;
  align-items: center;
  gap: 6px;
}

.profile-card-name {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: filter 0.15s;
}
.profile-card-name.masked {
  filter: blur(3px);
  user-select: none;
}

.profile-card-line2 {
  display: flex;
  align-items: center;
  gap: 6px;
}

.profile-card-iccid {
  flex: 1;
  min-width: 0;
  font-size: 11px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: filter 0.15s;
}
.profile-card-iccid.masked {
  filter: blur(3px);
  user-select: none;
}

/* 切换按钮 — toggle switch 样式 */
.profile-card-switch {
  width: 32px;
  height: 18px;
  border: none;
  border-radius: 999px;
  background: var(--muted-foreground);
  opacity: 0.3;
  cursor: pointer;
  display: flex;
  align-items: center;
  padding: 2px;
  flex-shrink: 0;
  transition: all 0.2s;
}
.profile-card-switch:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.profile-card-switch-dot {
  width: 14px;
  height: 14px;
  border-radius: 999px;
  background: #fff;
  transition: transform 0.2s;
  transform: translateX(0);
}

.profile-card-switch.on {
  background: var(--brand);
  opacity: 1;
}

.profile-card-switch.on .profile-card-switch-dot {
  transform: translateX(14px);
}

.profile-card-gear {
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
  flex-shrink: 0;
  transition: all 0.12s;
}
.profile-card-gear:hover {
  background: var(--background);
  color: var(--foreground);
}

.profile-card-line3 {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--muted-foreground);
}

.profile-card-class {
  opacity: 0.7;
}
</style>
