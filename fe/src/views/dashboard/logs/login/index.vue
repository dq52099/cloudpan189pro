<template>
  <div class="login-logs-page">
    <!-- 头部筛选 -->
    <div class="header">
      <div class="header-left">
        <n-input
          v-model:value="state.username"
          placeholder="用户名"
          clearable
          style="width: 140px"
        />
        <n-input v-model:value="state.addr" placeholder="地址/IP" clearable style="width: 140px" />
        <n-select
          v-model:value="state.event"
          :options="eventOptions"
          clearable
          placeholder="事件"
          style="width: 120px"
        />
        <n-select
          v-model:value="state.status"
          :options="statusOptions"
          clearable
          placeholder="状态"
          style="width: 120px"
        />
        <n-date-picker
          v-model:value="state.dateRange"
          type="datetimerange"
          clearable
          style="width: 280px"
          format="yyyy-MM-dd HH:mm:ss"
          value-format="yyyy-MM-ddTHH:mm:ssXXX"
          placeholder="选择时间范围"
        />
        <n-button type="primary" @click="handleSearch">搜索</n-button>
        <n-button @click="handleReset">重置</n-button>
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
        <n-dropdown trigger="click" :options="clearLogOptions" @select="handleClearLogs">
          <n-button type="error" ghost :loading="state.clearing">
            <template #icon>
              <n-icon>
                <TrashOutline />
              </n-icon>
            </template>
            清理日志
          </n-button>
        </n-dropdown>
      </div>
    </div>

    <!-- 表格 -->
    <n-data-table
      :columns="columns"
      :data="state.tableData"
      :loading="state.loading"
      :pagination="paginationReactive"
      :row-key="(row: Models.LoginLog) => row.id"
      class="login-logs-table"
      :scroll-x="1100"
      remote
    />
  </div>
</template>

<script setup lang="ts">
import { reactive, h, onMounted, onUnmounted } from 'vue'
import {
  NDataTable,
  NInput,
  NButton,
  NIcon,
  NSelect,
  NTag,
  NText,
  useMessage,
  useDialog,
  NDropdown,
  type DataTableColumns,
  type DropdownOption,
  type PaginationProps,
  NDatePicker,
} from 'naive-ui'
import {
  RefreshOutline,
  GlobeOutline,
  KeyOutline,
  PersonCircleOutline,
  ShieldOutline,
  WarningOutline,
  CheckmarkCircleOutline,
  CloseCircleOutline,
  RefreshCircleOutline,
  TrashOutline,
} from '@vicons/ionicons5'
import { getLoginLogList, clearLoginLogs, type LoginLogListQuery } from '@/api/loginlog'
import { formatDate } from '@/utils/format'
import { getListItems, getListTotal } from '@/utils/pagination'
import { normalizeLoginLogs } from '@/utils/responseGuards'
import { getErrorMessage } from '@/utils/api'
import {
  LOGIN_EVENT_OPTIONS,
  LOGIN_EVENT_TEXT_MAP,
  LOGIN_STATUS_OPTIONS,
  LOGIN_STATUS_TEXT_MAP,
  LOGIN_STATUS_TAG_MAP,
} from '@/constants/loginLog'

const message = useMessage()
const dialog = useDialog()

let isComponentMounted = false
let loginLogListRequestId = 0
let clearLoginLogRequestId = 0

const state = reactive({
  loading: false,
  clearing: false,
  tableData: [] as Models.LoginLog[],
  username: '',
  addr: '',
  event: null as Enums.LoginEvent | null,
  status: null as Enums.LoginStatus | null,
  dateRange: null as [number, number] | null,
})

// 选项
const eventOptions = LOGIN_EVENT_OPTIONS
const statusOptions = LOGIN_STATUS_OPTIONS
const clearLogOptions: DropdownOption[] = [
  { label: '清空全部', key: 'all' },
  { label: '保留最近 7 天', key: '7d' },
  { label: '保留最近 30 天', key: '30d' },
  { label: '保留最近 90 天', key: '90d' },
  { label: '保留最近 180 天', key: '180d' },
  { label: '保留最近 365 天', key: '365d' },
]

