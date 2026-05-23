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
        :expanded-keys="expandedKeys"
        @update:expanded-keys="handleExpandedKeysUpdate"
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

        <n-menu
          :options="menuOptions"
          :value="activeKey"
          :expanded-keys="expandedKeys"
          @update:expanded-keys="handleExpandedKeysUpdate"
          @update:value="handleMobileMenuSelect"
        />
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
                <n-text class="username">{{ userInfo.username }}</n-text>
                <n-text v-if="!isMobile && userStore.isAdmin" depth="3" class="user-role"
                  >管理员</n-text
                >
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
import { ref, computed, onMounted, onUnmounted, h, watch } from 'vue'
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
  FolderOutline as FolderIcon,
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

type AppMenuItem = {
  key: string
  label: string
  icon?: () => ReturnType<typeof h>
  route?: string
  adminOnly?: boolean
  children?: AppMenuItem[]
}

const menuTree: AppMenuItem[] = [
  {
    key: 'dashboard',
    label: '仪表盘',
    route: '/@dashboard',
    icon: () => h(NIcon, null, { default: () => h(HomeIcon) }),
  },
  {
    key: 'file',
    label: '文件管理',
    icon: () => h(NIcon, null, { default: () => h(FolderIcon) }),
    children: [
      {
        key: 'file-browse',
        label: '文件浏览',
        route: '/',
        icon: () => h(NIcon, null, { default: () => h(FileBrowserIcon) }),
      },
      {
        key: 'storage-manage',
        label: '存储管理',
        route: '/@dashboard/storages',
        icon: () => h(NIcon, null, { default: () => h(StorageIcon) }),
      },
      {
        key: 'auto-import',
        label: '自动入库',
        route: '/@dashboard/autoingest',
        icon: () => h(NIcon, null, { default: () => h(AutoIngestIcon) }),
      },
    ],
  },
  {
    key: 'user',
    label: '用户与权限',
    icon: () => h(NIcon, null, { default: () => h(UsersIcon) }),
    children: [
      {
        key: 'user-manage',
        label: '用户管理',
        route: '/@dashboard/users',
        adminOnly: true,
        icon: () => h(NIcon, null, { default: () => h(UsersIcon) }),
      },
      {
        key: 'group-manage',
        label: '用户组管理',
        route: '/@dashboard/usergroups',
        adminOnly: true,
        icon: () => h(NIcon, null, { default: () => h(UserGroupsIcon) }),
      },
      {
        key: 'token',
        label: '令牌管理',
        route: '/@dashboard/cloudtokens',
        icon: () => h(NIcon, null, { default: () => h(TokenIcon) }),
      },
      {
        key: 'personal-info',
        label: '个人资料',
        route: '/@dashboard/profile',
        icon: () => h(NIcon, null, { default: () => h(ProfileIcon) }),
      },
    ],
  },
  {
    key: 'extend',
    label: '扩展集成',
    adminOnly: true,
    icon: () => h(NIcon, null, { default: () => h(ExtensionsIcon) }),
    children: [
      {
        key: 'extend-feature',
        label: '拓展功能',
        route: '/@dashboard/extensions',
      },
      {
        key: 'telegram-bot',
        label: 'Telegram Bot',
        route: '/@dashboard/telegram',
      },
      {
        key: 'subscription',
        label: '订阅管理',
        route: '/@dashboard/subscriptions',
      },
    ],
  },
  {
    key: 'system',
    label: '系统管理',
    adminOnly: true,
    icon: () => h(NIcon, null, { default: () => h(SettingsIcon) }),
    children: [
      {
        key: 'aggregate-log',
        label: '聚合日志',
        route: '/@dashboard/logs',
        icon: () => h(NIcon, null, { default: () => h(TaskLogIcon) }),
      },
      {
        key: 'system-setting',
        label: '系统设置',
        route: '/@dashboard/settings',
        icon: () => h(NIcon, null, { default: () => h(SettingsIcon) }),
      },
    ],
  },
]

// 响应式检测
const isMobile = ref(false)
const mobileMenuVisible = ref(false)
const expandedKeys = ref<string[]>([])

// 侧边栏折叠状态
const collapsed = ref(false)

// 修改密码弹窗
const showChangePasswordModal = ref(false)

