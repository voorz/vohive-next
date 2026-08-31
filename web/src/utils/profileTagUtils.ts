/**
 * Profile Tag 工具函数
 *
 * 移植自 NekokoLPA2 的 profile_tag_utils.dart
 * SGP22 Nickname 字段编码方案：名称 + \n + 标签串
 *
 * 标签格式：
 * - 日期标签：d:YYMMDD 或 d:YYMMDD:备注
 * - 文本标签：t:文本内容
 * - 多标签用空格连接
 *
 * 转义规则：
 * - 空格 → \x11（内部空格符）
 * - 冒号 → \x03（内部冒号符）
 */

/** 标签分隔符（换行符 \x0A） */
export const TAG_SEPARATOR = '\n'

/** 内部空格符（转义后的空格） */
const INTERNAL_SPACE = '\x11'

/** 内部冒号符（转义后的冒号） */
const INTERNAL_COLON = '\x03'

/** 标签正则：匹配 d: 或 t: 开头的标签 */
const TAG_REGEX = /\b([td]:[^\s]+)/gi

/** 转义内容：空格→\x11，冒号→\x03 */
function escapeContent(s: string): string {
  return s.replaceAll(' ', INTERNAL_SPACE).replaceAll(':', INTERNAL_COLON)
}

/** 反转义内容：\x11→空格，\x03→冒号 */
function unescapeContent(s: string): string {
  return s.replaceAll(INTERNAL_SPACE, ' ').replaceAll(INTERNAL_COLON, ':')
}

/** 日期标签 */
export interface DateTag {
  type: 'date'
  raw: string
  date: Date
  note?: string
  /** 显示用日期字符串（yyyy-MM-dd） */
  displayDate: string
  /** 倒计时天数（正数=未来，负数=已过期） */
  countdownDays: number
  /** 是否已过期 */
  expired: boolean
}

/** 文本标签 */
export interface TextTag {
  type: 'text'
  raw: string
  text: string
}

export type ProfileTag = DateTag | TextTag

/** 解析后的 Nickname 结果 */
export interface ParsedNickname {
  /** 纯名称（不含标签） */
  name: string
  /** 标签列表 */
  tags: ProfileTag[]
}

/**
 * 解析 Nickname 字符串，拆分为纯名称 + 标签列表
 */
export function parseNickname(fullNickname: string | null | undefined): ParsedNickname {
  if (!fullNickname || fullNickname.trim() === '') {
    return { name: '', tags: [] }
  }

  const tags: ProfileTag[] = []
  const matches = fullNickname.matchAll(TAG_REGEX)
  for (const m of matches) {
    const raw = m[1]
    if (raw) {
      tags.push(parseTag(raw))
    }
  }

  // 从原字符串中移除标签和分隔符，得到纯名称
  let name = fullNickname
    .replaceAll(TAG_REGEX, '')
    .replaceAll(TAG_SEPARATOR, '')
    .trim()
  // 合并多余空格
  name = name.replaceAll(/\s+/g, ' ')

  return { name, tags }
}

/**
 * 解析单个标签字符串
 */
function parseTag(raw: string): ProfileTag {
  const lower = raw.toLowerCase()
  if (lower.startsWith('d:')) {
    return parseDateTag(raw)
  } else if (lower.startsWith('t:')) {
    return parseTextTag(raw)
  }
  // fallback：当作文本标签
  return parseTextTag('t:' + raw)
}

/**
 * 解析日期标签
 * 格式：d:YYMMDD 或 d:YYMMDD:备注 或 d:YYYYMMDD
 */
function parseDateTag(raw: string): DateTag {
  const parts = raw.split(':')
  // parts[0]='d', parts[1]=date, parts[2...]=note
  if (parts.length < 2) {
    return createDateTag(new Date(), undefined, raw)
  }

  const dateStr = parts[1].trim()
  let date: Date

  if (dateStr.length >= 8) {
    // YYYYMMDD 格式
    const clean = dateStr.substring(0, 8)
    const y = parseInt(clean.substring(0, 4), 10)
    const m = parseInt(clean.substring(4, 6), 10)
    const d = parseInt(clean.substring(6, 8), 10)
    date = new Date(y, m - 1, d)
  } else if (dateStr.length === 6) {
    // YYMMDD 格式
    const y = parseInt(dateStr.substring(0, 2), 10)
    const m = parseInt(dateStr.substring(2, 4), 10)
    const d = parseInt(dateStr.substring(4, 6), 10)
    date = new Date(2000 + y, m - 1, d)
  } else {
    date = new Date()
  }

  let note: string | undefined
  if (parts.length > 2) {
    note = unescapeContent(parts.slice(2).join(':'))
  }

  return createDateTag(date, note, raw)
}

function createDateTag(date: Date, note: string | undefined, raw: string): DateTag {
  const now = new Date()
  const target = new Date(date.getFullYear(), date.getMonth(), date.getDate())
  const diffMs = target.getTime() - now.getTime()
  const countdownDays = Math.ceil(diffMs / (1000 * 60 * 60 * 24))

  return {
    type: 'date',
    raw,
    date,
    note,
    displayDate: formatDate(date),
    countdownDays,
    expired: countdownDays < 0,
  }
}

/**
 * 解析文本标签
 * 格式：t:文本内容
 */
function parseTextTag(raw: string): TextTag {
  const content = raw.substring(2)
  return {
    type: 'text',
    raw,
    text: unescapeContent(content),
  }
}

/**
 * 格式化日期为 yyyy-MM-dd
 */
function formatDate(date: Date): string {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

/**
 * 将日期格式化为 YYMMDD（用于标签编码）
 */
function formatYYMMDD(date: Date): string {
  const y = String(date.getFullYear()).slice(-2)
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}${m}${d}`
}

/**
 * 创建日期标签的 raw 字符串
 */
export function createDateTagRaw(date: Date, note?: string): string {
  const datePart = formatYYMMDD(date)
  if (note && note.trim()) {
    return `d:${datePart}:${escapeContent(note.trim())}`
  }
  return `d:${datePart}`
}

/**
 * 创建文本标签的 raw 字符串
 */
export function createTextTagRaw(text: string): string {
  return `t:${escapeContent(text.trim())}`
}

/**
 * 将名称 + 标签列表格式化为完整 Nickname 字符串
 */
export function formatNickname(name: string, tags: ProfileTag[]): string {
  if (tags.length === 0) return name
  const joinedTags = tags.map((t) => t.raw).join(' ')
  return `${name}${TAG_SEPARATOR}${joinedTags}`
}

/**
 * 限制：最多 2 个标签（1 日期标签 + 1 文本标签）
 */
export const MAX_TAGS = 2
