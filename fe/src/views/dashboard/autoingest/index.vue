<template>
  <div class="autoingest-page">
    <!-- 顶部 Tabs -->
    <n-tabs type="line" v-model:value="activeTab">
      <n-tab name="plans">计划管理</n-tab>
      <n-tab name="logs">运行日志</n-tab>
    </n-tabs>

    <!-- 头部区域（Plans） -->
    <div v-if="activeTab === 'plans'" class="header">
      <div class="header-left">
        <n-input
          v-model:value="planQuery.name"
          placeholder="按名称搜索计划"
          clearable
          style="width: 240px; margin-right: 12px"
          @keyup.enter="handlePlanSearch"
        >
          <template #prefix>
            <n-icon :size="16" :depth="3">
              <SearchOutline />
            </n-icon>
          </template>
        </n-input>
        <n-button type="primary" @click="handlePlanSearch" style="margin-right: 8px">
          <template #icon>
            <n-icon>
              <SearchOutline />
            </n-icon>
          </template>
          搜索
        </n-button>
        <n-button @click="handlePlanReset">
          <template #icon>
            <n-icon>
              <RefreshOutline />
            </n-icon>
          </template>
          重置
        </n-button>
      </div>
      <div class="header-right">
        <n-button type="primary" @click="showCreateModal = true">
          <template #icon>
            <n-icon>
              <AddOutline />
            </n-icon>
          </template>
          新建入库计划
        </n-button>
      </div>
    </div>

    <!-- 批量操作栏（Plans） -->
    <div v-if="activeTab === 'plans' && selectedPlanIds.length > 0" class="batch-actions">
      <n-text depth="2">已选择 {{ selectedPlanIds.length }} 项</n-text>
      <n-button
        size="small"
        type="info"
        @click="handleBatchRetry"
        :disabled="batchActionLoading || !hasEnabledPlans"
        :loading="batchActionLoading"
      >
        <template #icon>
          <n-icon><RefreshOutline /></n-icon>
        </template>
        批量重试
      </n-button>
      <n-button
        size="small"
        type="primary"
        @click="handleBatchRefresh"
        :disabled="batchActionLoading || !hasEnabledPlans"
        :loading="batchActionLoading"
      >
        <template #icon>
          <n-icon><RefreshOutline /></n-icon>
        </template>
        批量扫描
      </n-button>
      <n-button
        size="small"
        type="success"
        @click="handleBatchEnable"
        :disabled="batchActionLoading || !hasDisabledPlans"
        :loading="batchActionLoading"
      >
        <template #icon>
          <n-icon><CheckmarkCircleOutline /></n-icon>
        </template>
        批量启用
      </n-button>
      <n-button
        size="small"
        type="warning"
        @click="handleBatchDisable"
        :disabled="batchActionLoading || !hasEnabledPlans"
        :loading="batchActionLoading"
      >
        <template #icon>
          <n-icon><CloseCircleOutline /></n-icon>
        </template>
        批量停用
      </n-button>
      <n-button
        size="small"
        type="error"
        @click="handleBatchDelete"
        :disabled="batchActionLoading"
        :loading="batchActionLoading"
      >
        <template #icon>
          <n-icon><TrashOutline /></n-icon>
        </template>
        批量删除
      </n-button>
      <n-button size="small" @click="clearPlanSelection">取消选择</n-button>
    </div>

    <!-- 头部区域（Logs） -->
    <div v-else-if="activeTab === 'logs'" class="header">
      <div class="header-left">
        <n-select
          v-model:value="logQuery.planId"
          :options="planOptions"
          placeholder="按计划筛选"
          clearable
          style="width: 220px; margin-right: 8px"
        />
        <n-select
          v-model:value="logQuery.level"
          :options="logLevelOptions"
          placeholder="日志级别"
          clearable
          style="width: 160px; margin-right: 8px"
        />
        <n-button type="primary" @click="handleLogFilter" style="margin-right: 8px">
          <template #icon>
            <n-icon>
              <SearchOutline />
            </n-icon>
          </template>
          筛选
        </n-button>
        <n-button @click="handleLogReset">
          <template #icon>
            <n-icon>
              <RefreshOutline />
            </n-icon>
          </template>
          重置
        </n-button>
        <n-dropdown trigger="click" :options="clearLogOptions" @select="handleClearLogsSelect">
          <n-button type="error" style="margin-left: 8px">清理日志</n-button>
        </n-dropdown>
      </div>
      <div class="header-right">
        <n-text depth="3">最近刷新：{{ refreshTime.format('YYYY-MM-DD HH:mm:ss') }}</n-text>
      </div>
    </div>

    <!-- 计划管理表格 -->
    <n-data-table
      v-if="activeTab === 'plans'"
      :columns="planColumns"
      :data="planTable"
      :loading="planLoading"
      :pagination="planPagination"
      :row-key="(row: Models.AutoIngestPlan) => row.id"
      v-model:checked-row-keys="selectedPlanIds"
      @update:checked-row-keys="handlePlanSelectionChange"
      class="autoingest-table"
      remote
    />

    <!-- 运行日志表格 -->
    <n-data-table
      v-else
      :columns="logColumns"
      :data="logTable"
      :loading="logLoading"
      :pagination="logPagination"
      class="autoingest-table"
      remote
    />

    <!-- 新建计划弹窗（组件化） -->
    <CreatePlanModal
      v-model:show="showCreateModal"
      :cloud-token-options="cloudTokenOptions"
      @created="handlePlanCreated"
    />

    <!-- 修改计划弹窗（组件化） -->
    <EditPlanModal
      v-model:show="showEditModal"
      :plan="editingPlan"
      :cloud-token-options="cloudTokenOptions"
      @saved="fetchPlanList"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, h, onMounted, onUnmounted, computed } from 'vue'
