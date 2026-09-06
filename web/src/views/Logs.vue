<script setup lang="ts">
import { ref, h, watch, onMounted, onUnmounted } from 'vue'
import { ElButton, ElButtonGroup } from 'element-plus'
import LogsHistoryTab from '../components/LogsHistoryTab.vue'
import LogsRealtimeTab from '../components/LogsRealtimeTab.vue'
// import LogsNetworkTab from '../components/LogsNetworkTab.vue'  // 暂时隐藏
import LogsDetailDrawer from '../components/LogsDetailDrawer.vue'
import { useHeaderActionsStore } from '../stores/headerActions'
import type { LogEntry } from '../components/LogLine.vue'

// export type LogsTab = 'history' | 'realtime' | 'network'
export type LogsTab = 'history' | 'realtime'

const activeTab = ref<LogsTab>('realtime')
const detailVisible = ref(false)
const selectedLog = ref<LogEntry | null>(null)

const headerActions = useHeaderActionsStore()

const tabs: { id: LogsTab; label: string }[] = [
  { id: 'history',  label: '历史' },
  { id: 'realtime', label: '实时' },
]

function openDetail(log: LogEntry) {
  selectedLog.value = log
  detailVisible.value = true
}

function updateHeaderActions() {
  headerActions.setActions(
    h(ElButtonGroup, null, () =>
      tabs.map(tab =>
        h(
          ElButton,
          {
            key: tab.id,
            type: activeTab.value === tab.id ? 'primary' : '',
            onClick: () => { activeTab.value = tab.id },
          },
          () => tab.label
        )
      )
    )
  )
}

onMounted(() => {
  updateHeaderActions()
})
watch(activeTab, updateHeaderActions)

onUnmounted(() => {
  headerActions.clear()
})
</script>

<template>
  <div class="logs-page">
    <!-- 面板容器 -->
    <div class="logs-panel-wrapper">
      <!-- 内容区 -->
      <div class="logs-content">
        <LogsHistoryTab
          v-show="activeTab === 'history'"
          @open-detail="openDetail"
        />
        <LogsRealtimeTab
          v-show="activeTab === 'realtime'"
          :active="activeTab === 'realtime'"
          @open-detail="openDetail"
        />
        <!-- LogsNetworkTab 暂时隐藏，待后续集成 -->
      </div>
    </div>

    <!-- 详情抽屉（共享） -->
    <LogsDetailDrawer
      v-model="detailVisible"
      :log="selectedLog"
    />
  </div>
</template>

<style scoped>
.logs-page {
  display: flex;
  flex-direction: column;
  height: calc(100svh - 56px - 24px);
  min-height: 0;
  overflow: hidden;
}

.logs-panel-wrapper {
  border: 1px solid var(--border, #E9E9E9);
  border-radius: 8px;
  background: var(--card, #FFFFFF);
  box-shadow: var(--console-shadow-sm);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1;
}

.logs-content {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
</style>
