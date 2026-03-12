<template>
  <n-layout has-sider class="base-layout">
    <!-- 桌面端侧边栏 -->
    <n-layout-sider
      v-if="!isMobile"
      bordered
      collapse-mode="width"
      :collapsed-width="64"
      :width="200"
      :collapsed="collapsed"
      show-trigger
      @collapse="collapsed = true"
      @expand="collapsed = false"
    >
      <div class="logo">
        <div v-if="!collapsed" class="logo-content">
          <CloudPanLogo :size="28" variant="default" />
          <n-text strong class="logo-text">
            {{ systemInfo.title || '云盘管理系统' }}
          </n-text>
        </div>
        <div v-else class="logo-collapsed">
          <CloudPanLogo :size="24" variant="collapsed" />
        </div>
      </div>

      <n-menu
        :collapsed="collapsed"
        :collapsed-width="64"
        :collapsed-icon-size="22"
        :options="menuOptions"
        :value="activeKey"
        @update:value="handleMenuSelect"
      />
    </n-layout-sider>

    <!-- 移动端抽屉式侧边栏 -->
    <n-drawer
      v-if="isMobile"
      v-model:show="mobileMenuVisible"
      :width="240"
      placement="left"
      class="mobile-drawer"
    >
      <n-drawer-content title="" :native-scrollbar="false">
        <div class="mobile-logo">
          <CloudPanLogo :size="24" variant="default" />
          <n-text strong class="mobile-logo-text">
            {{ systemInfo.title || '云盘管理系统' }}
          </n-text>
        </div>

        <n-menu :options="menuOptions" :value="activeKey" @update:value="handleMobileMenuSelect" />
      </n-drawer-content>
    </n-drawer>

    <!-- 主内容区域 -->
    <n-layout style="flex: 1; display: flex; flex-direction: column">
      <!-- 顶部导航栏 -->
      <n-layout-header bordered class="header">
        <div class="header-content">
          <div class="header-left">
            <!-- 移动端菜单按钮 -->
            <n-button
              v-if="isMobile"
              text
              circle
              class="mobile-menu-btn"
              @click="mobileMenuVisible = true"
            >
              <template #icon>
                <n-icon size="20">
                  <MenuIcon />
                </n-icon>
              </template>
            </n-button>

            <!-- 面包屑导航 -->
            <n-breadcrumb v-if="!isMobile || breadcrumbs.length <= 2">
              <n-breadcrumb-item v-for="item in breadcrumbs" :key="item.path">
                {{ item.title }}
              </n-breadcrumb-item>
            </n-breadcrumb>

            <!-- 移动端简化标题 -->
            <n-text v-if="isMobile && breadcrumbs.length > 2" strong class="mobile-title">
              {{ breadcrumbs[breadcrumbs.length - 1]?.title }}
            </n-text>
          </div>

          <div class="header-right">
            <!-- 夜间模式切换（更明显的太阳/月亮按钮，带提示） -->
            <n-tooltip trigger="hover">
              <template #trigger>
                <n-button
                  circle
                  size="small"
                  class="theme-toggle-btn"
                  type="primary"
                  ghost
                  @click="themeStore.toggleTheme()"
                >
                  <template #icon>
                    <n-icon size="20">
                      <MoonIcon v-if="themeStore.isDark" />
                      <SunnyIcon v-else />
                    </n-icon>
                  </template>
                </n-button>
              </template>
              <span>{{ themeStore.isDark ? '切换为日间模式' : '切换为夜间模式' }}</span>
            </n-tooltip>

            <!-- 用户信息 -->
            <n-dropdown :options="userMenuOptions" @select="handleUserMenuSelect">
              <div class="user-info">
                <n-text v-if="!isMobile" class="username">{{ userInfo.username }}</n-text>
                <n-icon v-if="!isMobile" size="16" class="dropdown-icon">
                  <ChevronDownIcon />
                </n-icon>
              </div>
            </n-dropdown>
          </div>
        </div>
      </n-layout-header>

      <!-- 内容区域 -->
      <n-layout-content class="content" :native-scrollbar="false">
        <div class="content-wrapper">
          <router-view />
        </div>
      </n-layout-content>
    </n-layout>

    <!-- 修改密码弹窗 -->
    <ChangePasswordModal
      v-model:show="showChangePasswordModal"
      @success="handleChangePasswordSuccess"
    />
  </n-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, h } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  NLayout,
  NLayoutSider,
  NLayoutHeader,
  NLayoutContent,
  NMenu,
  NBreadcrumb,
  NBreadcrumbItem,
  NDropdown,
  NText,
  NIcon,
  NDrawer,
  NDrawerContent,
  NButton,
  useMessage,
  type MenuOption,
  type DropdownOption,
} from 'naive-ui'
import {
  ChevronDown as ChevronDownIcon,
  HomeOutline as HomeIcon,
  PeopleOutline as UsersIcon,
  PeopleCircleOutline as UserGroupsIcon,
  SettingsOutline as SettingsIcon,
  KeyOutline as TokenIcon,
  MenuOutline as MenuIcon,
  ServerOutline as StorageIcon,
  PersonOutline as ProfileIcon,
  FolderOpenOutline as FileBrowserIcon,
  DocumentTextOutline as TaskLogIcon,
  HammerOutline as AutoIngestIcon,
  SunnyOutline as SunnyIcon,
  MoonOutline as MoonIcon,
  AppsOutline as ExtensionsIcon,
} from '@vicons/ionicons5'
import { useAuthStore, useSystemStore, useThemeStore } from '@/stores'
import CloudPanLogo from '@/components/CloudPanLogo.vue'
import { useUserStore } from '@/stores'
import ChangePasswordModal from '@/components/profile/ChangePasswordModal.vue'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const authStore = useAuthStore()
const userStore = useUserStore()
const systemStore = useSystemStore()
const themeStore = useThemeStore()

