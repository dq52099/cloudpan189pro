<template>
  <div class="dashboard">
    <n-grid :cols="24" :x-gap="16" :y-gap="16">
      <!-- 个人信息展示区 -->
      <n-grid-item :span="12">
        <n-card title="个人信息" class="info-card">
          <n-descriptions
            :column="1"
            label-placement="left"
            label-style="width: 120px; font-weight: 500;"
          >
            <n-descriptions-item label="用户名">
              <n-text strong>{{ userInfo.username || '-' }}</n-text>
            </n-descriptions-item>
            <n-descriptions-item label="状态">
              <n-tag :type="getUserStatusType(userInfo.status)" size="small">
                {{ getUserStatusText(userInfo.status) }}
              </n-tag>
            </n-descriptions-item>
            <n-descriptions-item label="用户组">
              <n-text>{{ userInfo.groupName || '-' }}</n-text>
            </n-descriptions-item>
            <n-descriptions-item label="管理员权限">
              <n-tag :type="canUseAdminFeatures ? 'success' : 'default'" size="small">
                {{ canUseAdminFeatures ? '是' : '否' }}
              </n-tag>
            </n-descriptions-item>
          </n-descriptions>
        </n-card>
      </n-grid-item>

      <!-- 系统信息展示区 -->
      <n-grid-item :span="12">
        <n-card title="系统信息" class="info-card">
          <n-descriptions
            :column="1"
            label-placement="left"
            label-style="width: 120px; font-weight: 500;"
          >
            <n-descriptions-item label="站点名称">
              <n-text strong>{{ systemInfo.title || '-' }}</n-text>
            </n-descriptions-item>
            <n-descriptions-item label="站点URL">
              <n-text>{{ systemInfo.baseURL || '-' }}</n-text>
            </n-descriptions-item>
            <n-descriptions-item label="运行时间">
              <n-text>{{ systemInfo.runTimeHuman || '-' }}</n-text>
            </n-descriptions-item>
            <n-descriptions-item label="WebDAV认证">
              <n-tag :type="systemInfo.enableAuth ? 'success' : 'warning'" size="small">
                {{ systemInfo.enableAuth ? '需要认证' : '无需认证' }}
              </n-tag>
            </n-descriptions-item>
          </n-descriptions>
        </n-card>
      </n-grid-item>

      <!-- 资源统计展示区 -->
      <n-grid-item v-if="canUseAdminFeatures" :span="24">
        <n-card title="资源统计" class="resource-card">
          <div v-if="loadingSummary" class="summary-loading">资源统计加载中...</div>
          <n-grid v-else :cols="5" :x-gap="24" :y-gap="24">
            <n-grid-item>
              <div class="stat-item">
                <div class="stat-icon" style="background: #2080f0">
                  <n-icon size="24"><PeopleOutline /></n-icon>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ summaryData.users.total }}</div>
                  <div class="stat-label">用户总数</div>
                </div>
              </div>
            </n-grid-item>
            <n-grid-item>
              <div class="stat-item">
                <div class="stat-icon" style="background: #18a058">
                  <n-icon size="24"><FolderOutline /></n-icon>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ summaryData.mountPoints.total }}</div>
                  <div class="stat-label">挂载点</div>
                </div>
              </div>
            </n-grid-item>
            <n-grid-item>
              <div class="stat-item">
                <div class="stat-icon" style="background: #f0a20a">
                  <n-icon size="24"><KeyOutline /></n-icon>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ summaryData.cloudTokens.total }}</div>
                  <div class="stat-label">云盘令牌</div>
                </div>
              </div>
            </n-grid-item>
            <n-grid-item>
              <div class="stat-item">
                <div class="stat-icon" style="background: #9c27b0">
                  <n-icon size="24"><ShareSocialOutline /></n-icon>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ summaryData.subscribeShares }}</div>
                  <div class="stat-label">订阅分享</div>
                </div>
              </div>
            </n-grid-item>
            <n-grid-item>
              <div class="stat-item">
                <div class="stat-icon" style="background: #ff9800">
                  <n-icon size="24"><FolderOpenOutline /></n-icon>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ summaryData.virtualFiles.folders }}</div>
                  <div class="stat-label">文件夹数</div>
                </div>
              </div>
            </n-grid-item>
            <n-grid-item>
              <div class="stat-item">
                <div class="stat-icon" style="background: #00acc1">
                  <n-icon size="24"><DocumentOutline /></n-icon>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ summaryData.virtualFiles.files }}</div>
                  <div class="stat-label">文件数</div>
                </div>
              </div>
            </n-grid-item>
            <n-grid-item>
              <div class="stat-item">
                <div class="stat-icon" style="background: #7b1fa2">
                  <n-icon size="24"><PlayOutline /></n-icon>
                </div>
                <div class="stat-content">
                  <div class="stat-value">
                    {{ summaryData.media.enabled ? summaryData.media.strmFiles : 0 }}
                  </div>
                  <div class="stat-label">STRM文件</div>
                </div>
              </div>
            </n-grid-item>
            <n-grid-item>
              <div class="stat-item">
                <div class="stat-icon" style="background: #e42c1e">
                  <n-icon size="24"><ServerOutline /></n-icon>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ summaryData.autoIngest.plans }}</div>
                  <div class="stat-label">入库计划</div>
                </div>
              </div>
            </n-grid-item>
            <n-grid-item>
              <div class="stat-item">
                <div class="stat-icon" style="background: #00bcd4">
                  <n-icon size="24"><TimeOutline /></n-icon>
                </div>
                <div class="stat-content">
                  <div class="stat-value">{{ summaryData.tasks.running }}</div>
                  <div class="stat-label">运行中任务</div>
                </div>
              </div>
            </n-grid-item>
          </n-grid>
        </n-card>
      </n-grid-item>
    </n-grid>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import {
  NGrid,
  NGridItem,
  NCard,
  NDescriptions,
  NDescriptionsItem,
  NTag,
  NText,
  NIcon,
  useMessage,
} from 'naive-ui'
import {
  PeopleOutline,
  FolderOutline,
  FolderOpenOutline,
  DocumentOutline,
  KeyOutline,
  PlayOutline,
  ServerOutline,
  TimeOutline,
  ShareSocialOutline,
} from '@vicons/ionicons5'
import { useSystemStore, useUserStore } from '@/stores'
import { getResourceSummary, type ResourceSummary } from '@/api/resource'

