<template>
  <div class="task-logs-page">
    <!-- 头部区域 -->
    <div class="header">
      <div class="header-left">
        <n-input
          v-model:value="state.searchKeyword"
          placeholder="请输入任务标题搜索"
          clearable
          style="width: 200px; margin-right: 12px"
          @keyup.enter="handleSearch"
        />
        <n-select
          v-model:value="state.statusFilter"
          placeholder="任务状态"
          clearable
          style="width: 120px; margin-right: 12px"
          :options="statusOptions"
        />
        <n-select
          v-model:value="state.typeFilter"
          placeholder="任务类型"
          clearable
          style="width: 120px; margin-right: 12px"
          :options="typeOptions"
        />
        <n-date-picker
          v-model:value="state.dateRange"
          type="datetimerange"
          clearable
          style="width: 300px; margin-right: 12px"
          format="yyyy-MM-dd HH:mm:ss"
          value-format="yyyy-MM-ddTHH:mm:ssXXX"
          placeholder="选择时间范围"
        />
        <n-button type="primary" @click="handleSearch" style="margin-right: 8px"> 搜索 </n-button>
        <n-button @click="handleReset"> 重置 </n-button>
        <n-button type="error" ghost :loading="state.clearing" @click="handleClearLogs" style="margin-left: 16px">
          <template #icon>
            <n-icon>
              <TrashOutline />
            </n-icon>
          </template>
          清空日志
        </n-button>
      </div>
      <div class="header-right">
        <n-button :loading="state.loading" @click="handleRefresh">
          <template #icon>
            <n-icon>
              <RefreshOutline />
            </n-icon>
          </template>
          刷新
        </n-button>
      </div>
    </div>

    <!-- 任务日志列表表格 -->
    <n-data-table
      :columns="columns"
      :data="state.tableData"
      :loading="state.loading"
      :pagination="paginationReactive"
      class="task-logs-table"
      :row-key="(row: Models.FileTaskLog) => row.id"
      :scroll-x="1000"
      remote
    />

    <!-- 任务详情弹窗 -->
    <n-modal
      v-model:show="state.showDetailModal"
      preset="card"
      title="任务详情"
      style="width: 800px"
    >
      <div v-if="state.currentTask" class="task-detail">
        <n-descriptions :column="2" label-placement="left" bordered>
          <n-descriptions-item label="任务ID">
            {{ state.currentTask.id }}
          </n-descriptions-item>
          <n-descriptions-item label="任务标题">
            {{ state.currentTask.title }}
          </n-descriptions-item>
          <n-descriptions-item label="任务类型">
            <n-tag :type="getTypeTagType(state.currentTask.type)" size="small">
              {{ getTypeText(state.currentTask.type) }}
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item label="任务状态">
            <n-tag :type="getStatusTagType(state.currentTask.status)" size="small">
              {{ getStatusText(state.currentTask.status) }}
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item label="开始时间">
            {{ formatDate(state.currentTask.beginAt) }}
          </n-descriptions-item>
          <n-descriptions-item label="结束时间">
            {{ state.currentTask.endAt ? formatDate(state.currentTask.endAt) : '未结束' }}
          </n-descriptions-item>
          <n-descriptions-item label="执行时长">
            {{ formatDuration(state.currentTask.duration) }}
          </n-descriptions-item>
          <n-descriptions-item label="进度">
            {{
              state.currentTask.total > 0
                ? `${state.currentTask.completed}/${state.currentTask.total}`
                : '无进度信息'
            }}
          </n-descriptions-item>
          <n-descriptions-item label="文件ID">
            {{ state.currentTask.fileId }}
          </n-descriptions-item>
          <n-descriptions-item label="用户ID">
            {{ state.currentTask.userId }}
          </n-descriptions-item>
          <n-descriptions-item label="创建时间" :span="2">
            {{ formatDate(state.currentTask.createdAt) }}
          </n-descriptions-item>
        </n-descriptions>

        <n-divider />

        <div class="task-description">
          <h4>任务描述</h4>
          <n-text>{{ state.currentTask.desc || '无描述' }}</n-text>
        </div>

        <div v-if="state.currentTask.result" class="task-result">
          <h4>执行结果</h4>
          <n-code :code="state.currentTask.result" language="json" />
        </div>

        <div v-if="state.currentTask.errorMsg" class="task-error">
          <h4>错误信息</h4>
          <n-alert type="error" :show-icon="false">
            {{ state.currentTask.errorMsg }}
          </n-alert>
        </div>

        <div
          v-if="state.currentTask.addition && Object.keys(state.currentTask.addition).length > 0"
          class="task-addition"
        >
          <h4>附加信息</h4>
          <n-code :code="JSON.stringify(state.currentTask.addition, null, 2)" language="json" />
        </div>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { reactive, onMounted, h } from 'vue'