// 系统信息
const systemInfo = systemStore.get()
const userInfo = userStore.get()

// 响应式检测
const isMobile = ref(false)
const mobileMenuVisible = ref(false)

// 侧边栏折叠状态
const collapsed = ref(false)

// 修改密码弹窗
const showChangePasswordModal = ref(false)

// 当前激活的菜单项
const activeKey = computed(() => route.path)

// 检测屏幕尺寸
const checkScreenSize = () => {
  isMobile.value = window.innerWidth <= 768
  // 在移动端切换时关闭菜单
  if (!isMobile.value) {
    mobileMenuVisible.value = false
  }
}

// 面包屑导航
const breadcrumbs = computed(() => {
  const matched = route.matched.filter((item) => item.meta?.title)
  return matched.map((item) => ({
    title: item.meta?.title as string,
    path: item.path,
  }))
})

/**
 * 菜单选项
 * 折叠状态下返回扁平菜单（不显示分组标题，避免样式错乱）
 * 展开状态下返回分组菜单（主要功能 / 文件浏览 / 管理功能）
 */
const menuOptions = computed((): MenuOption[] => {
  // 基础项（非分组）
  const mainItems: MenuOption[] = [
    {
      label: '仪表盘',
      key: '/@dashboard',
      icon: () => h(NIcon, null, { default: () => h(HomeIcon) }),
    },
    {
      label: '个人资料',
      key: '/@dashboard/profile',
      icon: () => h(NIcon, null, { default: () => h(ProfileIcon) }),
    },
    {
      label: '令牌管理',
      key: '/@dashboard/cloudtokens',
      icon: () => h(NIcon, null, { default: () => h(TokenIcon) }),
    },
    {
      label: '存储管理',
      key: '/@dashboard/storages',
      icon: () => h(NIcon, null, { default: () => h(StorageIcon) }),
    },
    {
      label: '自动入库',
      key: '/@dashboard/autoingest',
      icon: () => h(NIcon, null, { default: () => h(AutoIngestIcon) }),
    },
  ]

  const browseItems: MenuOption[] = [
    {
      label: '文件浏览',
      key: '/@dashboard/file-browser',
      icon: () => h(NIcon, null, { default: () => h(FileBrowserIcon) }),
    },
  ]

  const adminItems: MenuOption[] = [
    {
      label: '用户管理',
      key: '/@dashboard/users',
      icon: () => h(NIcon, null, { default: () => h(UsersIcon) }),
    },
    {
      label: '用户组管理',
      key: '/@dashboard/usergroups',
      icon: () => h(NIcon, null, { default: () => h(UserGroupsIcon) }),
    },
    {
      label: '拓展功能',
      key: '/@dashboard/extensions',
      icon: () => h(NIcon, null, { default: () => h(ExtensionsIcon) }),
    },
    {
      label: 'Telegram Bot',
      key: '/@dashboard/telegram',
      icon: () => h(NIcon, null, { default: () => h(ExtensionsIcon) }),
    },
    {
      label: '订阅管理',
      key: '/@dashboard/subscriptions',
      icon: () => h(NIcon, null, { default: () => h(ExtensionsIcon) }),
    },
    {
      label: '聚合日志',
      key: '/@dashboard/logs',
      icon: () => h(NIcon, null, { default: () => h(TaskLogIcon) }),
    },
    {
      label: '系统设置',
      key: '/@dashboard/settings',
      icon: () => h(NIcon, null, { default: () => h(SettingsIcon) }),
    },
  ]

  // 折叠时：扁平菜单（仅图标项，无分组标题）
  if (collapsed.value) {
    const flat: MenuOption[] = [...mainItems, ...browseItems]
    if (userStore.isAdmin) {
      flat.push(...adminItems)
    }
    return flat
  }

  // 展开时：分组菜单
  const mainGroup: MenuOption = {
    type: 'group',
    label: '主要功能',
    key: 'group-main',
    children: mainItems,
  }

  const browseGroup: MenuOption = {
    type: 'group',
    label: '文件浏览',
    key: 'group-browse',
    children: browseItems,
  }

  const groups: MenuOption[] = [mainGroup, browseGroup]
  if (userStore.isAdmin) {
    groups.push({
      type: 'group',
      label: '管理功能',
      key: 'group-admin',
      children: adminItems,
    })
  }
  return groups
})

