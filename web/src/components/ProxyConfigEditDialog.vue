<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { parseProxyUrl } from '../utils/proxyUrlParser'
import { Link24Regular } from '@vicons/fluent'
import CountryFlag from './CountryFlag.vue'
import type { UpstreamProxyWithMeta, UpstreamProxyFormData } from '../types/proxy-config'
import type { UpstreamProxyCountry, UpstreamProxyCountryRule } from '../types/api'

const props = defineProps<{
  visible: boolean
  editing: UpstreamProxyWithMeta | null
  countries: UpstreamProxyCountry[]
  existingRules: UpstreamProxyCountryRule[]
}>()

const emit = defineEmits<{
  'update:visible': [val: boolean]
  'save': [form: UpstreamProxyFormData, pendingCountryCodes: string[]]
  'add-rule': [countryCode: string]
  'delete-rule': [countryCode: string]
}>()

const form = ref<UpstreamProxyFormData>({
  name: '',
  addr: '',
  username: '',
  password: '',
  enabled: true
})

const selectedCountry = ref('')
const pendingRules = ref<string[]>([])

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val)
})

const isEditing = computed(() => !!props.editing)
const title = computed(() => isEditing.value ? '编辑前置代理' : '新增前置代理')

// 当前代理已配置的国家规则（编辑模式）或待提交规则（新建模式）
const currentProxyRules = computed(() => {
  if (!props.editing) return []
  return props.existingRules.filter(r => r.upstream_proxy_id === props.editing!.id)
})

// 新建模式下待提交的国家列表，附带国家信息用于展示
const pendingRuleDetails = computed(() => {
  return pendingRules.value.map(code => {
    const c = props.countries.find(c => c.country_code === code)
    return {
      country_code: code,
      country_name: c?.country_name || code,
      mccs: c?.mccs || []
    }
  })
})

// 可选国家（排除已配置到其他代理的，新建模式排除已选中的）
const availableCountries = computed(() => {
  if (!props.editing) {
    return props.countries.filter(c => !pendingRules.value.includes(c.country_code))
  }
  return props.countries.filter(country => {
    const rule = props.existingRules.find(r => r.country_code === country.country_code)
    return !rule || rule.upstream_proxy_id === props.editing?.id
  })
})

watch(() => props.visible, (val) => {
  if (val) {
    if (props.editing) {
      form.value = {
        name: props.editing.name,
        addr: props.editing.addr,
        username: props.editing.username,
        password: props.editing.password,
        enabled: props.editing.enabled
      }
    } else {
      form.value = { name: '', addr: '', username: '', password: '', enabled: true }
    }
    selectedCountry.value = ''
    pendingRules.value = []
  }
})

function onAddrInput() {
  const result = parseProxyUrl(form.value.addr)
  if (result.valid) {
    if (result.username) form.value.username = result.username
    if (result.password) form.value.password = result.password
  }
}

function formatCountryLabel(country: UpstreamProxyCountry) {
  const mccs = country.mccs?.length ? ` · MCC ${country.mccs.join('/')}` : ''
  return `${country.country_code} · ${country.country_name || country.country_code}${mccs}`
}

function handleAddRule() {
  if (!selectedCountry.value) return
  if (isEditing.value) {
    emit('add-rule', selectedCountry.value)
  } else {
    if (!pendingRules.value.includes(selectedCountry.value)) {
      pendingRules.value.push(selectedCountry.value)
    }
  }
  selectedCountry.value = ''
}

function handleDeletePendingRule(code: string) {
  pendingRules.value = pendingRules.value.filter(c => c !== code)
}

