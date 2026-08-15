/**
 * SDP Bridge — WebRTC SDP ↔ 传统 SIP SDP 转换
 *
 * ============================================================
 * 背景
 * ============================================================
 * 浏览器 WebRTC RTCPeerConnection 生成的 SDP 包含 ICE candidate、
 * DTLS fingerprint 等属性，RTP profile 为 UDP/TLS/RTP/SAVPF。
 *
 * 后端 voicehost.ParseSDP 期望传统格式：
 *   c=IN IP4 <ip>  m=audio <port> RTP/AVP <payloads>  a=sendrecv
 *
 * ============================================================
 * 转换策略
 * ============================================================
 * getDescription（offer/answer → 发给后端）:
 *   1. 调用默认 WebRTC SDH，等 ICE gathering 完成后获取完整 SDP
 *   2. 从 a=candidate 行提取 IP:Port，替换 c= 和 m= 行
 *   3. 去掉 WebRTC 特有行（a=fingerprint, a=setup, a=candidate, a=ssrc 等）
 *   4. 将 m= 行的 RTP profile 改为 RTP/AVP
 *   → 返回传统 SDP 给后端 sipgw/voicehost
 *
 * setDescription（后端返回的传统 SDP → 给 RTCPeerConnection）:
 *   1. 收到后端的传统 SDP（c= 有真实 IP，m= 有真实端口）
 *   2. 在传统 SDP 基础上补充 WebRTC 必需属性：
 *      a=ice-ufrag, a=ice-pwd, a=fingerprint, a=setup, a=rtcp-mux
 *      a=candidate（用 c= 和 m= 的 IP:Port 生成 host candidate）
 *   3. 将 m= 行的 RTP profile 改为 UDP/TLS/RTP/SAVPF
 *   → 设置给 RTCPeerConnection.setRemoteDescription()
 *
 * 注意：ice-ufrag/ice-pwd/fingerprint 使用占位值，
 * RTCPeerConnection 在 setRemoteDescription 时不会验证这些值的真实性，
 * 它只会在 ICE 连接建立时使用。由于后端 RTP relay 会直接发 RTP 包到
 * 浏览器的 candidate 地址，浏览器会收到 RTP 流并播放。
 */
import {
  SessionDescriptionHandler,
  Web,
} from 'sip.js'
import type {
  SessionDescriptionHandler as ISessionDescriptionHandler,
  SessionDescriptionHandlerFactory,
  SessionDescriptionHandlerModifier,
  BodyAndContentType,
  SessionDescriptionHandlerOptions,
} from 'sip.js'
import type { Session } from 'sip.js'

// ============================================================
// WebRTC → 传统 SDP
// ============================================================

export function webRTCToLegacySDP(sdp: string): string {
  const lines = sdp.split(/\r?\n/)
  const candidate = pickBestIceCandidate(lines)

  const out: string[] = []
  let inAudio = false

  for (const line of lines) {
    if (line === '') continue

    // 会话级
    if (line.startsWith('v=')) { out.push(line); continue }
    if (line.startsWith('o=')) { out.push(replaceOriginIP(line, candidate.ip)); continue }
    if (line.startsWith('s=')) { out.push(line); continue }
    if (line.startsWith('t=')) { out.push(line); continue }

    // c= 行：替换为 candidate IP
    if (line.startsWith('c=IN IP')) {
      const ip = candidate.ip || extractConnectionIP(line)
      const ver = ip.includes(':') ? 'IP6' : 'IP4'
      out.push(`c=IN ${ver} ${ip}`)
      continue
    }

    // m=audio 行
    if (line.startsWith('m=audio ')) {
      inAudio = true
      const port = candidate.port || extractMediaPort(line)
      const payloads = extractPayloadTypes(line)
      out.push(`m=audio ${port} RTP/AVP ${payloads.join(' ')}`)
      continue
    }

    // 非 audio media 段跳过
    if (line.startsWith('m=') && !line.startsWith('m=audio')) {
      inAudio = false
      continue
    }

    if (!inAudio) {
      // 跳过会话级 WebRTC 属性
      if (isWebRTCSessionAttribute(line)) continue
      // 保留其他会话级行
      if (!line.startsWith('a=')) out.push(line)
      continue
    }

    // audio media 段
    if (isWebRTCMediaAttribute(line)) continue
    if (line.startsWith('a=rtcp:')) {
      if (candidate.port) {
        const ver = candidate.ip.includes(':') ? 'IP6' : 'IP4'
        out.push(`a=rtcp:${candidate.port} IN ${ver} ${candidate.ip}`)
      }
      continue
    }
    // 保留 a=rtpmap, a=fmtp, a=sendrecv/recvonly/sendonly/inactive, a=ptime, a=maxptime
    out.push(line)
  }

  return out.join('\r\n') + '\r\n'
}

