<script setup lang="ts">
import { Call24Regular, Delete24Regular } from '@vicons/fluent'

defineProps<{
  contacts: Array<{
    id: string
    name: string
    number: string
    createdAt: number
  }>
}>()

defineEmits<{
  call: [number: string]
  delete: [id: string]
  add: []
}>()
</script>

<template>
  <div class="voice-contacts">
    <el-empty v-if="contacts.length === 0" description="暂无联系人" :image-size="60">
      <el-button type="primary" size="small" @click="$emit('add')">添加联系人</el-button>
    </el-empty>
    <div v-else class="contacts-list">
      <div
        v-for="contact in contacts"
        :key="contact.id"
        class="contact-row"
      >
        <div class="contact-avatar">{{ contact.name.charAt(0).toUpperCase() }}</div>
        <div class="contact-info">
          <div class="contact-name">{{ contact.name }}</div>
          <div class="contact-number">{{ contact.number }}</div>
        </div>
        <div class="contact-actions" @click.stop>
          <button class="contact-action-btn" title="拨打" @click="$emit('call', contact.number)">
            <el-icon size="16"><Call24Regular /></el-icon>
          </button>
          <button class="contact-action-btn danger" title="删除" @click="$emit('delete', contact.id)">
            <el-icon size="16"><Delete24Regular /></el-icon>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.voice-contacts {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.contacts-list {
  display: flex;
  flex-direction: column;
}

.contact-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  transition: background 0.12s;
}

.contact-row:hover {
  background: var(--accent);
}

.contact-avatar {
  width: 36px;
  height: 36px;
  border-radius: 999px;
  background: color-mix(in oklab, var(--brand) 15%, transparent);
  color: var(--brand);
  font-size: 14px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.contact-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.contact-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.contact-number {
  font-size: 12px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
}

.contact-actions {
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.15s;
  flex-shrink: 0;
}

.contact-row:hover .contact-actions {
  opacity: 1;
}

.contact-action-btn {
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: all 0.15s;
}

.contact-action-btn:hover {
  background: var(--accent);
  color: var(--foreground);
}

.contact-action-btn.danger:hover {
  background: #ef4444;
  color: #fff;
}
</style>