const userStore = useUserStore()
const systemStore = useSystemStore()
const message = useMessage()

const userInfo = userStore.get()
const systemInfo = systemStore.get()
const canUseAdminFeatures = computed(() => userInfo.isAdmin)

const loadingSummary = ref(false)
let isDashboardMounted = false
let summaryRequestId = 0
const summaryData = ref<ResourceSummary>({
  users: { total: 0, active: 0, disabled: 0 },
  userGroups: 0,
  mountPoints: { total: 0, enabled: 0, autoRefresh: 0 },
  cloudTokens: { total: 0, active: 0 },
  virtualFiles: { folders: 0, files: 0 },
  subscribeShares: 0,
  media: { enabled: false, strmFiles: 0, mediaFiles: 0 },
  autoIngest: { plans: 0, logs24h: 0 },
  tasks: { pending: 0, running: 0, failed: 0, completed: 0 },
})

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

const isSafeNonNegativeInteger = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

const normalizeCountGroup = <T extends readonly string[]>(
  value: unknown,
  keys: T
): Record<T[number], number> | null => {
  if (!isRecord(value)) {
    return null
  }

  const result = {} as Record<T[number], number>
  for (const key of keys as readonly T[number][]) {
    const item = value[key]
    if (!isSafeNonNegativeInteger(item)) {
      return null
    }
    result[key] = item
  }

  return result
}

