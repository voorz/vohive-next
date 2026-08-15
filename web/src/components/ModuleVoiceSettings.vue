<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import QrcodeVue from 'qrcode.vue'
import { ArrowSync24Regular } from '@vicons/fluent'
import { api } from '../stores/auth'
import { systemService } from '../services/system'
import { useDevicesStore } from '../stores/devices'

interface VoiceGatewayConfig {
  sip: {
    listen: string
    transport: string
    realm: string
    external_ip: string
    ws_listen: string
    wss_listen: string
    wss_cert_file: string
    wss_key_file: string
  }
  user: {
    username: string
    password: string
    device_id: string
  }
  media: {
    rtp_port_min: number
    rtp_port_max: number
    codecs: string[]
  }
  linphone_push: {
    linphone_user: string
    linphone_password: string
  }
}

const loading = ref(false)
const saving = ref(false)
const readonly = ref(true)
const regenerating = ref(false)
const webUsername = ref('')
const devicesStore = useDevicesStore()

const deviceOptions = computed(() =>
  devicesStore.list.map(d => ({ label: d.name || d.id, value: d.id }))
)

const form = ref<VoiceGatewayConfig>({
  sip: {
    listen: '0.0.0.0:5060',
    transport: 'udp',
    realm: 'vohive.local',
    external_ip: '',
    ws_listen: '',
    wss_listen: '',
    wss_cert_file: '',
    wss_key_file: '',
  },
  user: {
    username: '',
    password: '',
    device_id: '',
  },
  media: {
    rtp_port_min: 10000,
    rtp_port_max: 20000,
    codecs: ['PCMU/8000', 'PCMA/8000'],
  },
  linphone_push: {
    linphone_user: '',
    linphone_password: '',
  },
})

const codecInput = ref('')
const codecOptions = ['PCMU/8000', 'PCMA/8000', 'Opus/48000/2']

async function loadWebUsername() {
  try {
    const info = await systemService.getInfo()
    if (info?.ok && info.data?.username) {
      webUsername.value = info.data.username
    }
  } catch {
    // ignore
  }
}

