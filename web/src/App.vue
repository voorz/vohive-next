<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from './stores/auth'
import zhCn from 'element-plus/es/locale/lang/zh-cn'

type ThemeMode = 'auto' | 'light' | 'dark'

const THEME_KEY = 'theme'

const route = useRoute()
const auth = useAuthStore()

const themeMode = ref<ThemeMode>(
  ['auto', 'light', 'dark'].includes(localStorage.getItem(THEME_KEY) || '')
    ? (localStorage.getItem(THEME_KEY) as ThemeMode)
    : 'auto'
)
const systemPrefersDark = ref(false)
const isDark = computed(() =>
  themeMode.value === 'auto' ? systemPrefersDark.value : themeMode.value === 'dark'
)

function setTheme(mode: ThemeMode) {
  themeMode.value = mode
  localStorage.setItem(THEME_KEY, mode)
}

function updateHtmlClass() {
  if (isDark.value) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

onMounted(() => {
  if (typeof window !== 'undefined') {
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    systemPrefersDark.value = mq.matches
    mq.addEventListener('change', (e) => {
      systemPrefersDark.value = e.matches
    })

    // 跨标签页主题同步
    window.addEventListener('storage', (e: StorageEvent) => {
      if (e.key === THEME_KEY && e.newValue) {
        const mode = e.newValue as ThemeMode
        if (['auto', 'light', 'dark'].includes(mode)) {
          themeMode.value = mode
        }
      }
    })
  }
  updateHtmlClass()
})

watch(isDark, () => updateHtmlClass())

const AuthenticatedShell = defineAsyncComponent(() => import('./layouts/AuthenticatedShell.vue'))
const UnauthenticatedShell = defineAsyncComponent(() => import('./layouts/UnauthenticatedShell.vue'))
const shell = computed(() =>
  auth.isAuthenticated && route.name !== 'Login' ? AuthenticatedShell : UnauthenticatedShell
)
</script>

<template>
  <el-config-provider :locale="zhCn">
    <div class="h-screen w-screen overflow-hidden bg-[#F8F8F8] dark:bg-[#111111] text-gray-900 dark:text-gray-100 font-sans selection:bg-indigo-500 selection:text-white transition-colors duration-300">
      <Suspense>
        <template #default>
          <component :is="shell" :is-dark="isDark" :theme="themeMode" @set-theme="setTheme" />
        </template>
        <template #fallback>
          <div></div>
        </template>
      </Suspense>
    </div>
  </el-config-provider>
</template>

<style>
/* Custom Scrollbar */
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}
::-webkit-scrollbar-track {
  background: transparent;
}
::-webkit-scrollbar-thumb {
  background: #cbd5e1;
  border-radius: 4px;
}
.dark ::-webkit-scrollbar-thumb {
  background: #334155;
}
::-webkit-scrollbar-thumb:hover {
  background: #94a3b8;
}
.dark ::-webkit-scrollbar-thumb:hover {
  background: #475569;
}
</style>
