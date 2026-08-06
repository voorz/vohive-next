/**
 * 前置代理链接串解析工具
 * 仅支持 SOCKS5 协议（VoWiFi 要求 UDP Associate）
 * 支持 socks5:// 前缀 / 裸地址 / IPv6 格式
 */

export type ProxyProtocol = 'socks5'

export type ParsedProxy = {
  protocol: ProxyProtocol
  host: string
  port: number
  username?: string
  password?: string
  raw: string
  valid: boolean
  error?: string
}

/**
 * 解析单行代理链接串
 *
 * 支持的格式：
 *   socks5://host:port
 *   socks5://user:pass@host:port
 *   host:port              （默认 socks5）
 *   user:pass@host:port    （默认 socks5）
 *   host:port:user:pass    （冒号分隔四段，无 @）
 *   host:port:user         （仅用户名）
 *   [ipv6]:port            （IPv6 格式）
 *   user:pass@[ipv6]:port
 */
export function parseProxyUrl(input: string): ParsedProxy {
  const raw = input.trim()
  if (!raw) {
    return { protocol: 'socks5', host: '', port: 0, raw, valid: false, error: '空行' }
  }

  // 带 socks5:// 前缀
  const protoMatch = raw.match(/^socks5:\/\/(.+)$/i)
  if (protoMatch) {
    const rest = protoMatch[1]
    const result = parseAuthAndHost(rest)
    if (!result) {
      return { protocol: 'socks5', host: '', port: 0, raw, valid: false, error: '格式错误' }
    }
    return validatePort({ ...result, protocol: 'socks5', raw })
  }

  // 拒绝 http:// 前缀（前置代理仅支持 SOCKS5）
  if (/^https?:\/\//i.test(raw)) {
    return { protocol: 'socks5', host: '', port: 0, raw, valid: false, error: '仅支持 SOCKS5' }
  }

  // 无协议前缀：默认 socks5
  const result = parseAuthAndHost(raw)
  if (!result) {
    return { protocol: 'socks5', host: '', port: 0, raw, valid: false, error: '格式错误' }
  }
  return validatePort({ ...result, protocol: 'socks5', raw })
}

/**
 * 从 "user:pass@host:port" 或 "host:port" 或 "[ipv6]:port" 中提取认证信息和地址
 */
function parseAuthAndHost(
  input: string
): { host: string; port: number; username?: string; password?: string } | null {
  let rest = input
  let username: string | undefined
  let password: string | undefined

  // 提取 user:pass@ 部分（注意不能和 host:port 混淆）
  // 只在 @ 后面有有效的 host:port 时才算认证信息
  const atIndex = rest.lastIndexOf('@')
  if (atIndex > 0) {
    const authPart = rest.substring(0, atIndex)
    const hostPortPart = rest.substring(atIndex + 1)
    // authPart 应该是 user:pass 格式
    const colonInAuth = authPart.indexOf(':')
    if (colonInAuth >= 0) {
      username = authPart.substring(0, colonInAuth)
      password = authPart.substring(colonInAuth + 1)
    } else {
      username = authPart
      password = ''
    }
    rest = hostPortPart
  }

  // 解析 host:port，处理 IPv6 [::1]:port 格式
  const ipv6Match = rest.match(/^\[(.+)\]:(\d+)$/)
  if (ipv6Match) {
    return { host: ipv6Match[1], port: parseInt(ipv6Match[2], 10), username, password }
  }

  // 普通 host:port:user:pass 格式（无 @ 分隔符，冒号分隔四段）
  // 例: 1.2.3.4:1080:user:pass
  const hostPortUserPassMatch = rest.match(/^([^:]+):(\d+):([^:]+):(.+)$/)
  if (hostPortUserPassMatch) {
    return {
      host: hostPortUserPassMatch[1],
      port: parseInt(hostPortUserPassMatch[2], 10),
      username: username ?? hostPortUserPassMatch[3],
      password: password ?? hostPortUserPassMatch[4],
    }
  }

  // 普通 host:port:user 格式（仅用户名，无密码）
  const hostPortUserMatch = rest.match(/^([^:]+):(\d+):([^:]+)$/)
  if (hostPortUserMatch) {
    return {
      host: hostPortUserMatch[1],
      port: parseInt(hostPortUserMatch[2], 10),
      username: username ?? hostPortUserMatch[3],
      password,
    }
  }

  // 普通 host:port
  const hostPortMatch = rest.match(/^([^:]+):(\d+)$/)
  if (hostPortMatch) {
    return { host: hostPortMatch[1], port: parseInt(hostPortMatch[2], 10), username, password }
  }

  return null
}

function validatePort(data: {
  host: string
  port: number
  protocol: ProxyProtocol
  username?: string
  password?: string
  raw: string
}): ParsedProxy {
  if (!data.host) {
    return { ...data, valid: false, error: '地址为空' }
  }
  if (data.port < 1 || data.port > 65535) {
    return { ...data, valid: false, error: '端口无效' }
  }
  return { ...data, valid: true }
}

/**
 * 批量解析多行代理链接串
 * 空行自动跳过，不包含在结果中
 */
export function parseProxyBatch(input: string): ParsedProxy[] {
  const lines = input.split(/\r?\n/)
  const results: ParsedProxy[] = []
  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed) continue
    results.push(parseProxyUrl(trimmed))
  }
  return results
}
