/**
 * Voice Service — SIP.js 封装
 *
 * 浏览器通过 SIP.js over WSS 连接 sipgw.Registrar，
 * 实现 REGISTER / INVITE / BYE / DTMF 等 SIP 信令操作。
 *
 * 后端 sipgw 自动路由到 VoWiFi / VoLTE / CS。
 */
import {
  UserAgent,
  Registerer,
  RegistererState,
  Session,
  SessionState,
  Inviter,
  Invitation,
} from 'sip.js'
import { createBridgeSDHFactory } from './sdp-bridge'

export type CallState = 'idle' | 'dialing' | 'ringing' | 'connected' | 'hanging'

export interface VoiceConfig {
  /** WSS 连接地址，如 wss://192.168.1.1:5061 */
  wsUrl: string
  /** SIP 用户名 */
  username: string
  /** SIP 密码 */
  password: string
  /** SIP 认证域 */
  realm: string
  /** 显示名称 */
  displayName?: string
}

export interface CallEvent {
  state: CallState
  number?: string
  /** 通话方向 */
  direction?: 'outgoing' | 'incoming'
}

export type CallEventHandler = (event: CallEvent) => void

class VoiceService {
  private ua: UserAgent | null = null
  private registerer: Registerer | null = null
  private session: Session | null = null
  private config: VoiceConfig | null = null
  private handlers: Set<CallEventHandler> = new Set()
  private registered = false

  /** 初始化并注册 */
  async connect(config: VoiceConfig): Promise<void> {
    // 如果已经连接且配置相同，跳过
    if (this.ua && this.config?.wsUrl === config.wsUrl && this.config?.username === config.username) {
      return
    }
    // 先断开旧连接
    await this.disconnect()

    this.config = config

    const sipDomain = config.realm || 'vohive.local'
    const sipUri = `sip:${config.username}@${sipDomain}`

    this.ua = new UserAgent({
      uri: UserAgent.makeURI(sipUri),
      transportOptions: {
        server: config.wsUrl,
      },
      authorizationUsername: config.username,
      authorizationPassword: config.password,
      displayName: config.displayName || config.username,
      // 使用桥接 SDH：WebRTC SDP ↔ 传统 SIP SDP 转换
      sessionDescriptionHandlerFactory: createBridgeSDHFactory(),
      logBuiltinEnabled: import.meta.env.DEV,
      logLevel: import.meta.env.DEV ? 'debug' : 'warn',
    })

    // 来电回调
    const ua = this.ua!
    ua.delegate = {
      onInvite: (invitation: Invitation) => {
        this.session = invitation
        const callerNumber = this.extractNumber(invitation.remoteIdentity.uri.toString())
        this.emit({ state: 'ringing', number: callerNumber, direction: 'incoming' })

        invitation.stateChange.addListener((state: SessionState) => {
          if (state === SessionState.Terminated) {
            this.session = null
            this.emit({ state: 'idle' })
          }
        })
      },
    }

    this.registerer = new Registerer(this.ua)

    this.registerer.stateChange.addListener((state: RegistererState) => {
      this.registered = state === RegistererState.Registered
    })

    await this.ua.start()
    await this.registerer.register()
  }

  /** 断开连接 */
  async disconnect(): Promise<void> {
    if (this.session) {
      this.hangup()
    }
    if (this.registerer) {
      try { await this.registerer.unregister() } catch { /* ignore */ }
      this.registerer = null
    }
    if (this.ua) {
      try { await this.ua.stop() } catch { /* ignore */ }
      this.ua = null
    }
    this.config = null
    this.registered = false
  }

  /** 是否已注册 */
  isRegistered(): boolean {
    return this.registered
  }

  /** 检查麦克风权限，确保可用 */
  private async ensureMicrophone(): Promise<void> {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false })
      // 立即释放，SIP.js 会自己重新获取
      stream.getTracks().forEach(t => t.stop())
    } catch (e) {
      throw new Error('无法访问麦克风，请检查浏览器权限设置')
    }
  }

  /** 拨号 */
  async call(number: string): Promise<void> {
    if (!this.ua || !this.registered) {
      throw new Error('SIP 未注册，无法拨号')
    }
    if (this.session) {
      throw new Error('已有通话进行中')
    }

    // 提前检查麦克风权限
    await this.ensureMicrophone()

    const sipDomain = this.config?.realm || 'vohive.local'
    const targetUri = UserAgent.makeURI(`sip:${number}@${sipDomain}`)
    if (!targetUri) {
      throw new Error(`无效的号码: ${number}`)
    }

    // 创建呼出会话
    const inviter = new Inviter(this.ua, targetUri, {
      sessionDescriptionHandlerOptions: {
        constraints: {
          audio: true,
          video: false,
        },
      },
    })
    this.session = inviter

    this.emit({ state: 'dialing', number, direction: 'outgoing' })

    inviter.stateChange.addListener((state: SessionState) => {
      switch (state) {
        case SessionState.Establishing:
          this.emit({ state: 'ringing', number })
          break
        case SessionState.Established:
          this.emit({ state: 'connected', number })
          break
        case SessionState.Terminating:
          this.emit({ state: 'hanging' })
          break
        case SessionState.Terminated:
          this.session = null
          this.emit({ state: 'idle' })
          break
      }
    })

    await inviter.invite()
  }

  /** 接听来电 */
  async answer(): Promise<void> {
    if (!this.session || !(this.session instanceof Invitation)) return
    // 提前检查麦克风权限
    await this.ensureMicrophone()
    await (this.session as Invitation).accept({
      sessionDescriptionHandlerOptions: {
        constraints: {
          audio: true,
          video: false,
        },
      },
    })
    this.emit({ state: 'connected' })
  }

  /** 挂断 */
  hangup(): void {
    if (!this.session) return
    this.emit({ state: 'hanging' })
    const session = this.session
    this.session = null
    // 如果是呼出且还没接通，用 cancel；否则用 bye
    if (session instanceof Inviter && session.state === SessionState.Initial) {
      session.cancel().catch(() => {})
    } else {
      session.bye().catch(() => {})
    }
  }

  /** 发送 DTMF */
  sendDTMF(digit: string): void {
    if (!this.session) return
    // SIP.js 通过 Session.info() 发送 SIP INFO DTMF
    const session = this.session
    session.info({
      requestOptions: {
        body: {
          contentDisposition: 'render',
          contentType: 'application/dtmf-relay',
          content: `Signal=${digit}\nDuration=250`,
        },
      },
    }).catch(() => {})
  }

  /** 注册状态变更回调 */
  onCallEvent(handler: CallEventHandler): () => void {
    this.handlers.add(handler)
    return () => this.handlers.delete(handler)
  }

  private emit(event: CallEvent) {
    this.handlers.forEach(h => h(event))
  }

  /** 从 SIP URI 提取号码 */
  private extractNumber(uri: string): string {
    // sip:+1234567890@vohive.local → +1234567890
    const match = uri.match(/sip:([^@]+)@/)
    return match ? match[1] : uri
  }
}

export const voiceService = new VoiceService()
