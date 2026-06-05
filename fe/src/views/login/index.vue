<template>
  <div class="login-container" :class="{ dark: themeStore.isDark }">
    <div class="login-card">
      <div class="header-actions">
        <n-switch :value="themeStore.isDark" @update:value="themeStore.toggleTheme">
          <template #checked>夜间模式</template>
          <template #unchecked>日间模式</template>
        </n-switch>
      </div>
      <div class="login-header">
        <h1>{{ systemInfo.title || '云盘分享系统' }}</h1>
        <p>请登录您的账户</p>
      </div>

      <n-form ref="formRef" :model="formData" :rules="rules" size="large" :show-label="false">
        <n-form-item path="username">
          <n-input
            v-model:value="formData.username"
            placeholder="用户名"
            :input-props="{ autocomplete: 'username' }"
          >
            <template #prefix>
              <n-icon :component="PersonOutline" />
            </template>
          </n-input>
        </n-form-item>

        <n-form-item path="password">
          <n-input
            v-model:value="formData.password"
            type="password"
            placeholder="密码"
            show-password-on="mousedown"
            :input-props="{ autocomplete: 'current-password' }"
            @keydown.enter="handleLogin"
          >
            <template #prefix>
              <n-icon :component="LockClosedOutline" />
            </template>
          </n-input>
        </n-form-item>

        <n-form-item>
          <n-button
            type="primary"
            size="large"
            :loading="loading"
            :block="true"
            @click="handleLogin"
          >
            登录
          </n-button>
        </n-form-item>
      </n-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onUnmounted, ref, reactive } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage, type FormInst } from 'naive-ui'
import { PersonOutline, LockClosedOutline } from '@vicons/ionicons5'
import { type LoginRequest } from '@/api/auth'
import { useSystemStore, useAuthStore, useThemeStore } from '@/stores'
import { getErrorMessage } from '@/utils/api'
import { getSafePostLoginRedirectPath } from '@/utils/redirect'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const systemStore = useSystemStore()
const authStore = useAuthStore()
const themeStore = useThemeStore()

const systemInfo = systemStore.get()

const formRef = ref<FormInst>()
const loading = ref(false)
let isComponentMounted = true
let loginRequestId = 0

const formData = reactive<LoginRequest>({
  username: '',
  password: '',
})

const rules = {
  username: [
    {
      required: true,
      message: '请输入用户名',
      trigger: ['input', 'blur'],
    },
  ],
  password: [
    {
      required: true,
      message: '请输入密码',
      trigger: ['input', 'blur'],
    },
  ],
}

const isCurrentLoginRequest = (requestId: number) =>
  isComponentMounted && requestId === loginRequestId

const getRedirectTarget = () => {
  const redirect = route.query.redirect

  return getSafePostLoginRedirectPath(Array.isArray(redirect) ? redirect[0] : redirect)
}

const handleLogin = async () => {
  if (loading.value) return

  const requestId = ++loginRequestId
  loading.value = true
  try {
    await formRef.value?.validate()
    await authStore.login(formData)
    if (!isCurrentLoginRequest(requestId)) return

    message.success('登录成功')
    await router.push(getRedirectTarget())
  } catch (error: unknown) {
    if (!isCurrentLoginRequest(requestId)) return

    const errorMessage = getErrorMessage(error, '登录失败，请检查用户名和密码')

    console.error('登录失败:', errorMessage)
    message.error(errorMessage)
  } finally {
    if (isCurrentLoginRequest(requestId)) {
      loading.value = false
    }
  }
}

onUnmounted(() => {
  isComponentMounted = false
  loginRequestId += 1
})
</script>

<style scoped>
.login-container {
  min-height: 100dvh;
  display: grid;
  place-items: center;
  padding: clamp(16px, 4vw, 40px);
  background:
    linear-gradient(180deg, rgb(255 255 255 / 72%), rgb(255 255 255 / 0%)), var(--n-color);
}

.login-container.dark {
  background: var(--n-color-target);
}

.login-container.dark .login-card {
  box-shadow: 0 18px 42px rgb(0 0 0 / 28%);
}

.header-actions {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 24px;
}

.login-card {
  width: 100%;
  max-width: 400px;
  background: var(--n-card-color);
  border-radius: 8px;
  padding: 32px;
  box-shadow: 0 14px 38px rgb(15 23 42 / 10%);
  border: 1px solid var(--n-border-color);
}

.login-header {
  text-align: center;
  margin-bottom: 28px;
}

.login-header h1 {
  font-size: 24px;
  font-weight: 600;
  color: var(--n-text-color);
  margin: 0 0 8px;
  overflow-wrap: anywhere;
}

.login-header p {
  font-size: 15px;
  color: var(--n-text-color-2);
  margin: 0;
}

.n-form-item {
  margin-bottom: 20px;
}

.n-form-item:last-child {
  margin-bottom: 0;
  margin-top: 32px;
}

@media (width <= 768px) {
  .login-card {
    padding: 24px;
  }
}
</style>
