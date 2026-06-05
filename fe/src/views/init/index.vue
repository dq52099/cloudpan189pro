<template>
  <div class="init-container" :class="{ dark: themeStore.isDark }">
    <div class="init-card">
      <div class="init-header">
        <div class="header-actions">
          <n-switch
            :value="themeStore.isDark"
            :disabled="loading"
            @update:value="themeStore.toggleTheme"
          >
            <template #checked>夜间模式</template>
            <template #unchecked>日间模式</template>
          </n-switch>
        </div>
        <h1>系统初始化</h1>
        <p>欢迎使用云盘分享系统，请完成系统初始化配置</p>
      </div>

      <n-form ref="formRef" :model="formData" :rules="rules" size="large" label-placement="top">
        <n-form-item label="系统标题" path="title">
          <n-input
            v-model:value="formData.title"
            placeholder="请输入系统标题"
            :disabled="loading"
          />
        </n-form-item>

        <n-form-item label="系统基础URL" path="baseURL">
          <n-input-group>
            <n-input
              v-model:value="formData.baseURL"
              placeholder="请输入系统基础URL"
              :disabled="loading"
            />
            <n-button
              type="primary"
              ghost
              :loading="autoGetUrlLoading"
              :disabled="loading"
              @click="autoGetBaseURL"
            >
              <template #icon>
                <n-icon :component="RefreshOutline" />
              </template>
              自动获取
            </n-button>
          </n-input-group>
        </n-form-item>

        <n-form-item label="启用认证" path="enableAuth">
          <div class="auth-setting">
            <div class="auth-setting__control">
              <n-switch v-model:value="formData.enableAuth" :disabled="loading" />
              <n-text>{{ formData.enableAuth ? '启用认证' : '关闭认证' }}</n-text>
            </div>
            <n-text class="auth-setting__help" depth="3">
              关闭后文件浏览和后台管理都将以匿名管理员身份访问
            </n-text>
          </div>
        </n-form-item>

        <n-form-item label="超级管理员用户名" path="superUsername">
          <n-input
            v-model:value="formData.superUsername"
            placeholder="请输入超级管理员用户名（3-20位）"
            :disabled="loading"
          >
            <template #prefix>
              <n-icon :component="PersonOutline" />
            </template>
          </n-input>
        </n-form-item>

        <n-form-item label="超级管理员密码" path="superPassword">
          <n-input
            v-model:value="formData.superPassword"
            type="password"
            placeholder="请输入超级管理员密码（6-20位）"
            show-password-on="mousedown"
            :disabled="loading"
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
            :disabled="loading"
            :block="true"
            @click="handleInit"
          >
            <template #icon>
              <n-icon :component="CheckmarkOutline" />
            </template>
            完成初始化
          </n-button>
        </n-form-item>
      </n-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage, type FormInst } from 'naive-ui'
import {
  RefreshOutline,
  PersonOutline,
  LockClosedOutline,
  CheckmarkOutline,
} from '@vicons/ionicons5'
import { initSystem, type InitSystemRequest } from '@/api/setting'
import { useSystemStore, useThemeStore } from '@/stores'
import { getErrorMessage } from '@/utils/api'
import { normalizeHttpBaseURL } from '@/utils/url'

const router = useRouter()
const message = useMessage()
const systemStore = useSystemStore()
const themeStore = useThemeStore()

const formRef = ref<FormInst>()
const loading = ref(false)
const autoGetUrlLoading = ref(false)
let isMounted = true
let initRequestSeq = 0

const formData = reactive<InitSystemRequest>({
  title: '云盘分享系统',
  baseURL: '',
  enableAuth: true,
  superUsername: '',
  superPassword: '',
})

const rules = {
  title: [
    {
      required: true,
      message: '请输入系统标题',
      trigger: ['input', 'blur'],
    },
  ],
  baseURL: [
    {
      required: true,
      message: '请输入系统基础URL',
      trigger: ['input', 'blur'],
    },
    {
      trigger: ['input', 'blur'],
      validator: (_rule: unknown, value: string) => {
        if (!normalizeHttpBaseURL(value)) {
          return new Error('系统基础 URL 必须是有效的 http/https 地址')
        }

        return true
      },
    },
  ],
  superUsername: [
    {
      required: true,
      message: '请输入超级管理员用户名',
      trigger: ['input', 'blur'],
      validator: (_rule: unknown, value: string) => {
        if (!value) return new Error('请输入超级管理员用户名')
        if (value.length < 3 || value.length > 20) {
          return new Error('用户名长度应为3-20位')
        }
        return true
      },
    },
  ],
  superPassword: [
    {
      required: true,
      message: '请输入超级管理员密码',
      trigger: ['input', 'blur'],
      validator: (_rule: unknown, value: string) => {
        if (!value) return new Error('请输入超级管理员密码')
        if (value.length < 6 || value.length > 20) {
          return new Error('密码长度应为6-20位')
        }
        return true
      },
    },
  ],
}