import {
  NDataTable,
  NButton,
  NIcon,
  NInput,
  NText,
  NSelect,
  NSpace,
  NPopconfirm,
  NDropdown,
  NTabs,
  NTab,
  NTag,
  useMessage,
  useDialog,
  type DataTableColumns,
  type DataTableRowKey,
  type DropdownOption,
  type PaginationProps,
} from 'naive-ui'
import {
  SearchOutline,
  RefreshOutline,
  AddOutline,
  TrashOutline,
  CheckmarkCircleOutline,
  CloseCircleOutline,
  CreateOutline,
} from '@vicons/ionicons5'
import {
  getAutoIngestPlanList,
  getAutoIngestLogList,
  deleteAutoIngestPlan,
  enableAutoIngestPlan,
  disableAutoIngestPlan,
  refreshAutoIngestPlan,
  retryFailedAutoIngest,
  clearAutoIngestLogs,
  retryAutoIngestPlan,
  batchRetryPlan,
  batchRefreshPlan,
  batchDeletePlan,
  batchEnablePlan,
  batchDisablePlan,
  type PlanLogResult,
} from '@/api/autoingest'
import { getCloudTokenList } from '@/api/cloudtoken'
import dayjs from 'dayjs'
import { AUTO_INGEST_SOURCE_TYPE_OPTIONS } from '@/constants/autoIngest'
import CreatePlanModal from '@/components/autoingest/CreatePlanModal.vue'
import EditPlanModal from '@/components/autoingest/EditPlanModal.vue'
import { type ApiResponse, type BatchOperationResponse } from '@/utils/api'

const message = useMessage()
const dialog = useDialog()
let isPageAlive = true

const isActiveRequest = (requestId: number, latestRequestId: number) =>
  isPageAlive && requestId === latestRequestId

// Tabs
const activeTab = ref<'plans' | 'logs'>('plans')

// Refresh Time
const refreshTime = ref(dayjs())

// Cloud Token options
let cloudTokenRequestId = 0
const cloudTokenOptions = ref<{ label: string; value: number }[]>([])
const loadCloudTokens = () => {
  const requestId = ++cloudTokenRequestId

  getCloudTokenList({ noPaginate: true })
    .then((res: ApiResponse<Models.PaginationResponse<Models.CloudToken>>) => {
      if (!isActiveRequest(requestId, cloudTokenRequestId)) {
        return
      }

      if (res.code === 200 && res.data) {
        cloudTokenOptions.value = res.data.data.map((t) => ({
          label: t.name || `令牌${t.id}`,
          value: t.id,
        }))

        return
      }

      message.error(res.msg || '获取令牌列表失败')
    })
    .catch((err: unknown) => {
      if (!isActiveRequest(requestId, cloudTokenRequestId)) {
        return
      }

      console.error('获取令牌列表失败:', err)
      message.error('获取令牌列表失败')
    })
}

