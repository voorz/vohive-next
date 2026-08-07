import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { AppError } from '../types/domain'
import type { CarrierListItem, CarrierDetail, CarrierProfile } from '../types/api'
import { carrierService } from '../services/carrier'

// 3GPP 标准完整模板（所有字段 + 默认值），用于"从标准模板创建"
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

// 注入运营商基础字段作为模板身份标签
function injectCarrierIdentity(profile: CarrierProfile, carrierName: string, key: string, detail: CarrierDetail): CarrierProfile {
  profile.id = `${carrierName}_${key.replace(/-/g, '')}`
  profile.name = detail.name
  profile.mcc = detail.mcc
  profile.mnc = detail.mnc
  if (detail.ike_addr) {
    if (!profile.ike) profile.ike = {}
    profile.ike.addr = detail.ike_addr
  }
  if (detail.device_ims_tac) {
    if (!profile.device) profile.device = {}
    profile.device.ims_tac = detail.device_ims_tac
  }
  if (detail.device_ims_cell_id) {
    if (!profile.device) profile.device = {}
    profile.device.ims_cell_id = detail.device_ims_cell_id
  }
  return {
    id: profile.id,
    name: profile.name,
    mcc: profile.mcc,
    mnc: profile.mnc,
    ike: profile.ike,
    eap: profile.eap,
    ims: profile.ims,
    e911: profile.e911,
    device: profile.device,
    blocked: profile.blocked
  }
}

export const useCarrierStore = defineStore('carrier', () => {
  const carriers = ref<CarrierListItem[]>([])
  const selectedKey = ref('')
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
    carriers.value.find(c => c.key === selectedKey.value) || null
  )

  // 当前预览的配置
  const previewConfig = computed<CarrierProfile | null>(() => {
    if (!detail.value) return null
    if (previewTarget.value === 'system') return detail.value.system_default
    return editingConfig.value || detail.value.user_config
  })

  async function fetchCarriers() {
    loading.value = true
    error.value = null

    const result = await carrierService.list()
    if (result.ok) {
      carriers.value = result.data
      if (carriers.value.length > 0 && !selectedKey.value) {
        await selectCarrier(carriers.value[0].key)
      }
    } else {
      error.value = result.error
    }
    loading.value = false
  }

  async function selectCarrier(key: string) {
    if (selectedKey.value === key && detail.value) return
    selectedKey.value = key
    previewTarget.value = 'user'
    dirty.value = false
    await fetchDetail()
  }

  async function fetchDetail() {
    if (!selectedKey.value) return
    detailLoading.value = true

    const result = await carrierService.get(selectedKey.value)
    if (result.ok) {
      detail.value = result.data
      if (detail.value.user_config) {
        editingConfig.value = JSON.parse(JSON.stringify(detail.value.user_config))
      } else {
        editingConfig.value = null
      }
    } else {
      error.value = result.error
    }
    detailLoading.value = false
  }

  function setPreviewTarget(target: 'system' | 'user') {
    previewTarget.value = target
  }

  function setEditMode(mode: 'param' | 'code') {
    editMode.value = mode
  }

  // 从系统默认创建用户配置
  function createFromSystemDefault() {
    if (!detail.value?.system_default) return
    const sys = JSON.parse(JSON.stringify(detail.value.system_default)) as CarrierProfile
    const merged = mergeWithStandard(sys)
    const carrierName = detail.value.name.toLowerCase().replace(/\s+/g, '')
    injectCarrierIdentity(merged, carrierName, selectedKey.value, detail.value)
    editingConfig.value = merged
    dirty.value = true
    previewTarget.value = 'user'
  }

  // 从 3GPP 标准模板创建用户配置
  function createFromStandardTemplate() {
    if (!detail.value) return
    const tpl = JSON.parse(JSON.stringify(standardTemplate)) as CarrierProfile
    const carrierName = detail.value.name.toLowerCase().replace(/\s+/g, '')
    injectCarrierIdentity(tpl, carrierName, selectedKey.value, detail.value)
    editingConfig.value = tpl
    dirty.value = true
    previewTarget.value = 'user'
  }

  function markDirty() {
    dirty.value = true
  }

  function syncFromJson(json: CarrierProfile) {
    editingConfig.value = json
    dirty.value = true
  }

  async function saveUserConfig() {
    if (!editingConfig.value || !detail.value) return false
    saving.value = true

    const result = await carrierService.save(
      selectedKey.value,
      {
        name: detail.value.name,
        ike_addr: detail.value.ike_addr,
        device_ims_tac: detail.value.device_ims_tac,
        device_ims_cell_id: detail.value.device_ims_cell_id,
        config: editingConfig.value,
        active: detail.value.active
      }
    )

    if (result.ok) {
      detail.value.user_config = JSON.parse(JSON.stringify(editingConfig.value))
      dirty.value = false
      const item = carriers.value.find(c => c.key === selectedKey.value)
      if (item) item.has_user_config = true
      saving.value = false
      return true
    }

    error.value = result.error
    saving.value = false
    return false
  }

  async function activateUserConfig() {
    if (!detail.value) return false
    const result = await carrierService.activate(selectedKey.value)
    if (result.ok) {
      detail.value.active = true
      const item = carriers.value.find(c => c.key === selectedKey.value)
      if (item) item.active = true
      return true
    }
    error.value = result.error
    return false
  }

  async function deactivateUserConfig() {
    if (!detail.value) return false
    const result = await carrierService.deactivate(selectedKey.value)
    if (result.ok) {
      detail.value.active = false
      const item = carriers.value.find(c => c.key === selectedKey.value)
      if (item) item.active = false
      return true
    }
    error.value = result.error
    return false
  }

  async function deleteUserConfig() {
    if (!detail.value) return false
    const result = await carrierService.deleteConfig(selectedKey.value)
    if (result.ok) {
      detail.value.user_config = null
      detail.value.active = false
      editingConfig.value = null
      dirty.value = false
      const item = carriers.value.find(c => c.key === selectedKey.value)
      if (item) {
        item.has_user_config = false
        item.active = false
      }
      return true
    }
    error.value = result.error
    return false
  }

  /** 从 plmn-index 批量添加运营商 */
  async function addCarriersFromIndex(plmns: string[]) {
    const result = await carrierService.addCarriersFromIndex(plmns)
    if (result.ok) {
      await fetchCarriers()
      return true
    }
    error.value = result.error
    return false
  }

async function removeCarrier(key: string) {
  // 新架构：只从 carrier_visible 删除
  const visibleResult = await carrierService.removeCarrierVisible(key)
  if (visibleResult.ok) {
      const idx = carriers.value.findIndex(c => c.key === key)
      if (idx >= 0) carriers.value.splice(idx, 1)
      if (selectedKey.value === key) {
        if (carriers.value.length > 0) {
          await selectCarrier(carriers.value[0].key)
        } else {
          selectedKey.value = ''
          detail.value = null
        }
      }
      return true
    }
    error.value = visibleResult.error
    return false
  }

  return {
    // state
    carriers,
    selectedKey,
    selectedCarrier,
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
    addCarriersFromIndex,
    removeCarrier
  }
})
