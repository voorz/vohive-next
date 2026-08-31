<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage, ElMessageBox, ElLoading } from 'element-plus'
import { devicesService } from '../services/devices'
import { errorMessage } from '../services/http'
import type { EsimProfileItem } from '../types/api'
import EsimCardPolicyInline from './EsimCardPolicyInline.vue'
import { Delete24Regular, Edit24Regular } from '@vicons/fluent'

const props = defineProps<{
  visible: boolean
  profile: EsimProfileItem | null
  aidHex: string
  deviceId: string
  deviceOnline?: boolean
  isPCSC?: boolean
}>()

const emit = defineEmits<{
  'update:visible': [val: boolean]
  renamed: []
  deleted: []
  'policy-changed': []
}>()

const nameValue = ref('')
const nameEditing = ref(false)
const deleting = ref(false)
const saving = ref(false)

watch(() => props.visible, (open) => {
  if (open && props.profile) {
    nameValue.value = props.profile.name || ''
    nameEditing.value = false
  }
})

watch(() => props.profile, (p) => {
  if (p) nameValue.value = p.name || ''
})

function startEditName() {
  nameEditing.value = true
}

async function saveName() {
  if (!props.profile) return
  const name = nameValue.value.trim()
  if (!name) {
    ElMessage.warning('名称不能为空')
    return
  }
  if (name === props.profile.name) {
    nameEditing.value = false
    return
  }
  saving.value = true
  try {
    const result = await devicesService.renameEsimProfile(props.deviceId, props.profile.iccid, {
      name,
      aid_hex: props.aidHex
    })
    if (!result.ok) throw new Error(result.error.message || '修改名称失败')
    ElMessage.success('名称修改成功')
    emit('renamed')
    nameEditing.value = false
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, '修改名称失败'))
  } finally {
    saving.value = false
  }
}

async function deleteProfile() {
  if (!props.profile) return
  const iccid = props.profile.iccid
  const last4 = iccid.slice(-4)
  const { value: input } = await ElMessageBox.prompt(
    `此操作不可逆！请输入 ICCID 后 4 位「${last4}」以确认删除 Profile「${props.profile.name}」`,
    '删除 Profile',
    {
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
      inputPattern: new RegExp(`^${last4}$`),
      inputErrorMessage: `请输入 ${last4} 以确认`,
      inputPlaceholder: `输入 ${last4}`,
      type: 'error',
      confirmButtonClass: '!bg-red-600 !border-red-600 hover:!bg-red-700'
    }
  ).catch(() => ({ value: '' }))
  if (input !== last4) return

  const loading = ElLoading.service({
    lock: true,
    text: '正在删除 eSIM Profile...',
    background: 'rgba(0, 0, 0, 0.7)'
  })
  deleting.value = true
  try {
    const result = await devicesService.deleteEsimProfile(props.deviceId, iccid, props.aidHex)
    if (!result.ok) throw new Error(result.error.message || '删除失败')
    ElMessage.success('Profile 删除成功')
    emit('deleted')
    emit('update:visible', false)
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, '删除失败'))
  } finally {
    loading.close()
    deleting.value = false
  }
}

function close() {
  emit('update:visible', false)
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="(v: boolean) => emit('update:visible', v)"
    title="设置 eSIM"
    width="min(440px, 90vw)"
    :close-on-click-modal="false"
  >
    <div v-if="profile" class="settings-content">
      <!-- 名称 -->
      <div class="settings-section">
        <div class="settings-section-label">名称</div>
        <div class="name-row">
          <el-input
            v-model="nameValue"
            :disabled="!nameEditing"
            placeholder="输入名称"
            size="default"
            class="name-input"
            @keyup.enter="saveName"
          />
          <el-button
            v-if="!nameEditing"
            text
            size="default"
            class="name-action-btn"
            @click="startEditName"
          >
            <el-icon size="16"><Edit24Regular /></el-icon>
            <span>修改</span>
          </el-button>
          <el-button
            v-else
            type="primary"
            size="default"
            class="name-action-btn"
            :loading="saving"
            @click="saveName"
          >保存</el-button>
        </div>
      </div>

      <!-- 卡策略 -->
      <div class="settings-section">
        <div class="settings-section-label">指定 eSIM 策略</div>
        <div class="settings-section-hint">设置此 eSIM 的启动方式，改动将在下一次激活时持续生效</div>
        <EsimCardPolicyInline
          :device-id="deviceId"
          :iccid="profile.iccid"
          :is-active-card="profile.state === 1"
          :device-online="deviceOnline === true"
          :is-p-c-s-c="isPCSC"
          @policy-changed="emit('policy-changed')"
        />
      </div>

    </div>

    <template #footer>
      <div class="settings-footer">
        <el-button
          type="danger"
          plain
          :loading="deleting"
          @click="deleteProfile"
        >
          <el-icon size="14"><Delete24Regular /></el-icon>
          <span>{{ deleting ? '删除中...' : '删除 eSIM' }}</span>
        </el-button>
        <el-button @click="close">关闭</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.settings-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.settings-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.settings-section-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.settings-section-hint {
  font-size: 11px;
  color: var(--muted-foreground);
  line-height: 1.5;
  padding: 0 2px;
}

/* Name 行 */
.name-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.name-input {
  flex: 1;
  min-width: 0;
}

.name-action-btn {
  flex-shrink: 0;
}

/* 底部 */
.settings-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
</style>
