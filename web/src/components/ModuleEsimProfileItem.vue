<script setup lang="ts">
import { computed, watch } from 'vue'
import type { EsimProfileItem } from '../types/api'
import { Settings24Regular, CalendarAdd24Regular, Tag24Regular } from '@vicons/fluent'
import CountryFlag from './CountryFlag.vue'
import CarrierIcon from './CarrierIcon.vue'
import { phoneToIso } from '../utils/phone-flag'
import { mccToIso } from '../composables/plmn-info'
import { useProviderLogo, autoDownloadIcon } from '../composables/useProviderLogo'
import { parseNickname, type DateTag, type TextTag } from '../utils/profileTagUtils'

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

// 解析 Nickname：拆分纯名称 + 标签
const parsed = computed(() => parseNickname(props.profile.name))
const displayName = computed(() => parsed.value.name || props.profile.service_provider_name || props.profile.iccid)
const dateTags = computed(() => parsed.value.tags.filter((t): t is DateTag => t.type === 'date'))
const textTags = computed(() => parsed.value.tags.filter((t): t is TextTag => t.type === 'text'))

// 优先用 PLMN (MCC+MNC) 匹配国旗，回退用手机号 phoneToIso
const flagIso = computed(() => {
  const iso = mccToIso(props.profile.mcc, props.profile.mnc)
  if (iso) return iso
  return phoneToIso(parsed.value.name)
})
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

