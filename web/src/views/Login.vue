<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useRoute, useRouter } from 'vue-router'
import { useSiteConfig } from '../composables/useSiteConfig'
import { WeatherSunny24Regular, WeatherMoon24Regular } from '@vicons/fluent'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const { siteConfig } = useSiteConfig()

const form = ref({ username: '', password: '' })
const loading = ref(false)
const isDark = ref(false)

// 主题切换
function applyTheme(dark: boolean) {
  isDark.value = dark
  if (dark) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
  localStorage.setItem('theme', dark ? 'dark' : 'light')
}

function toggleTheme() {
  applyTheme(!isDark.value)
}

onMounted(() => {
  // 读取主题偏好
  const saved = localStorage.getItem('theme')
  if (saved === 'dark') {
    applyTheme(true)
  } else if (saved === 'light') {
    applyTheme(false)
  } else {
    // 跟随系统
    applyTheme(window.matchMedia('(prefers-color-scheme: dark)').matches)
  }
})

async function handleLogin() {
  const { ElMessage } = await import('element-plus')
  if (!form.value.username || !form.value.password) {
    ElMessage.warning('Please enter username and password')
    return
  }
  loading.value = true
  const success = await auth.login(form.value.username, form.value.password)
  loading.value = false
  if (success) {
    ElMessage.success('Welcome back')
    const q = typeof route.query.redirect === 'string' ? route.query.redirect : ''
    let redirect = q ? decodeURIComponent(q) : ''
    if (!redirect) {
      try { redirect = sessionStorage.getItem('post_login_redirect') || '' } catch {}
    }
    if (redirect) {
      try { sessionStorage.removeItem('post_login_redirect') } catch {}
      router.push(redirect)
    } else {
      router.push('/')
    }
  } else {
    ElMessage.error('Login failed, please check credentials')
  }
}
</script>

<template>
  <div class="login-page">
    <!-- 主题切换按钮 -->
    <button class="theme-toggle" @click="toggleTheme" title="Toggle theme">
      <component :is="isDark ? WeatherSunny24Regular : WeatherMoon24Regular" />
    </button>

    <!-- 登录卡片 -->
    <div class="login-card">
      <!-- Logo + 站点名 -->
      <div class="login-logo">
        <img v-if="siteConfig.has_logo" :src="'/api/site/logo'" alt="Logo" class="login-logo-img" />
        <div v-else class="login-logo-placeholder">V</div>
        <span class="login-site-name">{{ siteConfig.name }}</span>
      </div>

      <!-- 标题 -->
      <h1 class="login-title">Welcome back</h1>
      <p class="login-subtitle">Login with your Apple or Google account</p>

      <!-- 分隔线 -->
      <div class="login-separator"></div>

      <!-- 表单 -->
      <form @submit.prevent="handleLogin" class="login-form">
        <label class="login-label" for="username">Name</label>
        <input
          id="username"
          v-model="form.username"
          type="text"
          autocomplete="username"
          :disabled="loading"
          class="login-input"
          placeholder=""
        />

        <label class="login-label" for="password">Password</label>
        <input
          id="password"
          v-model="form.password"
          type="password"
          autocomplete="current-password"
          :disabled="loading"
          class="login-input"
          placeholder=""
          @keyup.enter="handleLogin"
        />

        <button type="submit" :disabled="loading" class="login-button">
          <span v-if="loading" class="login-spinner"></span>
          <span v-else>Login</span>
        </button>
      </form>
    </div>

    <!-- 底部文字 -->
    <p class="login-footer">
      By clicking continue, you agree to our<br />
      <a href="#">Terms of Service</a> and <a href="#">Privacy Policy</a>.
    </p>
  </div>
</template>

<style scoped>
.login-page {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  position: relative;
}

/* 主题切换按钮 */
.theme-toggle {
  position: absolute;
  top: 16px;
  right: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--card);
  color: var(--muted-foreground);
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s;
  z-index: 10;
}

.theme-toggle:hover {
  border-color: var(--brand);
  color: var(--foreground);
}

.theme-toggle :deep(svg) {
  width: 20px;
  height: 20px;
}

/* 登录卡片 */
.login-card {
  width: 100%;
  max-width: 400px;
  padding: 32px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--card);
}

/* Logo 区域 */
.login-logo {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-bottom: 24px;
}

.login-logo-img {
  height: 28px;
  width: auto;
  object-fit: contain;
}

.login-logo-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: var(--brand);
  color: white;
  font-size: 14px;
  font-weight: 700;
}

.login-site-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--foreground);
}

/* 标题 */
.login-title {
  text-align: center;
  font-size: 20px;
  font-weight: 600;
  color: var(--foreground);
  margin: 0 0 4px;
}

.login-subtitle {
  text-align: center;
  font-size: 13px;
  color: var(--muted-foreground);
  margin: 0 0 20px;
}

/* 分隔线 */
.login-separator {
  height: 1px;
  background: var(--border);
  margin: 0 0 24px;
}

/* 表单 */
.login-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.login-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--foreground);
  margin-bottom: -8px;
}

.login-input {
  height: 40px;
  width: 100%;
  padding: 0 12px;
  border: 1px solid var(--input);
  border-radius: 8px;
  background: var(--background);
  color: var(--foreground);
  font-size: 14px;
  outline: none;
  transition: border-color 0.15s;
}

.login-input:focus {
  border-color: var(--brand);
}

.login-input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* 登录按钮 */
.login-button {
  height: 40px;
  width: 100%;
  border: none;
  border-radius: 8px;
  background: var(--brand);
  color: white;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.15s;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.login-button:hover {
  opacity: 0.9;
}

.login-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.login-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* 底部文字 */
.login-footer {
  margin-top: 24px;
  text-align: center;
  font-size: 12px;
  color: var(--muted-foreground);
  line-height: 1.6;
}

.login-footer a {
  color: var(--brand);
  text-decoration: underline;
  text-underline-offset: 2px;
}
</style>
