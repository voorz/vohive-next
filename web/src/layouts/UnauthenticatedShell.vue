<script setup lang="ts">
// 主题切换由 App.vue 管理，登录页不显示主题按钮
</script>

<template>
  <div class="h-screen flex items-center justify-center bg-[var(--muted)] transition-colors duration-300">
    <router-view v-slot="{ Component }">
      <Suspense>
        <template #default>
          <component :is="Component" />
        </template>
<template #fallback>
<div></div>
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
  min-width: 32px;
  height: 32px;
  padding: 0 6px;
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
  width: 18px;
  height: 18px;
}
</style>