const isCurrentInitRequest = (requestSeq: number) => isMounted && requestSeq === initRequestSeq

type AutoGetBaseURLOptions = {
  silent?: boolean
}

// 自动获取baseURL
const autoGetBaseURL = (options: AutoGetBaseURLOptions = {}) => {
  const silent = options.silent === true

  if (!silent) {
    autoGetUrlLoading.value = true
  }

  try {
    const { protocol, host } = window.location
    formData.baseURL = `${protocol}//${host}`

    if (!silent) {
      message.success('已自动获取系统基础URL')
    }
  } catch (error) {
    const errorMessage = getErrorMessage(error, '自动获取URL失败')

    console.error('自动获取URL失败:', errorMessage)

    if (!silent) {
      message.error(errorMessage)
    }
  } finally {
    if (!silent) {
      autoGetUrlLoading.value = false
    }
  }
}

// 处理初始化
const handleInit = async () => {
  if (loading.value) {
    return
  }

  const requestSeq = ++initRequestSeq
  loading.value = true

  try {
    await formRef.value?.validate()
    if (!isCurrentInitRequest(requestSeq)) {
      return
    }

    const normalizedBaseURL = normalizeHttpBaseURL(formData.baseURL)
    if (!normalizedBaseURL) {
      message.warning('系统基础 URL 必须是有效的 http/https 地址')

      return
    }

    formData.baseURL = normalizedBaseURL
    const payload: InitSystemRequest = { ...formData, baseURL: normalizedBaseURL }
    const response = await initSystem(payload)
    if (!isCurrentInitRequest(requestSeq)) {
      return
    }

    if (response.code === 200) {
      message.success('系统初始化成功')
      // 刷新系统信息
      const refreshResponse = await systemStore.refresh()
      if (!isCurrentInitRequest(requestSeq)) {
        return
      }

      const nextEnableAuth =
        refreshResponse?.code === 200 && refreshResponse.data
          ? refreshResponse.data.enableAuth
          : payload.enableAuth

      await router.push(nextEnableAuth ? '/@login' : '/@dashboard')
    } else {
      message.error(response.msg || '初始化失败')
    }
  } catch (error: unknown) {
    if (!isCurrentInitRequest(requestSeq)) {
      return
    }

    const errorMessage = getErrorMessage(error, '初始化失败，请检查配置信息')

    console.error('初始化失败:', errorMessage)
    message.error(errorMessage)
  } finally {
    if (isCurrentInitRequest(requestSeq)) {
      loading.value = false
    }
  }
}

// 页面加载时自动获取baseURL
onMounted(() => {
  autoGetBaseURL({ silent: true })
})

onBeforeUnmount(() => {
  isMounted = false
  initRequestSeq++
})
</script>

<style scoped>
.init-container {
  min-height: 100dvh;
  display: grid;
  place-items: center;
  padding: clamp(16px, 4vw, 40px);
  background:
    linear-gradient(180deg, rgb(255 255 255 / 72%), rgb(255 255 255 / 0%)), var(--n-color);
}

.init-container.dark {
  background: var(--n-color-target);
}

.init-container.dark .init-card {
  box-shadow: 0 18px 42px rgb(0 0 0 / 28%);
}

.init-container.dark .init-header h1 {
  text-shadow: none;
}

.header-actions {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 24px;
}

.init-card {
  width: 100%;
  max-width: 500px;
  background: var(--n-card-color);
  border-radius: 8px;
  padding: 32px;
  box-shadow: 0 14px 38px rgb(15 23 42 / 10%);
  border: 1px solid var(--n-border-color);
}

.init-header {
  text-align: center;
  margin-bottom: 28px;
}

.init-header h1 {
  font-size: 24px;
  font-weight: 600;
  color: var(--n-text-color);
  margin: 0 0 8px;
  overflow-wrap: anywhere;
}

.init-header p {
  font-size: 15px;
  color: var(--n-text-color-2);
  margin: 0;
  line-height: 1.5;
}

.n-form-item:last-child {
  margin-bottom: 0;
}

.auth-setting {
  display: grid;
  gap: 8px;
  width: 100%;
}

.auth-setting__control {
  display: flex;
  align-items: center;
  gap: 10px;
}

.auth-setting__help {
  font-size: 14px;
  line-height: 1.5;
}

@media (width <= 768px) {
  .init-card {
    padding: 24px;
    max-width: none;
  }
}
</style>
