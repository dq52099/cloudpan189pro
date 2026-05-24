import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore, useSystemStore } from '@/stores'
import { useUserStore } from '@/stores'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/@login',
      name: 'Login',
      component: () => import('@/views/login/index.vue'),
      meta: {
        requiresAuth: false,
        title: '登录',
      },
    },
    {
      path: '/@init',
      name: 'Init',
      component: () => import('@/views/init/index.vue'),
      meta: {
        requiresAuth: false,
        title: '初始化',
      },
    },
    {
      path: '/@dashboard',
      component: () => import('@/layouts/BaseLayout.vue'),
      meta: {
        requiresAuth: true,
      },
      children: [
        {
          path: '',
          name: 'Dashboard',
          component: () => import('@/views/dashboard/index.vue'),
          meta: {
            title: '仪表盘',
            requiresAuth: true,
          },
        },
        {
          path: 'users',
          name: 'Users',
          component: () => import('@/views/dashboard/users/index.vue'),
          meta: {
            title: '用户管理',
            requiresAuth: true,
            requiresAdmin: true,
          },
        },
        {
          path: 'usergroups',
          name: 'UserGroups',
          component: () => import('@/views/dashboard/usergroups/index.vue'),
          meta: {
            title: '用户组管理',
            requiresAuth: true,
            requiresAdmin: true,
          },
        },
        {
          path: 'cloudtokens',
          name: 'CloudTokens',
          component: () => import('@/views/dashboard/cloudtokens/index.vue'),
          meta: {
            title: '令牌管理',
            requiresAuth: true,
          },
        },
        {
          path: 'storages',
          name: 'Storages',
          component: () => import('@/views/dashboard/storages/index.vue'),
          meta: {
            title: '存储管理',
            requiresAuth: true,
          },
        },
        {
          path: 'autoingest',
          name: 'AutoIngest',
          component: () => import('@/views/dashboard/autoingest/index.vue'),
          meta: {
            title: '自动入库',
            requiresAuth: true,
          },
        },
        {
          path: 'telegram',
          name: 'Telegram',
          component: () => import('@/views/dashboard/telegram/index.vue'),
          meta: {
            title: 'Telegram Bot',
            requiresAuth: true,
            requiresAdmin: true,
          },
        },
        {
          path: 'subscriptions',
          name: 'Subscriptions',
          component: () => import('@/views/dashboard/subscriptions/index.vue'),
          meta: {
            title: '订阅管理',
            requiresAuth: true,
            requiresAdmin: true,
          },
        },
        {
          path: 'settings',
          name: 'Settings',
          component: () => import('@/views/dashboard/settings/index.vue'),
          meta: {
            title: '系统设置',
            requiresAuth: true,
            requiresAdmin: true,
          },
        },
        {
          path: 'profile',
          name: 'Profile',
          component: () => import('@/views/dashboard/profile/index.vue'),
          meta: {
            title: '个人资料',
            requiresAuth: true,
          },
        },
        {
          path: 'extensions',
          name: 'Extensions',
          component: () => import('@/views/dashboard/extensions/index.vue'),
          meta: {
            title: '拓展功能',
            requiresAuth: true,
            requiresAdmin: true,
          },
          children: [
            {
              path: '',
              name: 'ExtensionsIndex',
              redirect: { name: 'ExtensionsMedia' },
            },
            {
              path: 'media',
              name: 'ExtensionsMedia',
              component: () => import('@/views/dashboard/extensions/media/index.vue'),
              meta: {
                title: 'STRM 生成',
                requiresAuth: true,
                requiresAdmin: true,
              },
            },
          ],
        },
        {
          path: 'logs',
          name: 'Logs',
          component: () => import('@/views/dashboard/logs/index.vue'),
          meta: {
            title: '聚合日志',
            requiresAuth: true,
            requiresAdmin: true,
          },
          children: [
            {
              path: '',
              name: 'LogsIndex',
              redirect: { name: 'LogsEngine' },
            },
            {
              path: 'engine',
              name: 'LogsEngine',
              component: () => import('@/views/dashboard/logs/engine/index.vue'),
              meta: {
                title: '执行日志',
                requiresAuth: true,
                requiresAdmin: true,
              },
            },
            {
              path: 'file',
              name: 'LogsFile',
              component: () => import('@/views/dashboard/logs/file/index.vue'),
              meta: {
                title: '任务日志',
                requiresAuth: true,
                requiresAdmin: true,
              },
            },
            {
              path: 'login',
              name: 'LogsLogin',
              component: () => import('@/views/dashboard/logs/login/index.vue'),
              meta: {
                title: '登录日志',
                requiresAuth: true,
                requiresAdmin: true,
              },
            },
          ],
        },
      ],
    },
    {
      path: '/',
      name: 'FileBrowser',
      component: () => import('@/views/file-browser/index.vue'),
      meta: {
        title: '文件浏览',
        requiresAuth: true,
      },
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'FileBrowserPath',
      component: () => import('@/views/file-browser/index.vue'),
      meta: {
        title: '文件浏览',
        requiresAuth: true,
      },
    },
  ],
})

// 路由守卫
router.beforeEach(async (to, _, next) => {
  const authStore = useAuthStore()
  const userStore = useUserStore()
  const systemStore = useSystemStore()

  await systemStore.ensureLoaded()
  const systemInfo = systemStore.get()

  // 检查系统是否已初始化
  if (!systemInfo.initialized && to.name !== 'Init') {
    next('/@init')
    return
  }

  // 如果系统已初始化但访问初始化页面，跳转到仪表盘
  if (systemInfo.initialized && to.name === 'Init') {
    next('/@dashboard')
    return
  }

  if (systemInfo.initialized && !systemInfo.enableAuth) {
    if (to.name === 'Login' || to.name === 'Profile') {
      next('/@dashboard')
      return
    }

    next()
    return
  }

  // 检查是否需要认证
  if (to.meta.requiresAuth) {
    // 需要认证的路由
    if (!authStore.isLogin) {
      // 未登录，跳转到登录页
      next('/@login')
      return
    }

    if (!authStore.isAccessTokenValid) {
      try {
        const token = await authStore.doRefreshToken(true)
        if (!token) {
          next('/@login')
          return
        }
      } catch (error) {
        console.error('刷新登录状态失败:', error)
        next('/@login')
        return
      }
    }

    // 检查是否需要管理员权限
    if (to.meta.requiresAdmin) {
      let refreshedUser: Models.UserInfo | null | undefined

      try {
        refreshedUser = await userStore.refresh()
      } catch (error) {
        console.error('刷新用户权限失败:', error)
        next(authStore.isLogin ? '/@dashboard' : '/@login')
        return
      }

      if (!refreshedUser?.isAdmin) {
        // 需要管理员权限但用户不是管理员，跳转到仪表板首页
        next('/@dashboard')
        return
      }
    }

    next()
  } else if (to.path === '/@login' && authStore.isLogin) {
    // 已登录用户访问登录页，跳转到仪表板
    next('/@dashboard')
  } else {
    // 不需要认证的路由，直接通过
    next()
  }
})

export default router