// 容量胶囊颜色分级：<30kb 品牌色，30-50kb 黄色，>50kb 红色
const sizePillClass = computed(() => {
  const bytes = props.profile.profile_size_bytes ?? 0
  if (bytes <= 0) return ''
  const kb = bytes / 1024
  if (kb < 30) return 'size-ok'
  if (kb <= 50) return 'size-warn'
  return 'size-danger'
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
          <span v-else class="profile-card-logo-fallback">{{ (profile.service_provider_name || displayName || '?').charAt(0).toUpperCase() }}</span>
        </div>

        <!-- 内容区 -->
        <div class="profile-card-content">
          <!-- 第一行：国旗 + 名称 + 切换按钮 -->
          <div class="profile-card-line1">
            <CountryFlag v-if="flagIso" :iso="flagIso" :size="18" />
            <span class="profile-card-name" :class="{ masked: !showSensitive }">
              {{ displayName }}
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
          </div>

          <!-- 第二行：ICCID + 齿轮 -->
          <div class="profile-card-line2">
            <span class="profile-card-iccid" :class="{ masked: !showSensitive }">
              {{ profile.iccid }}
            </span>
            <el-button
              class="profile-card-gear"
              text
              size="small"
              @click.stop="emit('open-settings', profile, aidHex)"
            >
              <el-icon size="16"><Settings24Regular /></el-icon>
            </el-button>
          </div>

          <!-- 第三行：AID -->
          <div v-if="profile.isdp_aid" class="profile-card-line2">
            <span class="profile-card-isdp" :class="{ masked: !showSensitive }">
              {{ profile.isdp_aid.toUpperCase() }}
            </span>
          </div>

<!-- 第四行：运营商名称 + ProfileName -->
<div v-if="profile.service_provider_name || profile.profile_name" class="profile-card-line3">
  <span v-if="profile.service_provider_name" class="profile-card-spn">{{ profile.service_provider_name }}</span>
  <span v-if="profile.profile_name" class="profile-card-alias">| {{ profile.profile_name }}</span>
</div>

          <!-- 第五行：PLMN + 容量胶囊 + 文本标签 -->
          <div v-if="profile.mcc || profile.mnc || profile.profile_size_formatted || textTags.length > 0" class="profile-card-gid-row">
            <span v-if="profile.mcc || profile.mnc" class="profile-card-plmn-tag">
              {{ profile.mcc }}{{ profile.mnc ? '-' + profile.mnc : '' }}
            </span>
            <span
              v-if="profile.profile_size_formatted"
              class="profile-card-size-pill"
              :class="sizePillClass"
            >
              {{ profile.profile_size_formatted }}
            </span>
            <span
              v-for="tag in textTags"
              :key="tag.raw"
              class="profile-card-tag-pill tag-text"
            >
              <el-icon size="10" class="tag-pill-icon"><Tag24Regular /></el-icon>
              {{ tag.text }}
            </span>
          </div>

          <!-- 第六行：日期标签胶囊 -->
          <div v-if="dateTags.length > 0" class="profile-card-date-row">
            <span
              v-for="tag in dateTags"
              :key="tag.raw"
              class="profile-card-tag-pill"
              :class="tag.expired ? 'tag-date-expired' : 'tag-date'"
            >
              <el-icon size="10" class="tag-pill-icon"><CalendarAdd24Regular /></el-icon>
              <span class="tag-pill-date">{{ tag.displayDate }}</span>
              <span v-if="tag.note" class="tag-pill-note">{{ tag.note }}</span>
              <span v-if="tag.countdownDays >= 0" class="tag-pill-countdown">({{ tag.countdownDays }}天)</span>
              <span v-if="tag.expired" class="tag-pill-expired">已过期</span>
            </span>
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
  align-items: flex-start;
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
  flex: 1;
  min-width: 0;
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

.profile-card-isdp {
  font-size: 10px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
  opacity: 0.6;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
  transition: filter 0.15s;
}
.profile-card-isdp.masked {
  filter: blur(3px);
  user-select: none;
}

/* 容量胶囊 + PLMN 标签 — 统一描边样式 */
.profile-card-size-pill,
.profile-card-plmn-tag {
  display: inline-flex;
  align-items: center;
  font-size: 10px;
  font-weight: 600;
  font-family: var(--oomol-font-mono);
  color: var(--muted-foreground);
  background: color-mix(in oklab, var(--muted-foreground) 12%, transparent);
  border: 1px solid color-mix(in oklab, var(--muted-foreground) 20%, transparent);
  padding: 1px 6px;
  border-radius: 999px;
  flex-shrink: 0;
}

/* 容量胶囊颜色分级 */
.profile-card-size-pill.size-ok {
  color: var(--success);
  background: color-mix(in oklab, var(--success) 12%, transparent);
  border-color: color-mix(in oklab, var(--success) 24%, transparent);
}
.profile-card-size-pill.size-warn {
  color: var(--warning);
  background: color-mix(in oklab, var(--warning) 14%, transparent);
  border-color: color-mix(in oklab, var(--warning) 28%, transparent);
}
.profile-card-size-pill.size-danger {
  color: var(--destructive);
  background: color-mix(in oklab, var(--destructive) 14%, transparent);
  border-color: color-mix(in oklab, var(--destructive) 28%, transparent);
}

/* 切换按钮 — toggle switch 样式 */
.profile-card-switch {
  width: 32px;
  height: 18px;
  border: none;
  border-radius: 999px;
  background: var(--muted-foreground);
  opacity: 0.4;
  cursor: pointer;
  display: flex;
  align-items: center;
  padding: 2px;
  flex-shrink: 0;
  transition: all 0.2s;
}
.profile-card-switch:hover:not(:disabled) {
  opacity: 0.6;
}
.profile-card-switch:disabled {
  opacity: 0.3;
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
  padding: 0;
  min-height: auto;
}
.profile-card-gear:hover,
.profile-card-gear:focus {
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

.profile-card-gid-row {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow: hidden;
}

.profile-card-gid {
  font-family: var(--oomol-font-mono);
  opacity: 0.5;
  font-size: 10px;
  color: var(--muted-foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.profile-card-spn {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.profile-card-alias {
  font-size: 11px;
  color: var(--muted-foreground);
  opacity: 0.7;
  flex-shrink: 0;
  white-space: nowrap;
}

/* 标签胶囊 — 日期 + 文本 */
.profile-card-tag-pill {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 10px;
  font-weight: 600;
  font-family: var(--oomol-font-mono);
  padding: 1px 6px;
  border-radius: 999px;
  border: 1px solid transparent;
  flex-shrink: 0;
  white-space: nowrap;
}

.tag-pill-icon {
  flex-shrink: 0;
}

.profile-card-tag-pill.tag-text {
  color: var(--brand);
  background: color-mix(in oklab, var(--brand) 12%, transparent);
  border-color: color-mix(in oklab, var(--brand) 24%, transparent);
}

.profile-card-tag-pill.tag-date {
  color: var(--warning);
  background: color-mix(in oklab, var(--warning) 14%, transparent);
  border-color: color-mix(in oklab, var(--warning) 28%, transparent);
}

.profile-card-tag-pill.tag-date-expired {
  color: var(--destructive);
  background: color-mix(in oklab, var(--destructive) 14%, transparent);
  border-color: color-mix(in oklab, var(--destructive) 28%, transparent);
}

.tag-pill-date {
  font-family: var(--oomol-font-mono);
}

.tag-pill-note {
  opacity: 0.8;
}

.tag-pill-countdown {
  opacity: 0.7;
  font-size: 9px;
}

.tag-pill-expired {
  font-size: 9px;
  font-weight: 700;
  opacity: 0.9;
}

/* 日期标签行 */
.profile-card-date-row {
  display: flex;
  align-items: center;
  gap: 4px;
  overflow: hidden;
}
</style>