const isValidClearCount = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isFinite(value)
}

// 分页
const paginationReactive = reactive<PaginationProps>({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
  prefix: ({ itemCount }) => `共 ${itemCount} 条`,
  onChange: (page) => {
    paginationReactive.page = page
    fetchList()
  },
  onUpdatePageSize: (pageSize) => {
    paginationReactive.pageSize = pageSize
    paginationReactive.page = 1
    fetchList()
  },
})

const getStatusTagType = (s: string) => {
  return LOGIN_STATUS_TAG_MAP[s as keyof typeof LOGIN_STATUS_TAG_MAP] || 'default'
}
const getStatusIcon = (s: string) => {
  switch (s) {
    case 'success':
      return CheckmarkCircleOutline
    case 'failed':
      return CloseCircleOutline
    case 'blocked':
      return WarningOutline
    default:
      return ShieldOutline
  }
}
const getMethodIcon = (m: string) => {
  switch (m) {
    case 'web':
      return GlobeOutline
    case 'api':
      return KeyOutline
    case 'app':
      return PersonCircleOutline
    case 'cli':
      return RefreshCircleOutline
    default:
      return GlobeOutline
  }
}

// 列
const columns: DataTableColumns<Models.LoginLog> = [
  {
    title: '序号',
    key: 'index',
    width: 80,
    align: 'center',
    render(_, index) {
      const start = ((paginationReactive.page || 1) - 1) * (paginationReactive.pageSize || 10)
      return start + index + 1
    },
  },
  { title: '用户名', key: 'username', minWidth: 100, align: 'center', ellipsis: { tooltip: true } },
  {
    title: '来源',
    key: 'method',
    width: 80,
    align: 'center',
    render(row) {
      return h(
        NTag,
        { type: 'info', size: 'small' },
        {
          icon: () => h(NIcon, { size: 12 }, { default: () => h(getMethodIcon(row.method)) }),
          default: () => row.method,
        }
      )
    },
  },
  {
    title: '事件',
    key: 'event',
    width: 100,
    align: 'center',
    render(row) {
      const text = LOGIN_EVENT_TEXT_MAP[row.event as keyof typeof LOGIN_EVENT_TEXT_MAP] || row.event
      return h(NText, null, { default: () => text })
    },
  },
  {
    title: '状态',
    key: 'status',
    width: 80,
    align: 'center',
    render(row) {
      return h(
        NTag,
        { type: getStatusTagType(row.status), size: 'small' },
        {
          icon: () => h(NIcon, { size: 12 }, { default: () => h(getStatusIcon(row.status)) }),
          default: () =>
            LOGIN_STATUS_TEXT_MAP[row.status as keyof typeof LOGIN_STATUS_TEXT_MAP] || row.status,
        }
      )
    },
  },
  { title: '地址/IP', key: 'addr', minWidth: 120, align: 'center', ellipsis: { tooltip: true } },
  { title: 'UA', key: 'userAgent', minWidth: 160, align: 'center', ellipsis: { tooltip: true } },
  {
    title: '原因',
    key: 'reason',
    minWidth: 200,
    align: 'center',
    ellipsis: { tooltip: true },
  },
  {
    title: '时间',
    key: 'createdAt',
    width: 180,
    align: 'center',
    render(row) {
      return formatDate(row.createdAt)
    },
  },
]

