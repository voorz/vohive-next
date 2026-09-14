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
  applyNetwork: (enabled: boolean, next: PolicyMirror, prev: PolicyMirror) => Promise<ToggleResult>
  applyVoWiFi: (enabled: boolean, next: PolicyMirror, prev: PolicyMirror) => Promise<ToggleResult>
  applyAirplane: (enabled: boolean, next: PolicyMirror, prev: PolicyMirror) => Promise<ToggleResult>
  applyVolte?: (enabled: boolean, next: PolicyMirror, prev: PolicyMirror) => Promise<ToggleResult>
  onChanged?: () => void
}

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
      // 开 VoWiFi 时强制 airplane=on：VoWiFi 接管射频等效飞行模式；
      // 关 VoWiFi 时保持飞行意图以防基站风控（后端 NormalizeCardPolicy 同步）。
      return { ...cur, vowifi_enabled: true, airplane_enabled: true, network_enabled: false, volte_enabled: false }
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
    // 保存执行前快照（供执行器判断互斥项之前是否开着）
    const prev = { ...local.value }
    // 提前同步互斥字段到 UI（让被关的开关立即变灰）
    local.value.vowifi_enabled = next.vowifi_enabled
    local.value.airplane_enabled = next.airplane_enabled
    local.value.volte_enabled = next.volte_enabled
    const result = await executors.applyNetwork(val, next, prev)
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
    // 保存执行前快照
    const prev = { ...local.value }
    // 提前同步互斥字段到 UI（含 airplane_enabled：开 VoWiFi 时强制 airplane=on）
    local.value.network_enabled = next.network_enabled
    local.value.airplane_enabled = next.airplane_enabled
    local.value.volte_enabled = next.volte_enabled
    const result = await executors.applyVoWiFi(val, next, prev)
    vowifiPending.value = false
    if (!result.ok) {
      local.value.vowifi_enabled = !val
      local.value.airplane_enabled = prev.airplane_enabled
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
    // 保存执行前快照
    const prev = { ...local.value }
    // 提前同步互斥字段到 UI
    local.value.network_enabled = next.network_enabled
    local.value.volte_enabled = next.volte_enabled
    const result = await executors.applyAirplane(val, next, prev)
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
    // 保存执行前快照
    const prev = { ...local.value }
    // 提前同步互斥字段到 UI
    local.value.vowifi_enabled = next.vowifi_enabled
    local.value.airplane_enabled = next.airplane_enabled
    const result = await executors.applyVolte(val, next, prev)
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
