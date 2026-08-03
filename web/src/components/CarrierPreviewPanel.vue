<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { ElMessage } from 'element-plus'
import { useCarrierStore } from '../stores/carrier'
import { Shield24Regular, Person24Regular, Copy24Regular, ArrowDownload24Regular } from '@vicons/fluent'
import type { CarrierProfile } from '../types/api'

const store = useCarrierStore()
const { previewConfig, previewTarget, detail } = storeToRefs(store)

// 预览模式: 'json' | 'param'
const previewMode = ref<'json' | 'param'>('json')

const jsonText = computed(() => {
  if (!previewConfig.value) return '// 无配置'
  return JSON.stringify(previewConfig.value, null, 2)
})

const title = computed(() => {
  if (previewTarget.value === 'system') return '系统默认预览'
  return '用户配置预览'
})

const subtitle = computed(() => {
  if (!detail.value) return ''
  if (previewTarget.value === 'system') return `${detail.value.name} · 系统内置只读模板`
  if (detail.value.active) return `${detail.value.name} · 🟢 使用中`
  return `${detail.value.name} · ⚪ 空闲`
})

const isSystem = computed(() => previewTarget.value === 'system')

// 参数模式：按分组提取非空字段
const paramSections = computed(() => {
  const cfg = previewConfig.value
  if (!cfg) return []

  const sections: { label: string; items: { label: string; value: string }[] }[] = []

  // IKE
  const ikeItems: { label: string; value: string }[] = []
  if (cfg.ike) {
    const i = cfg.ike
    if (i.addr) ikeItems.push({ label: 'ePDG 地址', value: i.addr })
    if (i.port) ikeItems.push({ label: 'ePDG 端口', value: String(i.port) })
    if (i.proposals?.length) ikeItems.push({ label: 'IKE 提议', value: i.proposals.join(', ') })
    if (i.esp_proposals?.length) ikeItems.push({ label: 'ESP 提议', value: i.esp_proposals.join(', ') })
    if (i.dpd_interval) ikeItems.push({ label: 'DPD 间隔', value: `${i.dpd_interval}s` })
    if (i.nat_keepalive) ikeItems.push({ label: 'NAT 保活', value: `${i.nat_keepalive}s` })
    if (i.ip_stack) ikeItems.push({ label: 'IP 协议栈', value: i.ip_stack })
    if (i.apn) ikeItems.push({ label: 'APN', value: i.apn })
    if (i.replay_window) ikeItems.push({ label: '抗重放窗口', value: String(i.replay_window) })
    if (i.enable_esn) ikeItems.push({ label: 'ESN', value: '是' })
    if (i.disable_eap_mac_validation) ikeItems.push({ label: '禁用 EAP MAC 校验', value: '是' })
  }
  if (ikeItems.length) sections.push({ label: 'IKE/ePDG', items: ikeItems })

  // EAP
  const eapItems: { label: string; value: string }[] = []
  if (cfg.eap) {
    const e = cfg.eap
    if (e.challenge_mode) eapItems.push({ label: '挑战模式', value: e.challenge_mode })
    if (e.app_preference) eapItems.push({ label: 'USIM/ISIM 偏好', value: e.app_preference })
    if (e.identity_source) eapItems.push({ label: '身份来源', value: e.identity_source })
    if (e.device_model) eapItems.push({ label: '设备型号', value: e.device_model })
    if (e.device_identity_enabled) eapItems.push({ label: '发送设备身份', value: '是' })
  }
  if (eapItems.length) sections.push({ label: 'EAP-AKA', items: eapItems })

  // IMS
  const imsItems: { label: string; value: string }[] = []
  if (cfg.ims) {
    const m = cfg.ims
    if (m.sec_agree_mode) imsItems.push({ label: '安全协商模式', value: m.sec_agree_mode })
    if (m.transport_mode) imsItems.push({ label: '传输模式', value: m.transport_mode })
    if (m.initial_authorization) imsItems.push({ label: '首次认证变体', value: m.initial_authorization })
    if (m.expires) imsItems.push({ label: '注册有效期', value: `${m.expires}s` })
    if (m.user_agent) imsItems.push({ label: 'User-Agent', value: m.user_agent })
    if (m.supported_header) imsItems.push({ label: 'Supported 头', value: m.supported_header })
    if (m.pcscf_addr) imsItems.push({ label: 'P-CSCF 地址', value: m.pcscf_addr })
    if (m.domain) imsItems.push({ label: 'IMS 域名', value: m.domain })
    if (m.contact_features) imsItems.push({ label: 'Contact 特性', value: m.contact_features })
    if (m.security_client_format) imsItems.push({ label: 'Security-Client 格式', value: m.security_client_format })
    if (m.contact_param_order?.length) imsItems.push({ label: 'Contact 参数顺序', value: m.contact_param_order.join(', ') })
    if (m.include_pani) imsItems.push({ label: '首次 REGISTER 带 PANI', value: '是' })
    if (m.include_pani_authenticated) imsItems.push({ label: '认证后带 PANI', value: '是' })
    if (m.strict_security_server_offer) imsItems.push({ label: '严格匹配 Security-Server', value: '是' })
    if (m.enable_initial_reject_fallback) imsItems.push({ label: '首次拒绝后回退', value: '是' })
    if (m.include_accept_contact) imsItems.push({ label: 'Accept-Contact', value: '是' })
    if (m.include_p_preferred_id) imsItems.push({ label: 'P-Preferred-Identity', value: '是' })
    if (m.include_p_visited_network_id) imsItems.push({ label: 'P-Visited-Network-ID', value: '是' })
    if (m.include_p_access_network_info) imsItems.push({ label: 'P-Access-Network-Info', value: '是' })
    if (m.include_route) imsItems.push({ label: 'Route', value: '是' })
    if (m.include_security_client) imsItems.push({ label: 'Security-Client', value: '是' })
  }
  if (imsItems.length) sections.push({ label: 'IMS REGISTER', items: imsItems })

  // E911
  const e911Items: { label: string; value: string }[] = []
  if (cfg.e911) {
    const e = cfg.e911
    if (e.enabled) e911Items.push({ label: '启用', value: '是' })
    if (e.provider) e911Items.push({ label: '服务商', value: e.provider })
    if (e.entitlement_endpoint) e911Items.push({ label: 'Entitlement 端点', value: e.entitlement_endpoint })
    if (e.websheet) e911Items.push({ label: 'Websheet', value: e.websheet })
  }
  if (e911Items.length) sections.push({ label: 'E911', items: e911Items })

  // Device
  const devItems: { label: string; value: string }[] = []
  if (cfg.device) {
    const d = cfg.device
    if (d.imei) devItems.push({ label: 'IMEI', value: d.imei })
    if (d.ims_tac) devItems.push({ label: 'LTE TAC', value: String(d.ims_tac) })
    if (d.ims_cell_id) devItems.push({ label: 'LTE Cell ID', value: String(d.ims_cell_id) })
    if (d.ims_cell_id_mode) devItems.push({ label: 'Cell ID 模式', value: d.ims_cell_id_mode })
  }
  if (cfg.blocked) devItems.push({ label: '禁止 VoWiFi', value: '是' })
  if (devItems.length) sections.push({ label: '设备身份', items: devItems })

  return sections
})

