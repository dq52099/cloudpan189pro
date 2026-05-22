<template>
  <n-config-provider :theme="theme" :theme-overrides="themeOverrides">
    <n-global-style />
    <n-message-provider>
      <n-notification-provider>
        <n-dialog-provider>
          <n-modal-provider>
            <router-view />
          </n-modal-provider>
        </n-dialog-provider>
      </n-notification-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import {
  NModalProvider,
  NDialogProvider,
  NMessageProvider,
  NNotificationProvider,
  NGlobalStyle,
} from 'naive-ui'
import { useThemeStore } from '@/stores'
import { createTheme, createThemeOverrides } from '@/theme'

const themeStore = useThemeStore()

const theme = computed(() => createTheme(themeStore.isDark))
const themeOverrides = computed(() => createThemeOverrides(themeStore.isDark))

// 应用启动时初始化主题
onMounted(() => {
  themeStore.initTheme()
})
</script>