// 拉取列表
const fetchList = () => {
  if (!isComponentMounted) {
    return
  }

  const requestId = ++loginLogListRequestId

  state.loading = true
  const params: LoginLogListQuery = {
    currentPage: paginationReactive.page ?? 1,
    pageSize: paginationReactive.pageSize ?? 10,
  }
  if (state.username) params.username = state.username
  if (state.addr) params.addr = state.addr
  if (state.event) params.event = state.event
  if (state.status) params.status = state.status
  if (state.dateRange && state.dateRange.length === 2) {
    params.beginAt = new Date(state.dateRange[0]).toISOString()
    params.endAt = new Date(state.dateRange[1]).toISOString()
  }

  getLoginLogList(params)
    .then((res) => {
      if (!isComponentMounted || requestId !== loginLogListRequestId) {
        return
      }

      if (res.code === 200 && res.data) {
        const rawItems = getListItems<Models.LoginLog>(res.data)
        const items = normalizeLoginLogs(rawItems)
        const total = getListTotal(res.data)

        if (!items) {
          message.error('获取登录日志失败：响应数据格式异常')

          return
        }

        state.tableData = items
        if (total === null) {
          message.warning('登录日志响应缺少有效总数，已保留原分页统计')
        } else {
          paginationReactive.itemCount = total
        }

        return
      }

      message.error(res.msg || '获取登录日志失败')
    })
    .catch((err) => {
      if (!isComponentMounted || requestId !== loginLogListRequestId) {
        return
      }

      console.error(err)
      message.error(getErrorMessage(err, '获取登录日志失败'))
    })
    .finally(() => {
      if (isComponentMounted && requestId === loginLogListRequestId) {
        state.loading = false
      }
    })
}

const handleSearch = () => {
  paginationReactive.page = 1
  fetchList()
}
const handleReset = () => {
  state.username = ''
  state.addr = ''
  state.event = null
  state.status = null
  state.dateRange = null
  paginationReactive.page = 1
  fetchList()
}
const handleRefresh = () => fetchList()

const handleClearLogs = (key: string | number) => {
  if (state.clearing) {
    return
  }

  const duration = key === 'all' ? undefined : String(key)
  const selectedLabel =
    clearLogOptions.find((option) => option.key === key)?.label?.toString() || '清理日志'

  dialog.warning({
    title: selectedLabel,
    content: duration
      ? `确定要删除 ${selectedLabel.replace('保留', '')}以外的登录日志吗？此操作不可撤销。`
      : '确定要清空所有登录日志吗？此操作不可撤销。',
    positiveText: '确认清理',
    negativeText: '取消',
    onPositiveClick: () => {
      if (state.clearing) {
        return
      }

      const requestId = ++clearLoginLogRequestId

      state.clearing = true
      clearLoginLogs(duration ? { duration } : undefined)
        .then((res) => {
          if (!isComponentMounted || requestId !== clearLoginLogRequestId) {
            return
          }

          if (res.code === 200) {
            if (isValidClearCount(res.data)) {
              message.success(`登录日志已清理，删除 ${res.data} 条`)
            } else {
              message.warning('清理完成但响应统计缺失/异常')
            }

            paginationReactive.page = 1
            fetchList()
          } else {
            message.error(res.msg || '清空失败')
          }
        })
        .catch((err) => {
          if (!isComponentMounted || requestId !== clearLoginLogRequestId) {
            return
          }

          message.error(err instanceof Error ? err.message : '清空失败')
        })
        .finally(() => {
          if (isComponentMounted && requestId === clearLoginLogRequestId) {
            state.clearing = false
          }
        })
    },
  })
}

// 仅在组件挂载时发起请求，配合 Tabs 的 v-if 保证按需加载
onMounted(() => {
  isComponentMounted = true
  fetchList()
})

onUnmounted(() => {
  isComponentMounted = false
  loginLogListRequestId += 1
  clearLoginLogRequestId += 1
})
</script>

<style scoped>
.login-logs-page {
  padding: 0;
}

.header {
  margin-bottom: 16px;
  display: flex;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.header-left {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
}

.header-right {
  display: flex;
  gap: 8px;
  align-items: center;
}

.login-logs-table {
  background: var(--n-card-color);
  border-radius: 6px;
}

.login-logs-table :deep(.n-data-table-th) {
  text-align: center;
  font-weight: 600;
}

.login-logs-table :deep(.n-data-table-td) {
  text-align: center;
}
</style>
