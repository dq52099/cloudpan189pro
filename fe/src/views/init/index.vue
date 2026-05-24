<template>
  <div class="init-container" :class="{ dark: themeStore.isDark }">
    <!-- 背景装饰元素 -->
    <div class="bg-decoration">
      <div class="floating-circle circle-1"></div>
      <div class="floating-circle circle-2"></div>
      <div class="floating-circle circle-3"></div>
      <div class="floating-circle circle-4"></div>
      <div class="floating-circle circle-5"></div>
      <div class="wave wave-1"></div>
      <div class="wave wave-2"></div>
    </div>

    <div class="init-card">
      <div class="init-header">
        <div class="header-actions">
          <n-switch :value="themeStore.isDark" @update:value="themeStore.toggleTheme">
            <template #checked>夜间模式</template>
            <template #unchecked>日间模式</template>
          </n-switch>
        </div>
        <h1>系统初始化</h1>
        <p>欢迎使用云盘分享系统，请完成系统初始化配置</p>
      </div>

      <n-form ref="formRef" :model="formData" :rules="rules" size="large" label-placement="top">
        <n-form-item label="系统标题" path="title">
          <n-input v-model:value="formData.title" placeholder="请输入系统标题" />
        </n-form-item>

        <n-form-item label="系统基础URL" path="baseURL">
          <n-input-group>
            <n-input v-model:value="formData.baseURL" placeholder="请输入系统基础URL" />
            <n-button type="primary" ghost :loading="autoGetUrlLoading" @click="autoGetBaseURL">
              <template #icon>
                <n-icon :component="RefreshOutline" />
              </template>
              自动获取
            </n-button>
          </n-input-group>
        </n-form-item>

        <n-form-item label="启用认证" path="enableAuth">
          <n-switch v-model:value="formData.enableAuth">
            <template #checked> 启用 </template>
            <template #unchecked> 禁用 </template>
          </n-switch>
          <n-text depth="3" style="margin-left: 12px; font-size: 14px">
            启用后需要登录才能访问WEBDAV
          </n-text>
        </n-form-item>

        <template v-if="formData.enableAuth">
          <n-form-item label="超级管理员用户名" path="superUsername">
            <n-input
              v-model:value="formData.superUsername"
              placeholder="请输入超级管理员用户名（3-20位）"
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
            >
              <template #prefix>
                <n-icon :component="LockClosedOutline" />
              </template>
            </n-input>
          </n-form-item>
        </template>

        <n-form-item>
          <n-button
            type="primary"
            size="large"
            :loading="loading"
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
import { ref, reactive, onMounted } from 'vue'
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

const router = useRouter()
const message = useMessage()
const systemStore = useSystemStore()
const themeStore = useThemeStore()

const formRef = ref<FormInst>()
const loading = ref(false)
const autoGetUrlLoading = ref(false)

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
      pattern: /^https?:\/\/.+/,
      message: '请输入有效的URL地址',
      trigger: ['input', 'blur'],
    },
  ],
  superUsername: [
    {
      required: true,
      message: '请输入超级管理员用户名',
      trigger: ['input', 'blur'],
      validator: (_rule: unknown, value: string) => {
        if (!formData.enableAuth) return true
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
        if (!formData.enableAuth) return true
        if (!value) return new Error('请输入超级管理员密码')
        if (value.length < 6 || value.length > 20) {
          return new Error('密码长度应为6-20位')
        }
        return true
      },
    },
  ],
}

// 自动获取baseURL
const autoGetBaseURL = () => {
  autoGetUrlLoading.value = true
  try {
    const { protocol, host } = window.location
    formData.baseURL = `${protocol}//${host}`
    message.success('已自动获取系统基础URL')
  } catch (error) {
    console.error('自动获取URL失败:', error)
    message.error('自动获取URL失败')
  } finally {
    autoGetUrlLoading.value = false
  }
}

// 处理初始化
const handleInit = async () => {
  try {
    await formRef.value?.validate()
    loading.value = true
    const response = await initSystem(formData)
    if (response.code === 200) {
      message.success('系统初始化成功')
      // 刷新系统信息
      const refreshResponse = await systemStore.refresh()
      const systemInfo = systemStore.get()
      const refreshEnableAuth = refreshResponse?.data?.enableAuth
      const enableAuth = (refreshEnableAuth ?? systemInfo.enableAuth) && formData.enableAuth
      await router.push(enableAuth ? '/@login' : '/@dashboard')
    } else {
      message.error(response.msg || '初始化失败')
    }
  } catch (error: unknown) {
    console.error('初始化失败:', error)
    message.error('初始化失败，请检查配置信息')
  } finally {
    loading.value = false
  }
}

