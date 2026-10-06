<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useCarrierStore } from '../stores/carrier'
import CarrierEditArea from './CarrierEditArea.vue'
import ListSkeleton from './ListSkeleton.vue'
import EmptyState from './EmptyState.vue'
import { Add24Regular } from '@vicons/fluent'
import CarrierIcon from './CarrierIcon.vue'
import CountryFlag from './CountryFlag.vue'
import { mccToIso, loadPlmnInfo, getPlmnInfo } from '../composables/plmn-info'

const emit = defineEmits<{
  'open-search': []
}>()

const store = useCarrierStore()
const { detail, selectedCarrier, detailLoading, carriers, selectedKey, pullLoading, pullResult } = storeToRefs(store)

// 手动拉取表单
const pullMcc = ref('')
const pullMnc = ref('')

async function handlePull() {
  if (!pullMcc.value.trim() || !pullMnc.value.trim()) return
  await store.pullCarrierYAML(pullMcc.value.trim(), pullMnc.value.trim())
}

// 窄屏下拉选择
function handleSelectChange(value: string) {
  store.selectCarrier(value)
}

const selectValue = computed(() => selectedKey.value)

// 触发加载
onMounted(() => loadPlmnInfo())

// 选中运营商变化时查询 PLMN 信息（直接用 MCC+MNC，与 CarrierIcon 同源）
const plmnInfo = computed(() => {
  const mcc = selectedCarrier.value?.mcc
  const mnc = selectedCarrier.value?.mnc
  if (!mcc) return null
  const plmn = mnc ? `${mcc}-${mnc}` : mcc
  return getPlmnInfo(plmn)
})

const countryName = computed(() => plmnInfo.value?.country?.name || '')
const countryIso = computed(() => {
  const mcc = selectedCarrier.value?.mcc
  const mnc = selectedCarrier.value?.mnc
  return mccToIso(mcc, mnc)
})
const countryCode = computed(() => plmnInfo.value?.country?.code || '')

// 激活状态文案
const activationStatus = computed(() => {
  if (!detail.value) return ''
  if (detail.value.active) return '自定义生效'
  if (detail.value.system_default) return '默认配置生效'
  return ''
})
</script>