// ============================================================
// 传统 SDP → WebRTC SDP
// ============================================================

export function legacyToWebRTCSDP(sdp: string): string {
  const lines = sdp.split(/\r?\n/)
  const out: string[] = []
  let inAudio = false
  let connectionIP = ''
  let mediaPort = ''
  let hasRtcpMux = false

  for (const line of lines) {
    if (line === '') continue

    // 提取 c= 行 IP
    if (line.startsWith('c=IN IP')) {
      const m = line.match(/c=IN\s+IP[46]\s+(\S+)/)
      if (m) connectionIP = m[1]
      out.push(line)
      continue
    }

    if (line.startsWith('m=audio ')) {
      inAudio = true
      const m = line.match(/^m=audio\s+(\d+)\s+\S+\s+(.*)$/)
      if (m) {
        mediaPort = m[1]
        out.push(`m=audio ${mediaPort} UDP/TLS/RTP/SAVPF ${m[2]}`)
      } else {
        out.push(line)
      }
      continue
    }

    if (line.startsWith('m=') && !line.startsWith('m=audio')) {
      inAudio = false
      out.push(line)
      continue
    }

    if (inAudio) {
      if (line.startsWith('a=rtcp-mux')) hasRtcpMux = true
      out.push(line)
    } else {
      out.push(line)
    }
  }

  // 后处理：在 audio 段添加缺失的 WebRTC 必需属性
  return addWebRTCAttributes(out.join('\r\n'), connectionIP, mediaPort, hasRtcpMux)
}

// ============================================================
// 辅助函数
// ============================================================

interface IceCandidate {
  ip: string
  port: number
}

function pickBestIceCandidate(lines: string[]): IceCandidate {
  let bestHost: IceCandidate | null = null
  let bestSrflx: IceCandidate | null = null

  for (const line of lines) {
    if (!line.startsWith('a=candidate:')) continue
    const parts = line.substring(12).split(/\s+/)
    if (parts.length < 8) continue
    const typ = parts[7]?.replace('typ', '') || ''
    const ip = parts[4]
    const port = parseInt(parts[5], 10)
    if (!ip || !port || ip === '0.0.0.0') continue
    if (ip.includes(':') && !ip.startsWith('::ffff:')) continue

    if (typ === 'srflx' && !bestSrflx) bestSrflx = { ip, port }
    else if (typ === 'host' && !bestHost) bestHost = { ip, port }
  }

  return bestSrflx || bestHost || { ip: '', port: 0 }
}

function replaceOriginIP(line: string, newIP: string): string {
  if (!newIP) return line
  const m = line.match(/^(o=\S+\s+\S+\s+\S+\s+IN\s+IP[46]\s+)/)
  if (m) {
    const ver = newIP.includes(':') ? 'IP6' : 'IP4'
    return `o=${line.substring(2).replace(/IN\s+IP[46]\s+\S+/, `IN ${ver} ${newIP}`)}`
  }
  return line
}

function extractConnectionIP(line: string): string {
  const m = line.match(/c=IN\s+IP[46]\s+(\S+)/)
  return m ? m[1] : '127.0.0.1'
}

function extractMediaPort(line: string): number {
  const m = line.match(/m=audio\s+(\d+)/)
  return m ? parseInt(m[1], 10) : 0
}

function extractPayloadTypes(line: string): number[] {
  const m = line.match(/m=audio\s+\d+\s+\S+\s+(.*)$/)
  if (!m) return [0, 8, 101]
  return m[1].split(/\s+/).map((p) => parseInt(p, 10)).filter((p) => !isNaN(p))
}

function isWebRTCSessionAttribute(line: string): boolean {
  return (
    line.startsWith('a=fingerprint:') ||
    line.startsWith('a=setup:') ||
    line.startsWith('a=ice-ufrag:') ||
    line.startsWith('a=ice-pwd:') ||
    line.startsWith('a=ice-options:') ||
    line.startsWith('a=group:') ||
    line.startsWith('a=msid-semantic:') ||
    line.startsWith('a=extmap:') ||
    line.startsWith('a=identity:')
  )
}

