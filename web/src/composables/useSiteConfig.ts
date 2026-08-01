import { ref } from 'vue'
import { systemService } from '../services/system'

export type SiteConfig = {
  name: string
  subtitle: string
  has_logo: boolean
  has_favicon: boolean
}

const siteConfig = ref<SiteConfig>({
  name: 'VoHive',
  subtitle: 'VoWiFi 管理控制台',
  has_logo: false,
  has_favicon: false,
})

let loaded = false

function applyToDOM() {
  document.title = siteConfig.value.name
  // 如果有自定义 favicon，更新 link 标签
  if (siteConfig.value.has_favicon) {
    const link = document.querySelector('link[rel="icon"]') as HTMLLinkElement | null
    if (link) {
      link.setAttribute('href', '/api/site/favicon')
    }
  }
}

export function useSiteConfig() {
  async function load() {
    if (loaded) return
    loaded = true
    try {
      const res = await systemService.getSiteConfig()
      if (res.ok) {
        siteConfig.value = res.data
        applyToDOM()
      }
    } catch {
      // keep defaults
    }
  }

  async function reload() {
    loaded = false
    await load()
  }

  return { siteConfig, load, reload, applyToDOM }
}
