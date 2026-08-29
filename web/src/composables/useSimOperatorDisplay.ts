/**
 * SIM 卡原始运营商名称显示 — 三层 fallback 逻辑
 *
 * 从项目初始化阶段的旧 Devices.vue 恢复，作为独立 composable 供多组件复用。
 *
 * Fallback 优先级：
 *   1. SIM EF_SPN (native_spn)  — SIM 卡服务提供商名称
 *   2. SIM PNN/OPL 记录          — EF_PNN + EF_OPL 网络名称记录
 *   3. mcc-mnc-table.json 码表   — 按 MCC+MNC 查询运营商名称
 *
 * 数据源：
 *   - native_spn / pnn / opl: 后端 modem 状态字段
 *   - mcc-mnc-table.json: musalbas/mcc-mnc-table GitHub 仓库，localStorage 缓存 7 天
 *
 * 使用方式：
 *   const { simOperatorDisplay, mccMncReady } = useSimOperatorDisplay(deviceRef)
 */

import { ref, computed } from 'vue'
import type { Ref } from 'vue'
import type { DeviceOverviewItem, ModemStatus, PNNRecord } from '../types/api'
import {
  getMccMncIndex,
  isoToFlagEmoji,
  type MccMncRow
} from '../utils/mcc-mnc'

// ─── 工具函数 ───

function normalizeSPN(v: unknown): string {
  return String(v ?? '').trim()
}

function nativeMccMnc(modem: ModemStatus | undefined): string {
  const mcc = String(modem?.native_mcc ?? '').trim()
  const mnc = String(modem?.native_mnc ?? '').trim()
  return mcc && mnc ? `${mcc}${mnc}` : ''
}

// ─── PNN/OPL 层 ───

function pnnDisplayName(record: PNNRecord | undefined): string {
  return normalizeSPN(record?.full_name) || normalizeSPN(record?.short_name)
}

function firstPNNName(records: PNNRecord[] | undefined): string {
  if (!Array.isArray(records)) return ''
  for (const r of records) {
    const name = pnnDisplayName(r)
    if (name) return name
  }
  return ''
}

function oplMatchesNativePLMN(oplPLMN: string | undefined, nativePLMN: string): boolean {
  const pattern = String(oplPLMN ?? '').trim().toLowerCase()
  if (!pattern || !nativePLMN) return false
  if (pattern === nativePLMN) return true
  // 无通配符：前缀匹配（短 PLMN 编码兼容）
  if (!pattern.includes('x')) return pattern.length < nativePLMN.length && nativePLMN.startsWith(pattern)
  // 通配符匹配（'x' = 任意数字）
  if (pattern.length !== nativePLMN.length) return false
  for (let i = 0; i < pattern.length; i++) {
    if (pattern[i] !== 'x' && pattern[i] !== nativePLMN[i]) return false
  }
  return true
}

function pnnNameFromOPL(modem: ModemStatus | undefined): string {
  const nativePLMN = nativeMccMnc(modem)
  if (!nativePLMN || !Array.isArray(modem?.opl) || !Array.isArray(modem?.pnn)) return ''
  for (const opl of modem.opl) {
    if (!oplMatchesNativePLMN(opl?.plmn, nativePLMN)) continue
    const pnnRecord = Number(opl?.pnn_record ?? 0)
    if (!pnnRecord) continue
    const name = pnnDisplayName(modem.pnn.find((record) => record.record === pnnRecord))
    if (name) return name
  }
  return ''
}

// ─── mcc-mnc 码表层 ───

function flagForMccMnc(index: Map<string, MccMncRow> | null, code: string): string {
  const row = index?.get(code)
  return row ? isoToFlagEmoji(row.iso) : ''
}

function formatNamedOperator(index: Map<string, MccMncRow> | null, name: string, code: string): string {
  const flag = flagForMccMnc(index, code)
  if (!code) return flag ? `${flag} ${name}` : name
  return `${flag ? flag + ' ' : ''}${name} (${code})`
}

function formatMccMncOperator(index: Map<string, MccMncRow> | null, code: string): string {
  if (!index || !code) return code
  const row = index.get(code)
  if (!row) return code
  const name = normalizeSPN(row.network) || normalizeSPN(row.country)
  return name ? formatNamedOperator(index, name, code) : code
}

// ─── Composable ───

type DeviceSource = Ref<DeviceOverviewItem | null> | (() => DeviceOverviewItem | null)

/**
 * 计算 SIM 卡原始运营商显示名称
 *
 * @param source 响应式设备引用（Ref）或 getter 函数
 * @returns simOperatorDisplay: 计算后的显示字符串；mccMncReady: 码表是否已加载
 */
export function useSimOperatorDisplay(source: DeviceSource) {
  const mccMncIndex = ref<Map<string, MccMncRow> | null>(null)

  // 懒加载 mcc-mnc 码表
  getMccMncIndex().then((index) => {
    mccMncIndex.value = index
  }).catch(() => {})

  const mccMncReady = computed(() => mccMncIndex.value !== null)

  const simOperatorDisplay = computed(() => {
    const d = typeof source === 'function' ? source() : source.value
    if (!d) return '--'
    const modem = d?.modem
    const spn = normalizeSPN(modem?.native_spn)
    const pnn = pnnNameFromOPL(modem) || firstPNNName(modem?.pnn)
    const mccmnc = nativeMccMnc(modem)
    const index = mccMncIndex.value
    if (spn) return formatNamedOperator(index, spn, mccmnc)
    if (pnn) return formatNamedOperator(index, pnn, mccmnc)
    return mccmnc ? formatMccMncOperator(index, mccmnc) : '--'
  })

  return { simOperatorDisplay, mccMncReady }
}
