<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useHeaderActionsStore } from '../stores/headerActions'
import ErrorBoundary from '../components/ErrorBoundary.vue'
import { debugCollector } from '../debug/collector'
import { useSiteConfig } from '../composables/useSiteConfig'
import {
  Mail24Regular,
  Settings24Regular,
  SignOut24Regular,
  Board24Regular,
  Phone24Regular,
  Globe24Regular,
  DocumentText24Regular,
  Desktop24Regular,
  WeatherSunny24Regular,
  WeatherMoon24Regular,
  ArrowSync24Regular,
  ChevronLeft24Regular,
  ChevronRight24Regular
} from '@vicons/fluent'

type ThemeMode = 'auto' | 'light' | 'dark'

defineProps<{
  theme: ThemeMode
  isDark: boolean
}>()

const emit = defineEmits<{
  'set-theme': [mode: ThemeMode]
}>()

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const headerActions = useHeaderActionsStore()
const { siteConfig, load: loadSiteConfig } = useSiteConfig()
const debugOpen = ref(false)
const refreshing = ref(false)
const lang = ref(localStorage.getItem('lang') || 'zh')
const isSmallScreen = ref(false)
const collapsed = ref(false)
const DebugPanel = defineAsyncComponent(() => import('../components/DebugPanel.vue'))

function syncScreenSize() {
  if (typeof window === 'undefined') return
  const mq = window.matchMedia('(max-width: 960px)')
  isSmallScreen.value = mq.matches
  collapsed.value = mq.matches
}

const menuItems = [
  { path: '/', label: '仪表盘', icon: Board24Regular },
  { path: '/devices', label: '设备管理', icon: Phone24Regular },
  { path: '/proxy', label: '代理管理', icon: Globe24Regular },
  { path: '/sms', label: '短信中心', icon: Mail24Regular },
  { path: '/logs', label: '实时日志', icon: DocumentText24Regular },
  { path: '/settings', label: '系统设置', icon: Settings24Regular }
]

const themeOptions = [
  { value: 'auto' as ThemeMode, icon: Desktop24Regular, label: '自动' },
  { value: 'light' as ThemeMode, icon: WeatherSunny24Regular, label: '浅色' },
  { value: 'dark' as ThemeMode, icon: WeatherMoon24Regular, label: '深色' }
]

const currentNavItem = computed(() => {
  const section = route.path.split('/').filter(Boolean)[0]
  const item = menuItems.find((m) => {
    if (m.path === '/') return !section
    return m.path === '/' + section
  })
  return item ?? menuItems[0]
})

const currentNavTitle = computed(() => currentNavItem.value?.label ?? '')
const currentNavIcon = computed(() => currentNavItem.value?.icon ?? Board24Regular)

function isActive(path: string): boolean {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}

async function handleLogout() {
  const { ElMessageBox } = await import('element-plus')
  const confirmed = await ElMessageBox.confirm('确认退出登录？', '提示', {
    confirmButtonText: '退出',
    cancelButtonText: '取消',
    type: 'warning'
  })
    .then(() => true)
    .catch(() => false)
  if (!confirmed) return
  auth.logout()
  router.push('/login')
}

function handleRefresh() {
  if (refreshing.value) return
  refreshing.value = true
  setTimeout(() => {
    refreshing.value = false
  }, 800)
}

function onKeydown(e: KeyboardEvent) {
  if (e.ctrlKey && e.shiftKey && String(e.key || '').toLowerCase() === 'd') {
    e.preventDefault()
    debugOpen.value = !debugOpen.value
    localStorage.setItem('debug_panel_open', debugOpen.value ? '1' : '0')
  }
}

function toggleCollapse() {
  collapsed.value = !collapsed.value
}

onMounted(() => {
  syncScreenSize()
  window.addEventListener('resize', syncScreenSize, { passive: true })
  const saved = localStorage.getItem('debug_panel_open')
  debugOpen.value = saved === '1'
  window.addEventListener('keydown', onKeydown)
  loadSiteConfig()
})

