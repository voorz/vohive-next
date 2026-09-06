<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage, ElMessageBox, ElLoading } from 'element-plus'
import { devicesService } from '../services/devices'
import { errorMessage } from '../services/http'
import type { EsimProfileItem } from '../types/api'
import EsimCardPolicyInline from './EsimCardPolicyInline.vue'
import { Delete24Regular } from '@vicons/fluent'
import {
  parseNickname,
  formatNickname,
  createDateTagRaw,
  createTextTagRaw,
  type ProfileTag,
  type DateTag,
} from '../utils/profileTagUtils'

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

// === 名称 + 标签状态 ===
// nameValue 只存纯名称（不含标签编码）
const nameValue = ref('')
const editing = ref(false)
const deleting = ref(false)
const saving = ref(false)

// 标签编辑状态
const dateTagEnabled = ref(false)
const dateTagValue = ref<Date | null>(null)
const dateTagNote = ref('')
const textTagEnabled = ref(false)
const textTagValue = ref('')

// 当前已保存的标签列表（从 profile.name 解析）
const savedTags = ref<ProfileTag[]>([])

// 从原始 name 字段解析出的纯名称
const parsedName = computed(() => {
  if (!props.profile) return ''
  const parsed = parseNickname(props.profile.name)
  return parsed.name
})

watch(() => props.visible, (open) => {
  if (open && props.profile) {
    const parsed = parseNickname(props.profile.name)
    nameValue.value = parsed.name
    savedTags.value = parsed.tags
    editing.value = false
    initTagEditors()
  }
})

watch(() => props.profile, (p) => {
  if (p) {
    const parsed = parseNickname(p.name)
    nameValue.value = parsed.name
    savedTags.value = parsed.tags
    initTagEditors()
  }
})

// 初始化标签编辑器（从已保存标签回填）
function initTagEditors() {
  const dateTag = savedTags.value.find((t): t is DateTag => t.type === 'date')
  const textTag = savedTags.value.find((t) => t.type === 'text')

  if (dateTag) {
    dateTagEnabled.value = true
    dateTagValue.value = new Date(dateTag.date)
    dateTagNote.value = dateTag.note || ''
  } else {
    dateTagEnabled.value = false
    dateTagValue.value = null
    dateTagNote.value = ''
  }

  if (textTag) {
    textTagEnabled.value = true
    textTagValue.value = textTag.text
  } else {
    textTagEnabled.value = false
    textTagValue.value = ''
  }
}

function startEdit() {
  editing.value = true
}

function cancelEdit() {
  editing.value = false
  // 回滚到已保存状态
  if (props.profile) {
    const parsed = parseNickname(props.profile.name)
    nameValue.value = parsed.name
    savedTags.value = parsed.tags
    initTagEditors()
  }
}

// 构建当前编辑中的标签列表
function buildTagsFromEditors(): ProfileTag[] {
  const tags: ProfileTag[] = []
  if (dateTagEnabled.value && dateTagValue.value) {
    tags.push({
      type: 'date',
      raw: createDateTagRaw(dateTagValue.value, dateTagNote.value),
      date: dateTagValue.value,
      note: dateTagNote.value.trim() || undefined,
      displayDate: '',
      countdownDays: 0,
      expired: false,
    })
  }
  if (textTagEnabled.value && textTagValue.value.trim()) {
    tags.push({
      type: 'text',
      raw: createTextTagRaw(textTagValue.value),
      text: textTagValue.value.trim(),
    })
  }
  return tags
}

// 保存名称（含标签）
async function saveName() {
  if (!props.profile) return
  const name = nameValue.value.trim()
  if (!name) {
    ElMessage.warning('名称不能为空')
    return
  }

  // 组装完整 Nickname（纯名称 + 标签）
  const tags = buildTagsFromEditors()
  const fullNickname = formatNickname(name, tags)

  // 检查是否有变化
  if (fullNickname === props.profile.name) {
    editing.value = false
    return
  }

  saving.value = true
  try {
    const result = await devicesService.renameEsimProfile(props.deviceId, props.profile.iccid, {
      name: fullNickname,
      aid_hex: props.aidHex
    })
    if (!result.ok) throw new Error(result.error.message || '修改名称失败')
    ElMessage.success('保存成功')
    emit('renamed')
    editing.value = false
  } catch (e: unknown) {
    ElMessage.error(errorMessage(e, '保存失败'))
  } finally {
    saving.value = false
  }
}

// 名称和标签是否有未保存改动
const hasUnsavedChanges = computed(() => {
  if (!props.profile) return false
  const name = nameValue.value.trim()
  const tags = buildTagsFromEditors()
  const fullNickname = formatNickname(name, tags)
  return fullNickname !== props.profile.name
})

async function deleteProfile() {
  if (!props.profile) return
  const iccid = props.profile.iccid
  const last4 = iccid.slice(-4)
  const { value: input } = await ElMessageBox.prompt(
    `此操作不可逆！请输入 ICCID 后 4 位「${last4}」以确认删除 Profile「${parsedName.value}」`,
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
        <div class="settings-section-hint">名称和标签会随 Nickname 一起写入 eUICC，跨设备持久保留</div>

        <el-input
          v-model="nameValue"
          :disabled="!editing"
          placeholder="输入名称"
          size="default"
          class="name-input"
          @keyup.enter="saveName"
        />

        <div class="tag-edit-area">
          <div class="tag-edit-item">
            <el-checkbox v-model="dateTagEnabled" :disabled="!editing">日期标签</el-checkbox>
            <div v-if="dateTagEnabled" class="tag-edit-controls">
              <el-date-picker
                v-model="dateTagValue"
                type="date"
                placeholder="选择日期"
                size="default"
                format="YYYY-MM-DD"
                :clearable="true"
                :disabled="!editing"
                class="tag-date-picker"
              />
              <el-input
                v-model="dateTagNote"
                placeholder="备注（可选）"
                size="default"
                class="tag-note-input"
                maxlength="30"
                :disabled="!editing"
              />
            </div>
          </div>

          <div class="tag-edit-item">
            <el-checkbox v-model="textTagEnabled" :disabled="!editing">文本标签</el-checkbox>
            <div v-if="textTagEnabled" class="tag-edit-controls">
              <el-input
                v-model="textTagValue"
                placeholder="输入文本（如：备用卡）"
                size="default"
                maxlength="20"
                class="tag-text-input"
                :disabled="!editing"
              />
            </div>
          </div>
        </div>

        <div class="save-row">
          <el-button
            v-if="!editing"
            type="primary"
            size="default"
            @click="startEdit"
          >管理</el-button>
          <template v-else>
            <el-button
              type="primary"
              size="default"
              :loading="saving"
              :disabled="!hasUnsavedChanges"
              @click="saveName"
            >保存</el-button>
            <el-button
              size="default"
              @click="cancelEdit"
            >取消</el-button>
          </template>
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
  gap: 12px;
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

/* 名称输入框 */
.name-input {
  width: 100%;
}

/* 标签编辑区域 */
.tag-edit-area {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.tag-edit-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.tag-edit-controls {
  display: flex;
  gap: 8px;
  align-items: center;
}

.tag-date-picker {
  flex-shrink: 0;
}

.tag-note-input {
  flex: 1;
  min-width: 0;
}

.tag-text-input {
  flex: 1;
}

/* 保存行 */
.save-row {
  display: flex;
  gap: 8px;
}

/* 底部 */
.settings-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
</style>