// 页面加载时自动获取baseURL
onMounted(() => {
  autoGetBaseURL()
})
</script>

<style scoped>
.init-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #74b9ff 0%, #0984e3 50%, #6c5ce7 100%);
  padding: 20px;
  position: relative;
  overflow: hidden;
}

.init-container.dark {
  /* 暗色：统一采用主题背景，避免高饱和渐变造成干扰 */
  background: var(--n-color-target);
}

.init-container.dark .bg-decoration {
  display: none;
}

/* 暗色下卡片阴影更柔和，文本不加发光 */
.init-container.dark .init-card {
  box-shadow: 0 12px 32px rgb(0 0 0 / 30%);
}

.init-container.dark .init-header h1 {
  text-shadow: none;
}

.header-actions {
  position: absolute;
  right: 16px;
  top: 16px;
}

/* 背景装饰元素 */
.bg-decoration {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  z-index: 1;
}

/* 浮动圆形装饰 */
.floating-circle {
  position: absolute;
  border-radius: 50%;
  background: rgb(255 255 255 / 10%);
  backdrop-filter: blur(10px);
  animation: float 6s ease-in-out infinite;
}

.circle-1 {
  width: 80px;
  height: 80px;
  top: 10%;
  left: 10%;
  animation-delay: 0s;
}

.circle-2 {
  width: 120px;
  height: 120px;
  top: 20%;
  right: 15%;
  animation-delay: 1s;
}

.circle-3 {
  width: 60px;
  height: 60px;
  bottom: 30%;
  left: 20%;
  animation-delay: 2s;
}

.circle-4 {
  width: 100px;
  height: 100px;
  bottom: 15%;
  right: 10%;
  animation-delay: 3s;
}

.circle-5 {
  width: 40px;
  height: 40px;
  top: 50%;
  left: 5%;
  animation-delay: 4s;
}

/* 波浪装饰 */
.wave {
  position: absolute;
  width: 200%;
  height: 200px;
  background: rgb(255 255 255 / 5%);
  border-radius: 50%;
  animation: wave 8s ease-in-out infinite;
}

.wave-1 {
  top: -100px;
  left: -50%;
  animation-delay: 0s;
}

.wave-2 {
  bottom: -100px;
  right: -50%;
  animation-delay: 4s;
}

/* 动画效果 */
@keyframes float {
  0%,
  100% {
    transform: translateY(0) rotate(0deg);
  }

  50% {
    transform: translateY(-20px) rotate(180deg);
  }
}

@keyframes wave {
  0%,
  100% {
    transform: scale(1) rotate(0deg);
    opacity: 0.3;
  }

  50% {
    transform: scale(1.1) rotate(180deg);
    opacity: 0.1;
  }
}

.init-card {
  width: 100%;
  max-width: 500px;
  background: var(--n-card-color);
  backdrop-filter: blur(20px);
  border-radius: 16px;
  padding: 40px;
  box-shadow: 0 25px 50px rgb(0 0 0 / 15%);
  border: 1px solid var(--n-border-color);
  position: relative;
  z-index: 2;
}

/* 日间模式：参考登录页玻璃感卡片，降低纯白压迫感 */
.init-container:not(.dark) .init-card {
  background: rgb(255 255 255 / 92%);
  border: 1px solid rgb(255 255 255 / 20%);
  box-shadow: 0 25px 50px rgb(0 0 0 / 15%);
}

.init-header {
  text-align: center;
  margin-bottom: 32px;
}

.init-header h1 {
  font-size: 28px;
  font-weight: 600;
  color: var(--n-text-color);
  margin: 16px 0 8px;
  text-shadow: 0 2px 4px rgb(0 0 0 / 10%);
}

.init-header p {
  font-size: 16px;
  color: var(--n-text-color-2);
  margin: 0;
}

.n-form-item:last-child {
  margin-bottom: 0;
}

/* 响应式设计 */
@media (width <= 768px) {
  .floating-circle {
    display: none;
  }

  .wave {
    display: none;
  }

  .init-card {
    margin: 20px;
    padding: 30px;
    max-width: none;
  }
}
</style>