onUnmounted(() => {
  window.removeEventListener('resize', syncScreenSize)
  window.removeEventListener('keydown', onKeydown)
})

watch(
  () => debugOpen.value,
  (v) => {
    localStorage.setItem('debug_panel_open', v ? '1' : '0')
  }
)

watch(
  () => route.fullPath,
  () => {
    if (isSmallScreen.value) collapsed.value = true
  }
)

watch(
  () => debugCollector.openPanelRequestAt.value,
  (ts) => {
    if (!ts) return
    debugOpen.value = true
  }
)

watch(lang, (v) => {
  localStorage.setItem('lang', v)
})
</script>

<template>
  <div class="app-shell" :class="{ 'is-mobile': isSmallScreen }" v-if="auth.isAuthenticated && route.name !== 'Login'">
    <!-- 遮罩层（窄屏展开侧栏时显示） -->
    <div
      v-if="isSmallScreen && !collapsed"
      class="sidebar-overlay"
      @click="collapsed = true"
    ></div>
    <aside class="sidebar" :class="{ 'mobile-hidden': isSmallScreen && collapsed }">
      <!-- 品牌区 -->
      <div class="brand">
        <div v-if="siteConfig.has_logo" class="brand-logo-wrap">
          <img :src="'/api/site/logo'" alt="Logo" class="brand-logo-img" />
        </div>
        <div v-else class="brand-mark">V</div>
        <div class="brand-text">
          <div class="brand-name">{{ siteConfig.name }}</div>
          <div class="brand-subtitle">{{ siteConfig.subtitle }}</div>
        </div>
      </div>

      <!-- 导航 -->
      <nav class="sidebar-nav" aria-label="主导航">
        <router-link
          v-for="item in menuItems"
          :key="item.path"
          :to="item.path"
          class="nav-item"
          :class="{ active: isActive(item.path) }"
        >
          <component :is="item.icon" class="nav-icon" />
          <span>{{ item.label }}</span>
        </router-link>
      </nav>

      <!-- 底部功能区 -->
      <div class="sidebar-footer">
        <!-- 语言选择 -->
        <div class="language-select">
          <span class="language-select-label">语言</span>
          <select v-model="lang" class="language-select-trigger" aria-label="语言">
            <option value="zh">中文</option>
            <option value="en">English</option>
          </select>
        </div>

        <!-- 主题分段控件 -->
        <div class="theme-control">
          <span>主题</span>
          <div class="theme-segmented-control" role="radiogroup" aria-label="主题">
            <button
              v-for="opt in themeOptions"
              :key="opt.value"
              type="button"
              class="theme-segment"
              :class="{ active: theme === opt.value }"
              role="radio"
              :aria-checked="theme === opt.value"
              :title="opt.label"
              @click="emit('set-theme', opt.value)"
            >
              <component :is="opt.icon" class="theme-icon" />
            </button>
          </div>
        </div>

        <!-- 运行状态 -->
        <div class="runtime-status">
          <span class="status-dot ok"></span>
          <span>运行中</span>
        </div>

        <!-- 操作按钮 -->
        <div class="sidebar-footer-actions">
          <button
            type="button"
            class="footer-btn icon-btn"
            :disabled="refreshing"
            title="刷新"
            @click="handleRefresh"
          >
            <component :is="ArrowSync24Regular" class="footer-icon" :class="{ spin: refreshing }" />
          </button>
          <button type="button" class="footer-btn" @click="handleLogout">
            <component :is="SignOut24Regular" class="footer-icon" />
            <span>登出</span>
          </button>
        </div>
      </div>
    </aside>

    <!-- 主内容区 -->
    <div class="main-region">
      <header class="shell-header">
        <div class="shell-header-title">
          <button
            v-if="isSmallScreen"
            type="button"
            class="sidebar-toggle-btn"
            :title="collapsed ? '展开侧栏' : '收起侧栏'"
            @click="toggleCollapse"
          >
            <component :is="collapsed ? ChevronRight24Regular : ChevronLeft24Regular" class="toggle-icon" />
          </button>
          <component :is="currentNavIcon" class="header-icon" />
          <h1>{{ currentNavTitle }}</h1>
        </div>
        <div class="shell-header-actions">
          <component :is="headerActions.actions" v-if="headerActions.actions" />
        </div>
      </header>

      <main class="main">
        <div class="main-inner">
          <router-view v-slot="{ Component, route: r }">
            <ErrorBoundary v-if="Component" title="页面渲染失败">
              <component :is="Component" :key="r.path" />
            </ErrorBoundary>
          </router-view>
        </div>
      </main>
    </div>

    <DebugPanel v-model="debugOpen" />
  </div>
