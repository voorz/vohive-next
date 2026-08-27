<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useCarrierStore } from '../stores/carrier'
import { ChevronDown20Regular } from '@vicons/fluent'
import type { CarrierProfile } from '../types/api'
import ConfigFieldLabel from './ConfigFieldLabel.vue'
import TagInputWithPresets from './TagInputWithPresets.vue'
import { configAnnotations } from '../data/carrier-config-annotations'

const store = useCarrierStore()
const { editingConfig } = storeToRefs(store)

const cfg = computed(() => editingConfig.value as CarrierProfile)

const expanded = ref<Record<string, boolean>>({
  ike: true,
  eap: false,
  ims: false,
  voice: false,
  e911: false,
  device: false
})

function toggle(key: string) {
  expanded.value[key] = !expanded.value[key]
}

function ensureIke() { if (!cfg.value.ike) cfg.value.ike = {} }
function ensureEap() { if (!cfg.value.eap) cfg.value.eap = {} }
function ensureIms() { if (!cfg.value.ims) cfg.value.ims = {} }
function ensureE911() { if (!cfg.value.e911) cfg.value.e911 = {} }
function ensureDevice() { if (!cfg.value.device) cfg.value.device = {} }
function ensureRegPolicy() { if (!cfg.value.ims) cfg.value.ims = {}; if (!cfg.value.ims!.register_policy) cfg.value.ims!.register_policy = {} }

// 下拉选项
const ipStackOptions = [
  { label: '自动', value: '' },
  { label: 'IPv4', value: 'ipv4' },
  { label: 'IPv6', value: 'ipv6' },
  { label: 'IPv4/IPv6', value: 'ipv4v6' }
]

const challengeModeOptions = [
  { label: '标准', value: 'standard' },
  { label: '校验码', value: 'checkcode' },
  { label: '省略', value: 'omit' }
]

const appPreferenceOptions = [
  { label: '自动', value: 'auto' },
  { label: 'USIM', value: 'usim' },
  { label: 'ISIM', value: 'isim' }
]

const identitySourceOptions = [
  { label: '推导', value: 'derived' },
  { label: 'ISIM', value: 'isim' },
  { label: '自动', value: 'auto' }
]

const secAgreeModeOptions = [
  { label: '自动', value: 'auto' },
  { label: '开启', value: 'on' },
  { label: '关闭', value: 'off' }
]

const transportModeOptions = [
  { label: '自动 (UDP优先)', value: 'auto' },
  { label: 'UDP', value: 'udp' },
  { label: 'TCP', value: 'tcp' }
]

const initialAuthOptions = [
  { label: 'AKA空URI优先', value: 'aka_empty_uri_first' },
  { label: 'AKA空', value: 'aka_empty' },
  { label: 'AKA零响应', value: 'aka_zero_response' },
  { label: 'AKA零响应URI优先', value: 'aka_zero_response_uri_first' },
  { label: '无', value: 'none' }
]

const contactFeaturesOptions = [
  { label: 'IMS特性', value: 'ims_features' },
  { label: '小米手机', value: 'phone_xiaomi' },
  { label: '仅短信', value: 'sms_only' }
]

const securityClientFormatOptions = [
  { label: '完整空格', value: 'full_spaced' },
  { label: '手机多行', value: 'phone_multi' },
  { label: '精简空格', value: 'minimal_spaced' }
]

const cellIdModeOptions = [
  { label: 'QMI优先', value: 'qmi_first' },
  { label: '仅运营商', value: 'carrier_only' },
  { label: '无', value: 'none' }
]

const authIdentityOptions = [
  { label: 'IMSI 域名', value: 'imsi_home_domain' },
  { label: 'Private ID', value: 'private_id' },
  { label: 'IMSI+域名前缀', value: 'prefixed_imsi_home_domain' },
  { label: 'IMSI Phone URI', value: 'imsi_phone_uri' }
]

function onInput() {
  store.markDirty()
}

// 生成随机 wlan-node-id PANI 值
function generateRandomPANI() {
  const hex = Array.from({length: 12}, () => Math.floor(Math.random() * 16).toString(16)).join('')
  if (cfg.value.ims) {
    cfg.value.ims.fixed_pani = `IEEE-802.11;i-wlan-node-id=${hex};network-provided`
  } else {
    ensureIms()
    cfg.value.ims!.fixed_pani = `IEEE-802.11;i-wlan-node-id=${hex};network-provided`
  }
  onInput()
}

