<template>
  <div class="file-browser">
    <!-- 顶部导航栏 -->
    <div class="header">
      <div class="breadcrumb-section">
        <n-breadcrumb>
          <n-breadcrumb-item @click="navigateToPath('/')">
            <n-icon :component="HomeOutline" />
          </n-breadcrumb-item>
          <n-breadcrumb-item
            v-for="(item, index) in breadcrumbs"
            :key="index"
            @click="navigateToPath(decodeOpenHref(item.href))"
            :clickable="index < breadcrumbs.length - 1"
          >
            {{ item.name }}
          </n-breadcrumb-item>
        </n-breadcrumb>
      </div>

      <div class="actions">
        <!-- 新增：批量删除按钮 -->
        <n-button
          v-if="selectedRowKeys.length > 0"
          type="error"
          text
          :loading="batchDeleteSubmitting"
          :disabled="
            loading ||
            batchDeleteSubmitting ||
            batchDeleteDialogOpen ||
            selectedDownloadingRowCount > 0
          "
          @click="handleBatchDelete"
        >
          <template #icon>
            <n-icon :component="TrashOutline" />
          </template>
          删除({{ selectedRowKeys.length }})
        </n-button>

        <n-divider vertical v-if="selectedRowKeys.length > 0" />

        <n-button text @click="goBack" :disabled="!canGoBack">
          <template #icon>
            <n-icon :component="ArrowUndoOutline" />
          </template>
          返回上级
        </n-button>
        <n-button text @click="goAdmin">
          <template #icon>
            <n-icon :component="SettingsOutline" />
          </template>
          后台
        </n-button>
        <n-button text @click="showSearch = true">
          <template #icon>
            <n-icon :component="SearchOutline" />
          </template>
          搜索
        </n-button>
        <n-button text @click="refreshCurrentPath" :loading="loading">
          <template #icon>
            <n-icon :component="RefreshOutline" />
          </template>
          刷新
        </n-button>
      </div>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <n-spin size="large" />
    </div>

    <!-- 根据isDir判断渲染不同组件 -->
    <template v-else-if="fileInfo">
      <!-- 目录：显示文件列表 -->
      <div v-if="fileInfo.isDir" class="directory-view">
        <!--
          修改点：传入 checked-row-keys 和更新事件
          注意：你需要确保 FileList 组件接收这些 props 并传递给内部的 n-data-table
        -->
        <FileList
          :file-list="fileInfo.children || []"
          :loading="false"
          :downloading-row-keys="downloadingRowKeys"
          v-model:checked-row-keys="selectedRowKeys"
          @file-click="handleFileClick"
          @download="downloadFile"
        />
      </div>
      <!-- 文件：显示文件详情 -->
      <FileDetail v-else :file-info="fileInfo" :loading="false" />
    </template>

    <!-- 搜索弹窗 -->
    <SearchFilesDialog
      :show="showSearch"
      :currentDirId="fileInfo && fileInfo.isDir ? fileInfo.id : null"
      @update:show="(v) => (showSearch = v)"
      @select="onSearchSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton,
  NIcon,
  NBreadcrumb,
  NBreadcrumbItem,
  NSpin,
  NDivider,
  useMessage,
  useDialog, // 引入 useDialog
} from 'naive-ui'
import {
  HomeOutline,
  RefreshOutline,
  ArrowUndoOutline,
  SearchOutline,
  SettingsOutline,
  TrashOutline, // 引入删除图标
} from '@vicons/ionicons5'
import {
  openFile,
  createDownloadUrl,
  batchDeleteFiles, // 引入批量删除API
  type FileChild,
  type BreadcrumbItem,
  type FileOpenResponse,
  type FileSearchItem,
} from '@/api/file'
import { FileList, FileDetail, SearchFilesDialog } from '@/components/file-browser'
import { normalizeCreateDownloadUrlResponse } from '@/utils/responseGuards'
import { getErrorMessage } from '@/utils/api'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog() // 初始化 dialog

// 响应式数据
const loading = ref(false)
const fileInfo = ref<FileOpenResponse | null>(null)
const currentPath = ref('/')
const breadcrumbs = ref<BreadcrumbItem[]>([])
const showSearch = ref(false)
// 新增：选中的文件ID列表
const selectedRowKeys = ref<number[]>([])
const batchDeleteSubmitting = ref(false)
const batchDeleteDialogOpen = ref(false)
const downloadingRowKeys = ref<number[]>([])
let isComponentMounted = true
let fileOpenRequestId = 0
const openApiBasePath = '/api/file/open'

