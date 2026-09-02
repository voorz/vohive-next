/**
 * 运营商配置参数注解数据
 * 每个参数项的 key 对应 CarrierConfigForm 中的 label 文本
 * 用于在表单 label 旁显示 Popover 弹出框
 */

export interface ParamAnnotation {
  /** 参数作用说明 */
  desc: string
  /** 推荐配置 / 默认值 */
  recommend?: string
  /** 可选值说明（对下拉框有用） */
  options?: Record<string, string>
  /** 额外提示（如"部分运营商必须开启"） */
  note?: string
}

/** 按 section → 参数 key 组织的注解表 */
export const configAnnotations: Record<string, Record<string, ParamAnnotation>> = {
  ike: {
    'ePDG 地址': {
      desc: 'Evolved Packet Data Gateway 的 FQDN 或 IP 地址，用于 IKEv2/IPsec 隧道连接。空值时自动按 3GPP 规范生成（epdg.mnc<MNC>.mcc<MCC>.pub.3gppnetwork.org）。',
      recommend: '空（自动生成）',
      note: '少数运营商使用非标准 ePDG 地址时需手动指定',
    },
    'ePDG 端口': {
      desc: 'ePDG 服务监听的 UDP 端口，IKEv2 标准端口为 500。IPsec NAT-T 穿透时使用 4500。',
      recommend: '500',
    },
    'IKE 提议': {
      desc: 'IKEv2 SA 的加密提议列表，格式：cipher-hash-prf-dhgroup。按顺序尝试，首个匹配的生效。',
      recommend: 'aes256-sha256-prfsha256-modp2048',
      note: '部分旧 ePDG 仅支持 aes128-sha1-prfsha1-modp2048',
    },
    'ESP 提议': {
      desc: 'ESP（Encapsulating Security Payload）提议，用于 IPsec 数据加密。格式：cipher-hash。',
      recommend: 'aes256-sha256',
    },
    'DPD 间隔 (秒)': {
      desc: 'Dead Peer Detection（死点检测）间隔。定期发送 IKE INFORMATIONAL 请求检测对端是否存活。0=禁用。',
      recommend: '0（禁用）或 30',
    },
    'NAT 保活 (秒)': {
      desc: 'NAT Keepalive 间隔。在 NAT 环境下定期发送 keepalive 包维持端口映射。0=禁用。',
      recommend: '20',
    },
    '重认证间隔 (秒)': {
      desc: '强制 IKEv2 重新认证的时间间隔。0=不强制，由 SA 生命周期决定。',
      recommend: '0（不强制）',
    },
    '抗重放窗口': {
      desc: 'IPsec 抗重放窗口大小。较大的窗口允许更多的乱序包通过，但消耗更多内存。',
      recommend: '32',
    },
    'IP 协议栈': {
      desc: 'IKEv2/IPsec 隧道使用的 IP 协议栈版本。',
      options: {
        '': '自动（运营商默认）',
        ipv4: '仅 IPv4',
        ipv6: '仅 IPv6',
        ipv4v6: 'IPv4/IPv6 双栈',
      },
      recommend: '自动',
    },
    'APN': {
      desc: 'IKEv2 隧道内部使用的 APN（Access Point Name），部分运营商隧道内需特定 APN 才能注册 IMS。',
      recommend: 'ims',
    },
    'RFOff 延迟 (秒)': {
      desc: '切卡后射频关闭到重新开启之间的延迟时间。部分运营商 SIM 重载需要足够延迟才能正确刷新。',
      recommend: '5',
    },
    '启用 ESN': {
      desc: 'Extended Sequence Numbers（扩展序列号），64 位序列号空间。部分运营商 ePDG 要求启用 ESN 才能建立 SA。',
      recommend: '关闭（旧运营商兼容性更好）',
    },
    'EAP MAC 校验': {
      desc: 'EAP 消息的 MAC 地址校验。部分 ePDG 实现在 EAP 阶段对 MAC 有非标准处理。',
      recommend: '关闭',
    },
    'TICKET_REQUEST': {
      desc: '在第一轮 IKE_AUTH 中发送 N(TICKET_REQUEST) Notify（RFC 5723 会话恢复）。3HK 等 ePDG 需要此 Notify 才能通过认证。Three UK 等部分 ePDG 会因此拒绝。',
      recommend: '关闭',
      note: '3HK 需要开启',
    },
    '首轮 IKE_AUTH 发 CP': {
      desc: '在第一轮 IKE_AUTH 请求中包含 CP(CFG_REQUEST) 载荷，请求 ePDG 下发 IP/DNS/P-CSCF 配置。部分 ePDG（如 3HK）在首轮含 CP 时会拒绝认证。',
      recommend: '开启',
      note: '3HK 需要关闭',
    },
  },

  eap: {
    '挑战模式': {
      desc: 'EAP-AKA/AKA\' 挑战响应模式。',
      options: {
        standard: '标准模式，完整 AT_RES 响应',
        minimal: '最小模式，AT_RES + AT_CHECKCODE 回显 ePDG 值 + AT_MAC',
        checkcode: '校验码模式，使用 AT_CHECKCODE 简化流程',
        omit: '省略挑战，直接发送空响应',
      },
      recommend: '标准',
    },
    'USIM/ISIM 偏好': {
      desc: '选择使用 USIM 还是 ISIM 应用进行 EAP-AKA 认证。部分运营商 ISIM 不可用时需回退到 USIM。',
      options: {
        auto: '自动（优先 ISIM，回退 USIM）',
        usim: '强制使用 USIM',
        isim: '强制使用 ISIM',
      },
      recommend: '自动',
    },
    '身份来源': {
      desc: 'EAP Identity 阶段使用的身份来源。',
      options: {
        derived: '从 IMSI 推导（默认 3GPP 规范）',
        isim: '从 ISIM 应用读取 IMPI',
        auto: '自动选择',
      },
      recommend: '推导',
    },
    '设备型号': {
      desc: 'EAP DEVICE_IDENTITY notify 中发送的设备型号标识，部分运营商用于设备识别/白名单匹配。',
      recommend: '空（不发送）或匹配实际设备',
    },
    '发送设备身份通知': {
      desc: '是否在 EAP 阶段发送 DEVICE_IDENTITY notify 消息。部分运营商需要此信息进行设备验证。',
      recommend: '关闭',
    },
  },

  ims: {
    '安全协商模式': {
      desc: 'IMS 安全协商（sec-agree）行为模式。控制 REGISTER 请求中是否携带 Security-Client / Require: sec-agree 等 SIP 安全头。',
      options: {
        auto: '自动（根据运营商响应动态调整）',
        on: '强制开启 sec-agree',
        off: '关闭 sec-agree',
      },
      recommend: '自动',
      note: 'Vodafone UK 等运营商需要强制开启（on）',
    },
    '传输模式': {
      desc: 'SIP 信令传输协议。',
      options: {
        auto: '自动（优先 UDP，失败回退 TCP）',
        udp: '强制 UDP',
        tcp: '强制 TCP',
      },
      recommend: '自动',
    },
    '首次认证变体': {
      desc: '首次 REGISTER 请求的认证变体策略。不同运营商对首次 REGISTER 的 Authorization 头格式要求不同。',
      options: {
        aka_empty_uri_first: 'AKA 空 URI 优先（推荐，兼容性最好）',
        aka_empty: 'AKA 空响应',
        aka_zero_response: 'AKA 零响应',
        aka_zero_response_uri_first: 'AKA 零响应 URI 优先',
        none: '不发送 Authorization',
      },
      recommend: 'AKA空URI优先',
    },
    '注册有效期 (秒)': {
      desc: 'REGISTER 请求中的 Expires 头值，表示注册有效期。到期后需重新注册。',
      recommend: '600（10 分钟）',
    },
    '认证身份格式': {
      desc: 'Authorization 头中的 username/identity 格式。',
      options: {
        imsi_home_domain: 'IMSI@home.domain（3GPP 标准）',
        private_id: 'Private Identity（ISIM 读取）',
        prefixed_imsi_home_domain: '前缀+IMSI@home.domain',
        imsi_phone_uri: 'IMSI Phone URI 格式',
      },
      recommend: 'IMSI 域名',
    },
    '本地 SIP 端口': {
      desc: '本地 SIP 监听端口。',
      recommend: '5060（标准）',
    },
    'User-Agent': {
      desc: 'SIP User-Agent 头，模拟特定设备型号。部分运营商 P-CSCF 根据 UA 进行差异化处理。',
      recommend: '空（使用默认 UA）或匹配目标设备',
    },
    'Supported 头': {
      desc: 'SIP Supported 头，声明支持的 SIP 扩展特性。',
      recommend: 'path,sec-agree,gruu',
    },
    'Allow 头': {
      desc: 'SIP Allow 头，声明允许的 SIP 方法。',
      recommend: '空（使用默认方法集）',
    },
    'P-CSCF 地址': {
      desc: 'P-CSCF（Proxy Call Session Control Function）地址，覆盖 DNS 自动发现。格式：host:port 或 host。',
      recommend: '空（自动发现）',
      note: '仅在 DNS 无法发现 P-CSCF 时手动指定',
    },
    'IMS 域名': {
      desc: 'IMS 注册域名（home network domain），如 ims.mnc001.mcc234.3gppnetwork.org。空值时从 IMSI 自动生成。',
      recommend: '空（自动生成）',
    },
    'Realm': {
      desc: 'SIP Digest/Security 协商中的 realm 值。空值时使用 IMS 域名。',
      recommend: '空（使用域名）',
    },
    'ICSI Ref': {
      desc: 'IMS Communication Service Identifier，标识 IMS 通信服务类型。',
      recommend: 'urn%3Aurn-7%3A3gpp-service.ims.icsi.mmtel',
    },
    'Contact 特性': {
      desc: 'Contact 头中携带的特性参数集，影响 P-CSCF 对终端能力的判断。',
      options: {
        ims_features: '标准 IMS 特性（音频+视频+短信）',
        phone_xiaomi: '小米手机特性集',
        sms_only: '仅短信（无语音）',
      },
      recommend: 'IMS特性',
    },
    'Security-Client 格式': {
      desc: 'Security-Client 头的格式化方式，影响安全机制列表的编码格式。',
      options: {
        full_spaced: '完整空格分隔（3GPP 标准）',
        phone_multi: '手机多行格式',
        minimal_spaced: '精简空格格式',
      },
      recommend: '完整空格',
    },
    '固定 PANI': {
      desc: 'P-Access-Network-Info 头中的固定 IEEE 802.11 标识。部分运营商要求特定格式的 WiFi 节点 ID。',
      recommend: '空（自动生成随机值）',
    },
    'TCP Keepalive (秒)': {
      desc: 'TCP 模式下的 Keepalive 间隔。防止 NAT 或防火墙超时断开 TCP 连接。',
      recommend: '30',
    },
    'OPTIONS Ping (秒)': {
      desc: '定期发送 SIP OPTIONS 请求探测 P-CSCF 可达性。0=禁用。',
      recommend: '45',
    },
    'Contact 参数顺序': {
      desc: 'Contact 头参数的排列顺序，部分运营商 P-CSCF 对参数顺序敏感。',
      recommend: 'access_type, audio, smsip, icsi_ref, sip_instance',
    },
    '临时失败状态码': {
      desc: 'REGISTER 返回这些状态码时视为临时失败，自动重试。',
      recommend: '408, 480, 500, 502, 503, 504',
    },
    '禁止状态码': {
      desc: 'REGISTER 返回这些状态码时视为永久禁止，停止重试。',
      recommend: '403',
    },
    '首次拒绝回退状态码': {
      desc: '首次 REGISTER 返回这些状态码时，切换认证变体重试。',
      recommend: '400, 403, 480, 500',
    },
    '临时失败重试间隔 (秒)': {
      desc: '临时失败后的重试间隔。0=使用默认退避策略。',
      recommend: '0（默认退避）',
    },
    '首次 REGISTER 带 PANI': {
      desc: '首次 REGISTER 请求中是否携带 P-Access-Network-Info 头。',
      recommend: '关闭',
      note: '部分运营商首次 REGISTER 不允许 PANI（会返回错误）',
    },
    '认证后 REGISTER 带 PANI': {
      desc: '认证成功后的 REGISTER 请求中是否携带 P-Access-Network-Info 头。',
      recommend: '开启',
    },
    'Require: sec-agree': {
      desc: 'REGISTER 请求中是否携带 Require: sec-agree 头，强制 P-CSCF 执行安全协商。',
      recommend: '关闭',
      note: 'Vodafone UK 等运营商需要开启',
    },
    'Proxy-Require: sec-agree': {
      desc: 'REGISTER 请求中是否携带 Proxy-Require: sec-agree 头。',
      recommend: '关闭',
      note: 'Vodafone UK 等运营商需要开启',
    },
    '首次 REGISTER 带空 AKA Authorization': {
      desc: '首次 REGISTER 使用空的 Digest Authorization 头而非 AKA 挑战响应。',
      recommend: '关闭',
    },
    '严格匹配 Security-Server': {
      desc: '严格匹配 P-CSCF 返回的 Security-Server 头中的安全机制。关闭时使用宽松匹配。',
      recommend: '开启',
    },
    '首次拒绝后回退重试': {
      desc: '首次 REGISTER 被拒绝后，自动切换认证变体并重试。',
      recommend: '关闭',
    },
    '省略 Route 头': {
      desc: '认证后的 REGISTER 请求中省略 Route 头。',
      recommend: '关闭',
      note: 'Vodafone UK 需要开启（omit_route=true）',
    },
    '首次 REGISTER 精简头': {
      desc: '首次 REGISTER 请求仅发送最少的必要头，减少被拒绝的风险。',
      recommend: '关闭',
    },
    '强制头中端口 5060': {
      desc: '强制 SIP 头中的端口为 5060（即使实际使用非标准端口）。',
      recommend: '关闭',
    },
    'Security-Client 省略 prot/mod': {
      desc: 'Security-Client 头中省略 prot 和 mod 参数，仅保留 alg/ealg。',
      recommend: '关闭',
    },
    '400 时探测 Security-Client': {
      desc: '收到 400 Bad Request 时，自动探测 P-CSCF 期望的 Security-Client 格式。',
      recommend: '开启',
    },
    'Authorization 含 Connection-Keepalive': {
      desc: 'Authorization 头中包含 Connection-Keepalive 参数。',
      recommend: '关闭',
    },
    'Security-Client 含服务器参数': {
      desc: 'Security-Client 头中包含服务器端参数（spare/none 等）。',
      recommend: '关闭',
    },
    '回退时 Security-Client 含服务器参数': {
      desc: '认证回退重试时，Security-Client 头中包含服务器端参数。',
      recommend: '关闭',
    },
    'Accept-Contact 头': {
      desc: 'REGISTER 请求中是否携带 Accept-Contact 头，声明可接受的联系方式。',
      recommend: '关闭',
    },
    'P-Preferred-Identity 头': {
      desc: 'REGISTER 请求中是否携带 P-Preferred-Identity 头。',
      recommend: '关闭',
    },
    'P-Visited-Network-ID 头': {
      desc: 'REGISTER 请求中是否携带 P-Visited-Network-ID 头。',
      recommend: '关闭',
    },
    'P-Access-Network-Info 头': {
      desc: 'REGISTER 请求中是否携带 P-Access-Network-Info 头，提供接入网信息。',
      recommend: '开启',
    },
    'Route 头': {
      desc: 'REGISTER 请求中是否携带 Route 头。',
      recommend: '开启',
    },
    'Cellular-Network-Info 头': {
      desc: 'REGISTER 请求中是否携带 Cellular-Network-Info 头。',
      recommend: '关闭',
    },
    'Security-Client 头': {
      desc: 'REGISTER 请求中是否携带 Security-Client 头，声明终端支持的安全机制。',
      recommend: '开启',
    },
    'Require: sec-agree 头': {
      desc: 'REGISTER 请求中是否携带 Require: sec-agree 头。',
      recommend: '关闭',
    },
    'Contact URI 随机 UUID': {
      desc: '每次 REGISTER 使用随机 UUID 作为 Contact URI 的 user 部分，防止被 P-CSCF 去重。',
      recommend: '开启',
    },
  },

  voice: {
    'Supported': {
      desc: '语音会话（INVITE 等）请求中的 Supported 头。空值时继承 REGISTER 配置。',
      recommend: '空（继承 REGISTER）',
    },
    'Allow': {
      desc: '语音会话请求中的 Allow 头，声明允许的 SIP 方法。',
      recommend: '空（默认 INVITE,ACK,CANCEL,BYE,...）',
    },
    'Accept-Contact': {
      desc: '语音 INVITE 请求中的 Accept-Contact 头。',
      recommend: '空（不发送）',
    },
    'P-Preferred-Service': {
      desc: '语音 INVITE 请求中的 P-Preferred-Service 头，标识请求的服务类型。',
      recommend: '空（不发送）',
    },
  },

  e911: {
    '启用 E911': {
      desc: '启用北美 E911 紧急呼叫定位服务。美国运营商（FCC 要求）必须支持。',
      recommend: '关闭（非美国运营商）',
      note: '美国运营商（AT&T/T-Mobile/Verizon 等）需要开启',
    },
    '服务商': {
      desc: 'E911 定位服务提供商名称。',
      recommend: 'intrado',
    },
    'Entitlement 端点': {
      desc: 'E911 服务授权检查的 HTTP 端点 URL。',
      recommend: '由运营商提供',
    },
    'Websheet URL': {
      desc: 'E911 地址输入页面的 websheet URL，用于用户手动输入紧急地址。',
      recommend: '由运营商提供',
    },
  },

  device: {
    'IMEI': {
      desc: 'International Mobile Equipment Identity，设备唯一标识。部分运营商用于设备白名单匹配。',
      recommend: '空（使用模组实际 IMEI）',
      note: '15 位数字，空值时使用模组/读卡器返回的真实 IMEI',
    },
    'Cell ID 模式': {
      desc: 'P-Access-Network-Info 中 Cell ID 的获取方式。',
      options: {
        qmi_first: '优先从 QMI 获取，回退运营商配置',
        carrier_only: '仅使用运营商配置的值',
        none: '不发送 Cell ID',
      },
      recommend: 'QMI优先',
    },
    'LTE TAC': {
      desc: 'LTE Tracking Area Code，用于 P-Access-Network-Info 中的 location info。',
      recommend: '0（使用 QMI 实时获取）',
    },
    'LTE Cell ID': {
      desc: 'LTE Cell ID，用于 P-Access-Network-Info 中的 location info。',
      recommend: '0（使用 QMI 实时获取）',
    },
    'REGISTER 模拟档案': {
      desc: 'REGISTER 请求模拟的目标设备档案名。部分运营商 P-CSCF 对特定设备型号有差异化处理。',
      recommend: '空（不模拟）',
    },
    '禁止此运营商 VoWiFi': {
      desc: '设为 true 时，不对此运营商发起 VoWiFi 注册（仅用于流量代理或短信场景）。',
      recommend: '关闭',
    },
  },
}