// -------- Plans --------
let planListRequestId = 0
const planLoading = ref(false)
const planTable = ref<Models.AutoIngestPlan[]>([])
const planQuery = reactive({
  name: '' as string | undefined,
})

// 批量选择
const selectedPlanIds = ref<number[]>([])
const selectedPlanRows = ref<Models.AutoIngestPlan[]>([])
const batchActionLoading = ref(false)
const hasEnabledPlans = computed(() => {
  return selectedPlanRows.value.some((p) => p.enabled)
})
const hasDisabledPlans = computed(() => {
  return selectedPlanRows.value.some((p) => !p.enabled)
})
type PlanRowAction = 'enable' | 'disable' | 'refresh' | 'retry' | 'delete' | 'retryFailed'
const planRowActions: PlanRowAction[] = ['enable', 'disable', 'refresh', 'retry', 'delete']
const planActionPending = ref<Set<string>>(new Set())
const planActionKey = (action: PlanRowAction, planId?: number) => `${action}:${planId ?? 'all'}`
const isPlanActionPending = (action: PlanRowAction, planId?: number) => {
  return planActionPending.value.has(planActionKey(action, planId))
}
const setPlanActionPending = (
  action: PlanRowAction,
  planId: number | undefined,
  pending: boolean
) => {
  const next = new Set(planActionPending.value)
  const key = planActionKey(action, planId)

  if (pending) {
    next.add(key)
  } else {
    next.delete(key)
  }

  planActionPending.value = next
}
const isPlanRowPending = (planId: number) => {
  return planRowActions.some((action) => isPlanActionPending(action, planId))
}
const clearPlanSelection = () => {
  selectedPlanIds.value = []
  selectedPlanRows.value = []
}
const syncSelectedPlanRows = () => {
  const visibleIds = new Set(planTable.value.map((p) => p.id))
  const selectedIds = selectedPlanIds.value.filter((id) => visibleIds.has(id))

  selectedPlanIds.value = selectedIds
  selectedPlanRows.value = planTable.value.filter((p) => selectedIds.includes(p.id))
}

// 处理复选框选择变化
const handlePlanSelectionChange = (keys: DataTableRowKey[]) => {
  const selectedIds = keys.filter((key): key is number => typeof key === 'number')

  selectedPlanIds.value = selectedIds
  selectedPlanRows.value = planTable.value.filter((p) => selectedIds.includes(p.id))
}

// 批量操作处理函数
const handleBatchRetry = () => {
  if (batchActionLoading.value) {
    return
  }

  const ids = [...selectedPlanIds.value]
  if (ids.length === 0) {
    return
  }

  batchActionLoading.value = true
  batchRetryPlan({ ids })
    .then((res: ApiResponse<BatchOperationResponse>) => {
      if (res.code === 200) {
        message.success(
          `批量重试完成：成功 ${res.data?.success || 0}，失败 ${res.data?.failed || 0}`
        )
        clearPlanSelection()
        fetchPlanList()
      } else {
        message.error(res.msg || '批量重试失败')
      }
    })
    .catch((err: unknown) => {
      console.error('批量重试失败', err)
      message.error('批量重试失败')
    })
    .finally(() => {
      batchActionLoading.value = false
    })
}

const handleBatchRefresh = () => {
  if (batchActionLoading.value) {
    return
  }

  const ids = [...selectedPlanIds.value]
  if (ids.length === 0) {
    return
  }

  batchActionLoading.value = true
  batchRefreshPlan({ ids })
    .then((res: ApiResponse<BatchOperationResponse>) => {
      if (res.code === 200) {
        message.success(
          `批量扫描完成：成功 ${res.data?.success || 0}，失败 ${res.data?.failed || 0}`
        )
        clearPlanSelection()
      } else {
        message.error(res.msg || '批量扫描失败')
      }
    })
    .catch((err: unknown) => {
      console.error('批量扫描失败', err)
      message.error('批量扫描失败')
    })
    .finally(() => {
      batchActionLoading.value = false
    })
}

