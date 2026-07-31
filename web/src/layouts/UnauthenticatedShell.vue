<script setup lang="ts">
import LoadingScreen from '../components/LoadingScreen.vue'
import {
  Desktop24Regular,
  WeatherSunny24Regular,
  WeatherMoon24Regular
} from '@vicons/fluent'

type ThemeMode = 'auto' | 'light' | 'dark'

defineProps<{
  theme: ThemeMode
  isDark: boolean
}>()

const emit = defineEmits<{
  'set-theme': [mode: ThemeMode]
}>()

const themeOptions = [
  { value: 'auto' as ThemeMode, icon: Desktop24Regular, label: '自动' },
  { value: 'light' as ThemeMode, icon: WeatherSunny24Regular, label: '浅色' },
  { value: 'dark' as ThemeMode, icon: WeatherMoon24Regular, label: '深色' }
]
</script>

<template>
  <div class="h-screen flex items-center justify-center bg-[var(--background)] transition-colors duration-300">
    <div class="absolute top-4 right-4 z-50">
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
    <router-view v-slot="{ Component }">
      <Suspense>
        <template #default>
          <component :is="Component" />
        </template>
        <template #fallback>
          <LoadingScreen />
        </template>
      </Suspense>
    </router-view>
  </div>
</template>

<style scoped>
.theme-segmented-control {
  display: inline-grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
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
</style>