// 用户菜单选项
const userMenuOptions: DropdownOption[] = [
  {
    label: '个人设置',
    key: 'profile',
  },
  {
    label: '修改密码',
    key: 'change-password',
  },
  {
    type: 'divider',
    key: 'divider',
  },
  {
    label: '退出登录',
    key: 'logout',
  },
]

// 处理菜单选择
const handleMenuSelect = (key: string) => {
  router.push(key)
}

// 处理移动端菜单选择
const handleMobileMenuSelect = (key: string) => {
  router.push(key)
  mobileMenuVisible.value = false // 选择后关闭菜单
}

// 处理用户菜单选择
const handleUserMenuSelect = (key: string) => {
  switch (key) {
    case 'profile':
      router.push('/@dashboard/profile')
      break
    case 'change-password':
      showChangePasswordModal.value = true
      break
    case 'logout':
      handleLogout()
      break
  }
}

// 修改密码成功回调
const handleChangePasswordSuccess = () => {
  // 密码修改成功后的处理，组件内部已经处理了成功提示和退出登录
  console.log('密码修改成功')
}

// 处理退出登录
const handleLogout = () => {
  authStore.logout()
  message.success('已退出登录')
  router.push('/@login')
}

// 初始化
onMounted(() => {
  // 初始化屏幕尺寸检测
  checkScreenSize()
  window.addEventListener('resize', checkScreenSize)

  userStore.refresh()
})

// 清理事件监听器
onUnmounted(() => {
  window.removeEventListener('resize', checkScreenSize)
})
</script>