const handleBatchEnable = () => {
  if (batchActionLoading.value) {
    return
  }

  const ids = [...selectedPlanIds.value]
  if (ids.length === 0) {
    return
  }

  batchActionLoading.value = true
  batchEnablePlan({ ids })
    .then((res: ApiResponse<BatchOperationResponse>) => {
      if (res.code === 200) {
        message.success(
          `批量启用完成：成功 ${res.data?.success || 0}，失败 ${res.data?.failed || 0}`
        )
        clearPlanSelection()
        fetchPlanList()
      } else {
        message.error(res.msg || '批量启用失败')
      }
    })
    .catch((err: unknown) => {
      console.error('批量启用失败', err)
      message.error('批量启用失败')
    })
    .finally(() => {
      batchActionLoading.value = false
    })
}

const handleBatchDisable = () => {
  if (batchActionLoading.value) {
    return
  }

  const ids = [...selectedPlanIds.value]
  if (ids.length === 0) {
    return
  }

  batchActionLoading.value = true
  batchDisablePlan({ ids })
    .then((res: ApiResponse<BatchOperationResponse>) => {
      if (res.code === 200) {
        message.success(
          `批量停用完成：成功 ${res.data?.success || 0}，失败 ${res.data?.failed || 0}`
        )
        clearPlanSelection()
        fetchPlanList()
      } else {
        message.error(res.msg || '批量停用失败')
      }
    })
    .catch((err: unknown) => {
      console.error('批量停用失败', err)
      message.error('批量停用失败')
    })
    .finally(() => {
      batchActionLoading.value = false
    })
}

const handleBatchDelete = () => {
  if (batchActionLoading.value || selectedPlanIds.value.length === 0) {
    return
  }

  dialog.warning({
    title: '批量删除计划',
    content: `确定要删除选中的 ${selectedPlanIds.value.length} 个计划吗？此操作不可撤销。`,
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: () => {
      if (batchActionLoading.value) {
        return
      }

      const ids = [...selectedPlanIds.value]
      if (ids.length === 0) {
        return
      }

      batchActionLoading.value = true
      batchDeletePlan({ ids })
        .then((res: ApiResponse<BatchOperationResponse>) => {
          if (res.code === 200) {
            message.success(
              `批量删除完成：成功 ${res.data?.success || 0}，失败 ${res.data?.failed || 0}`
            )
            clearPlanSelection()
            fetchPlanList()
          } else {
            message.error(res.msg || '批量删除失败')
          }
        })
        .catch((err: unknown) => {
          console.error('批量删除失败', err)
          message.error('批量删除失败')
        })
        .finally(() => {
          batchActionLoading.value = false
        })
    },
  })
}

// 分页（对齐用户组管理）
const planPagination = reactive<PaginationProps>({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
  prefix: ({ itemCount }) => `共 ${itemCount} 条`,
  onChange: (page: number) => {
    planPagination.page = page
    clearPlanSelection()
    fetchPlanList()
  },
  onUpdatePageSize: (ps: number) => {
    planPagination.pageSize = ps
    planPagination.page = 1
    clearPlanSelection()
    fetchPlanList()
  },
})

const handlePlanSearch = () => {
  planPagination.page = 1
  clearPlanSelection()
  fetchPlanList()
}

const handlePlanReset = () => {
  planQuery.name = ''
  planPagination.page = 1
  clearPlanSelection()
  fetchPlanList()
}