// 计算属性
const canGoBack = computed(() => breadcrumbs.value.length > 0)
const selectedDownloadingRowCount = computed(() => {
  const downloadingIds = new Set(downloadingRowKeys.value)

  return selectedRowKeys.value.filter((id) => downloadingIds.has(id)).length
})

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

const isString = (value: unknown): value is string => {
  return typeof value === 'string'
}

const isSafeNonNegativeInteger = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

const isFileAddition = (value: unknown): value is Record<string, unknown> => {
  return value === undefined || value === null || isRecord(value)
}

const normalizeBaseFile = (
  value: unknown
): (Models.VirtualFile & { href: string; apiPath: string }) | null => {
  if (!isRecord(value)) {
    return null
  }

  if (
    !isSafeNonNegativeInteger(value.id) ||
    !isString(value.cloudId) ||
    !isSafeNonNegativeInteger(value.parentId) ||
    !isSafeNonNegativeInteger(value.topId) ||
    typeof value.isTop !== 'boolean' ||
    typeof value.isDir !== 'boolean' ||
    !isString(value.name) ||
    !isSafeNonNegativeInteger(value.size) ||
    !isString(value.hash) ||
    !isString(value.osType) ||
    !isFileAddition(value.addition) ||
    !isString(value.rev) ||
    !isString(value.createDate) ||
    !isString(value.modifyDate) ||
    !isString(value.createdAt) ||
    !isString(value.updatedAt) ||
    !isString(value.href) ||
    !isString(value.apiPath)
  ) {
    return null
  }

  return {
    id: value.id,
    cloudId: value.cloudId,
    parentId: value.parentId,
    topId: value.topId,
    isTop: value.isTop,
    isDir: value.isDir,
    name: value.name,
    size: value.size,
    hash: value.hash,
    osType: value.osType,
    addition: value.addition || {},
    rev: value.rev,
    createDate: value.createDate,
    modifyDate: value.modifyDate,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
    href: value.href,
    apiPath: value.apiPath,
  }
}

const normalizeFileChildren = (value: unknown): FileChild[] | null => {
  if (value === undefined) {
    return []
  }
  if (!Array.isArray(value)) {
    return null
  }

  const children: FileChild[] = []
  for (const item of value) {
    const child = normalizeBaseFile(item)
    if (!child) {
      return null
    }
    children.push(child)
  }

  return children
}

const normalizeBreadcrumbs = (value: unknown): BreadcrumbItem[] | null => {
  if (value === undefined || value === null) {
    return []
  }

  if (!Array.isArray(value)) {
    return null
  }

  const items: BreadcrumbItem[] = []
  for (const item of value) {
    if (!isRecord(item) || !isString(item.href) || !isString(item.name)) {
      return null
    }
    items.push({
      href: item.href,
      name: item.name,
    })
  }

  return items
}

const normalizeFileOpenResponse = (value: unknown): FileOpenResponse | null => {
  if (!isRecord(value)) {
    return null
  }

  const file = normalizeBaseFile(value)
  const children = normalizeFileChildren(value.children)
  const normalizedBreadcrumbs = normalizeBreadcrumbs(value.breadcrumbs)

  if (
    !file ||
    !children ||
    !normalizedBreadcrumbs ||
    !isSafeNonNegativeInteger(value.childrenTotal)
  ) {
    return null
  }

  return {
    ...file,
    children,
    childrenTotal: value.childrenTotal,
    breadcrumbs: normalizedBreadcrumbs,
  }
}

// 方法
const isCurrentFileOpenRequest = (requestId: number) => {
  return isComponentMounted && fileOpenRequestId === requestId
}