import {
  NDataTable,
  NInput,
  NButton,
  NIcon,
  NSelect,
  NDatePicker,
  NTag,
  NModal,
  NDescriptions,
  NDescriptionsItem,
  NDivider,
  NText,
  NCode,
  NAlert,
  NProgress,
  useMessage,
  useDialog,
  type DataTableColumns,
  type PaginationProps,
} from 'naive-ui'
import {
  RefreshOutline,
  EyeOutline,
  CheckmarkCircleOutline,
  CloseCircleOutline,
  TimeOutline,
  PlayOutline,
  TrashOutline,
} from '@vicons/ionicons5'
import { getFileLogList, clearTaskLogs } from '@/api/taskstate'
import { formatDate } from '@/utils/format'
import {
  TASK_TYPE_OPTIONS,
  TASK_TYPE_TEXT_MAP,
  TASK_TYPE_TAG_MAP,
  TASK_STATUS_OPTIONS,
  TASK_STATUS_TEXT_MAP,
  TASK_STATUS_TAG_MAP,
} from '@/constants/taskTypes'

const state = reactive({
  tableData: [] as Models.FileTaskLog[],
  loading: false,
  clearing: false,
  searchKeyword: '',
  statusFilter: null as string | null,
  typeFilter: null as string | null,
  dateRange: null as [number, number] | null,
  showDetailModal: false,
  currentTask: null as Models.FileTaskLog | null,
})

// 消息提示
const message = useMessage()
const dialog = useDialog()

// 状态选项（使用常量定义）
const statusOptions = TASK_STATUS_OPTIONS

// 类型选项（使用常量定义）
const typeOptions = TASK_TYPE_OPTIONS

// 分页配置
const paginationReactive = reactive<PaginationProps>({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
  prefix: ({ itemCount }) => `共 ${itemCount} 条`,
  simple: false,
  disabled: false,
  onChange: (page: number) => {
    paginationReactive.page = page
    fetchTaskLogList()
  },
  onUpdatePageSize: (pageSize: number) => {
    paginationReactive.pageSize = pageSize
    paginationReactive.page = 1
    fetchTaskLogList()
  },
})

// 获取任务日志列表
const fetchTaskLogList = () => {
  state.loading = true

  const params: Record<string, unknown> = {
    currentPage: paginationReactive.page,
    pageSize: paginationReactive.pageSize,
  }

  // 添加搜索条件
  if (state.searchKeyword) {
    params.title = state.searchKeyword
  }
  if (state.statusFilter) {
    params.status = state.statusFilter
  }
  if (state.typeFilter) {
    params.type = state.typeFilter
  }
  if (state.dateRange && state.dateRange.length === 2) {
    params.beginAt = new Date(state.dateRange[0]).toISOString()
    params.endAt = new Date(state.dateRange[1]).toISOString()
  }

  getFileLogList(params)
    .then((response) => {
      if (response.data) {
        state.tableData = response.data.data || []
        paginationReactive.itemCount = response.data.total || 0
      }
    })
    .catch((error) => {
      console.error('获取任务日志失败:', error)
      message.error(error?.message || '获取任务日志失败')
    })
    .finally(() => {
      state.loading = false
    })
}

// 搜索
const handleSearch = () => {
  paginationReactive.page = 1
  fetchTaskLogList()
}

// 重置
const handleReset = () => {
  state.searchKeyword = ''
  state.statusFilter = null
  state.typeFilter = null
  state.dateRange = null
  paginationReactive.page = 1
  fetchTaskLogList()
}

// 刷新
const handleRefresh = () => {
  fetchTaskLogList()
}

// 清空日志
const handleClearLogs = () => {
  dialog.warning({
    title: '清空任务日志',
    content: '确定要清空所有任务日志吗？此操作不可撤销。',
    positiveText: '确认清空',
    negativeText: '取消',
    onPositiveClick: () => {
      state.clearing = true
      clearTaskLogs()
        .then((res) => {
          if (res.code === 200) {
            message.success('任务日志已清空')
            fetchTaskLogList()
          } else {
            message.error(res.msg || '清空失败')
          }
        })
        .catch((err) => {
          message.error(err?.message || '清空失败')
        })
        .finally(() => {
          state.clearing = false
        })
    },
  })
}

// 查看详情
const handleViewDetail = (task: Models.FileTaskLog) => {
  state.currentTask = task
  state.showDetailModal = true
}

// 获取状态标签类型
const getStatusTagType = (status: string) => {
  return TASK_STATUS_TAG_MAP[status as keyof typeof TASK_STATUS_TAG_MAP] || 'default'
}

// 获取状态文本
const getStatusText = (status: string) => {
  return TASK_STATUS_TEXT_MAP[status as keyof typeof TASK_STATUS_TEXT_MAP] || '未知'
}

// 获取类型标签类型
const getTypeTagType = (type: string) => {
  return TASK_TYPE_TAG_MAP[type as keyof typeof TASK_TYPE_TAG_MAP] || 'default'
}

