import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { AppError } from '../types/domain'
import type { CarrierListItem, CarrierDetail, CarrierProfile } from '../types/api'
import { carrierService } from '../services/carrier'

// ═══════════════════════════════════════════════════════════
// Mock 数据 — 后端 API 就绪后替换为真实调用
// ═══════════════════════════════════════════════════════════

// 3GPP 标准完整模板（所有字段 + 默认值）
const standardTemplate: CarrierProfile = {
  ike: {
    addr: '', port: 500,
    proposals: ['aes256-sha256-prfsha256-modp2048', 'aes128-sha256-prfsha256-modp2048', 'aes256-sha1-prfsha1-modp2048', 'aes128-sha1-prfsha1-modp2048', 'aes256-sha256-prfsha256-modp1024', 'aes128-sha256-prfsha256-modp1024', 'aes256-sha1-prfsha1-modp1024'],
    esp_proposals: ['aes256-sha256', 'aes128-sha256', 'aes256-sha1', 'aes128-sha1'],
    dpd_interval: 0, nat_keepalive: 20, reauth_interval: 0,
    ip_stack: '', apn: 'ims', replay_window: 32,
    enable_esn: false, disable_eap_mac_validation: false
  },
  eap: {
    challenge_mode: 'standard', app_preference: 'auto', identity_source: 'derived',
    device_identity_enabled: false, device_model: ''
  },
  ims: {
    sec_agree_mode: 'auto', require_sec_agree: false, proxy_require_sec_agree: false,
    use_plain_digest_placeholder: false, initial_authorization: 'aka_empty_uri_first',
    include_pani: false, include_pani_authenticated: false, fixed_pani: '',
    user_agent: 'SimAdmin VoWiFi', supported_header: 'path,sec-agree,gruu', allow_header: '',
    contact_param_order: ['access_type', 'audio', 'smsip', 'icsi_ref', 'sip_instance'],
    contact_features: 'ims_features',
    security_client_mechanisms: [
      { alg: 'hmac-md5-96', ealg: 'des-ede3-cbc', prot: 'esp', mode: 'trans' },
      { alg: 'hmac-md5-96', ealg: 'aes-cbc', prot: 'esp', mode: 'trans' },
      { alg: 'hmac-md5-96', ealg: 'null', prot: 'esp', mode: 'trans' },
      { alg: 'hmac-sha-1-96', ealg: 'des-ede3-cbc', prot: 'esp', mode: 'trans' },
      { alg: 'hmac-sha-1-96', ealg: 'aes-cbc', prot: 'esp', mode: 'trans' },
      { alg: 'hmac-sha-1-96', ealg: 'null', prot: 'esp', mode: 'trans' }
    ],
    security_client_format: 'full_spaced', transport_mode: 'auto',
    strict_security_server_offer: false, enable_initial_reject_fallback: false,
    omit_route: false, minimal_initial_headers: false, force_header_port_5060: false,
    omit_initial_security_client_protocol: false, probe_initial_security_client_on_bad_request: false,
    include_connection_keepalive_in_auth: false, security_client_includes_server_params: false,
    fallback_includes_server_params_in_sec_cl: false, expires: 600,
    pcscf_addr: '', domain: '', realm: '', authorization_identity: 'imsi_home_domain',
    include_accept_contact: false, include_p_preferred_id: false,
    include_p_visited_network_id: false, include_p_access_network_info: false,
    include_route: false, include_cellular_network: false, include_security_client: false,
    include_require_sec_agree: false, contact_user_random: false,
    icsi_ref: 'urn%3Aurn-7%3A3gpp-service.ims.icsi.mmtel',
    voice_supported_header: '', voice_allow_header: '', voice_accept_contact: '', voice_p_preferred_service: '',
    ike_gateway_prefix_scores: [], tcp_keepalive_seconds: 30, options_ping_interval_seconds: 45,
    local_port: 5060,
    register_policy: {
      temporary_status_codes: [408, 480, 500, 502, 503, 504],
      forbidden_status_codes: [403],
      initial_reject_fallback_status_codes: [400, 403, 480, 500],
      temporary_retry_seconds: 0
    }
  },
  e911: { enabled: false, provider: '', websheet: '', entitlement_endpoint: '' },
  device: { imei: '', ims_tac: 0, ims_cell_id: 0, ims_cell_id_mode: 'qmi_first', ims_register_profile: '' },
  blocked: false
}