const planColumns: DataTableColumns<Models.AutoIngestPlan> = [
  {
    type: 'selection',
  },
  {
    title: '计划名称',
    key: 'name',
    width: 120,
    align: 'center',
    ellipsis: { tooltip: true },
  },
  {
    title: '来源类型',
    key: 'sourceType',
    width: 80,
    align: 'center',
    render: (row) => {
      const opt = AUTO_INGEST_SOURCE_TYPE_OPTIONS.find((o) => o.value === row.sourceType)
      return h(
        NTag,
        { type: 'primary', size: 'small', bordered: true },
        { default: () => opt?.label || '-' }
      )
    },
  },
  {
    title: '父目录',
    key: 'parentPath',
    width: 120,
    align: 'center',
    ellipsis: { tooltip: true },
    render: (row) =>
      h(
        'div',
        {
          style: 'max-width:240px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;',
          title: row.parentPath || '-',
        },
        row.parentPath || '-'
      ),
  },
  {
    title: '间隔(分钟)',
    key: 'autoIngestInterval',
    width: 90,
    align: 'center',
  },
  {
    title: '启用',
    key: 'enabled',
    width: 90,
    align: 'center',
    render: (row) => {
      const type = row.enabled ? 'success' : 'default'
      const label = row.enabled ? '启用' : '停用'
      return h(NTag, { type, size: 'small', bordered: true }, { default: () => label })
    },
  },
  {
    title: '统计',
    key: 'stats',
    width: 100,
    align: 'center',
    render: (row) =>
      h(
        'div',
        {
          class: 'stats-cell',
          style: 'display:flex;justify-content:center;gap:6px;align-items:center;',
        },
        [
          h(
            NTag,
            { type: 'success', size: 'small', bordered: true },
            { default: () => String(row.addCount || 0) }
          ),
          h('span', null, '/'),
          h(
            NTag,
            { type: 'error', size: 'small', bordered: true },
            { default: () => String(row.failedCount || 0) }
          ),
        ]
      ),
  },
  {
    title: '时间',
    key: 'time',
    width: 220,
    align: 'center',
    render: (row) =>
      h('div', { class: 'time-cell' }, [
        h('div', { class: 'time-line' }, [
          h('span', { class: 'time-label' }, '创建: '),
          h('span', { class: 'time-value' }, formatDT(row.createdAt)),
        ]),
        h('div', { class: 'time-line' }, [
          h('span', { class: 'time-label' }, '更新: '),
          h('span', { class: 'time-value' }, formatDT(row.updatedAt)),
        ]),
      ]),
  },
  {
    title: '操作',
    key: 'actions',
    minWidth: 220,
    align: 'center',
    render: (row) => {
      const rowPending = isPlanRowPending(row.id)
      const refreshPending = isPlanActionPending('refresh', row.id)
      const retryPending = isPlanActionPending('retry', row.id)
      const enablePending = isPlanActionPending('enable', row.id)
      const disablePending = isPlanActionPending('disable', row.id)
      const deletePending = isPlanActionPending('delete', row.id)

      return h(
        NSpace,
        { size: 'small', justify: 'center', align: 'center' },
        {
          default: () => [
            row.enabled
              ? [
                  h(
                    NButton,
                    {
                      size: 'tiny',
                      type: 'info',
                      secondary: true,
                      loading: refreshPending,
                      disabled: rowPending,
                      onClick: () => onRefresh(row),
                    },
                    {
                      icon: () => h(NIcon, { size: 12 }, { default: () => h(RefreshOutline) }),
                      default: () => '扫描',
                    }
                  ),
                  // 重试按钮（重新获取历史记录）
                  h(
                    NButton,
                    {
                      size: 'tiny',
                      type: 'success',
                      secondary: true,
                      loading: retryPending,
                      disabled: rowPending,
                      onClick: () => onRetry(row),
                    },
                    {
                      icon: () => h(NIcon, { size: 12 }, { default: () => h(RefreshOutline) }),
                      default: () => '重试',
                    }
                  ),
                  h(
                    NButton,
                    {
                      size: 'tiny',
                      type: 'primary',
                      secondary: true,
                      disabled: rowPending,
                      onClick: () => onEdit(row),
                    },
                    {
                      icon: () => h(NIcon, { size: 12 }, { default: () => h(CreateOutline) }),
                      default: () => '修改',
                    }
                  ),
                  h(
                    NButton,
                    {
                      size: 'tiny',
                      type: 'warning',
                      secondary: true,
                      loading: disablePending,
                      disabled: rowPending,
                      onClick: () => onDisable(row),
                    },
                    {
                      icon: () => h(NIcon, { size: 12 }, { default: () => h(CloseCircleOutline) }),
                      default: () => '停用',
                    }
                  ),
                ]
              : h(
                  NButton,
                  {
                    size: 'tiny',
                    type: 'success',
                    secondary: true,
                    loading: enablePending,
                    disabled: rowPending,
                    onClick: () => onEnable(row),
                  },
                  {
                    icon: () =>
                      h(NIcon, { size: 12 }, { default: () => h(CheckmarkCircleOutline) }),
                    default: () => '启用',
                  }
                ),
            h(
              NPopconfirm,
              {
                onPositiveClick: () => onDelete(row),
                negativeText: '取消',
                positiveText: '确认删除',
              },
              {
                trigger: () =>
                  h(
                    NButton,
                    {
                      size: 'tiny',
                      type: 'error',
                      secondary: true,
                      loading: deletePending,
                      disabled: rowPending,
                    },
                    {
                      icon: () => h(NIcon, { size: 12 }, { default: () => h(TrashOutline) }),
                      default: () => '删除',
                    }
                  ),
                default: () => `确定要删除计划 "${row.name || '#' + row.id}" 吗？此操作不可撤销。`,
              }
            ),
          ],
        }
      )
    },
  },
]

