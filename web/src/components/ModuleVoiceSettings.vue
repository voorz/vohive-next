<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../stores/auth'

interface VoiceUser {
  username: string
  password: string
  device_id?: string
}

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
  users: VoiceUser[]
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
  users: [],
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

async function loadConfig() {
  loading.value = true
  try {
    const res = await api.get('/settings/voice-gateway')
    if (res.data) {
      const d = res.data
      form.value.sip = { ...form.value.sip, ...d.sip }
      form.value.users = Array.isArray(d.users) ? d.users : []
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
    await api.put('/settings/voice-gateway', form.value)
    ElMessage.success('语音网关配置已保存')
    return true
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败')
    return false
  } finally {
    saving.value = false
  }
}

function setReadonly(val: boolean) {
  readonly.value = val
}

function addUser() {
  form.value.users.push({
    username: '',
    password: '',
  })
}

function removeUser(index: number) {
  form.value.users.splice(index, 1)
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

defineExpose({ saveConfig, loadConfig, setReadonly, saving })

onMounted(() => {
  loadConfig()
})
</script>

<template>
  <div class="voice-settings" v-loading="loading">
    <!-- SIP 服务 -->
    <div class="settings-section">
      <div class="section-title">SIP 服务</div>
      <div class="section-body">
        <div class="form-grid">
          <div class="form-item">
            <label class="form-label">监听地址</label>
            <el-input v-model="form.sip.listen" placeholder="0.0.0.0:5060" size="small" :disabled="readonly" />
          </div>
          <div class="form-item">
            <label class="form-label">传输协议</label>
            <el-select v-model="form.sip.transport" size="small" class="w-full" :disabled="readonly">
              <el-option label="UDP" value="udp" />
              <el-option label="TCP" value="tcp" />
              <el-option label="TLS" value="tls" />
            </el-select>
          </div>
          <div class="form-item">
            <label class="form-label">Realm</label>
            <el-input v-model="form.sip.realm" placeholder="vohive.local" size="small" :disabled="readonly" />
          </div>
          <div class="form-item">
            <label class="form-label">外部 IP</label>
            <el-input v-model="form.sip.external_ip" placeholder="公网 IP（可选）" size="small" :disabled="readonly" />
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
            <el-input v-model="form.sip.ws_listen" placeholder="留空不启用" size="small" :disabled="readonly" />
          </div>
          <div class="form-item">
            <label class="form-label">WSS 监听地址</label>
            <el-input v-model="form.sip.wss_listen" placeholder="留空不启用" size="small" :disabled="readonly" />
          </div>
          <div class="form-item">
            <label class="form-label">WSS 证书文件</label>
            <el-input v-model="form.sip.wss_cert_file" placeholder="证书路径" size="small" :disabled="readonly" />
          </div>
          <div class="form-item">
            <label class="form-label">WSS 私钥文件</label>
            <el-input v-model="form.sip.wss_key_file" placeholder="私钥路径" size="small" :disabled="readonly" />
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
            <el-input-number v-model="form.media.rtp_port_min" :min="1024" :max="65534" size="small" controls-position="right" class="w-full" :disabled="readonly" />
          </div>
          <div class="form-item">
            <label class="form-label">RTP 端口结束</label>
            <el-input-number v-model="form.media.rtp_port_max" :min="1024" :max="65534" size="small" controls-position="right" class="w-full" :disabled="readonly" />
          </div>
        </div>
        <div class="form-item mt-3">
          <label class="form-label">编解码</label>
          <div class="codec-list">
            <el-tag
              v-for="(c, i) in form.media.codecs"
              :key="i"
              size="small"
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
              size="small"
              filterable
              allow-create
              default-first-option
              placeholder="选择或输入编解码"
              class="flex-1"
            >
              <el-option v-for="c in codecOptions" :key="c" :label="c" :value="c" />
            </el-select>
            <el-button size="small" @click="addCodec">添加</el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 用户列表 -->
    <div class="settings-section">
      <div class="section-title">
        <span>软电话用户</span>
        <el-button v-if="!readonly" size="small" type="primary" plain @click="addUser">添加用户</el-button>
      </div>
      <div class="section-body">
        <div v-if="form.users.length === 0" class="empty-hint">
          暂无用户，点击右上角添加。
        </div>
        <div v-for="(u, i) in form.users" :key="i" class="user-row">
          <el-input v-model="u.username" placeholder="用户名" size="small" class="flex-1" :disabled="readonly" />
          <el-input v-model="u.password" placeholder="密码" size="small" type="password" show-password class="flex-1" :disabled="readonly" />
          <el-input v-model="u.device_id" placeholder="设备ID（可选，Linphone用）" size="small" class="flex-1" :disabled="readonly" />
          <el-button v-if="!readonly" size="small" type="danger" plain @click="removeUser(i)">删除</el-button>
        </div>
      </div>
    </div>

    <!-- 推送通知 -->
    <div class="settings-section">
      <div class="section-title">推送通知（Linphone APNs/FCM）</div>
      <div class="section-body">
        <div class="form-grid">
          <div class="form-item">
            <label class="form-label">Linphone User</label>
            <el-input v-model="form.linphone_push.linphone_user" placeholder="Linphone 推送用户名" size="small" :disabled="readonly" />
          </div>
          <div class="form-item">
            <label class="form-label">Linphone Password</label>
            <el-input v-model="form.linphone_push.linphone_password" placeholder="Linphone 推送密码" size="small" type="password" show-password :disabled="readonly" />
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

.user-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.empty-hint {
  font-size: 12px;
  color: var(--muted-foreground);
  padding: 8px 0;
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
  .user-row {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
