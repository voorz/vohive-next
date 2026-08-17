<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

const props = defineProps<{
  modelValue: boolean
  existingNumbers: string[]
}>()

const emit = defineEmits<{
  'update:modelValue': [val: boolean]
  save: [contact: { name: string; number: string }]
}>()

const contactName = ref('')
const contactNumber = ref('')
const contactSaving = ref(false)

function resetForm() {
  contactName.value = ''
  contactNumber.value = ''
}

function onClose() {
  resetForm()
}

function onSave() {
  const name = contactName.value.trim()
  const number = contactNumber.value.trim()

  if (!name) {
    ElMessage.warning('请输入姓名')
    return
  }
  if (!number) {
    ElMessage.warning('请输入号码')
    return
  }
  if (!/^[+]?[0-9*#]+$/.test(number)) {
    ElMessage.warning('号码格式不正确')
    return
  }
  if (props.existingNumbers.includes(number)) {
    ElMessage.warning('该号码已存在')
    return
  }

  contactSaving.value = true
  // 模拟异步保存
  setTimeout(() => {
    emit('save', { name, number })
    contactSaving.value = false
    resetForm()
    emit('update:modelValue', false)
  }, 300)
}
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    title="添加联系人"
    width="360px"
    @close="onClose"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <el-form label-width="60px" @submit.prevent="onSave">
      <el-form-item label="姓名">
        <el-input v-model="contactName" placeholder="请输入姓名" @keyup.enter="onSave" />
      </el-form-item>
      <el-form-item label="号码">
        <el-input v-model="contactNumber" placeholder="请输入号码" @keyup.enter="onSave" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="$emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="contactSaving" @click="onSave">保存</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
/* 组件自包含样式，目前无需额外样式 */
</style>