const filterMenuTree = (items: AppMenuItem[]): AppMenuItem[] => {
  return items
    .filter((item) => !item.adminOnly || !systemInfo.enableAuth || userStore.isAdmin)
    .map((item) => ({
      ...item,
      children: item.children ? filterMenuTree(item.children) : undefined,
    }))
    .filter((item) => !item.children || item.children.length > 0 || !!item.route)
}

const visibleMenuTree = computed(() => filterMenuTree(menuTree))

const routeKeyMap: Record<string, string> = {
  '/@dashboard': 'dashboard',
  '/': 'file-browse',
  '/@dashboard/storages': 'storage-manage',
  '/@dashboard/autoingest': 'auto-import',
  '/@dashboard/users': 'user-manage',
  '/@dashboard/usergroups': 'group-manage',
  '/@dashboard/cloudtokens': 'token',
  '/@dashboard/extensions': 'extend-feature',
  '/@dashboard/telegram': 'telegram-bot',
  '/@dashboard/subscriptions': 'subscription',
  '/@dashboard/logs': 'aggregate-log',
  '/@dashboard/settings': 'system-setting',
  '/@dashboard/profile': 'personal-info',
}

const activeKey = computed(() => {
  if (route.path === '/@dashboard/storages' && route.query.tab === 'overview') {
    return 'storage'
  }

  if (route.path !== '/' && !route.path.startsWith('/@dashboard')) {
    return 'file-browse'
  }

  return routeKeyMap[route.path] || route.path
})

const routeExpandedKeys = computed(() => {
  const parents: Record<string, string> = {
    'file-browse': 'file',
    'storage-manage': 'file',
    'auto-import': 'file',
    'user-manage': 'user',
    'group-manage': 'user',
    token: 'user',
    'personal-info': 'user',
    'extend-feature': 'extend',
    'telegram-bot': 'extend',
    subscription: 'extend',
    'aggregate-log': 'system',
    'system-setting': 'system',
  }

  const parentKey = parents[activeKey.value]

  return parentKey ? [parentKey] : []
})

watch(
  routeExpandedKeys,
  (keys) => {
    expandedKeys.value = Array.from(new Set([...expandedKeys.value, ...keys]))
  },
  { immediate: true }
)

// 检测屏幕尺寸
const checkScreenSize = () => {
  isMobile.value = window.innerWidth <= 768
  // 在移动端切换时关闭菜单
  if (!isMobile.value) {
    mobileMenuVisible.value = false
  }
}

const findMenuChain = (
  items: AppMenuItem[],
  key: string,
  parents: AppMenuItem[] = []
): AppMenuItem[] => {
  for (const item of items) {
    const chain = [...parents, item]

    if (item.key === key) {
      return chain
    }

    if (item.children?.length) {
      const childChain = findMenuChain(item.children, key, chain)
      if (childChain.length) {
        return childChain
      }
    }
  }

  return []
}

const breadcrumbs = computed(() => {
  const chain = findMenuChain(visibleMenuTree.value, activeKey.value)

  if (!chain.length) {
    const matched = route.matched.filter((item) => item.meta?.title)
    return matched.map((item) => ({
      title: item.meta?.title as string,
      path: item.path,
    }))
  }

  return chain.map((item) => ({
    title: item.label,
    path: item.route || item.key,
  }))
})

const menuOptions = computed((): MenuOption[] => {
  const toOption = (item: AppMenuItem): MenuOption => ({
    label: item.label,
    key: item.key,
    icon: item.icon,
    children: item.children?.map(toOption),
  })

  return visibleMenuTree.value.map(toOption)
})

const handleExpandedKeysUpdate = (keys: string[]) => {
  expandedKeys.value = keys
}

// 用户菜单选项
const userMenuOptions: DropdownOption[] = [
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
  const chain = findMenuChain(visibleMenuTree.value, key)
  const current = chain[chain.length - 1]
  if (current?.route) {
    router.push(current.route)
  }
}

// 处理移动端菜单选择
const handleMobileMenuSelect = (key: string) => {
  handleMenuSelect(key)
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

.user-role {
  font-size: 14px;
  color: var(--n-text-color-3);
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

  .username,
  .user-role {
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
