<template>
  <div class="engine-logs-page">
    <!-- 头部区域 -->
    <div class="header">
      <div class="header-left">
        <h2></h2>
      </div>
      <div class="header-right">
        <n-button @click="handleRefresh">
          <template #icon>
            <n-icon>
              <RefreshOutline />
            </n-icon>
          </template>
          刷新
        </n-button>
      </div>
    </div>

    <!-- 引擎状态卡片 -->
    <div class="status-cards">
      <n-card title="引擎状态" class="status-card">
        <div class="engine-status">
          <n-tag :type="state.engineData?.isRunning ? 'success' : 'error'" size="large">
            <template #icon>
              <n-icon>
                <component
                  :is="state.engineData?.isRunning ? PlayCircleOutline : StopCircleOutline"
                />
              </n-icon>
            </template>
            {{ state.engineData?.isRunning ? '运行中' : '已停止' }}
          </n-tag>
        </div>
      </n-card>

      <n-card title="任务统计" class="status-card">
        <div v-if="state.engineData?.stats" class="stats-grid">
          <div class="stat-item">
            <div class="stat-value">{{ state.engineData.stats.totalTasks }}</div>
            <div class="stat-label">总任务数</div>
          </div>
          <div class="stat-item">
            <div class="stat-value pending">{{ state.engineData.stats.pendingTasks }}</div>
            <div class="stat-label">待处理</div>
          </div>
          <div class="stat-item">
            <div class="stat-value running">{{ state.engineData.stats.runningTasks }}</div>
            <div class="stat-label">运行中</div>
          </div>
          <div class="stat-item">
            <div class="stat-value completed">{{ state.engineData.stats.completedTasks }}</div>
            <div class="stat-label">已完成</div>
          </div>
          <div class="stat-item">
            <div class="stat-value failed">{{ state.engineData.stats.failedTasks }}</div>
            <div class="stat-label">失败</div>
          </div>
        </div>
      </n-card>
    </div>

    <!-- 任务列表 -->
    <div class="task-lists">
      <!-- 正在运行的任务 -->
      <n-card title="正在运行的任务" class="task-list-card">
        <template #header-extra>
          <n-tag type="info" size="small">
            {{ state.engineData?.runningTasks?.length || 0 }} 个任务
          </n-tag>
        </template>
        <div v-if="state.loading" class="loading-container">
          <n-spin size="medium" />
        </div>
        <div v-else-if="!state.engineData?.runningTasks?.length" class="empty-container">
          <n-empty description="暂无正在运行的任务" />
        </div>
        <div v-else class="task-list">
          <div
            v-for="task in state.engineData.runningTasks"
            :key="task.id"
            class="task-item"
            @click="handleViewTaskDetail(task)"
          >
            <div class="task-header">
              <div class="task-id">{{ task.id }}</div>
              <n-tag type="info" size="small">{{ task.status }}</n-tag>
            </div>
            <div class="task-info">
              <div class="task-topic">主题: {{ task.topic }}</div>
              <div class="task-worker">Worker: {{ task.workerId || '未分配' }}</div>
            </div>
            <div class="task-time">
              <div>接收时间: {{ formatDateTime(task.receiveAt) }}</div>
              <div v-if="task.startAt">开始时间: {{ formatDateTime(task.startAt) }}</div>
            </div>
          </div>
        </div>
      </n-card>

      <!-- 待处理的任务 -->
      <n-card title="待处理的任务" class="task-list-card">
        <template #header-extra>
          <n-tag type="warning" size="small">
            {{ state.engineData?.pendingTasks?.length || 0 }} 个任务
          </n-tag>
        </template>
        <div v-if="state.loading" class="loading-container">
          <n-spin size="medium" />
        </div>
        <div v-else-if="!state.engineData?.pendingTasks?.length" class="empty-container">
          <n-empty description="暂无待处理的任务" />
        </div>
        <div v-else class="task-list">
          <div
            v-for="task in state.engineData.pendingTasks"
            :key="task.id"
            class="task-item"
            @click="handleViewTaskDetail(task)"
          >
            <div class="task-header">
              <div class="task-id">{{ task.id }}</div>
              <n-tag type="warning" size="small">{{ task.status }}</n-tag>
            </div>
            <div class="task-info">
              <div class="task-topic">主题: {{ task.topic }}</div>
              <div class="task-worker">Worker: {{ task.workerId || '未分配' }}</div>
            </div>
            <div class="task-time">
              <div>接收时间: {{ formatDateTime(task.receiveAt) }}</div>
            </div>
          </div>
        </div>
      </n-card>
    </div>

    <!-- 任务详情弹窗 -->
    <n-modal
      v-model:show="state.showTaskDetailModal"
      preset="card"
      title="任务详情"
      style="width: 800px"
    >
      <div v-if="state.currentTask" class="task-detail">
        <n-descriptions :column="2" label-placement="left" bordered>
          <n-descriptions-item label="任务ID">
            {{ state.currentTask.id }}
          </n-descriptions-item>
          <n-descriptions-item label="状态">
            <n-tag :type="getTaskStatusTagType(state.currentTask.status)" size="small">
              {{ state.currentTask.status }}
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item label="主题">
            {{ state.currentTask.topic }}
          </n-descriptions-item>
          <n-descriptions-item label="Worker ID">
            {{ state.currentTask.workerId || '未分配' }}
          </n-descriptions-item>
          <n-descriptions-item label="接收时间">
            {{ formatDateTime(state.currentTask.receiveAt) }}
          </n-descriptions-item>
          <n-descriptions-item label="开始时间">
            {{ state.currentTask.startAt ? formatDateTime(state.currentTask.startAt) : '未开始' }}
          </n-descriptions-item>
          <n-descriptions-item label="结束时间">
            {{ state.currentTask.endAt ? formatDateTime(state.currentTask.endAt) : '未结束' }}
          </n-descriptions-item>
          <n-descriptions-item label="载荷大小">
            {{ state.currentTask.payload?.length || 0 }} 字节
          </n-descriptions-item>
        </n-descriptions>

        <n-divider />

        <div
          v-if="state.currentTask.results && state.currentTask.results.length > 0"
          class="processor-results"
        >
          <h4>处理器结果</h4>
          <div class="results-list">
            <n-card
              v-for="(result, index) in state.currentTask.results"
              :key="index"
              size="small"
              class="result-card"
            >
              <n-descriptions :column="2" label-placement="left" size="small">
                <n-descriptions-item label="处理器ID">
                  {{ result.processorId }}
                </n-descriptions-item>
                <n-descriptions-item label="状态">
                  <n-tag :type="getProcessorStatusTagType(result.status)" size="small">
                    {{ result.status }}
                  </n-tag>
                </n-descriptions-item>
                <n-descriptions-item label="开始时间">
                  {{ result.startTime ? formatDateTime(result.startTime) : '未开始' }}
                </n-descriptions-item>
                <n-descriptions-item label="结束时间">
                  {{ result.endTime ? formatDateTime(result.endTime) : '未结束' }}
                </n-descriptions-item>
                <n-descriptions-item label="执行时长" :span="2">
                  {{ formatDuration(result.duration) }}
                </n-descriptions-item>
              </n-descriptions>
              <div v-if="result.error" class="processor-error">
                <n-alert type="error" :show-icon="false">
                  {{ result.error }}
                </n-alert>
              </div>
            </n-card>
          </div>
        </div>

        <div
          v-if="state.currentTask.payload && state.currentTask.payload.length > 0"
          class="task-payload"
        >
          <h4>载荷数据</h4>
          <n-code :code="formatPayload(state.currentTask.payload)" language="json" />
        </div>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { reactive, onMounted, onUnmounted } from 'vue'