async function loadConfig() {
  loading.value = true
  try {
    const res = await api.get('/settings/voice-gateway')
    if (res.data) {
      const d = res.data
      form.value.sip = { ...form.value.sip, ...d.sip }
      form.value.user = { ...form.value.user, ...d.user }
      form.value.media = { ...form.value.media, ...d.media }
      form.value.linphone_push = { ...form.value.linphone_push, ...d.linphone_push }
    }
  } catch {
    ElMessage.error('加载语音网关配置失败')
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  if (!form.value.sip.realm) {
    ElMessage.warning('sip.realm 不能为空')
    return false
  }
  saving.value = true
  try {
    const payload = {
      sip: form.value.sip,
      user: {
        password: form.value.user.password,
        device_id: form.value.user.device_id,
      },
      media: form.value.media,
      linphone_push: form.value.linphone_push,
    }
    await api.put('/settings/voice-gateway', payload)
    ElMessage.success('语音网关配置已保存')
    return true
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败')
    return false
  } finally {
    saving.value = false
  }
}

async function regeneratePassword() {
  regenerating.value = true
  try {
    const res = await api.post('/settings/voice-gateway/regenerate-password')
    if (res.data?.password) {
      form.value.user.password = res.data.password
      ElMessage.success('授权码已重新生成')
    }
  } catch {
    ElMessage.error('生成失败')
  } finally {
    regenerating.value = false
  }
}

function setReadonly(val: boolean) {
  readonly.value = val
}

function addCodec() {
  const c = codecInput.value.trim()
  if (c && !form.value.media.codecs.includes(c)) {
    form.value.media.codecs.push(c)
  }
  codecInput.value = ''
}

function removeCodec(index: number) {
  form.value.media.codecs.splice(index, 1)
}

// QR 码 SIP URI 构建
const sipHost = computed(() => {
  const listen = form.value.sip.listen || '0.0.0.0:5060'
  const [host, port] = listen.split(':')
  const realHost = (host === '0.0.0.0' || host === '::') ? window.location.hostname : host
  return `${realHost}:${port || '5060'}`
})

const sipTransport = computed(() => form.value.sip.transport || 'udp')

const qrValue = computed(() => {
  if (!form.value.user.username) return ''
  return `sip:${form.value.user.username}@${sipHost.value};transport=${sipTransport.value}`
})

defineExpose({ saveConfig, loadConfig, setReadonly, saving })

onMounted(async () => {
  await loadWebUsername()
  await loadConfig()
  // 确保用户名使用 Web 管理员用户名
  if (webUsername.value) {
    form.value.user.username = webUsername.value
  }
  // 加载设备列表
  await devicesStore.fetchList()
})
</script>

<template>
  <div class="voice-settings" v-loading="loading">
    <!-- Linphone 接入（QR + 设备绑定 + 推送通知） -->
    <div class="settings-section">
      <div class="section-title">Linphone 接入</div>
      <div class="section-body">
        <!-- QR -->
        <div class="qr-area">
          <QrcodeVue v-if="qrValue" :value="qrValue" :size="120" level="M" render-as="svg" :margin="1" class="qr-svg" />
          <div v-else class="qr-placeholder">配置后显示</div>
        </div>

        <!-- 用户名 + 授权码 -->
        <div class="form-grid">
          <div class="form-item">
            <label class="form-label">用户</label>
            <el-input :model-value="form.user.username || webUsername || ''" disabled />
          </div>
          <div class="form-item">
            <label class="form-label">授权码</label>
            <el-input :model-value="form.user.password || ''" readonly placeholder="点击右侧按钮生成">
              <template #append>
                <el-icon class="regen-icon" :class="{ 'spin': regenerating }" @click="!readonly && regeneratePassword()" :title="'重新生成'">
                  <ArrowSync24Regular />
                </el-icon>
              </template>
            </el-input>
          </div>
        </div>

        <!-- 设备 -->
        <div class="form-item">
          <label class="form-label">设备（可选）</label>
          <el-select v-model="form.user.device_id" class="!w-full" :disabled="readonly" filterable clearable placeholder="选择设备">
            <el-option v-for="opt in deviceOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </div>

        <!-- Linphone 账号 + 密码 -->
        <div class="form-grid">
          <div class="form-item">
            <label class="form-label">Linphone 账号</label>
            <el-input v-model="form.linphone_push.linphone_user" placeholder="sip.linphone.org 账号" :disabled="readonly" autocomplete="off" />
          </div>
          <div class="form-item">
            <label class="form-label">Linphone 密码</label>
            <el-input v-model="form.linphone_push.linphone_password" placeholder="sip.linphone.org 密码" :disabled="readonly" autocomplete="off" />
          </div>
        </div>
      </div>
    </div>

    <!-- SIP 服务 -->
    <div class="settings-section">
      <div class="section-title">SIP 服务</div>
      <div class="section-body">
        <div class="form-grid">
          <div class="form-item">
            <label class="form-label">监听地址</label>
            <el-input v-model="form.sip.listen" placeholder="0.0.0.0:5060" :disabled="readonly" />
          </div>
          <div class="form-item">
            <label class="form-label">传输协议</label>
            <el-select v-model="form.sip.transport" class="!w-full" :disabled="readonly">
              <el-option label="UDP" value="udp" />
              <el-option label="TCP" value="tcp" />
              <el-option label="TLS" value="tls" />
            </el-select>
          </div>
          <div class="form-item">
            <label class="form-label">Realm</label>
            <el-input v-model="form.sip.realm" placeholder="vohive.local" :disabled="readonly" />
          </div>
          <div class="form-item">
            <label class="form-label">外部 IP</label>
            <el-input v-model="form.sip.external_ip" placeholder="公网 IP（可选）" :disabled="readonly" />
          </div>
        </div>
      </div>
    </div>

    <!-- WS/WSS -->
    <div class="settings-section">
      <div class="section-title">WebSocket / WSS（浏览器接入）</div>
      <div class="section-body">
        <div class="form-grid">
          <div class="form-item">
            <label class="form-label">WS 监听地址</label>
            <el-input v-model="form.sip.ws_listen" placeholder="留空不启用" :disabled="readonly" />
          </div>
          <div class="form-item">
            <label class="form-label">WSS 监听地址</label>
            <el-input v-model="form.sip.wss_listen" placeholder="留空不启用" :disabled="readonly" />
          </div>
          <div class="form-item">
            <label class="form-label">WSS 证书文件</label>
            <el-input v-model="form.sip.wss_cert_file" placeholder="证书路径" :disabled="readonly" />
          </div>
          <div class="form-item">
            <label class="form-label">WSS 私钥文件</label>
            <el-input v-model="form.sip.wss_key_file" placeholder="私钥路径" :disabled="readonly" />
          </div>
        </div>
      </div>
    </div>

    <!-- RTP 媒体 -->
    <div class="settings-section">
      <div class="section-title">RTP 媒体</div>
      <div class="section-body">
        <div class="form-grid">
          <div class="form-item">
            <label class="form-label">RTP 端口起始</label>
            <el-input-number v-model="form.media.rtp_port_min" :min="1024" :max="65534" controls-position="right" class="w-full" :disabled="readonly" />
          </div>
          <div class="form-item">
            <label class="form-label">RTP 端口结束</label>
            <el-input-number v-model="form.media.rtp_port_max" :min="1024" :max="65534" controls-position="right" class="w-full" :disabled="readonly" />
          </div>
        </div>
        <div class="form-item mt-3">
          <label class="form-label">编解码</label>
          <div class="codec-list">
            <el-tag
              v-for="(c, i) in form.media.codecs"
              :key="i"
              :closable="!readonly"
              @close="removeCodec(i)"
              class="codec-tag"
            >
              {{ c }}
            </el-tag>
          </div>
          <div v-if="!readonly" class="flex items-center gap-2 mt-2">
            <el-select
              v-model="codecInput"
              filterable
              allow-create
              default-first-option
              placeholder="选择或输入编解码"
              class="!w-full flex-1"
            >
              <el-option v-for="c in codecOptions" :key="c" :label="c" :value="c" />
            </el-select>
            <el-button @click="addCodec">添加</el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.voice-settings {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 12px;
}

/* ── 接入卡片 ── */

.qr-area {
  display: flex;
  justify-content: center;
  margin-bottom: 14px;
}

.qr-svg {
  border-radius: 6px;
}

.qr-placeholder {
  width: 120px;
  height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: var(--muted-foreground);
  background: var(--muted);
  border-radius: 6px;
}

.spin {
  animation: spin 0.8s linear infinite;
}

.regen-icon {
  cursor: pointer;
  color: var(--muted-foreground);
  transition: color 0.15s;
}

.regen-icon:hover {
  color: var(--brand, #00BC7D);
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.settings-section .section-body > .form-item + .form-item,
.settings-section .section-body > .form-item + .form-grid,
.settings-section .section-body > .form-grid + .form-item,
.settings-section .section-body > .form-grid + .form-grid {
  margin-top: 12px;
}

/* ── 通用 section ── */
.settings-section {
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--background);
  overflow: hidden;
}

.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  font-size: 13px;
  font-weight: 700;
  color: var(--foreground);
  background: var(--muted);
  border-bottom: 1px solid var(--border);
}

.section-body {
  padding: 14px;
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
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

.codec-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.codec-tag {
  margin: 0;
}

.w-full {
  width: 100%;
}

.mt-2 {
  margin-top: 8px;
}

.mt-3 {
  margin-top: 12px;
}

@media (max-width: 480px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