</template>

<style scoped>
/* ============================================================
   AppShell 布局 — 迁移自 open-connector/web/src/styles/shell.css
   ============================================================ */

.app-shell {
  display: grid;
  grid-template-columns: 15.5rem minmax(0, 1fr);
  min-height: 100vh;
  min-height: 100svh;
}

/* ---------- 侧栏 ---------- */

.sidebar {
  position: sticky;
  top: 0;
  align-self: start;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  height: 100vh;
  height: 100svh;
  min-height: 0;
  border-right: 1px solid var(--sidebar-border);
  background: var(--sidebar);
  color: var(--sidebar-foreground);
}

/* ---------- 品牌区 ---------- */

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  padding: 0 6px;
}

.sidebar > .brand {
  min-height: 56px;
  padding: 0 20px;
  border-bottom: 1px solid var(--sidebar-border);
}

.brand-logo-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  flex-shrink: 0;
}

.brand-logo-img {
  max-height: 38px;
  max-width: 100%;
  object-fit: contain;
}

.brand-mark {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  font-weight: 700;
  font-size: 16px;
  color: var(--foreground);
  flex-shrink: 0;
}

.brand-text {
  min-width: 0;
}

.brand-name {
  line-height: 1.2;
  font-weight: 680;
  font-size: 15px;
  color: var(--sidebar-foreground);
}

.brand-subtitle {
  margin-top: 2px;
  font-size: 12px;
  line-height: 1.2;
  color: var(--muted-foreground);
}

/* ---------- 导航 ---------- */

.sidebar-nav {
  display: grid;
  align-content: start;
  min-height: 0;
  gap: 4px;
  overflow-y: auto;
  padding: 16px 12px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 9px;
  min-height: 36px;
  padding: 0 12px;
  border-radius: var(--radius-lg);
  color: var(--sidebar-foreground);
  text-align: left;
  text-decoration: none;
  font-size: 14px;
  transition: background 150ms ease, color 150ms ease;
}

.nav-item:hover,
.nav-item.active {
  background: var(--sidebar-accent);
  color: var(--sidebar-accent-foreground);
}

.nav-item.active {
  font-weight: 560;
}

.nav-icon {
  width: 16px;
  height: 16px;
  flex: 0 0 auto;
}

/* ---------- 侧栏底部 ---------- */

.sidebar-footer {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px 8px;
  padding: 12px;
  border-top: 1px solid var(--sidebar-border);
}

/* 语言选择器 */

.language-select {
  display: grid;
  grid-column: 1 / -1;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 8px;
}

.language-select-label,
.theme-control > span {
  color: var(--muted-foreground);
  font-size: 12px;
  font-weight: 620;
}

.language-select-trigger {
  width: 100%;
  height: 30px;
  padding: 0 8px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--background);
  font-size: 13px;
  color: var(--foreground);
  cursor: pointer;
  outline: none;
  transition: border-color 150ms ease;
}

.language-select-trigger:focus-visible {
  border-color: var(--ring);
}

/* 主题分段控件 */

