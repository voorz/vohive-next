import BaiwangIcon from '../assets/svgs/baiwang-600x800.svg'
import ModemIcon from '../assets/svgs/modem-600x800.svg'
import ReaderIcon from '../assets/svgs/estk-600x800.svg'

/**
 * 根据设备类型/厂商返回对应图标 SVG。
 * 用于 ModuleSearchDialog（发现设备卡片）和 ModuleListPanel（设备列表卡片）。
 *
 * 判断规则：
 * - esim_transport === 'pcsc' 或 type === 'pcsc' → 读卡器图标
 * - manufacturer 包含 BAIWANG → 百望图标
 * - 其他 → 通用模组图标
 */
export function getDeviceIcon(opts: { type?: string; esim_transport?: string; manufacturer?: string }): string {
  if (opts.type === 'pcsc' || opts.esim_transport === 'pcsc') return ReaderIcon
  if (opts.manufacturer?.toUpperCase() === 'BAIWANG') return BaiwangIcon
  return ModemIcon
}
