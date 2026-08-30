import { ref, watch, type Ref } from 'vue'

// 四开关镜像（不含 ip/apn——那两项在网络设置卡片编辑）
export type PolicyMirror = {
  network_enabled: boolean
  vowifi_enabled: boolean
  airplane_enabled: boolean
  volte_enabled: boolean
}

export type ToggleResult = { ok: boolean }

export type CardPolicyExecutors = {
  applyNetwork: (enabled: boolean, next: PolicyMirror) => Promise<ToggleResult>
  applyVoWiFi: (enabled: boolean, next: PolicyMirror) => Promise<ToggleResult>
  applyAirplane: (enabled: boolean, next: PolicyMirror) => Promise<ToggleResult>
  applyVolte?: (enabled: boolean, next: PolicyMirror) => Promise<ToggleResult>
  onChanged?: () => void
}

// 互斥规则（用户指定）：
// VoWiFi 开启  ⇒ 关蜂窝数据、关 VoLTE           （禁用飞行模式开关，不处理其状态）
// VoWiFi 关闭  ⇒ 无操作                          （恢复飞行模式可点击）
// 飞行模式开启  ⇒ 关蜂窝数据、关 VoLTE
// 飞行模式关闭  ⇒ 无操作（卡正式驻网由后端处理）
// 蜂窝数据开启  ⇒ 关 VoWiFi、关飞行模式          （不处理 VoLTE，可并存）
// 蜂窝数据关闭  ⇒ 无操作
// VoLTE 开启    ⇒ 关 VoWiFi、关飞行模式（如开着）
// VoLTE 关闭    ⇒ 无操作
// 关任一项 ⇒ 不动其它项
function nextMirror(
  cur: PolicyMirror,
  field: keyof PolicyMirror,
  val: boolean
): PolicyMirror {
  if (!val) {
    // 关闭任一项，不动其它
    return { ...cur, [field]: false }
  }
  // 开启时的互斥
  switch (field) {
    case 'vowifi_enabled':
      return { ...cur, vowifi_enabled: true, network_enabled: false, volte_enabled: false }
    case 'airplane_enabled':
      return { ...cur, airplane_enabled: true, network_enabled: false, volte_enabled: false }
    case 'network_enabled':
      return { ...cur, network_enabled: true, vowifi_enabled: false, airplane_enabled: false }
    case 'volte_enabled':
      return { ...cur, volte_enabled: true, vowifi_enabled: false, airplane_enabled: false }
    default:
      return { ...cur, [field]: true }
  }
}

export function useCardPolicyToggles(
  source: Ref<PolicyMirror | null>,
  executors: CardPolicyExecutors
) {
  const local = ref<PolicyMirror>({
    network_enabled: false,
    vowifi_enabled: false,
    airplane_enabled: false,
    volte_enabled: false,
  })

  const networkPending = ref(false)
  const networkFailed = ref(false)
  const vowifiPending = ref(false)
  const vowifiFailed = ref(false)
  const airplanePending = ref(false)
  const airplaneFailed = ref(false)
  const voltePending = ref(false)
  const volteFailed = ref(false)

  // 上游变化原地同步各字段（不整体替换对象，避免 el-switch 在 element-plus 2.13 崩溃）
  watch(
    source,
    (p) => {
      if (!p) return
      local.value.network_enabled = p.network_enabled
      local.value.vowifi_enabled = p.vowifi_enabled
      local.value.airplane_enabled = p.airplane_enabled
      local.value.volte_enabled = p.volte_enabled
      networkFailed.value = false
      vowifiFailed.value = false
      airplaneFailed.value = false
      volteFailed.value = false
    },
    { immediate: true }
  )

  async function onNetworkToggle(rawVal: string | number | boolean) {
    const val = rawVal as boolean
    networkPending.value = true
    networkFailed.value = false
    const next = nextMirror(local.value, 'network_enabled', val)
    // 提前同步互斥字段到 UI（让被关的开关立即变灰）
    local.value.vowifi_enabled = next.vowifi_enabled
    local.value.airplane_enabled = next.airplane_enabled
    local.value.volte_enabled = next.volte_enabled
    const result = await executors.applyNetwork(val, next)
    networkPending.value = false
    if (!result.ok) {
      local.value.network_enabled = !val
      networkFailed.value = true
      return
    }
    local.value.network_enabled = next.network_enabled
    local.value.vowifi_enabled = next.vowifi_enabled
    local.value.airplane_enabled = next.airplane_enabled
    local.value.volte_enabled = next.volte_enabled
    executors.onChanged?.()
  }

  async function onVoWiFiToggle(rawVal: string | number | boolean) {
    const val = rawVal as boolean
    vowifiPending.value = true
    vowifiFailed.value = false
    const next = nextMirror(local.value, 'vowifi_enabled', val)
    // 提前同步互斥字段到 UI
    local.value.network_enabled = next.network_enabled
    local.value.volte_enabled = next.volte_enabled
    const result = await executors.applyVoWiFi(val, next)
    vowifiPending.value = false
    if (!result.ok) {
      local.value.vowifi_enabled = !val
      vowifiFailed.value = true
      return
    }
    local.value.network_enabled = next.network_enabled
    local.value.vowifi_enabled = next.vowifi_enabled
    local.value.airplane_enabled = next.airplane_enabled
    local.value.volte_enabled = next.volte_enabled
    executors.onChanged?.()
  }

  async function onAirplaneToggle(rawVal: string | number | boolean) {
    const val = rawVal as boolean
    airplanePending.value = true
    airplaneFailed.value = false
    const next = nextMirror(local.value, 'airplane_enabled', val)
    // 提前同步互斥字段到 UI
    local.value.network_enabled = next.network_enabled
    local.value.volte_enabled = next.volte_enabled
    const result = await executors.applyAirplane(val, next)
    airplanePending.value = false
    if (!result.ok) {
      local.value.airplane_enabled = !val
      airplaneFailed.value = true
      return
    }
    local.value.network_enabled = next.network_enabled
    local.value.vowifi_enabled = next.vowifi_enabled
    local.value.airplane_enabled = next.airplane_enabled
    local.value.volte_enabled = next.volte_enabled
    executors.onChanged?.()
  }

  async function onVolteToggle(rawVal: string | number | boolean) {
    const val = rawVal as boolean
    if (!executors.applyVolte) return
    voltePending.value = true
    volteFailed.value = false
    const next = nextMirror(local.value, 'volte_enabled', val)
    const result = await executors.applyVolte(val, next)
    voltePending.value = false
    if (!result.ok) {
      local.value.volte_enabled = !val
      volteFailed.value = true
      return
    }
    local.value.network_enabled = next.network_enabled
    local.value.vowifi_enabled = next.vowifi_enabled
    local.value.airplane_enabled = next.airplane_enabled
    local.value.volte_enabled = next.volte_enabled
    executors.onChanged?.()
  }

  return {
    local,
    networkPending,
    networkFailed,
    vowifiPending,
    vowifiFailed,
    airplanePending,
    airplaneFailed,
    voltePending,
    volteFailed,
    onNetworkToggle,
    onVoWiFiToggle,
    onAirplaneToggle,
    onVolteToggle,
  }
}