// 深度合并：用标准默认值补全缺失字段
function mergeWithStandard(profile: CarrierProfile): CarrierProfile {
  const std = JSON.parse(JSON.stringify(standardTemplate)) as CarrierProfile
  return {
    ...std, ...profile,
    ike: { ...std.ike, ...profile.ike },
    eap: { ...std.eap, ...profile.eap },
    ims: { ...std.ims, ...profile.ims },
    e911: { ...std.e911, ...profile.e911 },
    device: { ...std.device, ...profile.device }
  }
}

const mockSystemDefaults: Record<string, CarrierProfile> = {
  '234-10': {
    id: 'giffgaff_23410', mcc: '234', mnc: '10',
    ike: { proposals: ['aes256-sha512-prfsha512-modp2048'], esp_proposals: ['aes256-sha512'] },
    ims: {
      sec_agree_mode: 'auto', include_pani_authenticated: true,
      strict_security_server_offer: true, enable_initial_reject_fallback: true,
      contact_param_order: ['access_type', 'audio', 'smsip', 'icsi_ref', 'sip_instance'],
      security_client_mechanisms: [
        { alg: 'hmac-md5-96', ealg: 'des-ede3-cbc', prot: 'esp', mode: 'trans' },
        { alg: 'hmac-md5-96', ealg: 'aes-cbc', prot: 'esp', mode: 'trans' },
        { alg: 'hmac-md5-96', ealg: 'null', prot: 'esp', mode: 'trans' },
        { alg: 'hmac-sha-1-96', ealg: 'des-ede3-cbc', prot: 'esp', mode: 'trans' },
        { alg: 'hmac-sha-1-96', ealg: 'aes-cbc', prot: 'esp', mode: 'trans' },
        { alg: 'hmac-sha-1-96', ealg: 'null', prot: 'esp', mode: 'trans' }
      ],
      transport_mode: 'auto', contact_features: 'ims_features',
      initial_authorization: 'aka_empty_uri_first', security_client_format: 'full_spaced',
      supported_header: 'path,sec-agree,gruu', user_agent: 'SimAdmin VoWiFi',
      include_accept_contact: true, include_p_preferred_id: true,
      include_p_visited_network_id: true, include_p_access_network_info: true,
      include_route: true, include_cellular_network: true, include_security_client: true
    },
    device: { ims_tac: 28673, ims_cell_id: 12345678 },
    blocked: false
  },
  '234-15': {
    id: 'vodafone_23415', mcc: '234', mnc: '15',
    ike: { proposals: ['aes256-sha256-prfsha256-modp2048'], esp_proposals: ['aes256-sha256'] },
    ims: {
      sec_agree_mode: 'auto', include_pani_authenticated: true,
      contact_param_order: ['access_type', 'audio', 'smsip', 'icsi_ref', 'sip_instance'],
      transport_mode: 'auto', contact_features: 'ims_features',
      initial_authorization: 'aka_empty_uri_first', security_client_format: 'full_spaced',
      supported_header: 'path,sec-agree,gruu', user_agent: 'SimAdmin VoWiFi',
      include_accept_contact: true, include_p_preferred_id: true
    },
    blocked: false
  },
  '234-20': {
    id: 'three_23420', mcc: '234', mnc: '20',
    ike: { proposals: ['aes256-sha256-prfsha256-modp2048'], esp_proposals: ['aes256-sha256'] },
    ims: {
      sec_agree_mode: 'auto', include_pani_authenticated: true,
      contact_param_order: ['access_type', 'audio', 'smsip', 'icsi_ref', 'sip_instance'],
      transport_mode: 'auto', contact_features: 'ims_features',
      initial_authorization: 'aka_empty_uri_first', security_client_format: 'full_spaced',
      supported_header: 'path,sec-agree,gruu', user_agent: 'SimAdmin VoWiFi',
      include_accept_contact: true, include_p_preferred_id: true,
      include_p_visited_network_id: true
    },
    blocked: false
  },
  '310-260': {
    id: 'tmobile_310260', mcc: '310', mnc: '260',
    ike: { proposals: ['aes256-sha256-prfsha256-modp2048'], esp_proposals: ['aes256-sha256'] },
    eap: { challenge_mode: 'standard', app_preference: 'auto', identity_source: 'derived' },
    ims: {
      sec_agree_mode: 'auto', include_pani_authenticated: true,
      contact_param_order: ['access_type', 'audio', 'smsip', 'icsi_ref', 'sip_instance'],
      transport_mode: 'auto', contact_features: 'ims_features',
      initial_authorization: 'aka_empty_uri_first', security_client_format: 'full_spaced',
      supported_header: 'path,sec-agree,gruu', user_agent: 'SimAdmin VoWiFi',
      include_accept_contact: true, include_p_preferred_id: true
    },
    e911: { enabled: true, provider: 'intrado', entitlement_endpoint: 'https://tmobile.e911.com/vowifi' },
    blocked: false
  },
  '460-0': {
    id: 'chinamobile_4600', mcc: '460', mnc: '0',
    ike: { proposals: ['aes256-sha256-prfsha256-modp2048'], esp_proposals: ['aes256-sha256'] },
    ims: {
      sec_agree_mode: 'auto', contact_param_order: ['access_type', 'audio', 'smsip', 'icsi_ref', 'sip_instance'],
      transport_mode: 'auto', contact_features: 'ims_features',
      initial_authorization: 'aka_empty_uri_first', security_client_format: 'full_spaced',
      supported_header: 'path,sec-agree,gruu', user_agent: 'SimAdmin VoWiFi'
    },
    blocked: false
  }
}