function handleSave() {
  if (!form.value.addr.trim()) {
    ElMessage.warning('代理地址不能为空')
    return
  }
  emit('save', { ...form.value }, [...pendingRules.value])
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    :title="title"
    width="600px"
    :close-on-click-modal="false"
    align-center
  >
    <div class="space-y-6 pb-2">
      <!-- Section: 代理信息 -->
      <div class="space-y-4">
        <div class="pc-dlg-section-header">
          <div class="pc-dlg-section-bar" style="background: var(--brand);" />
          <h3 class="pc-dlg-section-title">代理信息</h3>
        </div>

        <div class="space-y-1">
          <label class="pc-form-label">名称（留空自动生成）</label>
          <el-input v-model="form.name" placeholder="例如：日本代理" />
        </div>

        <div class="space-y-1">
          <label class="pc-form-label">代理地址（链接串自动识别）</label>
          <el-input
            v-model="form.addr"
            class="font-mono"
            placeholder="socks5://user:pass@host:port 或 host:port 或 host:port:user:pass"
            @input="onAddrInput"
          />
          <div class="pc-form-hint">粘贴 socks5 链接串、host:port 或 host:port:user:pass 后自动识别</div>
        </div>
      </div>

      <!-- 鉴权字段 -->
      <div class="grid grid-cols-2 gap-3">
        <div class="space-y-1">
          <label class="pc-form-label">用户名（可选）</label>
          <el-input v-model="form.username" placeholder="留空则免鉴权" />
        </div>
        <div class="space-y-1">
          <label class="pc-form-label">密码（可选）</label>
            <el-input
              v-model="form.password"
              type="text"
              autocomplete="off"
              placeholder="留空则免鉴权"
            />
        </div>
      </div>

      <!-- Section: 国家路由规则 -->
      <div class="space-y-4">
        <div class="pc-dlg-section-header">
          <div class="pc-dlg-section-bar" style="background: var(--info);" />
          <h3 class="pc-dlg-section-title">国家路由规则</h3>
        </div>

        <!-- Existing rules (编辑模式) -->
        <div v-if="isEditing && currentProxyRules.length > 0" class="space-y-2">
          <div
            v-for="rule in currentProxyRules"
            :key="rule.country_code"
            class="pc-rule-row"
          >
            <div class="flex items-center gap-2 min-w-0">
              <CountryFlag :iso="rule.country_code" :size="18" />
              <span class="text-sm font-semibold">{{ rule.country_code }} · {{ rule.country_name || rule.country_code }}</span>
              <span class="text-xs font-mono" style="color: var(--muted-foreground);">
                MCC {{ rule.mccs.join('/') || '-' }}
              </span>
            </div>
            <el-button size="small" type="danger" text @click="emit('delete-rule', rule.country_code)">
              删除
            </el-button>
          </div>
        </div>

        <!-- Pending rules (新建模式) -->
        <div v-if="!isEditing && pendingRuleDetails.length > 0" class="space-y-2">
          <div
            v-for="rule in pendingRuleDetails"
            :key="rule.country_code"
            class="pc-rule-row"
          >
            <div class="flex items-center gap-2 min-w-0">
              <CountryFlag :iso="rule.country_code" :size="18" />
              <span class="text-sm font-semibold">{{ rule.country_code }} · {{ rule.country_name }}</span>
              <span class="text-xs font-mono" style="color: var(--muted-foreground);">
                MCC {{ rule.mccs.join('/') || '-' }}
              </span>
            </div>
            <el-button size="small" type="danger" text @click="handleDeletePendingRule(rule.country_code)">
              删除
            </el-button>
          </div>
        </div>

        <div v-if="(isEditing && currentProxyRules.length === 0) || (!isEditing && pendingRuleDetails.length === 0)" class="pc-empty-mini">暂无国家规则，未配置的国家默认直连</div>

        <!-- Add rule -->
        <div class="flex items-center gap-2 mt-2">
          <el-select
            v-model="selectedCountry"
            placeholder="选择国家"
            class="flex-1"
            filterable
          >
            <el-option
              v-for="country in availableCountries"
              :key="country.country_code"
              :label="formatCountryLabel(country)"
              :value="country.country_code"
            >
              <div class="flex items-center justify-between w-full">
                <div class="flex items-center gap-2">
                  <CountryFlag :iso="country.country_code" :size="16" />
                  <span>{{ country.country_code }} · {{ country.country_name || country.country_code }}</span>
                </div>
                <span class="text-xs font-mono ml-2" style="color: var(--muted-foreground);">
                  MCC {{ country.mccs.join('/') }}
                </span>
              </div>
            </el-option>
          </el-select>
          <el-button
            type="primary"
            :disabled="!selectedCountry"
            @click="handleAddRule"
          >
            <el-icon class="mr-1.5"><Link24Regular /></el-icon>
            <span>添加规则</span>
          </el-button>
        </div>
        <div class="pc-form-hint">规则按 SIM 归属 MCC 解析国家。没有配置规则的国家默认直连。需要重启 VoWiFi 生效。</div>
      </div>

      <!-- Enable switch -->
      <div class="pc-switch-row">
        <div>
          <div class="pc-switch-title">启用代理</div>
          <div class="pc-switch-desc">禁用后绑定到该代理的国家规则会回退为直连</div>
        </div>
        <el-switch v-model="form.enabled" />
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-end gap-2">
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSave">
          {{ isEditing ? '更新' : '创建' }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.pc-dlg-section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
}
.pc-dlg-section-bar {
  width: 4px;
  height: 16px;
  border-radius: 999px;
}
.pc-dlg-section-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}
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
.pc-rule-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--muted);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 8px 12px;
}
.pc-empty-mini {
  text-align: center;
  padding: 12px;
  color: var(--muted-foreground);
  font-size: 12px;
  border: 1px dashed var(--border);
  border-radius: 6px;
}
.pc-switch-row {
  background: var(--muted);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.pc-switch-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--foreground);
}
.pc-switch-desc {
  font-size: 12px;
  color: var(--muted-foreground);
  margin-top: 2px;
}
</style>
