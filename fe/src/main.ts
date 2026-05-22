import { createApp, watchEffect } from 'vue'
import App from './App.vue'
import router from './router/index'
import { pinia, useSystemStore, useUserStore } from './stores'
import { registerNaiveComponents } from './plugins/naive'

import './style.css'

const app = createApp(App)

app.use(pinia)

// 动态设置页面标题：基于系统设置的 title
const systemStore = useSystemStore()
const userStore = useUserStore()
// 先从本地存储加载一次，避免首屏闪烁
systemStore.load()
userStore.load()

app.use(router)
registerNaiveComponents(app)

const setDocTitle = (t?: string) => {
  document.title = t && t.trim().length > 0 ? t : '云盘189分享'
}

// 首次根据已加载的信息设置
setDocTitle(systemStore.get().title)

// 拉取最新系统信息后再次覆盖
systemStore
  .refresh()
  .then(() => {
    setDocTitle(systemStore.get().title)
    if (!systemStore.get().initialized) {
      router.replace('/@init')
    }
  })
  .catch(() => {})

// 监听系统标题变化，实时更新 document.title
watchEffect(() => {
  const info = systemStore.get()
  setDocTitle(info.title)
})

app.mount('#app')
