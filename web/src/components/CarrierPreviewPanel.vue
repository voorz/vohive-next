<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useCarrierStore } from '../stores/carrier'
import { copyToClipboard } from '../utils/clipboard'
import { Shield24Regular, Sim24Regular, Copy24Regular, ArrowDownload24Regular } from '@vicons/fluent'

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
  return '配置预览'
})

const _subtitle = computed(() => {
  if (!detail.value) return ''
  if (previewTarget.value === 'system') return `${detail.value.name} · 系统内置只读模板`
  return `${detail.value.name} · 用户配置模板`
})

const isSystem = computed(() => previewTarget.value === 'system')

// 参数模式：按分组提取非空字段
const paramSections = computed(() => {
  const cfg = previewConfig.value
  if (!cfg) return []

  type Item = { label: string; value: string | string[] }
  const sections: { label: string; items: Item[] }[] = []
  const boolStr = (v: boolean | undefined) => v ? '是' : '否'

  // IKE
  const ikeItems: Item[] = []
  if (cfg.ike) {
    const i = cfg.ike
    if (i.addr) ikeItems.push({ label: 'ePDG 地址', value: i.addr })
    if (i.port) ikeItems.push({ label: 'ePDG 端口', value: String(i.port) })
    if (i.proposals?.length) ikeItems.push({ label: 'IKE 提议', value: i.proposals })
    if (i.esp_proposals?.length) ikeItems.push({ label: 'ESP 提议', value: i.esp_proposals })
    if (i.dpd_interval) ikeItems.push({ label: 'DPD 间隔', value: `${i.dpd_interval}s` })
    if (i.nat_keepalive) ikeItems.push({ label: 'NAT 保活', value: `${i.nat_keepalive}s` })
    if (i.reauth_interval) ikeItems.push({ label: '重认证间隔', value: `${i.reauth_interval}s` })
    if (i.ip_stack) ikeItems.push({ label: 'IP 协议栈', value: i.ip_stack })
    if (i.apn) ikeItems.push({ label: 'APN', value: i.apn })
    if (i.replay_window) ikeItems.push({ label: '抗重放窗口', value: String(i.replay_window) })
    if (i.enable_esn) ikeItems.push({ label: '启用 ESN', value: boolStr(i.enable_esn) })
    if (i.rekey_pfs) ikeItems.push({ label: 'Rekey PFS', value: String(i.rekey_pfs) })
    if (i.eap_mac_validation) ikeItems.push({ label: 'EAP MAC 校验', value: boolStr(i.eap_mac_validation) })
    if (i.ticket_request !== undefined) ikeItems.push({ label: 'TICKET_REQUEST', value: boolStr(i.ticket_request) })
    if (i.cp_in_first_auth !== undefined) ikeItems.push({ label: '首轮发 CP', value: boolStr(i.cp_in_first_auth) })
  }
  if (ikeItems.length) sections.push({ label: 'IKE/ePDG', items: ikeItems })

  // EAP
  const eapItems: Item[] = []
  if (cfg.eap) {
    const e = cfg.eap
    if (e.challenge_mode) eapItems.push({ label: '挑战模式', value: e.challenge_mode })
    if (e.app_preference) eapItems.push({ label: 'USIM/ISIM 偏好', value: e.app_preference })
    if (e.identity_source) eapItems.push({ label: '身份来源', value: e.identity_source })
    if (e.device_model) eapItems.push({ label: '设备型号', value: e.device_model })
    if (e.device_identity_enabled !== undefined) eapItems.push({ label: '发送设备身份', value: boolStr(e.device_identity_enabled) })
  }
  if (eapItems.length) sections.push({ label: 'EAP-AKA', items: eapItems })

  // IMS
  const imsItems: Item[] = []
  if (cfg.ims) {
    const m = cfg.ims
    if (m.sec_agree_mode) imsItems.push({ label: '安全协商模式', value: m.sec_agree_mode })
    if (m.require_sec_agree) imsItems.push({ label: 'Require: sec-agree', value: boolStr(m.require_sec_agree) })
    if (m.proxy_require_sec_agree) imsItems.push({ label: 'Proxy-Require: sec-agree', value: boolStr(m.proxy_require_sec_agree) })
    if (m.transport_mode) imsItems.push({ label: '传输模式', value: m.transport_mode })
    if (m.initial_authorization) imsItems.push({ label: '首次认证变体', value: m.initial_authorization })
    if (m.use_plain_digest_placeholder) imsItems.push({ label: '空 AKA Authorization', value: boolStr(m.use_plain_digest_placeholder) })
    if (m.expires) imsItems.push({ label: '注册有效期', value: `${m.expires}s` })
    if (m.user_agent) imsItems.push({ label: 'User-Agent', value: m.user_agent })
    if (m.supported_header) imsItems.push({ label: 'Supported 头', value: m.supported_header })
    if (m.allow_header) imsItems.push({ label: 'Allow 头', value: m.allow_header })
    if (m.pcscf_addr) imsItems.push({ label: 'P-CSCF 地址', value: m.pcscf_addr })
    if (m.domain) imsItems.push({ label: 'IMS 域名', value: m.domain })
    if (m.realm) imsItems.push({ label: 'Realm', value: m.realm })
    if (m.authorization_identity) imsItems.push({ label: '认证身份格式', value: m.authorization_identity })
    if (m.contact_features) imsItems.push({ label: 'Contact 特性', value: m.contact_features })
    if (m.security_client_format) imsItems.push({ label: 'Security-Client 格式', value: m.security_client_format })
    if (m.contact_param_order?.length) imsItems.push({ label: 'Contact 参数顺序', value: m.contact_param_order })
    if (m.security_client_mechanisms?.length) imsItems.push({ label: 'Security-Client 机制', value: m.security_client_mechanisms.map(s => `${s.alg}/${s.ealg}/${s.prot}/${s.mode}`) })
    if (m.fixed_pani) imsItems.push({ label: '固定 PANI', value: m.fixed_pani })
    if (m.icsi_ref) imsItems.push({ label: 'ICSI Ref', value: m.icsi_ref })
    if (m.local_port) imsItems.push({ label: '本地 SIP 端口', value: String(m.local_port) })
    if (m.tcp_keepalive_seconds) imsItems.push({ label: 'TCP Keepalive', value: `${m.tcp_keepalive_seconds}s` })
    if (m.options_ping_interval_seconds) imsItems.push({ label: 'OPTIONS Ping', value: `${m.options_ping_interval_seconds}s` })
    // 布尔开关
    if (m.include_pani) imsItems.push({ label: '首次 REGISTER 带 PANI', value: boolStr(m.include_pani) })
    if (m.include_pani_authenticated) imsItems.push({ label: '认证后带 PANI', value: boolStr(m.include_pani_authenticated) })
    if (m.strict_security_server_offer) imsItems.push({ label: '严格匹配 Security-Server', value: boolStr(m.strict_security_server_offer) })
    if (m.enable_initial_reject_fallback) imsItems.push({ label: '首次拒绝后回退', value: boolStr(m.enable_initial_reject_fallback) })
    if (m.omit_route) imsItems.push({ label: '省略 Route 头', value: boolStr(m.omit_route) })
    if (m.minimal_initial_headers) imsItems.push({ label: '首次 REGISTER 精简头', value: boolStr(m.minimal_initial_headers) })
    if (m.force_header_port_5060) imsItems.push({ label: '强制头中端口 5060', value: boolStr(m.force_header_port_5060) })
    if (m.omit_initial_security_client_protocol) imsItems.push({ label: 'Security-Client 省略 prot/mod', value: boolStr(m.omit_initial_security_client_protocol) })
    if (m.probe_initial_security_client_on_bad_request) imsItems.push({ label: '400 时探测 Security-Client', value: boolStr(m.probe_initial_security_client_on_bad_request) })
    if (m.include_connection_keepalive_in_auth) imsItems.push({ label: 'Authorization 含 Keepalive', value: boolStr(m.include_connection_keepalive_in_auth) })
    if (m.security_client_includes_server_params) imsItems.push({ label: 'Security-Client 含服务器参数', value: boolStr(m.security_client_includes_server_params) })
    if (m.fallback_includes_server_params_in_sec_cl) imsItems.push({ label: '回退时含服务器参数', value: boolStr(m.fallback_includes_server_params_in_sec_cl) })
    if (m.include_accept_contact) imsItems.push({ label: 'Accept-Contact', value: boolStr(m.include_accept_contact) })
    if (m.include_p_preferred_id) imsItems.push({ label: 'P-Preferred-Identity', value: boolStr(m.include_p_preferred_id) })
    if (m.include_p_visited_network_id) imsItems.push({ label: 'P-Visited-Network-ID', value: boolStr(m.include_p_visited_network_id) })
    if (m.include_p_access_network_info) imsItems.push({ label: 'P-Access-Network-Info', value: boolStr(m.include_p_access_network_info) })
    if (m.include_route) imsItems.push({ label: 'Route 头', value: boolStr(m.include_route) })
    if (m.include_cellular_network) imsItems.push({ label: 'Cellular-Network-Info', value: boolStr(m.include_cellular_network) })
    if (m.include_security_client) imsItems.push({ label: 'Security-Client 头', value: boolStr(m.include_security_client) })
    if (m.include_require_sec_agree) imsItems.push({ label: 'Require: sec-agree 头', value: boolStr(m.include_require_sec_agree) })
    if (m.contact_user_random) imsItems.push({ label: 'Contact URI 随机 UUID', value: boolStr(m.contact_user_random) })
    // 语音 INVITE 头
    if (m.voice_supported_header) imsItems.push({ label: 'Voice Supported', value: m.voice_supported_header })
    if (m.voice_allow_header) imsItems.push({ label: 'Voice Allow', value: m.voice_allow_header })
    if (m.voice_accept_contact) imsItems.push({ label: 'Voice Accept-Contact', value: m.voice_accept_contact })
    if (m.voice_p_preferred_service) imsItems.push({ label: 'Voice P-Preferred-Service', value: m.voice_p_preferred_service })
    // 注册策略
    if (m.register_policy) {
      const rp = m.register_policy
      if (rp.id) imsItems.push({ label: '注册策略 ID', value: rp.id })
      if (rp.temporary_status_codes?.length) imsItems.push({ label: '临时失败状态码', value: rp.temporary_status_codes.map(String) })
      if (rp.forbidden_status_codes?.length) imsItems.push({ label: '禁止状态码', value: rp.forbidden_status_codes.map(String) })
      if (rp.initial_reject_fallback_status_codes?.length) imsItems.push({ label: '首次拒绝回退状态码', value: rp.initial_reject_fallback_status_codes.map(String) })
      if (rp.temporary_retry_seconds) imsItems.push({ label: '临时失败重试间隔', value: `${rp.temporary_retry_seconds}s` })
    }
  }
  if (imsItems.length) sections.push({ label: 'IMS REGISTER', items: imsItems })

  // E911
  const e911Items: Item[] = []
  if (cfg.e911) {
    const e = cfg.e911
    if (e.enabled) e911Items.push({ label: '启用', value: boolStr(e.enabled) })
    if (e.provider) e911Items.push({ label: '服务商', value: e.provider })
    if (e.entitlement_endpoint) e911Items.push({ label: 'Entitlement 端点', value: e.entitlement_endpoint })
    if (e.websheet) e911Items.push({ label: 'Websheet', value: e.websheet })
  }
  if (e911Items.length) sections.push({ label: 'E911', items: e911Items })

  // Device
  const devItems: Item[] = []
  if (cfg.device) {
    const d = cfg.device
    if (d.imei) devItems.push({ label: 'IMEI', value: d.imei })
    if (d.ims_tac) devItems.push({ label: 'LTE TAC', value: String(d.ims_tac) })
    if (d.ims_cell_id) devItems.push({ label: 'LTE Cell ID', value: String(d.ims_cell_id) })
    if (d.ims_cell_id_mode) devItems.push({ label: 'Cell ID 模式', value: d.ims_cell_id_mode })
    if (d.ims_register_profile) devItems.push({ label: 'REGISTER 模拟档案', value: d.ims_register_profile })
  }
  if (cfg.blocked) devItems.push({ label: '禁止 VoWiFi', value: boolStr(cfg.blocked) })
  if (devItems.length) sections.push({ label: '设备身份', items: devItems })

  return sections
})

function copyJson() {
  copyToClipboard(jsonText.value, 'JSON 已复制到剪贴板')
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
            <component :is="isSystem ? Shield24Regular : Sim24Regular" />
          </el-icon>
        </div>
        <div>
          <div class="preview-title">{{ title }}</div>
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
              <div class="param-item-values">
                <span v-for="(v, idx) in (Array.isArray(item.value) ? item.value : [item.value])" :key="idx" class="param-item-value">{{ v }}</span>
              </div>
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
      <div class="preview-footer-actions">
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
  padding: 0 12px;
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
  border: 1px solid var(--border);
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
  padding: 12px;
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
  display: block;
}

.param-item-values {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
}

.param-empty {
  padding: 12px;
  text-align: center;
  color: var(--muted-foreground);
  font-size: 13px;
}

/* 底部操作栏 */
.preview-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

.preview-footer-info {
  font-size: 11px;
  color: var(--muted-foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.preview-footer-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}
</style>
