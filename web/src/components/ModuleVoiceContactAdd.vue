<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

const props = defineProps<{
  existingNumbers: string[]
}>()

const emit = defineEmits<{
  save: [contact: { name: string; number: string }]
  cancel: []
}>()

const name = ref('')
const number = ref('')

function onSave() {
  const n = name.value.trim()
  const num = number.value.trim()

  if (!n) {
    ElMessage.warning('请输入姓名')
    return
  }
  if (!num) {
    ElMessage.warning('请输入号码')
    return
  }
  if (!/^[+]?[0-9*#]+$/.test(num)) {
    ElMessage.warning('号码格式不正确')
    return
  }
  if (props.existingNumbers.includes(num)) {
    ElMessage.warning('该号码已存在')
    return
  }

  emit('save', { name: n, number: num })
  name.value = ''
  number.value = ''
}

function onCancel() {
  name.value = ''
  number.value = ''
  emit('cancel')
}
</script>

<template>
  <div class="contact-add">
    <div class="contact-add-form">
      <div class="form-item">
        <label class="form-label">姓名</label>
        <el-input v-model="name" placeholder="请输入姓名" @keyup.enter="onSave" />
      </div>
      <div class="form-item">
        <label class="form-label">号码</label>
        <el-input v-model="number" placeholder="请输入号码" @keyup.enter="onSave" />
      </div>
    </div>
    <div class="contact-add-actions">
      <el-button size="small" @click="onCancel">取消</el-button>
      <el-button size="small" type="primary" @click="onSave" class="!border-0">保存</el-button>
    </div>
  </div>
</template>

<style scoped>
.contact-add {
  display: flex;
  flex-direction: column;
  padding: 14px;
  gap: 16px;
}

.contact-add-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-width: 400px;
}

.contact-add-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  max-width: 400px;
}

.form-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
</style>
