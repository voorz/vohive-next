<script setup lang="ts">
import type { FrontendNotification } from '../composables/useNotificationStream'

defineProps<{
  notification: FrontendNotification
}>()

defineEmits<{
  dismiss: []
}>()
</script>

<template>
  <div class="call-overlay" @click="$emit('dismiss')">
    <div class="call-card" @click.stop>
      <div class="call-icon-ring">
        <div class="call-icon-inner">
          <svg viewBox="0 0 24 24" fill="none" class="call-icon-svg">
            <path d="M6.62 10.79a15.05 15.05 0 0 0 6.59 6.59l2.2-2.2a1 1 0 0 1 1.02-.24a11.36 11.36 0 0 0 3.57.57a1 1 0 0 1 1 1V20a1 1 0 0 1-1 1A17 17 0 0 1 3 4a1 1 0 0 1 1-1h3.5a1 1 0 0 1 1 1a11.36 11.36 0 0 0 .57 3.57a1 1 0 0 1-.24 1.02l-2.2 2.2z" fill="currentColor"/>
          </svg>
        </div>
      </div>
      <div class="call-title">{{ notification.title || '来电通知' }}</div>
      <pre class="call-body">{{ notification.body }}</pre>
      <div class="call-meta" v-if="notification.device_name || notification.device_id">
        {{ notification.device_name || notification.device_id }}
      </div>
      <button class="call-dismiss-btn" @click="$emit('dismiss')">关闭</button>
    </div>
  </div>
</template>

<style scoped>
.call-overlay {
  position: fixed;
  inset: 0;
  z-index: 100000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(4px);
  animation: call-fade-in 0.2s ease;
}

.call-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 40px 48px;
  background: var(--card, #1e1e2e);
  border: 1px solid var(--border, #333346);
  border-radius: 16px;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.4);
  animation: call-slide-in 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.call-icon-ring {
  width: 72px;
  height: 72px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in oklab, var(--brand, #6366f1) 15%, transparent);
  animation: call-pulse 1.5s ease-in-out infinite;
}

.call-icon-inner {
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--brand, #6366f1);
}

.call-icon-svg {
  width: 28px;
  height: 28px;
}

.call-title {
  font-size: 20px;
  font-weight: 700;
  color: var(--foreground, #cdd6f4);
}

.call-body {
  font-size: 15px;
  color: var(--muted-foreground, #a6adc8);
  line-height: 1.6;
  white-space: pre-wrap;
  font-family: inherit;
  margin: 0;
  text-align: center;
}

.call-meta {
  font-size: 13px;
  color: var(--muted-foreground, #585b70);
}

.call-dismiss-btn {
  margin-top: 8px;
  padding: 8px 32px;
  border: 1px solid var(--border, #333346);
  border-radius: 8px;
  background: transparent;
  color: var(--foreground, #cdd6f4);
  font-size: 14px;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s;
}

.call-dismiss-btn:hover {
  background: var(--accent, #313244);
  border-color: var(--brand, #6366f1);
}

@keyframes call-fade-in {
  from { opacity: 0; }
  to { opacity: 1; }
}

@keyframes call-slide-in {
  from { opacity: 0; transform: translateY(-20px) scale(0.96); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}

@keyframes call-pulse {
  0%, 100% { transform: scale(1); }
  50% { transform: scale(1.08); }
}
</style>