const mockUserConfigs: Record<string, CarrierProfile> = {
  '234-10': {
    id: 'giffgaff_23410', mcc: '234', mnc: '10',
    ike: { proposals: ['aes256-sha512-prfsha512-modp2048'], esp_proposals: ['aes256-sha512'], nat_keepalive: 20 },
    ims: {
      sec_agree_mode: 'auto', include_pani_authenticated: true,
      strict_security_server_offer: true, enable_initial_reject_fallback: true,
      contact_param_order: ['access_type', 'audio', 'smsip', 'icsi_ref', 'sip_instance'],
      transport_mode: 'auto', contact_features: 'ims_features',
      initial_authorization: 'aka_empty_uri_first', security_client_format: 'full_spaced',
      supported_header: 'path,sec-agree,gruu', user_agent: 'SimAdmin VoWiFi',
      include_accept_contact: true, include_p_preferred_id: true,
      expires: 600
    },
    device: { ims_tac: 28673, ims_cell_id: 12345678 },
    blocked: false
  },
  '310-260': {
    id: 'tmobile_310260', mcc: '310', mnc: '260',
    ike: { proposals: ['aes256-sha256-prfsha256-modp2048'], esp_proposals: ['aes256-sha256'], dpd_interval: 600 },
    eap: { challenge_mode: 'standard', app_preference: 'auto', identity_source: 'derived' },
    ims: {
      sec_agree_mode: 'auto', include_pani_authenticated: true,
      contact_param_order: ['access_type', 'audio', 'smsip', 'icsi_ref', 'sip_instance'],
      transport_mode: 'auto', contact_features: 'ims_features',
      initial_authorization: 'aka_empty_uri_first', security_client_format: 'full_spaced',
      supported_header: 'path,sec-agree,gruu', user_agent: 'SimAdmin VoWiFi',
      include_accept_contact: true, include_p_preferred_id: true,
      expires: 600
    },
    e911: { enabled: true, provider: 'intrado', entitlement_endpoint: 'https://tmobile.e911.com/vowifi' },
    blocked: false
  }
}

const mockCarrierList: CarrierListItem[] = [
  { mcc: '234', mnc: '10', name: 'giffgaff UK', ike_addr: '', device_ims_tac: 28673, device_ims_cell_id: 12345678, has_user_config: true, active: false, has_system_default: true },
  { mcc: '234', mnc: '15', name: 'Vodafone UK', ike_addr: '', device_ims_tac: 0, device_ims_cell_id: 0, has_user_config: false, active: false, has_system_default: true },
  { mcc: '234', mnc: '20', name: 'Three UK', ike_addr: '', device_ims_tac: 0, device_ims_cell_id: 0, has_user_config: false, active: false, has_system_default: true },
  { mcc: '310', mnc: '260', name: 'T-Mobile US', ike_addr: '', device_ims_tac: 0, device_ims_cell_id: 0, has_user_config: true, active: true, has_system_default: true },
  { mcc: '460', mnc: '0', name: 'China Mobile', ike_addr: '', device_ims_tac: 0, device_ims_cell_id: 0, has_user_config: false, active: false, has_system_default: true }
]

// ═══════════════════════════════════════════════════════════