function isWebRTCMediaAttribute(line: string): boolean {
  return (
    line.startsWith('a=candidate:') ||
    line.startsWith('a=fingerprint:') ||
    line.startsWith('a=setup:') ||
    line.startsWith('a=ice-ufrag:') ||
    line.startsWith('a=ice-pwd:') ||
    line.startsWith('a=ice-options:') ||
    line.startsWith('a=rtcp-mux') ||
    line.startsWith('a=rtcp-rsize') ||
    line.startsWith('a=ssrc:') ||
    line.startsWith('a=ssrc-group:') ||
    line.startsWith('a=extmap:') ||
    line.startsWith('a=mid:') ||
    line.startsWith('a=msid:') ||
    line.startsWith('a=rtcp-fb:') ||
    line.startsWith('a=identity:')
  )
}

function hasLine(lines: string[], prefix: string): boolean {
  return lines.some((l) => l.startsWith(prefix))
}

/**
 * 在 audio media 段补充 WebRTC 必需属性
 */
function addWebRTCAttributes(
  sdp: string,
  ip: string,
  port: string,
  hasRtcpMux: boolean,
): string {
  const lines = sdp.split(/\r?\n/)
  const out: string[] = []
  let inAudio = false
  let inserted = false

  for (const line of lines) {
    if (line === '') continue

    if (line.startsWith('m=audio ')) {
      inAudio = true
      out.push(line)
      // 在 m=audio 行后插入 WebRTC 必需属性
      if (!inserted) {
        // ice-ufrag / ice-pwd（占位值，RTCPeerConnection 不验证）
        if (!hasLine(lines, 'a=ice-ufrag:'))
          out.push('a=ice-ufrag:vohive')
        if (!hasLine(lines, 'a=ice-pwd:'))
          out.push('a=ice-pwd:vohivepassword1234567890')
        // fingerprint（占位值）
        if (!hasLine(lines, 'a=fingerprint:'))
          out.push('a=fingerprint:sha-256 00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00')
        // setup
        if (!hasLine(lines, 'a=setup:'))
          out.push('a=setup:actpass')
        // rtcp-mux
        if (!hasRtcpMux)
          out.push('a=rtcp-mux')
        // 生成 host candidate
        if (ip && port)
          out.push(`a=candidate:1 1 udp 2130706431 ${ip} ${port} typ host generation 0`)
        inserted = true
      }
      continue
    }

    if (line.startsWith('m=') && !line.startsWith('m=audio ')) {
      inAudio = false
      out.push(line)
      continue
    }

    out.push(line)
  }

  return out.join('\r\n') + '\r\n'
}

// ============================================================
// 自定义 SessionDescriptionHandlerFactory
// ============================================================

/**
 * 创建桥接 SDH 工厂
 *
 * 包装 SIP.js 默认的 WebRTC SDH：
 * - getDescription: WebRTC SDP → 传统 SDP（给后端）
 * - setDescription: 传统 SDP → WebRTC SDP（给 RTCPeerConnection）
 */
export function createBridgeSDHFactory(): SessionDescriptionHandlerFactory {
  const defaultFactory = Web.defaultSessionDescriptionHandlerFactory()

  const bridgeFactory: SessionDescriptionHandlerFactory = (
    session: Session,
    options?: any,
  ): ISessionDescriptionHandler => {
    const webSDH = defaultFactory(session, options) as SessionDescriptionHandler

    const bridgeSDH: ISessionDescriptionHandler = {
      close: () => webSDH.close(),

      hasDescription: (contentType: string) =>
        webSDH.hasDescription(contentType),

      getDescription: async (
        options?: SessionDescriptionHandlerOptions,
        modifiers?: Array<SessionDescriptionHandlerModifier>,
      ): Promise<BodyAndContentType> => {
        // 1. 调用默认 WebRTC SDH（等 ICE 完成后返回完整 SDP）
        const result = await webSDH.getDescription(options, modifiers)
        // 2. 转换为传统 SDP
        const legacySDP = webRTCToLegacySDP(result.body)
        return {
          body: legacySDP,
          contentType: 'application/sdp',
        }
      },

      setDescription: async (
        sdp: string,
        options?: SessionDescriptionHandlerOptions,
        modifiers?: Array<SessionDescriptionHandlerModifier>,
      ): Promise<void> => {
        // 1. 传统 SDP → WebRTC SDP
        const webRTCSDP = legacyToWebRTCSDP(sdp)
        // 2. 设置给 RTCPeerConnection
        await webSDH.setDescription(webRTCSDP, options, modifiers)
      },

      sendDtmf: (tones: string, options?: unknown) =>
        webSDH.sendDtmf(tones, options),
    }

    if (webSDH.rollbackDescription) {
      bridgeSDH.rollbackDescription = async () => {
        await webSDH.rollbackDescription!()
      }
    }

    return bridgeSDH
  }

  return bridgeFactory
}
