import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { AppError } from '../types/domain'
import type { CarrierListItem, CarrierDetail, CarrierProfile } from '../types/api'
import { carrierService } from '../services/carrier'

// 注入运营商基础字段作为模板身份标签
function injectCarrierIdentity(profile: CarrierProfile, carrierName: string, key: string, detail: CarrierDetail): CarrierProfile {
  profile.id = `${carrierName}_${detail.mcc}${detail.mnc}`
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

  // 从系统默认创建用户配置（后端 system_default 已含全部字段）
  function createFromSystemDefault() {
    if (!detail.value?.system_default) return
    const sys = JSON.parse(JSON.stringify(detail.value.system_default)) as CarrierProfile
    const carrierName = detail.value.name.toLowerCase().replace(/\s+/g, '')
    injectCarrierIdentity(sys, carrierName, selectedKey.value, detail.value)
    editingConfig.value = sys
    dirty.value = true
    previewTarget.value = 'user'
  }

  // 从 3GPP 标准模板创建（从后端获取 generic.json）
  async function createFromStandardTemplate() {
    if (!detail.value) return
    const result = await carrierService.getGenericProfile()
    if (!result.ok) return
    const tpl = JSON.parse(JSON.stringify(result.data)) as CarrierProfile
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