export const useCarrierStore = defineStore('carrier', () => {
  const carriers = ref<CarrierListItem[]>([])
  const selectedMcc = ref('')
  const selectedMnc = ref('')
  const detail = ref<CarrierDetail | null>(null)
  const loading = ref(false)
  const detailLoading = ref(false)
  const saving = ref(false)
  const error = ref<AppError | null>(null)

  // 预览目标: 'system' | 'user'
  const previewTarget = ref<'system' | 'user'>('user')

  // 编辑区模式: 'param' | 'code'
  const editMode = ref<'param' | 'code'>('param')

  // 当前正在编辑的用户配置（可变副本）
  const editingConfig = ref<CarrierProfile | null>(null)

  // 编辑区是否有未保存更改
  const dirty = ref(false)

  const selectedCarrier = computed(() =>
    carriers.value.find(c => c.mcc === selectedMcc.value && c.mnc === selectedMnc.value) || null
  )

  const plmnKey = computed(() => `${selectedMcc.value}-${selectedMnc.value}`)

  // 当前预览的配置
  const previewConfig = computed<CarrierProfile | null>(() => {
    if (!detail.value) return null
    if (previewTarget.value === 'system') return detail.value.system_default
    return editingConfig.value || detail.value.user_config
  })

  async function fetchCarriers() {
    loading.value = true
    error.value = null

    // TODO: 后端就绪后替换为真实 API
    // const result = await carrierService.list()
    // if (result.ok) { carriers.value = result.data } else { error.value = result.error }

    // Mock
    carriers.value = [...mockCarrierList]
    if (carriers.value.length > 0 && !selectedMcc.value) {
      await selectCarrier(carriers.value[0].mcc, carriers.value[0].mnc)
    }
    loading.value = false
  }

  async function selectCarrier(mcc: string, mnc: string) {
    if (selectedMcc.value === mcc && selectedMnc.value === mnc && detail.value) return
    selectedMcc.value = mcc
    selectedMnc.value = mnc
    previewTarget.value = 'user'
    dirty.value = false
    await fetchDetail()
  }

  async function fetchDetail() {
    if (!selectedMcc.value || !selectedMnc.value) return
    detailLoading.value = true

    // TODO: 后端就绪后替换为真实 API
    // const result = await carrierService.get(selectedMcc.value, selectedMnc.value)
    // if (result.ok) { detail.value = result.data } else { error.value = result.error }

    // Mock
    const key = `${selectedMcc.value}-${selectedMnc.value}`
    const item = carriers.value.find(c => c.mcc === selectedMcc.value && c.mnc === selectedMnc.value)
    if (item) {
      detail.value = {
        mcc: item.mcc, mnc: item.mnc, name: item.name,
        ike_addr: item.ike_addr,
        device_ims_tac: item.device_ims_tac,
        device_ims_cell_id: item.device_ims_cell_id,
        system_default: mockSystemDefaults[key] || null,
        user_config: item.has_user_config ? (mockUserConfigs[key] || null) : null,
        active: item.active
      }
      // 初始化编辑副本
      if (detail.value.user_config) {
        editingConfig.value = JSON.parse(JSON.stringify(detail.value.user_config))
      } else {
        editingConfig.value = null
      }
    }
    detailLoading.value = false
  }

  function setPreviewTarget(target: 'system' | 'user') {
    previewTarget.value = target
  }

  function setEditMode(mode: 'param' | 'code') {
    editMode.value = mode
  }

  // 从系统默认创建用户配置（自动补全缺失字段）
  function createFromSystemDefault() {
    if (!detail.value?.system_default) return
    const sys = JSON.parse(JSON.stringify(detail.value.system_default)) as CarrierProfile
    // 合并标准默认值补全缺失字段
    const merged = mergeWithStandard(sys)
    // id 格式: {name}_{mcc}{mnc}
    const carrierName = detail.value.name.toLowerCase().replace(/\s+/g, '')
    merged.id = `${carrierName}_${selectedMcc.value}${selectedMnc.value}`
    merged.mcc = selectedMcc.value
    merged.mnc = selectedMnc.value
    editingConfig.value = merged
    dirty.value = true
    previewTarget.value = 'user'
  }

  // 从 3GPP 标准模板创建用户配置（全部标准默认值）
  function createFromStandardTemplate() {
    if (!detail.value) return
    const tpl = JSON.parse(JSON.stringify(standardTemplate)) as CarrierProfile
    const carrierName = detail.value.name.toLowerCase().replace(/\s+/g, '')
    tpl.id = `${carrierName}_${selectedMcc.value}${selectedMnc.value}`
    tpl.mcc = selectedMcc.value
    tpl.mnc = selectedMnc.value
    editingConfig.value = tpl
    dirty.value = true
    previewTarget.value = 'user'
  }

  // 标记编辑内容已变更
  function markDirty() {
    dirty.value = true
  }

  // 从代码模式 JSON 同步回编辑配置
  function syncFromJson(json: CarrierProfile) {
    editingConfig.value = json
    dirty.value = true
  }

  async function saveUserConfig() {
    if (!editingConfig.value || !detail.value) return false
    saving.value = true

    // TODO: 后端就绪后替换为真实 API
    // const result = await carrierService.save(
    //   selectedMcc.value, selectedMnc.value,
    //   {
    //     name: detail.value.name,
    //     ike_addr: detail.value.ike_addr,
    //     device_ims_tac: detail.value.device_ims_tac,
    //     device_ims_cell_id: detail.value.device_ims_cell_id,
    //     config: editingConfig.value,
    //     active: detail.value.active
    //   }
    // )

    // Mock — 模拟保存延迟
    await new Promise(r => setTimeout(r, 300))
    detail.value.user_config = JSON.parse(JSON.stringify(editingConfig.value))
    dirty.value = false
    saving.value = false

    // 更新列表项状态
    const item = carriers.value.find(c => c.mcc === selectedMcc.value && c.mnc === selectedMnc.value)
    if (item) item.has_user_config = true

    return true
  }

  async function activateUserConfig() {
    if (!detail.value) return false
    // TODO: await carrierService.activate(selectedMcc.value, selectedMnc.value)
    await new Promise(r => setTimeout(r, 200))
    detail.value.active = true
    const item = carriers.value.find(c => c.mcc === selectedMcc.value && c.mnc === selectedMnc.value)
    if (item) item.active = true
    return true
  }

  async function deactivateUserConfig() {
    if (!detail.value) return false
    // TODO: await carrierService.deactivate(selectedMcc.value, selectedMnc.value)
    await new Promise(r => setTimeout(r, 200))
    detail.value.active = false
    const item = carriers.value.find(c => c.mcc === selectedMcc.value && c.mnc === selectedMnc.value)
    if (item) item.active = false
    return true
  }

  async function deleteUserConfig() {
    if (!detail.value) return false
    // TODO: await carrierService.deleteConfig(selectedMcc.value, selectedMnc.value)
    await new Promise(r => setTimeout(r, 200))
    detail.value.user_config = null
    detail.value.active = false
    editingConfig.value = null
    dirty.value = false
    const item = carriers.value.find(c => c.mcc === selectedMcc.value && c.mnc === selectedMnc.value)
    if (item) {
      item.has_user_config = false
      item.active = false
    }
    return true
  }

  async function addCarrier(name: string, mcc: string, mnc: string, ike_addr = '', device_ims_tac = 0, device_ims_cell_id = 0) {
    // TODO: await carrierService.add({ name, mcc, mnc, ike_addr, device_ims_tac, device_ims_cell_id })
    await new Promise(r => setTimeout(r, 200))
    carriers.value.push({
      mcc, mnc, name, ike_addr, device_ims_tac, device_ims_cell_id,
      has_user_config: false, active: false, has_system_default: false
    })
    return true
  }

  async function removeCarrier(mcc: string, mnc: string) {
    // TODO: await carrierService.remove(mcc, mnc)
    await new Promise(r => setTimeout(r, 200))
    const idx = carriers.value.findIndex(c => c.mcc === mcc && c.mnc === mnc)
    if (idx >= 0) carriers.value.splice(idx, 1)
    if (selectedMcc.value === mcc && selectedMnc.value === mnc) {
      if (carriers.value.length > 0) {
        await selectCarrier(carriers.value[0].mcc, carriers.value[0].mnc)
      } else {
        selectedMcc.value = ''
        selectedMnc.value = ''
        detail.value = null
      }
    }
    return true
  }

  return {
    // state
    carriers,
    selectedMcc,
    selectedMnc,
    selectedCarrier,
    plmnKey,
    detail,
    loading,
    detailLoading,
    saving,
    error,
    previewTarget,
    editMode,
    editingConfig,
    dirty,
    previewConfig,
    // actions
    fetchCarriers,
    selectCarrier,
    fetchDetail,
    setPreviewTarget,
    setEditMode,
    createFromSystemDefault,
    createFromStandardTemplate,
    markDirty,
    syncFromJson,
    saveUserConfig,
    activateUserConfig,
    deactivateUserConfig,
    deleteUserConfig,
    addCarrier,
    removeCarrier
  }
})
