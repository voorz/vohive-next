/**
 * ProxyConfig 组件间共享类型
 */

import type { ProxyDevice, UpstreamProxyCountry, UpstreamProxyCountryRule, UpstreamProxyLookupResult } from './api'

// ── 扩展 lookup 结果（补充 country_code 用于国旗显示）──

export type ProxyLookupResultWithCode = UpstreamProxyLookupResult & {
  country_code?: string  // ISO 国家码，用于国旗组件
}

// ── 前置代理（带 UI 元数据）──

export type UpstreamProxyWithMeta = {
  id: string
  name: string
  addr: string
  username: string
  password: string
  enabled: boolean
  ruleCount: number
  lookup: ProxyLookupResultWithCode | null
  _testing?: boolean  // UI 态：正在测延迟
}

// ── 出站代理（带运行状态）──

export type OutboundInstanceWithStatus = {
  id: string
  name: string
  device_id: string
  enabled: boolean
  mode: 'socks5' | 'http'
  listen_addr: string
  listen_port: number
  auth_enabled: boolean
  username: string
  password: string
  running: boolean
  last_error: string
}

// ── 编辑对话框表单 ──

export type UpstreamProxyFormData = {
  name: string
  addr: string           // 链接串原始输入或 host:port
  username: string
  password: string
  enabled: boolean
}

export type { ProxyDevice, UpstreamProxyCountry, UpstreamProxyCountryRule, UpstreamProxyLookupResult }