// ===== Tag 输入预设数据 =====

const ikeProposalsPresets: Record<string, string> = {
  'aes256-sha256-prfsha256-modp2048': 'AES-256-SHA256 (推荐)',
  'aes128-sha256-prfsha256-modp2048': 'AES-128-SHA256',
  'aes256-sha1-prfsha1-modp2048': 'AES-256-SHA1 (旧 ePDG 兼容)',
  'aes128-sha1-prfsha1-modp2048': 'AES-128-SHA1 (最旧)',
  'aes256gcm16-prfsha256-modp2048': 'AES-256-GCM',
  'aes128gcm16-prfsha256-modp2048': 'AES-128-GCM',
}

const espProposalsPresets: Record<string, string> = {
  'aes256-sha256': 'AES-256-SHA256 (推荐)',
  'aes128-sha256': 'AES-128-SHA256',
  'aes256-sha1': 'AES-256-SHA1 (旧)',
  'aes128-sha1': 'AES-128-SHA1 (最旧)',
  'aes256gcm16': 'AES-256-GCM',
  'aes128gcm16': 'AES-128-GCM',
}

const contactParamOrderPresets: Record<string, string> = {
  'access_type': 'Access Type',
  'audio': 'Audio',
  'smsip': 'SMS over IP',
  'icsi_ref': 'ICSI Ref',
  'sip_instance': 'SIP Instance',
}

const sipStatusCodesPresets: Record<string, string> = {
  '400': '400 Bad Request',
  '403': '403 Forbidden',
  '408': '408 Request Timeout',
  '480': '480 Temporarily Unavailable',
  '500': '500 Server Internal Error',
  '502': '502 Bad Gateway',
  '503': '503 Service Unavailable',
  '504': '504 Server Timeout',
}

// 数组 computed（直接绑定 TagInputWithPresets）
const ikeProposalsArr = computed({
  get: () => cfg.value.ike?.proposals || [],
  set: (v: string[]) => { ensureIke(); cfg.value.ike!.proposals = v; onInput() }
})

const espProposalsArr = computed({
  get: () => cfg.value.ike?.esp_proposals || [],
  set: (v: string[]) => { ensureIke(); cfg.value.ike!.esp_proposals = v; onInput() }
})

const contactParamOrderArr = computed({
  get: () => cfg.value.ims?.contact_param_order || [],
  set: (v: string[]) => { ensureIms(); cfg.value.ims!.contact_param_order = v; onInput() }
})

// 状态码：number[] ↔ string[]
const tempStatusCodesArr = computed({
  get: () => (cfg.value.ims?.register_policy?.temporary_status_codes || []).map(String),
  set: (v: string[]) => { ensureRegPolicy(); cfg.value.ims!.register_policy!.temporary_status_codes = v.map(s => parseInt(s)).filter(n => !isNaN(n)); onInput() }
})

const forbiddenStatusCodesArr = computed({
  get: () => (cfg.value.ims?.register_policy?.forbidden_status_codes || []).map(String),
  set: (v: string[]) => { ensureRegPolicy(); cfg.value.ims!.register_policy!.forbidden_status_codes = v.map(s => parseInt(s)).filter(n => !isNaN(n)); onInput() }
})

const fallbackStatusCodesArr = computed({
  get: () => (cfg.value.ims?.register_policy?.initial_reject_fallback_status_codes || []).map(String),
  set: (v: string[]) => { ensureRegPolicy(); cfg.value.ims!.register_policy!.initial_reject_fallback_status_codes = v.map(s => parseInt(s)).filter(n => !isNaN(n)); onInput() }
})
</script>