const loadPath = (path: string) => {
  if (!isComponentMounted) {
    return Promise.resolve()
  }

  const requestId = ++fileOpenRequestId
  loading.value = true
  // 切换路径时清空选中状态
  selectedRowKeys.value = []
  downloadingRowKeys.value = []

  return openFile(path)
    .then((response) => {
      if (!isCurrentFileOpenRequest(requestId)) return

      if (response.code === 200) {
        const openedFile = normalizeFileOpenResponse(response.data)
        if (!openedFile) {
          console.error('文件打开响应数据格式异常:', response.data)
          message.error('响应数据格式异常')

          return
        }

        fileInfo.value = openedFile
        currentPath.value = path
        breadcrumbs.value = openedFile.breadcrumbs
      } else {
        message.error(response.msg || '加载失败')
      }
    })
    .catch((error) => {
      if (!isCurrentFileOpenRequest(requestId)) return

      console.error('加载文件失败:', error)
      message.error(getErrorMessage(error, '加载文件失败'))
    })
    .finally(() => {
      if (isCurrentFileOpenRequest(requestId)) {
        loading.value = false
      }
    })
}

const handleFileClick = (file: FileChild) => {
  navigateToPath(decodeOpenHref(file.href))
}

const navigateToPath = (path: string) => {
  router.push({
    path: '/',
    query: { path: path },
  })
}

const decodeOpenHref = (href: string) => {
  const path = href.startsWith(openApiBasePath) ? href.slice(openApiBasePath.length) : href

  try {
    return decodeURIComponent(path || '/')
  } catch {
    return path || '/'
  }
}

const refreshCurrentPath = () => {
  return loadPath(currentPath.value)
}

const goAdmin = () => {
  router.push({ path: '/@dashboard' })
}

// 新增：处理批量删除
const handleBatchDelete = () => {
  if (
    selectedRowKeys.value.length === 0 ||
    loading.value ||
    batchDeleteSubmitting.value ||
    batchDeleteDialogOpen.value ||
    selectedDownloadingRowCount.value > 0 ||
    !isComponentMounted
  ) {
    return
  }

  const ids = [...new Set(selectedRowKeys.value)]
  batchDeleteDialogOpen.value = true

  dialog.warning({
    title: '确认删除',
    content: `确定要删除选中的 ${ids.length} 个文件/文件夹吗？此操作不可恢复。`,
    positiveText: '确定删除',
    negativeText: '取消',
    onAfterLeave: () => {
      if (!batchDeleteSubmitting.value) {
        batchDeleteDialogOpen.value = false
      }
    },
    onPositiveClick: () => {
      if (batchDeleteSubmitting.value || !isComponentMounted) {
        return
      }

      batchDeleteSubmitting.value = true

      return batchDeleteFiles({ ids })
        .then((res) => {
          if (!isComponentMounted) return

          if (res.code === 200) {
            message.success('删除任务已提交')
            selectedRowKeys.value = [] // 清空选中
            return refreshCurrentPath() // 刷新列表
          } else {
            message.error(res.msg || '删除失败')
          }
        })
        .catch((err) => {
          if (!isComponentMounted) return

          console.error(err)
          message.error(getErrorMessage(err, '删除请求出错'))
        })
        .finally(() => {
          if (isComponentMounted) {
            batchDeleteSubmitting.value = false
            batchDeleteDialogOpen.value = false
          }
        })
    },
  })
}

const onSearchSelect = (row: FileSearchItem) => {
  let targetPath = row.fullPath || '/'
  if (!row.isDir) {
    const idx = targetPath.lastIndexOf('/')
    targetPath = idx > 0 ? targetPath.slice(0, idx) : '/'
  }
  showSearch.value = false
  navigateToPath(targetPath)
}

const goBack = () => {
  if (breadcrumbs.value.length > 0) {
    const parentIndex = breadcrumbs.value.length - 2
    if (parentIndex >= 0) {
      navigateToPath(decodeOpenHref(breadcrumbs.value[parentIndex].href))
    } else {
      navigateToPath('/')
    }
  }
}