const fetchPlanList = () => {
  const requestId = ++planListRequestId

  planLoading.value = true
  getAutoIngestPlanList({
    currentPage: planPagination.page || 1,
    pageSize: planPagination.pageSize || 10,
    name: planQuery.name || undefined,
  })
    .then((res: ApiResponse<Models.PaginationResponse<Models.AutoIngestPlan>>) => {
      if (!isActiveRequest(requestId, planListRequestId)) {
        return
      }

      if (res.code === 200 && res.data) {
        planTable.value = res.data.data
        planPagination.itemCount = res.data.total
        syncSelectedPlanRows()
        // 更新 planOptions 供日志筛选使用
        planOptions.value = [
          { label: '全部计划', value: undefined },
          ...res.data.data.map((p: Models.AutoIngestPlan) => ({
            label: `${p.name || '#' + p.id}`,
            value: p.id,
          })),
        ]

        return
      }

      message.error(res.msg || '获取计划列表失败')
    })
    .catch((err: unknown) => {
      if (!isActiveRequest(requestId, planListRequestId)) {
        return
      }

      console.error('获取计划列表失败:', err)
      message.error('获取计划列表失败')
    })
    .finally(() => {
      if (!isActiveRequest(requestId, planListRequestId)) {
        return
      }

      planLoading.value = false
      refreshTime.value = dayjs()
    })
}

const onEnable = (row: Models.AutoIngestPlan) => {
  if (isPlanRowPending(row.id)) {
    return
  }

  setPlanActionPending('enable', row.id, true)
  enableAutoIngestPlan({ id: row.id })
    .then((res: ApiResponse) => {
      if (!isPageAlive) {
        return
      }

      if (res.code === 200) {
        message.success('已启用')
        fetchPlanList()
      } else {
        message.error(res.msg || '启用失败')
      }
    })
    .catch((err: unknown) => {
      if (!isPageAlive) {
        return
      }

      console.error('启用失败', err)
      message.error('启用失败')
    })
    .finally(() => {
      if (isPageAlive) {
        setPlanActionPending('enable', row.id, false)
      }
    })
}

const onDisable = (row: Models.AutoIngestPlan) => {
  if (isPlanRowPending(row.id)) {
    return
  }

  setPlanActionPending('disable', row.id, true)
  disableAutoIngestPlan({ id: row.id })
    .then((res: ApiResponse) => {
      if (!isPageAlive) {
        return
      }

      if (res.code === 200) {
        message.success('已停用')
        fetchPlanList()
      } else {
        message.error(res.msg || '停用失败')
      }
    })
    .catch((err: unknown) => {
      if (!isPageAlive) {
        return
      }

      console.error('停用失败', err)
      message.error('停用失败')
    })
    .finally(() => {
      if (isPageAlive) {
        setPlanActionPending('disable', row.id, false)
      }
    })
}

const onRefresh = (row: Models.AutoIngestPlan) => {
  if (isPlanRowPending(row.id)) {
    return
  }

  setPlanActionPending('refresh', row.id, true)
  refreshAutoIngestPlan({ planId: row.id })
    .then((res: ApiResponse) => {
      if (!isPageAlive) {
        return
      }

      if (res.code !== 200) {
        message.error(res.msg || '扫描下发失败')

        return
      }
      message.success('已下发扫描任务')
    })
    .catch((err: unknown) => {
      if (!isPageAlive) {
        return
      }

      console.error('扫描下发失败', err)
      message.error('扫描下发失败')
    })
    .finally(() => {
      if (isPageAlive) {
        setPlanActionPending('refresh', row.id, false)
      }
    })
}

const onRetry = (row: Models.AutoIngestPlan) => {
  if (isPlanRowPending(row.id)) {
    return
  }

  setPlanActionPending('retry', row.id, true)
  retryAutoIngestPlan({ id: row.id })
    .then((res: ApiResponse) => {
      if (!isPageAlive) {
        return
      }

      if (res.code === 200) {
        message.success('已下发重试任务，将重新获取所有历史记录')
        fetchPlanList()
      } else {
        message.error(res.msg || '重试失败')
      }
    })
    .catch((err: unknown) => {
      if (!isPageAlive) {
        return
      }

      console.error('重试失败', err)
      message.error('重试失败')
    })
    .finally(() => {
      if (isPageAlive) {
        setPlanActionPending('retry', row.id, false)
      }
    })
}