import {
  NCard,
  NButton,
  NIcon,
  NTag,
  NSpin,
  NEmpty,
  NModal,
  NDescriptions,
  NDescriptionsItem,
  NDivider,
  NCode,
  NAlert,
  useMessage,
} from 'naive-ui'
import { RefreshOutline, PlayCircleOutline, StopCircleOutline } from '@vicons/ionicons5'
import { getTaskEngineList } from '@/api/taskstate'
import { formatDateTime } from '@/utils/time'
import { getErrorMessage } from '@/utils/api'
import type { TaskEngineListResponse } from '@/api/taskstate'

type NormalizedTaskEngineListResponse = {
  isRunning: boolean
  stats: Models.TaskStats
  pendingTasks: Models.TaskInfo[]
  runningTasks: Models.TaskInfo[]
}

// 数据状态
const state = reactive({
  engineData: null as NormalizedTaskEngineListResponse | null,
  loading: false,
  showTaskDetailModal: false,
  currentTask: null as Models.TaskInfo | null,
})

// 消息提示
const message = useMessage()

const AUTO_REFRESH_INTERVAL = 5000
let refreshTimer: ReturnType<typeof setInterval> | null = null
let isComponentMounted = false
let engineStatusRequestId = 0
let hasEngineStatsContractWarning = false