.theme-control {
  display: grid;
  grid-column: 1 / -1;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 8px;
}

.theme-segmented-control {
  display: inline-grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  min-width: 0;
  padding: 2px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--muted);
}

.theme-segment {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 0;
  height: 26px;
  border: 0;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  transition: color 150ms ease, background 150ms ease;
}

.theme-segment:hover,
.theme-segment.active {
  color: var(--foreground);
}

.theme-segment.active {
  background: var(--background);
  box-shadow: var(--console-shadow-sm);
}

.theme-icon {
  width: 14px;
  height: 14px;
}

/* 运行状态 */

.runtime-status {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 8px;
  color: var(--muted-foreground);
  font-size: 13px;
}

.runtime-status span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-dot {
  flex: 0 0 auto;
  width: 8px;
  height: 8px;
  border-radius: 999px;
}

.status-dot.ok {
  background: var(--success);
}

.status-dot.error {
  background: var(--destructive);
}

/* 操作按钮 */

.sidebar-footer-actions {
  display: flex;
  gap: 8px;
}

.footer-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 30px;
  padding: 0 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--background);
  box-shadow: var(--console-shadow-sm);
  font-size: 13px;
  color: var(--foreground);
  cursor: pointer;
  transition: background 150ms ease, transform 150ms ease;
}

.footer-btn:hover {
  background: var(--accent);
}

.footer-btn:active {
  transform: translateY(0.5px);
}

.footer-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.footer-btn.icon-btn {
  width: 30px;
  padding: 0;
}

.footer-icon {
  width: 15px;
  height: 15px;
  flex-shrink: 0;
}

/* ---------- 主内容区 ---------- */

.main-region {
  min-width: 0;
  height: 100vh;
  height: 100svh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.shell-header {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-width: 0;
  height: 56px;
  padding: 0 24px;
  border-bottom: 1px solid var(--border);
  background: color-mix(in oklab, var(--background) 95%, transparent);
  backdrop-filter: blur(8px);
}

.shell-header-title {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 10px;
}

.header-icon {
  width: 16px;
  height: 16px;
  flex: 0 0 auto;
  color: var(--foreground);
}

.shell-header-title h1 {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  line-height: 1.45;
  font-weight: 660;
}

.shell-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 30px;
}

.main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
}

.main-inner {
  max-width: var(--page-max-width, 1240px);
  margin: 0 auto;
  padding: 24px 24px 24px;
}

/* ---------- 侧栏收起按钮（仅窄屏可见） ---------- */

.sidebar-toggle-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  flex: 0 0 auto;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--foreground);
  cursor: pointer;
  transition: background 150ms ease, border-color 150ms ease;
}

.sidebar-toggle-btn:hover {
  background: var(--accent);
  border-color: var(--border);
}

.toggle-icon {
  width: 16px;
  height: 16px;
}

/* ---------- 遮罩层 ---------- */

.sidebar-overlay {
  position: fixed;
  inset: 0;
  z-index: 29;
  background: rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(2px);
}

/* ---------- 窄屏侧栏 overlay 模式 ---------- */

@media (max-width: 960px) {
  .app-shell {
    grid-template-columns: minmax(0, 1fr);
  }

  .sidebar {
    position: fixed;
    top: 0;
    left: 0;
    z-index: 30;
    width: 15.5rem;
    height: 100vh;
    height: 100svh;
    transform: translateX(0);
    transition: transform 200ms ease;
    box-shadow: 4px 0 24px rgba(0, 0, 0, 0.08);
  }

  .sidebar.mobile-hidden {
    transform: translateX(-100%);
  }

  .main-inner {
    padding: 18px 24px 18px;
  }

  .shell-header {
    padding: 0 16px;
  }
}

@media (max-width: 640px) {
  .main-inner {
    padding: 18px 24px 18px;
  }

  .shell-header {
    padding: 0 12px;
  }
}
</style>