const onDelete = (row: Models.AutoIngestPlan) => {
  if (isPlanRowPending(row.id)) {
    return
  }

  setPlanActionPending('delete', row.id, true)
  deleteAutoIngestPlan({ id: row.id })
    .then((res: ApiResponse) => {
      if (!isPageAlive) {
        return
      }

      if (res.code !== 200) {
        message.error(res.msg || '删除失败')

        return
      }
      message.success('删除成功')
      fetchPlanList()
    })
    .catch((err: unknown) => {
      if (!isPageAlive) {
        return
      }

      console.error('删除失败', err)
      message.error('删除失败')
    })
    .finally(() => {
      if (isPageAlive) {
        setPlanActionPending('delete', row.id, false)
      }
    })
}

/** 由 CreatePlanModal 创建成功后刷新列表 */
const handlePlanCreated = () => {
  fetchPlanList()
}

const showCreateModal = ref(false)

// -------- 修改计划（弹窗：抽离为组件） --------
const showEditModal = ref(false)
const editingPlan = ref<Models.AutoIngestPlan | null>(null)
const onEdit = (row: Models.AutoIngestPlan) => {
  editingPlan.value = row
  showEditModal.value = true
}

// -------- Logs --------
let logListRequestId = 0
const logLoading = ref(false)
const logTable = ref<PlanLogResult[]>([])
const logQuery = reactive<{
  planId?: number
  level?: 'info' | 'warn' | 'error'
}>({
  planId: undefined,
  level: undefined,
})
const planOptions = ref<{ label: string; value: number | undefined }[]>([
  { label: '全部计划', value: undefined },
])
const logLevelOptions = [
  { label: '全部级别', value: undefined },
  { label: 'info', value: 'info' },
  { label: 'warn', value: 'warn' },
  { label: 'error', value: 'error' },
]

// 分页（Logs 同步风格）
const logPagination = reactive<PaginationProps>({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
  prefix: ({ itemCount }) => `共 ${itemCount} 条`,
  onChange: (page: number) => {
    logPagination.page = page
    fetchLogList()
  },
  onUpdatePageSize: (ps: number) => {
    logPagination.pageSize = ps
    logPagination.page = 1
    fetchLogList()
  },
})

const handleLogFilter = () => {
  logPagination.page = 1
  fetchLogList()
}
const handleLogReset = () => {
  logQuery.planId = undefined
  logQuery.level = undefined
  logPagination.page = 1
  fetchLogList()
}

const clearLogOptions: DropdownOption[] = [
  { label: '清空全部', key: 'all' },
  { label: '保留 7 天', key: '7d' },
  { label: '保留 30 天', key: '30d' },
  { label: '保留 90 天', key: '90d' },
]

const getClearLogText = (duration?: string) => {
  if (!duration) {
    return {
      title: '清空日志',
      content: '确定要清空所有运行日志吗？此操作不可撤销。',
      positiveText: '确认清空',
      successPrefix: '已清空',
    }
  }

  const days = duration.replace('d', '')

  return {
    title: '清理日志',
    content: `确定要清理 ${days} 天前的运行日志吗？将保留最近 ${days} 天日志。`,
    positiveText: '确认清理',
    successPrefix: '已清理',
  }
}

// 清理日志
const handleClearLogsSelect = (key: string | number) => {
  const duration = key === 'all' ? undefined : String(key)
  const text = getClearLogText(duration)

  dialog.warning({
    title: text.title,
    content: text.content,
    positiveText: text.positiveText,
    negativeText: '取消',
    onPositiveClick: () => {
      clearAutoIngestLogs(duration ? { duration } : undefined)
        .then((res) => {
          if (res.code === 200) {
            message.success(`${text.successPrefix} ${res.data} 条日志`)
            logPagination.page = 1
            fetchLogList()
          } else {
            message.error(res.msg || '清理失败')
          }
        })
        .catch((err: unknown) => {
          console.error('清理失败', err)
          message.error('清理失败')
        })
    },
  })
}