// 获取类型文本
const getTypeText = (type: string) => {
  return TASK_TYPE_TEXT_MAP[type as keyof typeof TASK_TYPE_TEXT_MAP] || type
}

// 格式化时长
const formatDuration = (duration: number) => {
  if (!duration || duration <= 0) return '0秒'

  if (duration < 1000) {
    return `${duration}毫秒`
  }

  const seconds = Math.floor(duration / 1000)
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

// 获取状态图标
const getStatusIcon = (status: string) => {
  switch (status) {
    case 'completed':
      return CheckmarkCircleOutline
    case 'running':
      return PlayOutline
    case 'pending':
      return TimeOutline
    case 'failed':
      return CloseCircleOutline
    default:
      return TimeOutline
  }
}

// 表格列定义
const columns: DataTableColumns<Models.FileTaskLog> = [
  {
    title: '序号',
    key: 'index',
    width: 80,
    align: 'center',
    render(_: Models.FileTaskLog, index) {
      const page = paginationReactive.page ?? 1
      const pageSize = paginationReactive.pageSize ?? 10
      return (page - 1) * pageSize + index + 1
    },
  },
  {
    title: '任务标题',
    key: 'title',
    width: 200,
    align: 'left',
    ellipsis: {
      tooltip: true,
    },
  },
  {
    title: '类型',
    key: 'type',
    width: 100,
    align: 'center',
    render(row) {
      return h(
        NTag,
        {
          type: getTypeTagType(row.type),
          size: 'small',
        },
        { default: () => getTypeText(row.type) }
      )
    },
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    align: 'center',
    render(row) {
      return h(
        NTag,
        {
          type: getStatusTagType(row.status),
          size: 'small',
        },
        {
          icon: () => h(NIcon, { size: 12 }, { default: () => h(getStatusIcon(row.status)) }),
          default: () => getStatusText(row.status),
        }
      )
    },
  },
  {
    title: '进度',
    key: 'progress',
    width: 120,
    align: 'center',
    render(row) {
      if (row.total <= 0) return '无进度信息'
      const percentage = Math.round((row.completed / row.total) * 100)
      return h(NProgress, {
        type: 'line',
        percentage,
        showIndicator: true,
        status: row.status === 'failed' ? 'error' : row.status === 'completed' ? 'success' : 'info',
        height: 8,
      })
    },
  },
  {
    title: '开始时间',
    key: 'beginAt',
    width: 160,
    align: 'center',
    render(row) {
      return formatDate(row.beginAt)
    },
  },
  {
    title: '执行时长',
    key: 'duration',
    width: 100,
    align: 'center',
    render(row) {
      return formatDuration(row.duration)
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 80,
    align: 'center',
    render(row) {
      return h(
        NButton,
        {
          size: 'tiny',
          type: 'primary',
          secondary: true,
          onClick: () => handleViewDetail(row),
        },
        {
          icon: () => h(NIcon, { size: 12 }, { default: () => h(EyeOutline) }),
          default: () => '详情',
        }
      )
    },
  },
]

// 初始化：仅在被挂载时请求，配合 Tabs v-if 从而避免多余请求
onMounted(() => {
  fetchTaskLogList()
})
</script>

<style scoped>
.task-logs-page {
  padding: 20px;
  background: var(--n-card-color);
  border-radius: 12px;
}

.header {
  margin-bottom: 20px;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 16px;
  padding: 16px;
  background: var(--n-color-hover);
  border-radius: 12px;
}

.header-left {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.task-logs-table {
  background: transparent;
  border-radius: 12px;
}

.task-logs-table :deep(.n-data-table-th) {
  text-align: center;
  font-weight: 600;
  background: var(--n-color-hover);
}

.task-logs-table :deep(.n-data-table-td) {
  text-align: center;
  padding: 12px 8px;
}

.task-detail {
  max-height: 600px;
  overflow-y: auto;
  padding: 8px;
}

.task-detail h4 {
  margin: 16px 0 8px;
  color: var(--n-text-color);
  font-weight: 600;
  font-size: 15px;
}

.task-description,
.task-result,
.task-error,
.task-addition {
  margin-top: 16px;
}

.task-result :deep(.n-code),
.task-addition :deep(.n-code) {
  max-height: 200px;
  overflow-y: auto;
  border-radius: 8px;
}

/* 响应式设计 */
@media (width <= 1200px) {
  .header {
    flex-direction: column;
    align-items: stretch;
  }

  .header-left {
    justify-content: flex-start;
  }

  .header-right {
    justify-content: flex-end;
  }
}

@media (width <= 768px) {
  .header-left {
    flex-direction: column;
    align-items: stretch;
  }

  .header-left > * {
    width: 100%;
    margin-right: 0 !important;
    margin-bottom: 8px;
  }

  .header-left > *:last-child {
    margin-bottom: 0;
  }
}
</style>