function copyJson() {
  navigator.clipboard.writeText(jsonText.value).then(() => {
    ElMessage.success('JSON 已复制到剪贴板')
  }).catch(() => {
    ElMessage.error('复制失败')
  })
}

function downloadJson() {
  const cfg = previewConfig.value
  if (!cfg) return
  const filename = `${cfg.id || 'carrier'}.json`
  const blob = new Blob([jsonText.value], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <div class="preview-panel">
    <!-- 预览头部 (60px) -->
    <div class="preview-header">
      <div class="preview-header-left">
        <div class="preview-header-icon" :class="{ 'is-system': isSystem }">
          <el-icon size="16">
            <component :is="isSystem ? Shield24Regular : Person24Regular" />
          </el-icon>
        </div>
        <div>
          <div class="preview-title">{{ title }}</div>
          <div class="preview-subtitle">{{ subtitle }}</div>
        </div>
      </div>
      <!-- 模式切换 -->
      <div class="preview-mode-tabs">
        <button
          class="preview-mode-tab"
          :class="{ active: previewMode === 'param' }"
          @click="previewMode = 'param'"
        >参数</button>
        <button
          class="preview-mode-tab"
          :class="{ active: previewMode === 'json' }"
          @click="previewMode = 'json'"
        >JSON</button>
      </div>
    </div>

    <!-- 预览内容 -->
    <div class="preview-body">
      <!-- JSON 模式 -->
      <pre v-if="previewMode === 'json'" class="preview-json">{{ jsonText }}</pre>

      <!-- 参数模式 -->
      <div v-else class="preview-params">
        <div v-for="section in paramSections" :key="section.label" class="param-section">
          <div class="param-section-label">{{ section.label }}</div>
          <div class="param-items">
            <div v-for="item in section.items" :key="item.label" class="param-item">
              <span class="param-item-label">{{ item.label }}</span>
              <span class="param-item-value">{{ item.value }}</span>
            </div>
          </div>
        </div>
        <div v-if="paramSections.length === 0" class="param-empty">
          无配置参数
        </div>
      </div>
    </div>

    <!-- 底部操作栏 -->
    <div class="preview-footer">
      <el-button size="small" @click="copyJson" :disabled="!previewConfig">
        <el-icon class="mr-1"><Copy24Regular /></el-icon>
        <span>复制 JSON</span>
      </el-button>
      <el-button size="small" @click="downloadJson" :disabled="!previewConfig">
        <el-icon class="mr-1"><ArrowDownload24Regular /></el-icon>
        <span>下载 JSON</span>
      </el-button>
    </div>
  </div>
</template>

<style scoped>
.preview-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  overflow: hidden;
}

.preview-header {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.preview-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.preview-header-icon {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  background: var(--background);
  border: 1px solid var(--border);
  color: var(--foreground);
}

.preview-header-icon.is-system {
  background: var(--background);
  border: 1px solid var(--border);
  color: var(--foreground);
}

.preview-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--foreground);
}

