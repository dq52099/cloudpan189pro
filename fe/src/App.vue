<template>
  <n-config-provider :theme="theme" :theme-overrides="themeOverrides">
    <n-global-style />
    <n-message-provider>
      <n-notification-provider>
        <n-dialog-provider>
          <n-modal-provider>
            <div v-if="showStartupStatus" class="startup-status">
              <div v-if="systemStore.loading && !systemStore.error" class="startup-status__loading">
                <n-spin size="medium" />
              </div>
              <div v-else class="startup-status__panel">
                <p class="startup-status__eyebrow">服务连接异常</p>
                <h1>无法加载系统信息</h1>
                <p class="startup-status__message">{{ systemStore.error || '网络错误' }}</p>
                <n-button type="primary" :loading="systemStore.loading" @click="retryStartup">
                  重试
                </n-button>
              </div>
            </div>
            <router-view v-else />
          </n-modal-provider>
        </n-dialog-provider>
      </n-notification-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton,
  NModalProvider,
  NDialogProvider,
  NMessageProvider,
  NNotificationProvider,
  NGlobalStyle,
  NSpin,
} from 'naive-ui'
import { useSystemStore, useThemeStore } from '@/stores'
import { createTheme, createThemeOverrides } from '@/theme'
import { getErrorMessage } from '@/utils/api'

const themeStore = useThemeStore()
const systemStore = useSystemStore()
const router = useRouter()

const theme = computed(() => createTheme(themeStore.isDark))
const themeOverrides = computed(() => createThemeOverrides(themeStore.isDark))
const showStartupStatus = computed(
  () => !systemStore.remoteLoaded && (systemStore.loading || !!systemStore.error)
)

const retryStartup = async () => {
  try {
    await systemStore.ensureLoaded()
    const target =
      `${window.location.pathname}${window.location.search}${window.location.hash}` || '/'
    await router.replace(target)
  } catch (error) {
    console.error('重新加载系统信息失败:', getErrorMessage(error, '重新加载系统信息失败'))
  }
}

// 应用启动时初始化主题
onMounted(() => {
  themeStore.initTheme()
})
</script>

<style scoped>
.startup-status {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background:
    linear-gradient(135deg, rgb(54 82 120 / 8%), transparent 34%),
    linear-gradient(315deg, rgb(31 122 140 / 8%), transparent 36%), var(--n-color);
}

.startup-status__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 120px;
}

.startup-status__panel {
  width: min(420px, 100%);
  padding: 28px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  background: var(--n-color);
  box-shadow: var(--n-box-shadow);
}

.startup-status__eyebrow {
  margin: 0 0 8px;
  color: var(--n-text-color-3);
  font-size: 13px;
}

.startup-status__panel h1 {
  margin: 0;
  color: var(--n-text-color);
  font-size: 22px;
  font-weight: 600;
}

.startup-status__message {
  margin: 12px 0 20px;
  color: var(--n-text-color-2);
  line-height: 1.6;
  overflow-wrap: anywhere;
}
</style>