<template>
  <div class="config-form" v-if="cfg">
    <!-- ════════ IKE/ePDG 阶段 ════════ -->
    <div class="faq-card">
      <div class="faq-header" @click="toggle('ike')">
        <span class="faq-title">IKE/ePDG 连接</span>
        <el-icon class="faq-arrow" :class="{ expanded: expanded.ike }" size="16"><ChevronDown20Regular /></el-icon>
      </div>
      <div v-show="expanded.ike" class="faq-body">
        <div class="form-grid">
          <div class="field">
            <ConfigFieldLabel label="ePDG 地址" section="ike" :annotations="configAnnotations" />
            <el-input :model-value="cfg.ike?.addr || ''" @update:model-value="(v: string) => { ensureIke(); cfg.ike!.addr = v; onInput() }" placeholder="空则自动生成 3GPP FQDN" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="ePDG 端口" section="ike" :annotations="configAnnotations" />
            <el-input-number :model-value="cfg.ike?.port || 0" @update:model-value="(v: number | undefined) => { ensureIke(); cfg.ike!.port = v || 0; onInput() }" :min="0" :max="65535" placeholder="500" class="!w-full" />
          </div>
          <div class="field col-span-2">
            <ConfigFieldLabel label="IKE 提议" section="ike" :annotations="configAnnotations" />
            <TagInputWithPresets v-model="ikeProposalsArr" :presets="ikeProposalsPresets" placeholder="手动输入或从预设选择" />
          </div>
          <div class="field col-span-2">
            <ConfigFieldLabel label="ESP 提议" section="ike" :annotations="configAnnotations" />
            <TagInputWithPresets v-model="espProposalsArr" :presets="espProposalsPresets" placeholder="手动输入或从预设选择" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="DPD 间隔 (秒)" section="ike" :annotations="configAnnotations" />
            <el-input-number :model-value="cfg.ike?.dpd_interval || 0" @update:model-value="(v: number | undefined) => { ensureIke(); cfg.ike!.dpd_interval = v || 0; onInput() }" :min="0" placeholder="0=禁用" class="!w-full" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="NAT 保活 (秒)" section="ike" :annotations="configAnnotations" />
            <el-input-number :model-value="cfg.ike?.nat_keepalive || 0" @update:model-value="(v: number | undefined) => { ensureIke(); cfg.ike!.nat_keepalive = v || 0; onInput() }" :min="0" placeholder="20" class="!w-full" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="重认证间隔 (秒)" section="ike" :annotations="configAnnotations" />
            <el-input-number :model-value="cfg.ike?.reauth_interval || 0" @update:model-value="(v: number | undefined) => { ensureIke(); cfg.ike!.reauth_interval = v || 0; onInput() }" :min="0" placeholder="0=不强制" class="!w-full" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="抗重放窗口" section="ike" :annotations="configAnnotations" />
            <el-input-number :model-value="cfg.ike?.replay_window || 0" @update:model-value="(v: number | undefined) => { ensureIke(); cfg.ike!.replay_window = v || 0; onInput() }" :min="0" placeholder="32" class="!w-full" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="IP 协议栈" section="ike" :annotations="configAnnotations" />
            <el-select :model-value="cfg.ike?.ip_stack || ''" @update:model-value="(v: string) => { ensureIke(); cfg.ike!.ip_stack = v; onInput() }" class="!w-full">
              <el-option v-for="opt in ipStackOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="field">
            <ConfigFieldLabel label="APN" section="ike" :annotations="configAnnotations" />
            <el-input :model-value="cfg.ike?.apn || ''" @update:model-value="(v: string) => { ensureIke(); cfg.ike!.apn = v; onInput() }" placeholder="ims" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="RFOff 延迟 (秒)" section="ike" :annotations="configAnnotations" />
            <el-input-number :model-value="cfg.ike?.rf_off_delay || 0" @update:model-value="(v: number | undefined) => { ensureIke(); cfg.ike!.rf_off_delay = v || 0; onInput() }" :min="0" placeholder="5" class="!w-full" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="启用 ESN" section="ike" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">Extended Sequence Numbers</div></div>
            <el-switch :model-value="cfg.ike?.enable_esn || false" @update:model-value="(v: string | number | boolean) => { ensureIke(); cfg.ike!.enable_esn = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="禁用 EAP MAC 校验" section="ike" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">disable_eap_mac_validation</div></div>
            <el-switch :model-value="cfg.ike?.disable_eap_mac_validation || false" @update:model-value="(v: string | number | boolean) => { ensureIke(); cfg.ike!.disable_eap_mac_validation = Boolean(v); onInput() }" />
          </div>
        </div>
      </div>
    </div>

    <!-- ════════ EAP-AKA 阶段 ════════ -->
    <div class="faq-card">
      <div class="faq-header" @click="toggle('eap')">
        <span class="faq-title">EAP-AKA 认证</span>
        <el-icon class="faq-arrow" :class="{ expanded: expanded.eap }" size="16"><ChevronDown20Regular /></el-icon>
      </div>
      <div v-show="expanded.eap" class="faq-body">
        <div class="form-grid">
          <div class="field">
            <ConfigFieldLabel label="挑战模式" section="eap" :annotations="configAnnotations" />
            <el-select :model-value="cfg.eap?.challenge_mode || ''" @update:model-value="(v: string) => { ensureEap(); cfg.eap!.challenge_mode = v; onInput() }" class="!w-full">
              <el-option v-for="opt in challengeModeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="field">
            <ConfigFieldLabel label="USIM/ISIM 偏好" section="eap" :annotations="configAnnotations" />
            <el-select :model-value="cfg.eap?.app_preference || ''" @update:model-value="(v: string) => { ensureEap(); cfg.eap!.app_preference = v; onInput() }" class="!w-full">
              <el-option v-for="opt in appPreferenceOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="field">
            <ConfigFieldLabel label="身份来源" section="eap" :annotations="configAnnotations" />
            <el-select :model-value="cfg.eap?.identity_source || ''" @update:model-value="(v: string) => { ensureEap(); cfg.eap!.identity_source = v; onInput() }" class="!w-full">
              <el-option v-for="opt in identitySourceOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="field">
            <ConfigFieldLabel label="设备型号" section="eap" :annotations="configAnnotations" />
            <el-input :model-value="cfg.eap?.device_model || ''" @update:model-value="(v: string) => { ensureEap(); cfg.eap!.device_model = v; onInput() }" placeholder="例如 rmx3366" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="发送设备身份通知" section="eap" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">EAP 阶段是否发送 DEVICE_IDENTITY notify</div></div>
            <el-switch :model-value="cfg.eap?.device_identity_enabled || false" @update:model-value="(v: string | number | boolean) => { ensureEap(); cfg.eap!.device_identity_enabled = Boolean(v); onInput() }" />
          </div>
        </div>
      </div>
    </div>

    <!-- ════════ IMS REGISTER 阶段 ════════ -->
    <div class="faq-card">
      <div class="faq-header" @click="toggle('ims')">
        <span class="faq-title">IMS REGISTER</span>
        <el-icon class="faq-arrow" :class="{ expanded: expanded.ims }" size="16"><ChevronDown20Regular /></el-icon>
      </div>
      <div v-show="expanded.ims" class="faq-body">
        <div class="form-grid">
          <div class="field">
            <ConfigFieldLabel label="安全协商模式" section="ims" :annotations="configAnnotations" />
            <el-select :model-value="cfg.ims?.sec_agree_mode || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.sec_agree_mode = v; onInput() }" class="!w-full">
              <el-option v-for="opt in secAgreeModeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="field">
            <ConfigFieldLabel label="传输模式" section="ims" :annotations="configAnnotations" />
            <el-select :model-value="cfg.ims?.transport_mode || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.transport_mode = v; onInput() }" class="!w-full">
              <el-option v-for="opt in transportModeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="field">
            <ConfigFieldLabel label="首次认证变体" section="ims" :annotations="configAnnotations" />
            <el-select :model-value="cfg.ims?.initial_authorization || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.initial_authorization = v; onInput() }" class="!w-full">
              <el-option v-for="opt in initialAuthOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="field">
            <ConfigFieldLabel label="注册有效期 (秒)" section="ims" :annotations="configAnnotations" />
            <el-input-number :model-value="cfg.ims?.expires || 0" @update:model-value="(v: number | undefined) => { ensureIms(); cfg.ims!.expires = v || 0; onInput() }" :min="0" placeholder="600" class="!w-full" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="认证身份格式" section="ims" :annotations="configAnnotations" />
            <el-select :model-value="cfg.ims?.authorization_identity || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.authorization_identity = v; onInput() }" class="!w-full">
              <el-option v-for="opt in authIdentityOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="field">
            <ConfigFieldLabel label="本地 SIP 端口" section="ims" :annotations="configAnnotations" />
            <el-input-number :model-value="cfg.ims?.local_port || 0" @update:model-value="(v: number | undefined) => { ensureIms(); cfg.ims!.local_port = v || 0; onInput() }" :min="0" :max="65535" placeholder="5060" class="!w-full" />
          </div>
          <div class="field col-span-2">
            <ConfigFieldLabel label="User-Agent" section="ims" :annotations="configAnnotations" />
            <el-input :model-value="cfg.ims?.user_agent || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.user_agent = v; onInput() }" placeholder="User-Agent: Apple iPhone17,2/26.6 (17,2; iOS 26.6; 23G82) Boot/3.0.0 VoIP/1.0 Carrier/59.0" />
          </div>
          <div class="field col-span-2">
            <ConfigFieldLabel label="Supported 头" section="ims" :annotations="configAnnotations" />
            <el-input :model-value="cfg.ims?.supported_header || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.supported_header = v; onInput() }" placeholder="path,sec-agree,gruu" />
          </div>
          <div class="field col-span-2">
            <ConfigFieldLabel label="Allow 头" section="ims" :annotations="configAnnotations" />
            <el-input :model-value="cfg.ims?.allow_header || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.allow_header = v; onInput() }" placeholder="空=默认" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="P-CSCF 地址" section="ims" :annotations="configAnnotations" />
            <el-input :model-value="cfg.ims?.pcscf_addr || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.pcscf_addr = v; onInput() }" placeholder="覆盖自动发现" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="IMS 域名" section="ims" :annotations="configAnnotations" />
            <el-input :model-value="cfg.ims?.domain || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.domain = v; onInput() }" placeholder="自动生成" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="Realm" section="ims" :annotations="configAnnotations" />
            <el-input :model-value="cfg.ims?.realm || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.realm = v; onInput() }" placeholder="自动生成" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="ICSI Ref" section="ims" :annotations="configAnnotations" />
            <el-input :model-value="cfg.ims?.icsi_ref || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.icsi_ref = v; onInput() }" placeholder="urn%3Aurn-7%3A3gpp-service.ims.icsi.mmtel" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="Contact 特性" section="ims" :annotations="configAnnotations" />
            <el-select :model-value="cfg.ims?.contact_features || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.contact_features = v; onInput() }" class="!w-full">
              <el-option v-for="opt in contactFeaturesOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="field">
            <ConfigFieldLabel label="Security-Client 格式" section="ims" :annotations="configAnnotations" />
            <el-select :model-value="cfg.ims?.security_client_format || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.security_client_format = v; onInput() }" class="!w-full">
              <el-option v-for="opt in securityClientFormatOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="field col-span-2">
            <ConfigFieldLabel label="固定 PANI" section="ims" :annotations="configAnnotations" />
            <div class="flex gap-2">
              <el-input :model-value="cfg.ims?.fixed_pani || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.fixed_pani = v; onInput() }" placeholder="空=自动生成 IEEE-802.11;i-wlan-node-id=000000000000" class="!flex-1" />
              <el-button @click="generateRandomPANI" class="!shrink-0">随机生成</el-button>
              <el-button @click="() => { ensureIms(); cfg.ims!.fixed_pani = ''; onInput() }" class="!shrink-0" plain>清空</el-button>
            </div>
          </div>
          <div class="field">
            <ConfigFieldLabel label="TCP Keepalive (秒)" section="ims" :annotations="configAnnotations" />
            <el-input-number :model-value="cfg.ims?.tcp_keepalive_seconds || 0" @update:model-value="(v: number | undefined) => { ensureIms(); cfg.ims!.tcp_keepalive_seconds = v || 0; onInput() }" :min="0" placeholder="30" class="!w-full" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="OPTIONS Ping (秒)" section="ims" :annotations="configAnnotations" />
            <el-input-number :model-value="cfg.ims?.options_ping_interval_seconds || 0" @update:model-value="(v: number | undefined) => { ensureIms(); cfg.ims!.options_ping_interval_seconds = v || 0; onInput() }" :min="0" placeholder="45" class="!w-full" />
          </div>
          <div class="field col-span-2">
            <ConfigFieldLabel label="Contact 参数顺序" section="ims" :annotations="configAnnotations" />
            <TagInputWithPresets v-model="contactParamOrderArr" :presets="contactParamOrderPresets" placeholder="手动输入或从预设选择" />
          </div>

          <!-- 注册策略 -->
          <div class="field col-span-2 section-divider">注册策略</div>
          <div class="field col-span-2">
            <ConfigFieldLabel label="临时失败状态码" section="ims" :annotations="configAnnotations" />
            <TagInputWithPresets v-model="tempStatusCodesArr" :presets="sipStatusCodesPresets" placeholder="手动输入或从预设选择" />
          </div>
          <div class="field col-span-2">
            <ConfigFieldLabel label="禁止状态码" section="ims" :annotations="configAnnotations" />
            <TagInputWithPresets v-model="forbiddenStatusCodesArr" :presets="sipStatusCodesPresets" placeholder="手动输入或从预设选择" />
          </div>
          <div class="field col-span-2">
            <ConfigFieldLabel label="首次拒绝回退状态码" section="ims" :annotations="configAnnotations" />
            <TagInputWithPresets v-model="fallbackStatusCodesArr" :presets="sipStatusCodesPresets" placeholder="手动输入或从预设选择" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="临时失败重试间隔 (秒)" section="ims" :annotations="configAnnotations" />
            <el-input-number :model-value="cfg.ims?.register_policy?.temporary_retry_seconds || 0" @update:model-value="(v: number | undefined) => { ensureRegPolicy(); cfg.ims!.register_policy!.temporary_retry_seconds = v || 0; onInput() }" :min="0" placeholder="0=默认" class="!w-full" />
          </div>

          <!-- 布尔开关组 -->
          <div class="field col-span-2 section-divider">布尔开关</div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="首次 REGISTER 带 PANI" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">include_pani</div></div>
            <el-switch :model-value="cfg.ims?.include_pani || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.include_pani = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="认证后 REGISTER 带 PANI" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">include_pani_authenticated</div></div>
            <el-switch :model-value="cfg.ims?.include_pani_authenticated || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.include_pani_authenticated = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="Require: sec-agree" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">require_sec_agree</div></div>
            <el-switch :model-value="cfg.ims?.require_sec_agree || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.require_sec_agree = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="Proxy-Require: sec-agree" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">proxy_require_sec_agree</div></div>
            <el-switch :model-value="cfg.ims?.proxy_require_sec_agree || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.proxy_require_sec_agree = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="首次 REGISTER 带空 AKA Authorization" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">use_plain_digest_placeholder</div></div>
            <el-switch :model-value="cfg.ims?.use_plain_digest_placeholder || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.use_plain_digest_placeholder = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="严格匹配 Security-Server" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">strict_security_server_offer</div></div>
            <el-switch :model-value="cfg.ims?.strict_security_server_offer || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.strict_security_server_offer = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="首次拒绝后回退重试" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">enable_initial_reject_fallback</div></div>
            <el-switch :model-value="cfg.ims?.enable_initial_reject_fallback || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.enable_initial_reject_fallback = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="省略 Route 头" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">omit_route — 认证后 REGISTER 不带 Route</div></div>
            <el-switch :model-value="cfg.ims?.omit_route || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.omit_route = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="首次 REGISTER 精简头" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">minimal_initial_headers</div></div>
            <el-switch :model-value="cfg.ims?.minimal_initial_headers || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.minimal_initial_headers = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="强制头中端口 5060" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">force_header_port_5060</div></div>
            <el-switch :model-value="cfg.ims?.force_header_port_5060 || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.force_header_port_5060 = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="Security-Client 省略 prot/mod" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">omit_initial_security_client_protocol</div></div>
            <el-switch :model-value="cfg.ims?.omit_initial_security_client_protocol || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.omit_initial_security_client_protocol = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="400 时探测 Security-Client" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">probe_initial_security_client_on_bad_request</div></div>
            <el-switch :model-value="cfg.ims?.probe_initial_security_client_on_bad_request || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.probe_initial_security_client_on_bad_request = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="Authorization 含 Connection-Keepalive" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">include_connection_keepalive_in_auth</div></div>
            <el-switch :model-value="cfg.ims?.include_connection_keepalive_in_auth || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.include_connection_keepalive_in_auth = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="Security-Client 含服务器参数" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">security_client_includes_server_params</div></div>
            <el-switch :model-value="cfg.ims?.security_client_includes_server_params || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.security_client_includes_server_params = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="回退时 Security-Client 含服务器参数" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">fallback_includes_server_params_in_sec_cl</div></div>
            <el-switch :model-value="cfg.ims?.fallback_includes_server_params_in_sec_cl || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.fallback_includes_server_params_in_sec_cl = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="Accept-Contact 头" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">include_accept_contact</div></div>
            <el-switch :model-value="cfg.ims?.include_accept_contact || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.include_accept_contact = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="P-Preferred-Identity 头" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">include_p_preferred_id</div></div>
            <el-switch :model-value="cfg.ims?.include_p_preferred_id || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.include_p_preferred_id = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="P-Visited-Network-ID 头" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">include_p_visited_network_id</div></div>
            <el-switch :model-value="cfg.ims?.include_p_visited_network_id || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.include_p_visited_network_id = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="P-Access-Network-Info 头" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">include_p_access_network_info</div></div>
            <el-switch :model-value="cfg.ims?.include_p_access_network_info || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.include_p_access_network_info = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="Route 头" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">include_route</div></div>
            <el-switch :model-value="cfg.ims?.include_route || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.include_route = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="Cellular-Network-Info 头" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">include_cellular_network</div></div>
            <el-switch :model-value="cfg.ims?.include_cellular_network || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.include_cellular_network = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="Security-Client 头" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">include_security_client</div></div>
            <el-switch :model-value="cfg.ims?.include_security_client || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.include_security_client = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="Require: sec-agree 头" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">include_require_sec_agree</div></div>
            <el-switch :model-value="cfg.ims?.include_require_sec_agree || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.include_require_sec_agree = Boolean(v); onInput() }" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="Contact URI 随机 UUID" section="ims" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">contact_user_random</div></div>
            <el-switch :model-value="cfg.ims?.contact_user_random || false" @update:model-value="(v: string | number | boolean) => { ensureIms(); cfg.ims!.contact_user_random = Boolean(v); onInput() }" />
          </div>
        </div>
      </div>
    </div>

    <!-- ════════ 语音会话 ════════ -->
    <div class="faq-card">
      <div class="faq-header" @click="toggle('voice')">
        <span class="faq-title">语音会话 (INVITE/MESSAGE)</span>
        <el-icon class="faq-arrow" :class="{ expanded: expanded.voice }" size="16"><ChevronDown20Regular /></el-icon>
      </div>
      <div v-show="expanded.voice" class="faq-body">
        <div class="form-grid">
          <div class="field col-span-2" style="color: var(--el-text-color-secondary); font-size: 0.85em;">
            非 REGISTER 请求 (INVITE/MESSAGE/UPDATE 等) 的 SIP 头配置。空值=使用默认或继承 REGISTER 配置。
          </div>
          <div class="field">
            <ConfigFieldLabel label="Supported" section="voice" :annotations="configAnnotations" />
            <el-input :model-value="cfg.ims?.voice_supported_header || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.voice_supported_header = v; onInput() }" placeholder="空=继承 REGISTER Supported" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="Allow" section="voice" :annotations="configAnnotations" />
            <el-input :model-value="cfg.ims?.voice_allow_header || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.voice_allow_header = v; onInput() }" placeholder="空=INVITE,ACK,CANCEL,BYE,..." />
          </div>
          <div class="field">
            <ConfigFieldLabel label="Accept-Contact" section="voice" :annotations="configAnnotations" />
            <el-input :model-value="cfg.ims?.voice_accept_contact || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.voice_accept_contact = v; onInput() }" placeholder="空=不发送 Accept-Contact" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="P-Preferred-Service" section="voice" :annotations="configAnnotations" />
            <el-input :model-value="cfg.ims?.voice_p_preferred_service || ''" @update:model-value="(v: string) => { ensureIms(); cfg.ims!.voice_p_preferred_service = v; onInput() }" placeholder="空=不发送 P-Preferred-Service" />
          </div>
        </div>
      </div>
    </div>

    <!-- ════════ E911 阶段 ════════ -->
    <div class="faq-card">
      <div class="faq-header" @click="toggle('e911')">
        <span class="faq-title">E911 紧急呼叫</span>
        <el-icon class="faq-arrow" :class="{ expanded: expanded.e911 }" size="16"><ChevronDown20Regular /></el-icon>
      </div>
      <div v-show="expanded.e911" class="faq-body">
        <div class="form-grid">
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="启用 E911" section="e911" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">美国运营商需要支持紧急呼叫定位</div></div>
            <el-switch :model-value="cfg.e911?.enabled || false" @update:model-value="(v: string | number | boolean) => { ensureE911(); cfg.e911!.enabled = Boolean(v); onInput() }" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="服务商" section="e911" :annotations="configAnnotations" />
            <el-input :model-value="cfg.e911?.provider || ''" @update:model-value="(v: string) => { ensureE911(); cfg.e911!.provider = v; onInput() }" placeholder="例如 intrado" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="Entitlement 端点" section="e911" :annotations="configAnnotations" />
            <el-input :model-value="cfg.e911?.entitlement_endpoint || ''" @update:model-value="(v: string) => { ensureE911(); cfg.e911!.entitlement_endpoint = v; onInput() }" placeholder="https://..." />
          </div>
          <div class="field col-span-2">
            <ConfigFieldLabel label="Websheet URL" section="e911" :annotations="configAnnotations" />
            <el-input :model-value="cfg.e911?.websheet || ''" @update:model-value="(v: string) => { ensureE911(); cfg.e911!.websheet = v; onInput() }" placeholder="E911 地址 websheet" />
          </div>
        </div>
      </div>
    </div>

    <!-- ════════ 设备身份 ════════ -->
    <div class="faq-card">
      <div class="faq-header" @click="toggle('device')">
        <span class="faq-title">设备身份</span>
        <el-icon class="faq-arrow" :class="{ expanded: expanded.device }" size="16"><ChevronDown20Regular /></el-icon>
      </div>
      <div v-show="expanded.device" class="faq-body">
        <div class="form-grid">
          <div class="field">
            <ConfigFieldLabel label="IMEI" section="device" :annotations="configAnnotations" />
            <el-input :model-value="cfg.device?.imei || ''" @update:model-value="(v: string) => { ensureDevice(); cfg.device!.imei = v; onInput() }" placeholder="15 位数字" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="Cell ID 模式" section="device" :annotations="configAnnotations" />
            <el-select :model-value="cfg.device?.ims_cell_id_mode || ''" @update:model-value="(v: string) => { ensureDevice(); cfg.device!.ims_cell_id_mode = v; onInput() }" class="!w-full">
              <el-option v-for="opt in cellIdModeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="field">
            <ConfigFieldLabel label="LTE TAC" section="device" :annotations="configAnnotations" />
            <el-input-number :model-value="cfg.device?.ims_tac || 0" @update:model-value="(v: number | undefined) => { ensureDevice(); cfg.device!.ims_tac = v || 0; onInput() }" :min="0" class="!w-full" />
          </div>
          <div class="field">
            <ConfigFieldLabel label="LTE Cell ID" section="device" :annotations="configAnnotations" />
            <el-input-number :model-value="cfg.device?.ims_cell_id || 0" @update:model-value="(v: number | undefined) => { ensureDevice(); cfg.device!.ims_cell_id = v || 0; onInput() }" :min="0" class="!w-full" />
          </div>
          <div class="field col-span-2">
            <ConfigFieldLabel label="REGISTER 模拟档案" section="device" :annotations="configAnnotations" />
            <el-input :model-value="cfg.device?.ims_register_profile || ''" @update:model-value="(v: string) => { ensureDevice(); cfg.device!.ims_register_profile = v; onInput() }" placeholder="例如 xiaomi_mi11" />
          </div>
          <div class="field col-span-2 form-switch-row">
            <div><ConfigFieldLabel label="禁止此运营商 VoWiFi" section="device" variant="switch" :annotations="configAnnotations" /><div class="switch-desc">blocked = true 时不发起 VoWiFi 注册</div></div>
            <el-switch :model-value="cfg.blocked || false" @update:model-value="(v: string | number | boolean) => { cfg.blocked = Boolean(v); onInput() }" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.config-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
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

.field.col-span-2 {
  grid-column: 1 / -1;
}

.section-divider {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding-top: 8px;
  border-top: 1px solid var(--border);
  margin-top: 4px;
}

.form-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.form-switch-row {
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

.field :deep(.el-input-number) {
  width: 100%;
}
</style>