const defaultTaskStats = (): Models.TaskStats => ({
  totalTasks: 0,
  pendingTasks: 0,
  runningTasks: 0,
  completedTasks: 0,
  failedTasks: 0,
})

const isFiniteNumber = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isFinite(value)
}

const isCompleteTaskStats = (value: unknown): value is Models.TaskStats => {
  if (!value || typeof value !== 'object') {
    return false
  }

  const stats = value as Partial<Models.TaskStats>

  return (
    isFiniteNumber(stats.totalTasks) &&
    isFiniteNumber(stats.pendingTasks) &&
    isFiniteNumber(stats.runningTasks) &&
    isFiniteNumber(stats.completedTasks) &&
    isFiniteNumber(stats.failedTasks)
  )
}

const normalizeTaskEngineData = (
  data: TaskEngineListResponse
): NormalizedTaskEngineListResponse => {
  const stats = isCompleteTaskStats(data.stats) ? data.stats : defaultTaskStats()

  return {
    isRunning: data.isRunning === true,
    stats,
    pendingTasks: Array.isArray(data.pendingTasks) ? data.pendingTasks : [],
    runningTasks: Array.isArray(data.runningTasks) ? data.runningTasks : [],
  }
}

const warnInvalidEngineStatsOnce = (data: TaskEngineListResponse) => {
  if (isCompleteTaskStats(data.stats)) {
    hasEngineStatsContractWarning = false

    return
  }

  if (!hasEngineStatsContractWarning) {
    message.warning('任务引擎统计响应缺失/异常，已用 0 显示')
    hasEngineStatsContractWarning = true
  }
}

// 获取任务引擎状态
const fetchEngineStatus = () => {
  if (!isComponentMounted) {
    return
  }

  const requestId = ++engineStatusRequestId

  state.loading = true

  getTaskEngineList()
    .then((response) => {
      if (!isComponentMounted || requestId !== engineStatusRequestId) {
        return
      }

      if (response.code === 200 && response.data) {
        warnInvalidEngineStatsOnce(response.data)
        state.engineData = normalizeTaskEngineData(response.data)

        return
      }

      message.error(response.msg || '获取执行日志失败')
    })
    .catch((error) => {
      if (!isComponentMounted || requestId !== engineStatusRequestId) {
        return
      }

      console.error('获取执行日志失败:', error)
      message.error(getErrorMessage(error, '获取执行日志失败'))
    })
    .finally(() => {
      if (isComponentMounted && requestId === engineStatusRequestId) {
        state.loading = false
      }
    })
}

// 刷新
const handleRefresh = () => {
  fetchEngineStatus()
}

// 查看任务详情
const handleViewTaskDetail = (task: Models.TaskInfo) => {
  state.currentTask = task
  state.showTaskDetailModal = true
}

// 获取任务状态标签类型
const getTaskStatusTagType = (status: string) => {
  switch (status.toLowerCase()) {
    case 'pending':
      return 'warning'
    case 'running':
      return 'info'
    case 'completed':
      return 'success'
    case 'failed':
      return 'error'
    default:
      return 'default'
  }
}

// 获取处理器状态标签类型
const getProcessorStatusTagType = (status: string) => {
  switch (status.toLowerCase()) {
    case 'success':
    case 'completed':
      return 'success'
    case 'running':
      return 'info'
    case 'failed':
    case 'error':
      return 'error'
    default:
      return 'default'
  }
}