<style scoped>
.base-layout {
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.logo {
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-bottom: 1px solid var(--n-border-color);
  margin-bottom: 8px;
}

.logo-content {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logo-collapsed {
  display: flex;
  align-items: center;
  justify-content: center;
}

.logo-text {
  font-size: 18px;
  color: var(--n-text-color);
  white-space: nowrap;
}

.logo-icon {
  color: var(--n-primary-color);
}

/* 移动端抽屉样式 */
.mobile-drawer :deep(.n-drawer-content) {
  padding: 0;
}

.mobile-logo {
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-bottom: 1px solid var(--n-border-color);
  margin-bottom: 8px;
  padding: 0 16px;
  gap: 12px;
}

.mobile-logo-text {
  font-size: 18px;
  color: var(--n-text-color);
  white-space: nowrap;
}

.header {
  height: 64px;
  display: flex;
  align-items: center;
  padding: 0 24px;
  position: sticky;
  top: 0;
  z-index: 100;
  background-color: var(--n-color);
}

.header-content {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-left {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.theme-toggle-btn {
  transition: all 0.2s ease;
}

/* 始终让图标有颜色（不依赖 hover） */
:deep(.theme-toggle-btn .n-icon),
:deep(.theme-toggle-btn .n-icon > svg) {
  color: var(--n-primary-color) !important;
  fill: var(--n-primary-color) !important;
}

/* ghost 按钮悬停/按下时，边框与图标颜色加深 */
.theme-toggle-btn:hover {
  border-color: var(--n-primary-color-hover) !important;
}

.theme-toggle-btn:hover :deep(.n-icon),
.theme-toggle-btn:hover :deep(.n-icon > svg) {
  color: var(--n-primary-color-hover) !important;
  fill: var(--n-primary-color-hover) !important;
}

.theme-toggle-btn:active {
  border-color: var(--n-primary-color-pressed) !important;
}

.theme-toggle-btn:active :deep(.n-icon),
.theme-toggle-btn:active :deep(.n-icon > svg) {
  color: var(--n-primary-color-pressed) !important;
  fill: var(--n-primary-color-pressed) !important;
}

.mobile-menu-btn {
  margin-right: 8px;
  color: var(--n-text-color-1) !important;
  transition: all 0.3s ease;
}

.mobile-menu-btn:hover {
  background-color: var(--n-hover-color) !important;
  color: var(--n-primary-color) !important;
}

.mobile-menu-btn:active {
  background-color: var(--n-pressed-color) !important;
  color: var(--n-primary-color-pressed) !important;
}

/* 确保图标在所有主题下都有足够的对比度 */
.mobile-menu-btn .n-icon {
  color: inherit;
}

.mobile-title {
  font-size: 16px;
  color: var(--n-text-color);
}

.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 6px;
  cursor: pointer;
  transition: background-color 0.3s;
}

.user-info:hover {
  background-color: var(--n-hover-color);
}

.username {
  font-size: 14px;
  color: var(--n-text-color);
}

.dropdown-icon {
  color: var(--n-text-color-3);
  transition: transform 0.3s;
}

.content {
  padding: 24px;
  overflow: auto;
  flex: 1; /* 自动占用剩余空间 */
  height: calc(100vh - 64px); /* 减去header高度 */
}

.content-wrapper {
  max-width: 1200px;
  margin: 0 auto;
  min-height: 100%;
}

/* 响应式设计 */
@media (width <= 768px) {
  .base-layout {
    height: 100vh;
    height: 100dvh;

    /* 动态视口高度，适配移动端地址栏 */
  }

  .header {
    padding: 0 16px;
    height: 56px;

    /* 移动端稍微降低高度 */
  }

  .header-left {
    gap: 8px;
  }

  .header-right {
    gap: 8px;
  }

  .content {
    padding: 16px;
    padding-bottom: env(safe-area-inset-bottom, 16px);
    height: calc(100vh - 56px); /* 移动端减去header高度 */

    /* 适配刘海屏底部安全区域 */
  }

  .content-wrapper {
    max-width: none;
    margin: 0;
  }

  .username {
    display: none;
  }

  .user-info {
    padding: 6px 8px;
  }

  /* 移动端菜单样式优化 */
  .mobile-drawer :deep(.n-menu-item) {
    padding: 12px 16px;
  }

  .mobile-drawer :deep(.n-menu-item-content) {
    padding: 8px 0;
  }

  .mobile-drawer :deep(.n-menu-item-content-header) {
    font-size: 16px;
  }
}

/* 超小屏幕适配 */
@media (width <= 480px) {
  .header {
    padding: 0 12px;
  }

  .content {
    padding: 12px;
  }

  .mobile-title {
    font-size: 14px;
  }

  .mobile-drawer {
    width: 100vw !important;
  }

  .mobile-drawer :deep(.n-drawer-content) {
    width: 100vw;
  }
}

/* 横屏适配 */
@media (width <= 768px) and (orientation: landscape) {
  .header {
    height: 48px;
  }

  .mobile-logo {
    height: 48px;
  }

  .content {
    padding: 12px 16px;
    height: calc(100vh - 48px); /* 横屏模式减去header高度 */
  }
}
</style>