.preview-subtitle {
  font-size: 11px;
  color: var(--muted-foreground);
  margin-top: 1px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 模式切换 */
.preview-mode-tabs {
  display: flex;
  gap: 2px;
  flex-shrink: 0;
}

.preview-mode-tab {
  padding: 4px 10px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--muted-foreground);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.12s;
}

.preview-mode-tab:hover {
  background: var(--accent);
  color: var(--foreground);
}

.preview-mode-tab.active {
  background: var(--background);
  border-color: var(--border);
  color: var(--foreground);
  box-shadow: var(--console-shadow-sm);
}

/* 预览内容 */
.preview-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 0;
}

.preview-json {
  margin: 0;
  padding: 12px;
  font-family: var(--oomol-font-mono);
  font-size: 11px;
  line-height: 1.5;
  color: var(--foreground);
  white-space: pre-wrap;
  word-break: break-all;
}

/* 参数模式 */
.preview-params {
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.param-section {
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: hidden;
}

.param-section-label {
  padding: 6px 10px;
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  background: var(--muted);
  border-bottom: 1px solid var(--border);
}

.param-items {
  display: flex;
  flex-direction: column;
}

.param-item {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--border);
}
.param-item:last-child {
  border-bottom: 0;
}

.param-item-label {
  font-size: 12px;
  color: var(--muted-foreground);
  flex-shrink: 0;
  min-width: 100px;
}

.param-item-value {
  font-size: 12px;
  color: var(--foreground);
  font-family: var(--oomol-font-mono);
  word-break: break-all;
}

.param-empty {
  padding: 24px;
  text-align: center;
  color: var(--muted-foreground);
  font-size: 13px;
}

/* 底部操作栏 */
.preview-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}
</style>