const logColumns: DataTableColumns<Models.AutoIngestLog> = [
  { title: 'ID', key: 'id', width: 90, align: 'center' },
  { title: '计划名称', key: 'planName', width: 100, align: 'center', ellipsis: { tooltip: true } },
  {
    title: '级别',
    key: 'level',
    width: 100,
    align: 'center',
    render: (row) => {
      const type = row.level === 'error' ? 'error' : row.level === 'warn' ? 'warning' : 'success'
      return h(NTag, { type, size: 'small', bordered: true }, { default: () => row.level })
    },
  },
  {
    title: '内容',
    key: 'content',
    minWidth: 360,
    align: 'center',
    render: (row) => h('div', { class: 'log-content' }, row.content),
  },
  {
    title: '时间',
    key: 'createdAt',
    width: 180,
    align: 'center',
    render: (row) => formatDT(row.createdAt),
  },
  {
    title: '操作',
    key: 'actions',
    width: 100,
    align: 'center',
    render: (row) => {
      if (row.level === 'error') {
        const retryFailedPending = isPlanActionPending('retryFailed', row.planId)

        return h(
          NButton,
          {
            size: 'small',
            type: 'warning',
            loading: retryFailedPending,
            disabled: retryFailedPending,
            onClick: () => onRetryFailed(row.planId),
          },
          { default: () => '重试' }
        )
      }
      return null
    },
  },
]

// 重试失败任务
const onRetryFailed = (planId?: number) => {
  if (isPlanActionPending('retryFailed', planId)) {
    return
  }

  setPlanActionPending('retryFailed', planId, true)
  retryFailedAutoIngest({ planId })
    .then((res) => {
      if (!isPageAlive) {
        return
      }

      if (res.code === 200) {
        message.success('已下发重试任务')
        fetchLogList()
      } else {
        message.error(res.msg || '重试失败')
      }
    })
    .catch((err: unknown) => {
      if (!isPageAlive) {
        return
      }

      console.error('重试失败', err)
      message.error('重试失败')
    })
    .finally(() => {
      if (isPageAlive) {
        setPlanActionPending('retryFailed', planId, false)
      }
    })
}

const fetchLogList = () => {
  const requestId = ++logListRequestId

  logLoading.value = true
  getAutoIngestLogList({
    currentPage: logPagination.page || 1,
    pageSize: logPagination.pageSize || 10,
    planId: logQuery.planId || undefined,
    level: logQuery.level || undefined,
  })
    .then((res: ApiResponse<Models.PaginationResponse<PlanLogResult>>) => {
      if (!isActiveRequest(requestId, logListRequestId)) {
        return
      }

      if (res.code === 200 && res.data) {
        logTable.value = res.data.data
        logPagination.itemCount = res.data.total

        return
      }

      message.error(res.msg || '获取日志失败')
    })
    .catch((err: unknown) => {
      if (!isActiveRequest(requestId, logListRequestId)) {
        return
      }

      console.error('获取日志失败:', err)
      message.error('获取日志失败')
    })
    .finally(() => {
      if (!isActiveRequest(requestId, logListRequestId)) {
        return
      }

      logLoading.value = false
      refreshTime.value = dayjs()
    })
}

// Utils
const formatDT = (v?: string) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-')

// Init
onMounted(() => {
  loadCloudTokens()
  fetchPlanList()
  fetchLogList()
})

onUnmounted(() => {
  isPageAlive = false
  cloudTokenRequestId++
  planListRequestId++
  logListRequestId++
})
</script>

<style scoped>
.autoingest-page {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* 头部样式参考用户组管理 */
.header {
  margin: 8px 0 4px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

/* 批量操作栏 */
.batch-actions {
  margin: 8px 0;
  padding: 8px 12px;
  background: var(--n-color-hover);
  border-radius: 6px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-left {
  display: flex;
  align-items: center;
}

.autoingest-table {
  background: var(--n-card-color);
  border-radius: 6px;
}

.autoingest-table :deep(.n-data-table-th) {
  text-align: center;
  font-weight: 600;
}

.autoingest-table :deep(.n-data-table-td) {
  text-align: center;
}

.cell-title {
  font-weight: 600;
}

.log-content {
  white-space: pre-wrap;
  word-break: break-all;
}

/* 时间列样式（合并为一列） */
.time-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.time-line {
  display: flex;
  gap: 6px;
  align-items: center;
  font-size: 12px;
}

.time-label {
  color: var(--n-text-color-2);
}

.time-value {
  color: var(--n-text-color);
}
</style>
