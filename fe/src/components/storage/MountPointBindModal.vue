<template>
  <div class="mount-bind-content">
    <!-- 批量操作区域 -->
    <div class="batch-actions">
      <n-card size="small" title="批量设置">
        <div class="batch-grid">
          <div class="batch-item">
            <n-text class="batch-label">路径前缀</n-text>
            <n-input
              v-model:value="storageSetting.pathPrefix"
              placeholder="必须以 / 开头"
              style="flex-grow: 1"
              :disabled="state.submitLoading"
            />
            <n-button
              type="primary"
              size="small"
              :disabled="state.submitLoading || !hasValidPathPrefix"
              @click="handleBatchApplyPathPrefix"
            >
              应用
            </n-button>
          </div>

          <div class="batch-item" v-if="!allTokenSwitchDisabled">
            <n-text class="batch-label">云盘令牌</n-text>
            <n-select
              :value="storageSetting.selectedToken"
              :options="cloudTokenOptions"
              placeholder="选择令牌"
              clearable
              style="flex-grow: 1"
              :disabled="state.submitLoading"
              @update:value="handleSelectedTokenUpdate"
            />
            <n-button
              type="primary"
              size="small"
              :disabled="state.submitLoading"
              @click="handleBatchApplyToken"
            >
              应用
            </n-button>
          </div>

          <div class="batch-item">
            <n-text class="batch-label">自动刷新</n-text>
            <n-switch
              v-model:value="storageSetting.enableAutoRefresh"
              :disabled="state.submitLoading"
            />
            <n-button
              v-if="storageSetting.enableAutoRefresh"
              type="primary"
              size="small"
              :disabled="state.submitLoading"
              @click="handleEditAutoRefreshConfig"
            >
              编辑
            </n-button>
          </div>
        </div>
        <template #footer>
          <n-text depth="3">
            提示：批量设置将应用到下方所有可编辑的行。路径前缀会与识别出的名称组合成完整挂载路径。
          </n-text>
        </template>
      </n-card>
    </div>

    <div class="table-container">
      <n-data-table
        :columns="columns"
        :data="tableData"
        :pagination="false"
        :bordered="false"
        :scroll-x="880"
        size="small"
        class="mount-table"
      />
    </div>

    <!-- 自动刷新配置弹窗 -->
    <n-modal
      :show="showAutoRefreshModal"
      preset="dialog"
      title="自动刷新配置"
      :closable="!state.submitLoading"
      :mask-closable="!state.submitLoading"
      :close-on-esc="!state.submitLoading"
      @update:show="handleAutoRefreshModalShowUpdate"
    >
      <div class="auto-refresh-config">
        <n-form
          ref="autoRefreshFormRef"
          :model="autoRefreshForm"
          :rules="autoRefreshRules"
          label-placement="left"
          label-width="120px"
        >
          <n-form-item label="刷新间隔(分钟)" path="refreshInterval">
            <n-input-number
              v-model:value="autoRefreshForm.refreshInterval"
              :min="30"
              :max="1440"
              placeholder="30-1440分钟"
              style="width: 100%"
              :disabled="state.submitLoading"
            />
          </n-form-item>

          <n-form-item label="持续天数" path="autoRefreshDays">
            <n-input-number
              v-model:value="autoRefreshForm.autoRefreshDays"
              :min="1"
              :max="365"
              placeholder="1-365天"
              style="width: 100%"
              :disabled="state.submitLoading"
            />
          </n-form-item>

          <n-form-item label="深度刷新" path="enableDeepRefresh">
            <n-switch
              v-model:value="autoRefreshForm.enableDeepRefresh"
              :disabled="state.submitLoading"
            />
          </n-form-item>
        </n-form>
      </div>

      <template #action>
        <n-button :disabled="state.submitLoading" @click="closeAutoRefreshModal">取消</n-button>
        <n-button type="primary" :disabled="state.submitLoading" @click="handleAutoRefreshConfirm">
          确认
        </n-button>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { reactive, computed, h, onMounted, onUnmounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import {
  NButton,
  NDataTable,
  NInput,
  NSelect,
  NText,
  useMessage,
  type DataTableColumns,
  NCard,
  NSwitch,
  NModal,
  NForm,
  NFormItem,
  NInputNumber,
} from 'naive-ui'
import {
  batchAddStorage,
  type AddStorageRequest,
  type BatchAddStorageResponse,
} from '@/api/storage'
import { getErrorMessage, type ApiResponse } from '@/utils/api'
import { getCloudTokenList } from '@/api/cloudtoken'
import { getListItems } from '@/utils/pagination'
import { normalizeCloudTokens } from '@/utils/responseGuards'
import { OS_TYPES, getOsTypeDisplayName, getOsTypeColor } from '@/utils/osType'
import { useSharedStore } from '@/stores/modules/shared'
import type { FormRules } from 'naive-ui'

export interface MountItem {
  name: string
  osType: string
  subscribeUser?: string
  shareCode?: string
  shareAccessCode?: string
  cloudToken?: number
  disableSwitchCloudToken?: boolean
  fileId?: string
  familyId?: string
  userName?: string
}

interface Props {
  items: MountItem[]
  defaultCloudToken?: number
}

interface Emits {
  (e: 'confirm', payload: { id: number; path: string }[]): void
  (e: 'cancel'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

// 消息提示
const message = useMessage()

// 状态管理
const state = reactive({
  submitLoading: false,
  cloudTokens: [] as Models.CloudToken[],
})

let isComponentMounted = false
let submitRequestVersion = 0
let cloudTokenRequestVersion = 0

const isCurrentSubmitRequest = (version: number) => {
  return isComponentMounted && submitRequestVersion === version
}

const isCurrentCloudTokenRequest = (version: number) => {
  return isComponentMounted && cloudTokenRequestVersion === version
}

const invalidatePendingWork = () => {
  submitRequestVersion++
  cloudTokenRequestVersion++
  state.submitLoading = false
}

// 共享存储
const sharedStore = useSharedStore()
const { storageSetting } = storeToRefs(sharedStore)

// 表格数据
interface TableRow extends MountItem {
  id: string
  localPath: string
  selectedCloudToken?: number
  mountError?: string
}

const tableData = reactive<TableRow[]>([])
const confirmedMountItems = reactive<{ id: number; path: string }[]>([])

// 自动刷新配置相关
const showAutoRefreshModal = ref(false)
const autoRefreshFormRef = ref<InstanceType<typeof NForm>>()

const autoRefreshForm = reactive({
  refreshInterval: 60,
  autoRefreshDays: 7,
  enableDeepRefresh: false,
})

const autoRefreshRules: FormRules = {
  refreshInterval: [
    {
      type: 'number' as const,
      min: 30,
      max: 1440,
      message: '刷新间隔必须在30-1440分钟之间',
      trigger: 'blur' as const,
    },
  ],
  autoRefreshDays: [
    {
      type: 'number' as const,
      min: 1,
      max: 365,
      message: '持续天数必须在1-365天之间',
      trigger: 'blur' as const,
    },
  ],
}

const nulCharacter = String.fromCharCode(0)
const maxLocalPathLength = 4096
const sanitizedFileNamePattern = /[<>:"?*\\/]/g
const pathTextEncoder = new TextEncoder()
const pathTextDecoder = new TextDecoder('utf-8', { fatal: true })

const hasInvalidPathCharacter = (path: string) =>
  path.includes('\\') || path.includes('\n') || path.includes('\r') || path.includes(nulCharacter)

const sanitizePathSegment = (segment: string) => {
  return segment.replace(/\|/g, '丨').replace(sanitizedFileNamePattern, '_').trim()
}

const escapeStoragePathPercent = (segment: string) => {
  return segment.replace(/%/g, '%25')
}

const getUtf8ByteLength = (value: string) => {
  return pathTextEncoder.encode(value).length
}

const hexToNumber = (value: string) => {
  const code = value.charCodeAt(0)
  if (code >= 48 && code <= 57) {
    return code - 48
  }

  if (code >= 65 && code <= 70) {
    return code - 65 + 10
  }

  if (code >= 97 && code <= 102) {
    return code - 97 + 10
  }

  return null
}

const decodePathSegment = (segment: string) => {
  const bytes: number[] = []
  for (let index = 0; index < segment.length; index++) {
    const char = segment[index]
    if (char === '%' && index + 2 < segment.length) {
      const high = hexToNumber(segment[index + 1])
      const low = hexToNumber(segment[index + 2])
      if (high !== null && low !== null) {
        bytes.push(high * 16 + low)
        index += 2

        continue
      }
    }

    const codePoint = segment.codePointAt(index)
    if (codePoint === undefined) {
      continue
    }

    const codePointChar = String.fromCodePoint(codePoint)
    for (const byte of pathTextEncoder.encode(codePointChar)) {
      bytes.push(byte)
    }
    index += codePointChar.length - 1
  }

  try {
    return pathTextDecoder.decode(new Uint8Array(bytes))
  } catch {
    return null
  }
}

const normalizePathSegment = (segment: string) => {
  const sanitizedSegment = sanitizePathSegment(segment)
  if (!sanitizedSegment) {
    return ''
  }

  return escapeStoragePathPercent(sanitizedSegment)
}

const getPathValidationError = (path: string, options?: { allowRoot?: boolean }) => {
  const value = path.trim()
  if (!value) {
    return '不能为空'
  }

  if (getUtf8ByteLength(value) > maxLocalPathLength) {
    return `长度不能超过 ${maxLocalPathLength}`
  }

  if (!value.startsWith('/')) {
    return '必须以 / 开头'
  }

  if (hasInvalidPathCharacter(value)) {
    return '不能包含反斜杠、换行或空字符'
  }

  let hasSegment = false
  for (const segment of value.split('/')) {
    if (!segment) {
      continue
    }

    hasSegment = true

    const decodedSegment = decodePathSegment(segment)
    if (decodedSegment === null) {
      return '包含不合法的 UTF-8 编码'
    }

    if (decodedSegment === '.' || decodedSegment === '..') {
      return '不能包含 . 或 .. 路径段'
    }

    if (decodedSegment.includes('/') || hasInvalidPathCharacter(decodedSegment)) {
      return '不能包含转义后的路径分隔符或非法字符'
    }

    if (!normalizePathSegment(decodedSegment)) {
      return '不能包含清理后为空的路径段'
    }
  }

  if (!options?.allowRoot && !hasSegment) {
    return '不能为根路径'
  }

  return null
}

const pathPrefixValidationError = computed(() =>
  getPathValidationError(storageSetting.value.pathPrefix, { allowRoot: true })
)

const getNormalizedLocalPathKey = (path: string) => {
  const value = path.trim()
  const segments: string[] = []

  for (const segment of value.split('/')) {
    if (!segment) {
      continue
    }

    const decodedSegment = decodePathSegment(segment)
    if (decodedSegment === null) {
      return null
    }

    const normalizedSegment = normalizePathSegment(decodedSegment)
    if (!normalizedSegment) {
      return null
    }

    segments.push(normalizedSegment)
  }

  return `/${segments.join('/')}`
}

const normalizePathPrefix = (pathPrefix: string) => {
  const normalizedPathPrefix = getNormalizedLocalPathKey(pathPrefix)
  if (!normalizedPathPrefix || normalizedPathPrefix === '/') {
    return '/'
  }

  return `${normalizedPathPrefix}/`
}

const joinLocalPathWithDisplayName = (pathPrefix: string, name: string) => {
  const prefix = normalizePathPrefix(pathPrefix || '/')
  const normalizedName = normalizePathSegment(name)
  if (!normalizedName) {
    return prefix
  }

  return `${prefix}${normalizedName}`
}

const duplicatedLocalPaths = computed(() => {
  const pathCounts = new Map<string, number>()
  for (const row of tableData) {
    const localPathKey = getNormalizedLocalPathKey(row.localPath)
    if (!localPathKey) {
      continue
    }

    pathCounts.set(localPathKey, (pathCounts.get(localPathKey) ?? 0) + 1)
  }

  return new Set(
    [...pathCounts.entries()].filter(([, count]) => count > 1).map(([localPathKey]) => localPathKey)
  )
})

const getRowPathError = (row: TableRow) => {
  const pathError = getPathValidationError(row.localPath)
  if (pathError) {
    return pathError
  }

  const localPathKey = getNormalizedLocalPathKey(row.localPath)
  if (localPathKey && duplicatedLocalPaths.value.has(localPathKey)) {
    return '不能重复'
  }

  return null
}

const normalizeCloudTokenID = (value: unknown) => {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) {
    return undefined
  }

  return value
}

const shouldUseCloudTokenFallback = (osType: string) => {
  return osType === OS_TYPES.PERSON_FOLDER || osType === OS_TYPES.FAMILY_FOLDER
}

const getInitialSelectedCloudToken = (item: MountItem) => {
  const itemCloudToken = normalizeCloudTokenID(item.cloudToken)
  if (item.disableSwitchCloudToken) {
    return itemCloudToken
  }

  if (itemCloudToken !== undefined) {
    return itemCloudToken
  }

  if (!shouldUseCloudTokenFallback(item.osType)) {
    return 0
  }

  return (
    normalizeCloudTokenID(props.defaultCloudToken) ??
    normalizeCloudTokenID(storageSetting.value.selectedToken)
  )
}

const hasBoundCloudToken = (cloudToken: unknown) => {
  const normalizedCloudToken = normalizeCloudTokenID(cloudToken)

  return normalizedCloudToken !== undefined && normalizedCloudToken > 0
}

const hasNonEmptyValue = (value: string | undefined) => {
  return typeof value === 'string' && value.trim().length > 0
}

const getRowParamsError = (row: TableRow) => {
  switch (row.osType) {
    case OS_TYPES.SUBSCRIBE:
      return hasNonEmptyValue(row.subscribeUser) ? null : '订阅用户不能为空'
    case OS_TYPES.SUBSCRIBE_SHARE_FOLDER:
      return hasNonEmptyValue(row.subscribeUser) && hasNonEmptyValue(row.shareCode)
        ? null
        : '订阅分享参数不完整'
    case OS_TYPES.SHARE_FOLDER:
      return hasNonEmptyValue(row.shareCode) ? null : '分享码不能为空'
    case OS_TYPES.PERSON_FOLDER:
      if (!hasNonEmptyValue(row.fileId)) {
        return '个人文件夹ID不能为空'
      }

      return hasBoundCloudToken(row.selectedCloudToken) ? null : '个人文件夹必须绑定云盘令牌'
    case OS_TYPES.FAMILY_FOLDER:
      if (!hasNonEmptyValue(row.fileId) || !hasNonEmptyValue(row.familyId)) {
        return '家庭云参数不完整'
      }

      return hasBoundCloudToken(row.selectedCloudToken) ? null : '家庭云必须绑定云盘令牌'
    default:
      return '挂载类型不支持'
  }
}

const getRowValidationError = (row: TableRow) => {
  const pathError = getRowPathError(row)
  if (pathError) {
    return `挂载路径${pathError}`
  }

  return getRowParamsError(row)
}

const invalidRows = computed(() =>
  tableData
    .map((row, index) => ({ index, error: getRowValidationError(row) }))
    .filter((row): row is { index: number; error: string } => Boolean(row.error))
)

const getConfirmedItems = () => {
  return confirmedMountItems.map((item) => ({ ...item }))
}

const normalizeRowsForSubmit = () => {
  for (const row of tableData) {
    const normalizedPath = getNormalizedLocalPathKey(row.localPath)
    if (normalizedPath) {
      row.localPath = normalizedPath
    }
  }
}

const appendConfirmedItems = (items: { id: number; path: string }[]) => {
  for (const item of items) {
    if (
      confirmedMountItems.some(
        (confirmed) => confirmed.id === item.id && confirmed.path === item.path
      )
    ) {
      continue
    }

    confirmedMountItems.push(item)
  }
}

const handleSelectedTokenUpdate = (value: unknown) => {
  storageSetting.value.selectedToken = normalizeCloudTokenID(value) ?? 0
}

// 计算属性
const cloudTokenOptions = computed(() => [
  { label: '不绑定', value: 0 },
  ...state.cloudTokens.map((token) => ({
    label: token.name,
    value: token.id,
  })),
])

const hasValidPathPrefix = computed(() => !pathPrefixValidationError.value)

const hasInvalidRows = computed(() => invalidRows.value.length > 0)

// 检查是否所有项目都禁用令牌切换
const allTokenSwitchDisabled = computed(
  () => tableData.length > 0 && tableData.every((row) => row.disableSwitchCloudToken)
)

// 表格列定义
const columns: DataTableColumns<TableRow> = [
  {
    title: '序号',
    key: 'index',
    width: 80,
    render: (_, index) => index + 1,
    align: 'center',
    ellipsis: { tooltip: true },
  },
  {
    title: '识别出的名称',
    key: 'name',
    width: 200,
    align: 'center',
    ellipsis: {
      tooltip: true,
    },
  },
  {
    title: '挂载类型',
    key: 'osType',
    width: 150,
    align: 'center',
    ellipsis: { tooltip: true },
    render: (row) => {
      const displayName = getOsTypeDisplayName(row.osType)
      const colorInfo = getOsTypeColor(row.osType)
      return h('span', { style: { color: colorInfo.textColor } }, displayName)
    },
  },
  {
    title: '挂载路径',
    key: 'localPath',
    width: 250,
    align: 'left',
    titleAlign: 'center',
    render: (row, index) => {
      const rowError = getRowValidationError(row)
      const feedback = rowError || row.mountError

      return h('div', { class: 'path-cell' }, [
        h(NInput, {
          value: row.localPath,
          placeholder: '必须以 / 开头，且不能为根路径',
          status: feedback ? ('error' as const) : undefined,
          disabled: state.submitLoading,
          onUpdateValue: (value: string) => {
            if (state.submitLoading) {
              return
            }

            tableData[index].localPath = value
            tableData[index].mountError = undefined
          },
        }),
        feedback
          ? h(
              NText,
              {
                class: 'path-error',
                type: 'error',
              },
              { default: () => feedback }
            )
          : null,
      ])
    },
  },
  {
    title: '绑定令牌',
    key: 'selectedCloudToken',
    width: 200,
    align: 'center',
    render: (row, index) => {
      return h(NSelect, {
        value: row.selectedCloudToken,
        options: cloudTokenOptions.value,
        placeholder: '选择云盘令牌',
        clearable: true,
        disabled: state.submitLoading || row.disableSwitchCloudToken,
        onUpdateValue: (value: unknown) => {
          if (state.submitLoading) {
            return
          }

          tableData[index].selectedCloudToken = normalizeCloudTokenID(value)
          tableData[index].mountError = undefined
        },
      })
    },
  },
]

// 初始化表格数据
const initTableData = () => {
  // 如果有用户名，强制设置默认路径前缀（覆盖localStorage保存的值）
  const firstItemWithUserName = props.items.find((item) => item.userName)
  if (firstItemWithUserName?.userName) {
    // 强制设置为订阅号默认路径
    sharedStore.resetPathPrefix(`/电影/${firstItemWithUserName.userName}/`)
  }

  const newItems = props.items.map((item, index) => {
    // 将识别出的名称作为单个路径段拼接，避免名称中的 / 或 % 改变挂载层级。
    const localPath = joinLocalPathWithDisplayName(storageSetting.value.pathPrefix, item.name)

    return {
      ...item,
      id: `item_${index}`,
      localPath,
      selectedCloudToken: getInitialSelectedCloudToken(item),
    } as TableRow
  })
  tableData.length = 0
  tableData.push(...newItems)
}

// 获取云盘令牌列表
const fetchCloudTokens = () => {
  const currentRequest = ++cloudTokenRequestVersion

  return getCloudTokenList({ noPaginate: true })
    .then((res) => {
      if (!isCurrentCloudTokenRequest(currentRequest)) {
        return
      }

      if (res.code === 200 && res.data) {
        const tokenItems = getListItems<Models.CloudToken>(res.data)
        const tokens = normalizeCloudTokens(tokenItems)
        if (!tokens) {
          message.error('获取云盘令牌列表失败：响应数据格式异常')

          return
        }

        state.cloudTokens = tokens

        return
      }
      message.error(res.msg || '获取云盘令牌列表失败')
    })
    .catch((error) => {
      if (!isCurrentCloudTokenRequest(currentRequest)) {
        return
      }

      message.error(getErrorMessage(error, '获取云盘令牌列表失败'))
    })
}

// 批量应用令牌
const handleBatchApplyToken = () => {
  if (state.submitLoading) {
    return
  }

  if (allTokenSwitchDisabled.value) {
    message.warning('所有项目都禁止修改令牌')
    return
  }

  // 将选中的令牌应用到所有未禁用的行
  let appliedCount = 0
  let skippedCount = 0

  tableData.forEach((row) => {
    if (!row.disableSwitchCloudToken) {
      row.selectedCloudToken = normalizeCloudTokenID(storageSetting.value.selectedToken)
      row.mountError = undefined
      appliedCount++
    } else {
      skippedCount++
    }
  })

  if (skippedCount > 0) {
    message.success(`已批量应用令牌设置到 ${appliedCount} 个项目，跳过 ${skippedCount} 个禁用项目`)
  } else {
    message.success('已批量应用令牌设置')
  }
}

// 批量应用路径前缀
const handleBatchApplyPathPrefix = () => {
  if (state.submitLoading) {
    return
  }

  if (!hasValidPathPrefix.value) {
    message.warning(`路径前缀${pathPrefixValidationError.value}`)
    return
  }

  // 确保前缀以 / 结尾（如果不是单独的 /）
  const prefix = normalizePathPrefix(storageSetting.value.pathPrefix)
  storageSetting.value.pathPrefix = prefix

  // 将路径前缀应用到所有行
  tableData.forEach((row) => {
    row.localPath = joinLocalPathWithDisplayName(prefix, row.name)
    row.mountError = undefined
  })

  message.success('已批量应用路径前缀设置')
}

// 取消
const handleCancel = () => {
  if (state.submitLoading) {
    return
  }

  invalidatePendingWork()
  sharedStore.restorePathPrefix()
  const confirmedItems = getConfirmedItems()
  if (confirmedItems.length > 0) {
    emit('confirm', confirmedItems)

    return
  }

  emit('cancel')
}

// 编辑自动刷新配置
const handleEditAutoRefreshConfig = () => {
  if (state.submitLoading) {
    return
  }

  // 从 store 同步当前配置到表单
  autoRefreshForm.refreshInterval = storageSetting.value.refreshInterval || 60
  autoRefreshForm.autoRefreshDays = storageSetting.value.autoRefreshDays || 7
  autoRefreshForm.enableDeepRefresh = storageSetting.value.enableDeepRefresh || false

  showAutoRefreshModal.value = true
}

const handleAutoRefreshModalShowUpdate = (show: boolean) => {
  if (show) {
    showAutoRefreshModal.value = true

    return
  }

  closeAutoRefreshModal()
}

const closeAutoRefreshModal = () => {
  if (state.submitLoading) {
    return
  }

  showAutoRefreshModal.value = false
}

// 确认自动刷新配置
const handleAutoRefreshConfirm = () => {
  if (state.submitLoading) {
    return
  }

  autoRefreshFormRef.value?.validate((errors: unknown) => {
    if (state.submitLoading) {
      return
    }

    if (errors) {
      message.error('请检查表单输入')
      return
    }

    // 保存配置到共享存储
    storageSetting.value.autoRefreshDays = autoRefreshForm.autoRefreshDays
    storageSetting.value.refreshInterval = autoRefreshForm.refreshInterval
    storageSetting.value.enableDeepRefresh = autoRefreshForm.enableDeepRefresh

    closeAutoRefreshModal()
    message.success('自动刷新配置已保存')
  })
}

// 构建请求数据
const buildRequests = (): AddStorageRequest[] => {
  return tableData.map((row) => ({
    localPath: row.localPath.trim(),
    osType: row.osType as AddStorageRequest['osType'],
    cloudToken: normalizeCloudTokenID(row.selectedCloudToken),
    subscribeUser: row.subscribeUser,
    shareCode: row.shareCode,
    shareAccessCode: row.shareAccessCode,
    fileId: row.fileId,
    familyId: row.familyId,
    enableAutoRefresh: storageSetting.value.enableAutoRefresh,
    autoRefreshDays: storageSetting.value.enableAutoRefresh
      ? storageSetting.value.autoRefreshDays
      : undefined,
    refreshInterval: storageSetting.value.enableAutoRefresh
      ? storageSetting.value.refreshInterval
      : undefined,
    enableDeepRefresh: storageSetting.value.enableAutoRefresh
      ? storageSetting.value.enableDeepRefresh
      : undefined,
  }))
}

const isNonNegativeInteger = (value: unknown): value is number => {
  return Number.isInteger(value) && Number(value) >= 0
}

const isPositiveSafeInteger = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isSafeInteger(value) && value > 0
}

const isOptionalString = (value: unknown): value is string | undefined => {
  return value === undefined || typeof value === 'string'
}

const isOptionalBoolean = (value: unknown): value is boolean | undefined => {
  return value === undefined || typeof value === 'boolean'
}

const isOptionalNonNegativeInteger = (value: unknown): value is number | undefined => {
  return value === undefined || isNonNegativeInteger(value)
}

const isBatchAddResultItem = (
  value: unknown
): value is BatchAddStorageResponse['results'][number] => {
  if (!value || typeof value !== 'object') {
    return false
  }

  const item = value as Partial<BatchAddStorageResponse['results'][number]>

  return (
    typeof item.localPath === 'string' &&
    typeof item.success === 'boolean' &&
    isOptionalNonNegativeInteger(item.id) &&
    isOptionalString(item.error) &&
    isOptionalBoolean(item.scanQueued) &&
    isOptionalString(item.scanError)
  )
}

const isBatchAddStorageResponse = (value: unknown): value is BatchAddStorageResponse => {
  if (!value || typeof value !== 'object') {
    return false
  }

  const data = value as Partial<BatchAddStorageResponse>
  const scanQueuedCount = data.scanQueuedCount ?? 0
  const scanFailedCount = data.scanFailedCount ?? 0

  if (
    !isNonNegativeInteger(data.successCount) ||
    !isNonNegativeInteger(data.failCount) ||
    !isOptionalNonNegativeInteger(data.scanQueuedCount) ||
    !isOptionalNonNegativeInteger(data.scanFailedCount) ||
    scanQueuedCount + scanFailedCount !== data.successCount ||
    !Array.isArray(data.results) ||
    !data.results.every(isBatchAddResultItem)
  ) {
    return false
  }

  return true
}

// 处理批量挂载结果
const handleMountResults = (response: ApiResponse<BatchAddStorageResponse>) => {
  if (response.code !== 200) {
    message.error(response.msg || '批量挂载失败')
    return
  }

  if (!isBatchAddStorageResponse(response.data)) {
    message.error('批量挂载响应统计缺失/异常')
    return
  }

  const { successCount, failCount, scanFailedCount = 0, results } = response.data

  const successItems: { id: number; path: string }[] = []
  for (const result of results) {
    if (!result.success || !isPositiveSafeInteger(result.id)) {
      continue
    }

    successItems.push({
      id: result.id,
      path: result.localPath,
    })
  }

  if (successItems.length !== successCount || results.length !== successCount + failCount) {
    message.error('批量挂载响应结果异常')
    return
  }

  appendConfirmedItems(successItems)

  const failedResultsByPath = new Map(
    results
      .filter((result) => !result.success)
      .map((result) => [result.localPath.trim(), result.error || '挂载失败'])
  )
  const successPaths = new Set(
    results.filter((result) => result.success).map((result) => result.localPath.trim())
  )

  for (let index = tableData.length - 1; index >= 0; index--) {
    const rowPath = tableData[index].localPath.trim()
    if (successPaths.has(rowPath)) {
      tableData.splice(index, 1)

      continue
    }

    tableData[index].mountError = failedResultsByPath.get(rowPath)
  }

  if (successCount > 0 && failCount === 0) {
    const totalSuccessCount = confirmedMountItems.length
    message.success(
      totalSuccessCount > successCount
        ? `成功挂载 ${successCount} 个存储点，累计 ${totalSuccessCount} 个`
        : `成功挂载 ${successCount} 个存储点`
    )
    if (scanFailedCount > 0) {
      message.warning(`${scanFailedCount} 个存储点已挂载，但扫描任务未提交`)
    }
    sharedStore.restorePathPrefix()
    emit('confirm', getConfirmedItems())

    return
  }

  if (successCount > 0) {
    message.warning(`成功挂载 ${successCount} 个存储点，失败 ${failCount} 个，请修正失败项后重试`)
    if (scanFailedCount > 0) {
      message.warning(`${scanFailedCount} 个存储点已挂载，但扫描任务未提交`)
    }

    return
  }

  const firstError = results.find((r) => !r.success)?.error
  message.error(`挂载全部失败${firstError ? `: ${firstError}` : ''}`)
}

// 确认挂载
const handleConfirm = () => {
  if (state.submitLoading) {
    return
  }

  // 验证数据
  if (hasInvalidRows.value) {
    const firstInvalidRow = invalidRows.value[0]
    message.warning(`第 ${firstInvalidRow.index + 1} 行${firstInvalidRow.error}`)
    return
  }
  normalizeRowsForSubmit()

  state.submitLoading = true
  const currentRequest = ++submitRequestVersion

  // 构建请求数据
  const requests = buildRequests()

  // 批量添加存储挂载（使用后台任务方式）
  return batchAddStorage({ items: requests })
    .then((response) => {
      if (!isCurrentSubmitRequest(currentRequest)) {
        return
      }

      handleMountResults(response)
    })
    .catch((error) => {
      if (!isCurrentSubmitRequest(currentRequest)) {
        return
      }

      const errorMessage = getErrorMessage(error, '批量挂载失败')

      console.error('批量挂载失败:', errorMessage)
      message.error(errorMessage)
    })
    .finally(() => {
      if (isCurrentSubmitRequest(currentRequest)) {
        state.submitLoading = false
      }
    })
}

// 组件挂载时初始化数据
onMounted(() => {
  isComponentMounted = true
  submitRequestVersion++
  cloudTokenRequestVersion++

  initTableData()
  fetchCloudTokens()
})

onUnmounted(() => {
  isComponentMounted = false
  invalidatePendingWork()
  sharedStore.restorePathPrefix()
})

defineExpose({
  handleConfirm,
  handleCancel,
  getConfirmedItems,
  state,
})
</script>

<style scoped>
.mount-bind-content {
  padding: 16px 0;
}

.batch-actions {
  margin-bottom: 16px;
}

.batch-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(300px, 100%), 1fr));
  gap: 16px;
}

.batch-item {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.batch-label {
  font-weight: 600;
  white-space: nowrap;
}

.table-container {
  max-height: 400px;
  overflow: auto;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
}

.mount-table {
  min-height: 200px;
}

.path-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.path-error {
  font-size: 12px;
  line-height: 1.3;
}

/* 固定表格标题 */
:deep(.n-data-table-thead) {
  position: sticky;
  top: 0;
  z-index: 1;
  background: var(--n-th-color);
}

.modal-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

/* 表格样式优化 */
:deep(.n-data-table-th) {
  background: var(--n-th-color);
  font-weight: 600;
}

:deep(.n-data-table-td) {
  padding: 12px 8px;
}

/* 响应式设计 */
@media (width <= 768px) {
  .modal-actions {
    flex-direction: column;
  }
}

@media (width <= 640px) {
  .mount-bind-content {
    padding: 8px 0;
  }

  .batch-grid {
    gap: 12px;
  }

  .batch-item {
    flex-wrap: wrap;
    align-items: stretch;
  }

  .batch-label {
    width: 100%;
  }

  .batch-item :deep(.n-input),
  .batch-item :deep(.n-select) {
    flex: 1 1 0;
    min-width: 0;
  }
}
</style>
