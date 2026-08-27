import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import App from './App.vue'
import { debugCollector } from './debug/collector'
import 'element-plus/dist/index.css'
// Element Plus: 暗色主题变量（全局需要）
import 'element-plus/theme-chalk/dark/css-vars.css'
import './style.css'
import { ElLoading } from 'element-plus'
import { loadOverridesFromIDB } from './composables/useOperatorIcon'

let bootFinished = false
let bootErrorOverlay: HTMLDivElement | null = null

function isChunkLoadLikeError(err: unknown) {
  const msg = (err instanceof Error ? err.message : typeof err === 'string' ? err : '') || ''
  return /Loading chunk|ChunkLoadError|dynamically imported module|Importing a module script failed|Failed to fetch dynamically imported module/i.test(
    msg
  )
}

function isScriptLoadErrorEvent(e: Event) {
  const t = e.target
  return t instanceof HTMLScriptElement && !!t.src
}

function showBootError(_err: unknown, opts: { force?: boolean } = {}) {
  if (bootFinished && !opts.force) return
  if (bootErrorOverlay) return
  try {
    const el = document.createElement('div')
    el.style.position = 'fixed'
    el.style.inset = '0'
    el.style.zIndex = '99999'
    el.style.background = '#000'
    el.style.color = '#fff'
    el.style.display = 'flex'
    el.style.alignItems = 'center'
    el.style.justifyContent = 'center'
    el.style.fontSize = '48px'
    el.style.fontWeight = 'bold'
    el.textContent = '404'
    bootErrorOverlay = el
    document.body.appendChild(el)
  } catch {
    // 忽略覆盖层渲染自身失败
  }
}

window.addEventListener('error', (e) => {
  const ev = e as ErrorEvent
  debugCollector.recordJsError(ev.error || ev.message, 'window.error')
  if (isScriptLoadErrorEvent(e)) {
    const t = e.target as HTMLScriptElement
    showBootError(`脚本加载失败：${String(t?.src || '')}`, { force: true })
    return
  }
  if (!bootFinished || isChunkLoadLikeError(ev.error || ev.message)) {
    showBootError(ev.error || ev.message, { force: isChunkLoadLikeError(ev.error || ev.message) })
  }
})

window.addEventListener('unhandledrejection', (e) => {
  const ev = e as PromiseRejectionEvent
  debugCollector.recordJsError(ev.reason, 'unhandledrejection')
  if (!bootFinished || isChunkLoadLikeError(ev.reason)) {
    showBootError(ev.reason, { force: isChunkLoadLikeError(ev.reason) })
  }
})

const app = createApp(App)

app.config.errorHandler = (err) => {
  debugCollector.recordJsError(err, 'vue.errorHandler')
  if (!bootFinished || isChunkLoadLikeError(err)) {
    showBootError(err, { force: isChunkLoadLikeError(err) })
  }
}

app.use(createPinia())
app.use(router)
app.use(ElLoading)

// 启动时加载用户覆盖的图标选择到内存缓存
// （getIconInfo 同步接口需要 override 数据在内存中）
loadOverridesFromIDB().catch(() => { /* 非关键路径，失败静默 */ })

router.onError((err) => {
  showBootError(err, { force: true })
})

app.mount('#app')
bootFinished = true