// 格式化时长
const formatDuration = (duration: number) => {
  if (!duration || duration <= 0) return '0秒'

  const seconds = Math.floor(duration / 1000000000) // Go的time.Duration是纳秒
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)

  if (hours > 0) {
    return `${hours}小时${minutes % 60}分${seconds % 60}秒`
  } else if (minutes > 0) {
    return `${minutes}分${seconds % 60}秒`
  } else {
    return `${seconds}秒`
  }
}

// 格式化载荷数据
const formatPayload = (payload: number[]) => {
  try {
    // 尝试将字节数组转换为字符串
    const str = String.fromCharCode(...payload)
    // 尝试解析为JSON
    const parsed = JSON.parse(str)
    return JSON.stringify(parsed, null, 2)
  } catch {
    // 如果解析失败，显示原始字节数组
    return JSON.stringify(payload, null, 2)
  }
}

// 启动自动刷新
const startAutoRefresh = () => {
  if (!isComponentMounted || refreshTimer) {
    return
  }

  refreshTimer = setInterval(() => {
    fetchEngineStatus()
  }, AUTO_REFRESH_INTERVAL)
}

// 停止自动刷新
const stopAutoRefresh = () => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

// 初始化：仅在组件挂载时加载，满足“切换到该页才请求”
onMounted(() => {
  isComponentMounted = true
  fetchEngineStatus()
  startAutoRefresh()
})

// 清理
onUnmounted(() => {
  isComponentMounted = false
  engineStatusRequestId += 1
  stopAutoRefresh()
})
</script>

<style scoped>
.engine-logs-page {
  padding: 0;
}

.header {
  margin-bottom: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-left h2 {
  margin: 0;
  color: var(--n-text-color);
  font-weight: 600;
}

.header-right {
  display: flex;
  align-items: center;
}

.status-cards {
  display: grid;
  grid-template-columns: 1fr 2fr;
  gap: 20px;
  margin-bottom: 20px;
}

.status-card {
  background: var(--n-card-color);
  border-radius: 6px;
}

.engine-status {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 20px 0;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 16px;
  text-align: center;
}

.stat-item {
  padding: 12px;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  margin-bottom: 4px;
}

.stat-value.pending {
  color: #f0a020;
}

.stat-value.running {
  color: #2080f0;
}

.stat-value.completed {
  color: #18a058;
}

.stat-value.failed {
  color: #d03050;
}

.stat-label {
  font-size: 12px;
  color: var(--n-text-color-2);
}

.task-lists {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

.task-list-card {
  background: var(--n-card-color);
  border-radius: 6px;
}

.loading-container,
.empty-container {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 40px 0;
}

.task-list {
  max-height: 500px;
  overflow-y: auto;
}

.task-item {
  padding: 16px;
  border-bottom: 1px solid var(--n-divider-color);
  cursor: pointer;
  transition: background-color 0.2s;
}

.task-item:hover {
  background-color: var(--n-color-hover);
}

.task-item:last-child {
  border-bottom: none;
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.task-id {
  font-family: monospace;
  font-size: 12px;
  color: var(--n-text-color-2);
  background: var(--n-color-hover);
  padding: 2px 6px;
  border-radius: 4px;
}

.task-info {
  margin-bottom: 8px;
}

.task-topic,
.task-worker {
  font-size: 13px;
  color: var(--n-text-color-1);
  margin-bottom: 2px;
}

.task-time {
  font-size: 12px;
  color: var(--n-text-color-2);
}

.task-time div {
  margin-bottom: 2px;
}

.task-detail {
  max-height: 600px;
  overflow-y: auto;
}

.task-detail h4 {
  margin: 16px 0 8px;
  color: var(--n-text-color);
  font-weight: 600;
}

.processor-results,
.task-payload {
  margin-top: 16px;
}

.results-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.result-card {
  border: 1px solid var(--n-divider-color);
}

.processor-error {
  margin-top: 8px;
}

.task-payload :deep(.n-code) {
  max-height: 200px;
  overflow-y: auto;
}

/* 响应式设计 */
@media (width <= 1200px) {
  .status-cards {
    grid-template-columns: 1fr;
  }

  .stats-grid {
    grid-template-columns: repeat(3, 1fr);
  }

  .task-lists {
    grid-template-columns: 1fr;
  }
}

@media (width <= 768px) {
  .header {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (width <= 480px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }
}
</style>