const downloadFile = (file: FileChild) => {
  if (file.isDir || downloadingRowKeys.value.includes(file.id) || !isComponentMounted) {
    return
  }

  const fileId = file.id
  const fileName = file.name
  const navigationRequestId = fileOpenRequestId
  selectedRowKeys.value = selectedRowKeys.value.filter((id) => id !== fileId)
  downloadingRowKeys.value = [...downloadingRowKeys.value, fileId]

  createDownloadUrl({ fileId: file.id })
    .then((response) => {
      if (!isComponentMounted || navigationRequestId !== fileOpenRequestId) return

      if (response.code === 200) {
        const data = normalizeCreateDownloadUrlResponse(response.data)
        if (!data) {
          message.error('创建下载链接失败：响应数据格式异常')

          return
        }

        const link = document.createElement('a')
        link.href = data.downloadUrl
        link.download = fileName
        document.body.appendChild(link)
        link.click()
        document.body.removeChild(link)
        message.success('开始下载')
      } else {
        message.error(response.msg || '创建下载链接失败')
      }
    })
    .catch((error) => {
      if (!isComponentMounted || navigationRequestId !== fileOpenRequestId) return

      console.error('下载失败:', error)
      message.error(getErrorMessage(error, '下载失败'))
    })
    .finally(() => {
      if (!isComponentMounted || navigationRequestId !== fileOpenRequestId) return

      downloadingRowKeys.value = downloadingRowKeys.value.filter((id) => id !== fileId)
    })
}

// 监听路由变化
watch(
  () => route.query.path,
  (newPath) => {
    const path = typeof newPath === 'string' && newPath ? newPath : '/'
    loadPath(path)
  },
  { immediate: true }
)

onUnmounted(() => {
  isComponentMounted = false
  fileOpenRequestId += 1
  loading.value = false
  batchDeleteSubmitting.value = false
  batchDeleteDialogOpen.value = false
  downloadingRowKeys.value = []
})
</script>

<style scoped>
.file-browser {
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px;
  background: var(--n-color-target);
  min-height: 100vh;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  background: var(--n-card-color);
  border-radius: 8px;
  margin-bottom: 20px;
  border: 1px solid var(--n-border-color);
  position: sticky;
  top: 0;
  z-index: 100;
  box-shadow: 0 2px 8px rgb(0 0 0 / 6%);
  backdrop-filter: saturate(180%) blur(2px);
}

.breadcrumb-section {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.actions {
  display: flex;
  gap: 8px;
  align-items: center; /* 确保垂直居中 */
}

.file-container {
  background: var(--n-card-color);
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
}

.list-header {
  display: grid;
  grid-template-columns: 1fr 120px 180px 120px;
  gap: 16px;
  padding: 12px 20px;
  background: var(--n-color-hover);
  border-bottom: 1px solid var(--n-border-color);
  font-weight: 500;
  color: var(--n-text-color);
  font-size: 14px;
}

.header-item {
  display: flex;
  align-items: center;
}

.header-item:nth-child(2),
.header-item:nth-child(3),
.header-item:nth-child(4) {
  justify-content: center;
}

.loading-container {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 60px 20px;
}

.file-list {
  min-height: 400px;
}

.file-item {
  border-bottom: 1px solid var(--n-divider-color);
  cursor: pointer;
  transition: background-color 0.2s;
}

.file-item:hover {
  background-color: var(--n-color-hover);
}

.file-item:last-child {
  border-bottom: none;
}

.file-info {
  display: grid;
  grid-template-columns: 1fr 120px 180px 120px;
  gap: 16px;
  padding: 12px 20px;
  align-items: center;
}

.file-icon-name {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.file-icon {
  font-size: 20px;
  color: var(--n-primary-color);
  flex-shrink: 0;
}

.file-name {
  font-size: 14px;
  color: var(--n-text-color);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-size {
  font-size: 14px;
  color: var(--n-text-color-2);
  text-align: center;
}

.file-date {
  font-size: 14px;
  color: var(--n-text-color-2);
  text-align: center;
}

.file-actions {
  display: flex;
  gap: 8px;
  justify-content: center;
}

.empty-state {
  padding: 60px 20px;
  text-align: center;
}

.file-preview {
  padding: 16px 0;
}

.preview-actions {
  margin-top: 24px;
}

/* 响应式设计 */
@media (width <= 768px) {
  .file-browser {
    padding: 12px;
  }

  .header {
    flex-direction: column;
    gap: 12px;
    align-items: stretch;
  }

  .actions {
    justify-content: center;
  }

  .list-header,
  .file-info {
    grid-template-columns: 1fr 80px 100px;
    gap: 8px;
    padding: 8px 12px;
  }

  .file-actions {
    flex-direction: column;
    gap: 4px;
  }

  .file-date {
    display: none;
  }
}

@media (width <= 480px) {
  .list-header,
  .file-info {
    grid-template-columns: 1fr 60px;
    gap: 8px;
  }

  .file-size {
    display: none;
  }

  .file-name {
    font-size: 13px;
  }
}
</style>
