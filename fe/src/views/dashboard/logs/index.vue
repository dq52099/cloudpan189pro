<template>
  <div class="logs-tabs-page">
    <n-tabs type="line" :value="activeTab" @update:value="handleTabChange">
      <n-tab name="engine">执行日志</n-tab>
      <n-tab name="file">任务日志</n-tab>
      <n-tab name="login">登录日志</n-tab>
    </n-tabs>

    <!-- 子路由内容渲染 -->
    <div class="content">
      <router-view />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NTabs, NTab } from 'naive-ui'

const router = useRouter()
const route = useRoute()

const activeTab = ref<string>('engine')

// 根据当前路由初始化/同步选中的 Tab
watch(
  () => route.name,
  (name) => {
    if (name === 'LogsEngine') {
      activeTab.value = 'engine'
    } else if (name === 'LogsFile') {
      activeTab.value = 'file'
    } else if (name === 'LogsLogin') {
      activeTab.value = 'login'
    }
  },
  { immediate: true }
)

// 切换 Tab 时进行路由跳转
const handleTabChange = (name: 'engine' | 'file' | 'login') => {
  if (name === 'engine') {
    router.push({ name: 'LogsEngine' })
  } else if (name === 'file') {
    router.push({ name: 'LogsFile' })
  } else if (name === 'login') {
    router.push({ name: 'LogsLogin' })
  }
}
</script>

<style scoped>
.logs-tabs-page {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.logs-tabs-page :deep(.n-tabs-tab) {
  padding: 8px 16px;
  font-size: 15px;
}

.content {
  background: var(--n-card-color);
  border-radius: 12px;
  overflow: hidden;
}
</style>