const normalizeResourceSummary = (value: unknown): ResourceSummary | null => {
  if (!isRecord(value)) {
    return null
  }

  const users = normalizeCountGroup(value.users, ['total', 'active', 'disabled'] as const)
  const mountPoints = normalizeCountGroup(value.mountPoints, [
    'total',
    'enabled',
    'autoRefresh',
  ] as const)
  const cloudTokens = normalizeCountGroup(value.cloudTokens, ['total', 'active'] as const)
  const virtualFiles = normalizeCountGroup(value.virtualFiles, ['folders', 'files'] as const)
  const autoIngest = normalizeCountGroup(value.autoIngest, ['plans', 'logs24h'] as const)
  const tasks = normalizeCountGroup(value.tasks, [
    'pending',
    'running',
    'failed',
    'completed',
  ] as const)
  const media = value.media

  if (
    !users ||
    !mountPoints ||
    !cloudTokens ||
    !virtualFiles ||
    !autoIngest ||
    !tasks ||
    !isRecord(media) ||
    typeof media.enabled !== 'boolean' ||
    !isSafeNonNegativeInteger(media.strmFiles) ||
    !isSafeNonNegativeInteger(media.mediaFiles) ||
    !isSafeNonNegativeInteger(value.userGroups) ||
    !isSafeNonNegativeInteger(value.subscribeShares)
  ) {
    return null
  }

  return {
    users,
    userGroups: value.userGroups,
    mountPoints,
    cloudTokens,
    virtualFiles,
    subscribeShares: value.subscribeShares,
    media: {
      enabled: media.enabled,
      strmFiles: media.strmFiles,
      mediaFiles: media.mediaFiles,
    },
    autoIngest,
    tasks,
  }
}

const loadSummary = async () => {
  if (!isDashboardMounted) {
    return
  }

  const requestId = ++summaryRequestId

  loadingSummary.value = true
  try {
    const res = await getResourceSummary()
    if (!isDashboardMounted || requestId !== summaryRequestId) {
      return
    }

    if (res.code === 200) {
      const summary = normalizeResourceSummary(res.data)
      if (!summary) {
        console.error('资源统计响应数据格式异常', res.data)
        message.error('资源统计响应数据格式异常')

        return
      }

      summaryData.value = summary
    } else {
      message.error(res.msg || '加载资源统计失败')
    }
  } catch (err) {
    if (!isDashboardMounted || requestId !== summaryRequestId) {
      return
    }

    console.error('加载资源统计失败', err)
    message.error('加载资源统计失败')
  } finally {
    if (isDashboardMounted && requestId === summaryRequestId) {
      loadingSummary.value = false
    }
  }
}

const getUserStatusType = (status: number) => {
  switch (status) {
    case 1:
      return 'success'
    case 2:
      return 'warning'
    case 0:
    default:
      return 'error'
  }
}

const getUserStatusText = (status: number) => {
  switch (status) {
    case 1:
      return '正常'
    case 2:
      return '受限'
    case 0:
    default:
      return '禁用'
  }
}

onMounted(() => {
  isDashboardMounted = true
  if (canUseAdminFeatures.value) {
    loadSummary()
  }
})

onUnmounted(() => {
  isDashboardMounted = false
  summaryRequestId++
  loadingSummary.value = false
})
</script>

<style scoped>
.dashboard {
  padding: 0;
  background: var(--n-color-target);
}

.info-card {
  height: 280px;
  background: var(--n-card-color);
  border: 1px solid var(--n-border-color);
  border-radius: 12px;
  box-shadow: 0 2px 8px rgb(0 0 0 / 6%);
}

.info-card :deep(.n-card__content) {
  height: calc(100% - 50px);
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.info-card :deep(.n-descriptions) {
  margin: 0;
}

.info-card :deep(.n-descriptions-item) {
  margin-bottom: 16px;
}

.info-card :deep(.n-descriptions-item:last-child) {
  margin-bottom: 0;
}

.info-card :deep(.n-descriptions-item__label) {
  color: var(--n-text-color-2);
}

.info-card :deep(.n-descriptions-item__content) {
  color: var(--n-text-color);
}

.resource-card {
  margin-top: 16px;
  background: var(--n-card-color);
  border: 1px solid var(--n-border-color);
  border-radius: 12px;
  box-shadow: 0 2px 8px rgb(0 0 0 / 6%);
}

.summary-loading {
  padding: 12px 4px;
  color: var(--n-text-color-3);
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background: var(--n-color-hover);
  border-radius: 8px;
}

.stat-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 12px;
  color: #fff;
}

.stat-content {
  flex: 1;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: var(--n-text-color);
}

.stat-label {
  font-size: 12px;
  color: var(--n-text-color-3);
}

/* 响应式设计 */
@media (width <= 1200px) {
  .info-card {
    height: auto;
    min-height: 250px;
  }
}

@media (width <= 768px) {
  .info-card {
    height: auto;
    min-height: 220px;
  }

  .info-card :deep(.n-descriptions-item) {
    margin-bottom: 12px;
  }
}
</style>