<template>
  <div class="detail-panel">
    <!-- 详情页头部 (60px) -->
    <div class="detail-header">
      <!-- 窄屏下拉选择器 + 添加按钮 -->
      <div class="detail-header-narrow">
        <CarrierIcon
          :mcc="selectedCarrier?.mcc || ''"
          :mnc="selectedCarrier?.mnc || ''"
          :name="selectedCarrier?.name"
          :carrier-key="selectedCarrier?.key"
          :size="38"
          class="narrow-logo"
        />
        <el-select
          :model-value="selectValue"
          @change="handleSelectChange"
          placeholder="选择运营商"
          class="!w-full"
        >
          <el-option
            v-for="c in carriers"
            :key="c.key"
            :label="`${c.name}  ${c.mcc}:${c.mnc}`"
            :value="c.key"
          />
        </el-select>
        <el-button size="small" type="primary" @click="emit('open-search')" class="!border-0 add-btn-narrow">
          <el-icon class="mr-1"><Add24Regular /></el-icon>
          <span>添加</span>
        </el-button>
      </div>
      <!-- 宽屏：图标盒子 + 运营商名 + 详细信息 -->
      <div class="detail-header-wide">
        <CarrierIcon :mcc="selectedCarrier?.mcc || ''" :mnc="selectedCarrier?.mnc || ''" :name="selectedCarrier?.name" :carrier-key="selectedCarrier?.key" :size="38" />
        <div class="detail-header-info">
          <div class="detail-header-name">{{ selectedCarrier?.name || '未选择' }}</div>
          <div class="detail-header-meta">
            <span class="detail-header-plmn">{{ selectedCarrier?.mcc }}:{{ selectedCarrier?.mnc }}</span>
            <span v-if="countryCode" class="detail-header-code">+{{ countryCode }}</span>
            <CountryFlag v-if="countryIso" :iso="countryIso" :size="16" class="detail-header-flag" />
            <span v-if="countryName" class="detail-header-country">{{ countryName }}</span>
            <span v-if="activationStatus" class="activation-status">{{ activationStatus }}</span>
          </div>
        </div>
      </div>
    </div>

    <ListSkeleton v-if="detailLoading && !detail" :rows="3" />

    <EmptyState
      v-else-if="!detail"
      title="选择一个运营商"
      subtitle="从左侧列表选择运营商查看配置详情"
    />

    <div v-else class="detail-content">
      <!-- 手动拉取 YAML -->
      <div class="pull-section">
        <div class="pull-title">手动拉取运营商配置</div>
        <div class="pull-form">
          <el-input v-model="pullMcc" placeholder="MCC" maxlength="3" class="!w-24" />
          <el-input v-model="pullMnc" placeholder="MNC" maxlength="3" class="!w-24" />
          <el-button type="primary" size="small" :loading="pullLoading" @click="handlePull">
            拉取
          </el-button>
        </div>
        <div v-if="pullResult" class="pull-result">
          <el-alert :title="pullResult.message" type="success" :closable="false" />
          <div class="pull-note">PLMN: {{ pullResult.plmn }}（未激活，需手动启用）</div>
        </div>
      </div>

      <!-- 系统默认（自动推导）提示 -->
      <div v-if="detail?.system_default?.derived" class="derived-note">
        <el-alert
          title="系统默认（自动推导）"
          :description="detail.system_default.derived_note || '按 3GPP 标准规则推导；非标准 ePDG 域名需手动拉取纠正'"
          type="info"
          :closable="false"
        />
      </div>

      <!-- 编辑区 -->
      <div class="edit-area-wrap">
        <CarrierEditArea />
      </div>
    </div>

  </div>
</template>

<style scoped>
.detail-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  padding: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  overflow: hidden;
}

/* 头部 — 60px 统一高度 */
.detail-header {
  height: 60px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 12px;
  border-bottom: 1px solid var(--border);
  overflow: hidden;
}

.detail-header-narrow {
  display: none;
  flex: 1;
  align-items: center;
  gap: 8px;
}

.narrow-logo {
  flex-shrink: 0;
}

.detail-header-narrow .el-select {
  flex: 1;
}

.add-btn-narrow {
  flex-shrink: 0;
}

.detail-header-wide {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

/* 图标盒子 — 38×38 / 6px 圆角 / 主题色 */
.detail-header-icon-box {
  width: 38px;
  height: 38px;
  border-radius: 6px;
  background: var(--background);
  border: 1px solid var(--border);
  color: var(--foreground);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.detail-header-info {
  flex: 1;
  min-width: 0;
}

.detail-header-name {
  font-size: 15px;
  font-weight: 700;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-header-plmn {
  font-size: 12px;
  color: var(--muted-foreground);
  font-family: var(--oomol-font-mono);
}

.detail-header-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 1px;
  font-size: 12px;
  color: var(--muted-foreground);
}

.detail-header-code {
  font-family: var(--oomol-font-mono);
  color: var(--brand);
  opacity: 0.8;
}

.detail-header-flag {
  opacity: 0.9;
}

.detail-header-country {
  opacity: 0.7;
}

.activation-status {
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  background: var(--muted);
  color: var(--muted-foreground);
}

/* 内容区 — 去掉 padding 让分割线贯通 */
.detail-content {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* 编辑区 — 填充全部空间 */
.edit-area-wrap {
  flex: 1;
  min-height: 0;
  padding: 12px;
}

/* 响应式：窄屏显示下拉选择器 */
@media (max-width: 768px) {
  .detail-header-narrow {
    display: flex;
  }
  .detail-header-wide {
    display: none;
  }
}
</style>
