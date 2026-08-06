<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { parseProxyBatch, type ParsedProxy } from '../utils/proxyUrlParser'

const props = defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  'update:visible': [val: boolean]
  'import': [proxies: ParsedProxy[]]
}>()

const textareaValue = ref('')
const parsedResults = ref<ParsedProxy[]>([])

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val)
})

const validCount = computed(() => parsedResults.value.filter(p => p.valid).length)
const errorCount = computed(() => parsedResults.value.filter(p => !p.valid).length)

watch(() => props.visible, (val) => {
  if (val) {
    textareaValue.value = ''
    doParse()
  }
})

function doParse() {
  parsedResults.value = parseProxyBatch(textareaValue.value)
}

function handleImport() {
  const valid = parsedResults.value.filter(p => p.valid)
  if (valid.length === 0) return
  emit('import', valid)
  dialogVisible.value = false
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    title="批量导入前置代理"
    width="680px"
    :close-on-click-modal="false"
    align-center
  >
    <div class="space-y-4 pb-2">
      <!-- Textarea -->
      <div class="space-y-1">
        <label class="pc-form-label">代理链接串（每行一个）</label>
        <el-input
          v-model="textareaValue"
          type="textarea"
          :rows="6"
          resize="none"
          class="font-mono"
          placeholder="socks5://user:pass@host:port&#10;host:1080&#10;host:port:user:pass"
          @input="doParse"
        />
        <div class="pc-form-hint">支持 socks5:// / host:port / host:port:user:pass 格式，自动识别</div>
      </div>

      <!-- Preview table -->
      <div class="space-y-2">
        <label class="pc-form-label">解析预览</label>
        <el-table :data="parsedResults" size="small" border>
          <el-table-column label="#" width="40" type="index" />
          <el-table-column label="协议" width="70">
            <template #default="{ row }">
              <span v-if="row.valid" class="font-mono text-xs">{{ row.protocol }}</span>
              <span v-else style="color: var(--muted-foreground);">—</span>
            </template>
          </el-table-column>
          <el-table-column label="地址" width="80" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="font-mono text-xs">{{ row.host || row.raw }}</span>
            </template>
          </el-table-column>
          <el-table-column label="端口" width="60">
            <template #default="{ row }">
              <span v-if="row.port" class="font-mono text-xs">{{ row.port }}</span>
              <span v-else style="color: var(--muted-foreground);">—</span>
            </template>
          </el-table-column>
          <el-table-column label="用户名" width="90">
            <template #default="{ row }">
              <span v-if="row.username" class="text-xs">{{ row.username }}</span>
              <span v-else style="color: var(--muted-foreground);">—</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag v-if="row.valid" size="small" type="success">有效</el-tag>
              <el-tag v-else size="small" type="danger">{{ row.error || '错误' }}</el-tag>
            </template>
          </el-table-column>
        </el-table>
        <div class="pc-form-hint">
          {{ validCount }} 条有效 · {{ errorCount }} 条错误
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-end gap-2">
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :disabled="validCount === 0"
          @click="handleImport"
        >
          导入 {{ validCount }} 条有效记录
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.pc-form-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--muted-foreground);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.pc-form-hint {
  font-size: 12px;
  color: var(--muted-foreground);
  margin-top: 4px;
}
</style>
