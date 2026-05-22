<template>
  <div class="person-mount-container">
    <div class="person-mount-content">
      <!-- 第一步：选择令牌 -->
      <div v-if="currentStep === 1" class="step-content">
        <div class="step-header">
          <n-text strong>选择云盘令牌</n-text>
          <n-text depth="3">请选择要使用的天翼云盘令牌</n-text>
        </div>

        <div class="token-section">
          <div v-if="tokenState.loading" class="loading-container">
            <n-spin size="large">
              <template #description>
                <n-text depth="2">正在加载令牌列表...</n-text>
              </template>
            </n-spin>
          </div>

          <div v-else-if="tokenState.tokens.length === 0" class="empty-state">
            <n-empty description="暂无可用令牌" size="large">
              <template #icon>
                <n-icon size="64" :depth="3"><KeyOutline /></n-icon>
              </template>
              <template #extra>
                <n-text depth="3">请先添加天翼云盘令牌</n-text>
              </template>
            </n-empty>
          </div>

          <div v-else class="token-list">
            <div
              v-for="token in tokenState.tokens"
              :key="token.id"
              class="token-card"
              :class="{ active: tokenState.selectedTokenId === token.id, disabled: bindLoading }"
              @click="handleSelectToken(token.id)"
            >
              <div class="token-info">
                <div class="token-header">
                  <n-text strong class="token-name">{{ token.name || '未命名令牌' }}</n-text>
                  <n-tag size="small" type="info" class="token-id-tag"> ID: {{ token.id }} </n-tag>
                </div>
                <n-text depth="3" class="token-username">{{ token.username }}</n-text>
              </div>
              <div class="token-actions">
                <n-icon v-if="tokenState.selectedTokenId === token.id" :size="20" color="#18a058">
                  <CheckmarkCircleOutline />
                </n-icon>
              </div>
            </div>
          </div>

          <div v-if="tokenState.tokens.length > 0" class="step-actions">
            <n-button
              type="primary"
              size="large"
              :disabled="!tokenState.selectedTokenId || bindLoading"
              @click="handleNextToFileSelection"
              style="width: 100%"
            >
              <template #icon
                ><n-icon><ArrowForwardOutline /></n-icon
              ></template>
              下一步：选择文件夹
            </n-button>
          </div>
        </div>
      </div>

      <!-- 第二步：选择文件夹 -->
      <div v-if="currentStep === 2" class="step-content">
        <div class="step-header">
          <n-text strong>选择文件夹</n-text>
          <n-text depth="3">请选择要挂载的个人文件夹（只能选择文件夹类型）</n-text>
        </div>

        <div class="file-section">
          <div v-if="fileState.loading" class="loading-container">
            <n-spin size="large">
              <template #description>
                <n-text depth="2">正在加载文件列表...</n-text>
              </template>
            </n-spin>
          </div>

          <div v-else class="file-tree-container">
            <div class="breadcrumb-container">
              <n-breadcrumb>
                <n-breadcrumb-item @click="handleNavigateToRoot">
                  <n-icon :size="16"><HomeOutline /></n-icon>
                  根目录
                </n-breadcrumb-item>
                <n-breadcrumb-item
                  v-for="(item, index) in fileState.breadcrumbs"
                  :key="item.id"
                  @click="handleNavigateToBreadcrumb(index)"
                >
                  {{ item.name }}
                </n-breadcrumb-item>
              </n-breadcrumb>
            </div>

            <div class="file-tree">
              <n-tree
                :data="fileTreeData"
                :render-label="renderTreeLabel"
                :render-prefix="renderTreePrefix"
                :render-suffix="renderTreeSuffix"
                :selected-keys="fileState.selectedKeys"
                :expanded-keys="fileState.expandedKeys"
                :loading="fileState.treeLoading"
                :disabled="bindLoading"
                block-line
                selectable
                @update:selected-keys="handleTreeSelect"
                @update:expanded-keys="handleTreeExpand"
              />
            </div>

            <div v-if="fileState.selectedFile" class="selected-info">
              <div class="selected-card">
                <div class="selected-header">
                  <n-icon :size="20" color="#18a058"><CheckmarkCircleOutline /></n-icon>
                  <n-text strong>已选择文件夹</n-text>
                </div>
                <div class="selected-details">
                  <n-text class="selected-name">{{ fileState.selectedFile.name }}</n-text>
                  <n-text depth="3" class="selected-path">
                    路径：{{ getSelectedFilePath() }}
                  </n-text>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="modal-actions">
      <n-button
        v-if="currentStep === 2"
        :disabled="bindLoading"
        @click="handleBackToTokenSelection"
      >
        <template #icon
          ><n-icon><ArrowBackOutline /></n-icon
        ></template>
        返回上一步
      </n-button>
      <n-button :disabled="bindLoading" @click="handleCancel">取消</n-button>
      <n-button
        v-if="currentStep === 2"
        type="primary"
        :loading="bindLoading"
        :disabled="!fileState.selectedFile || bindLoading"
        @click="handleConfirm"
      >
        绑定挂载点
      </n-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, h } from 'vue'
import {
  NIcon,
  NText,
  NButton,
  NSpin,
  NEmpty,
  NTag,
  NBreadcrumb,
  NBreadcrumbItem,
  NTree,
  useMessage,
} from 'naive-ui'
import {
  KeyOutline,
  CheckmarkCircleOutline,
  ArrowForwardOutline,
  ArrowBackOutline,
  HomeOutline,
  FolderOutline,
  DocumentOutline,
} from '@vicons/ionicons5'
import { getCloudTokenList } from '@/api/cloudtoken'
import { getPersonFiles } from '@/api/storage/advance'
import type { FileNode, GetPersonFilesQuery } from '@/api/storage/advance'
import { OS_TYPES } from '@/utils/osType'
import { useMountPointBind } from '@/composables/useMountPointBind'

// Emits
interface Emits {
  (e: 'confirm', data: { success: boolean }): void
  (e: 'cancel'): void
}
const emit = defineEmits<Emits>()

// Hooks
const message = useMessage()
const mountPointBind = useMountPointBind()

// 步骤控制
const currentStep = ref(1)

// 令牌状态
const tokenState = reactive({
  loading: false,
  tokens: [] as Models.CloudToken[],
  selectedTokenId: null as number | null,
})

// 文件状态
const fileState = reactive({
  loading: false,
  treeLoading: false,
  files: [] as FileNode[],
  selectedFile: null as FileNode | null,
  currentParentId: '-11', // 根目录ID
  breadcrumbs: [] as Array<{ id: string; name: string }>,
  selectedKeys: [] as string[],
  expandedKeys: [] as string[],
  treeData: new Map<string, FileNode[]>(),
})

const bindLoading = ref(false)
let isComponentMounted = false
let operationVersion = 0
let tokenRequestId = 0
let fileRequestId = 0
const activeFileRequestIds = new Map<string, number>()

const isCurrentOperation = (version: number) => {
  return isComponentMounted && operationVersion === version
}

const invalidatePendingWork = () => {
  operationVersion++
  bindLoading.value = false
  activeFileRequestIds.clear()
}

const isCurrentTokenRequest = (requestId: number, currentOperation: number) => {
  return isCurrentOperation(currentOperation) && tokenRequestId === requestId
}

const isCurrentFileRequest = (
  parentId: string,
  requestId: number,
  currentOperation: number,
  cloudToken: number
) => {
  return (
    isCurrentOperation(currentOperation) &&
    activeFileRequestIds.get(parentId) === requestId &&
    tokenState.selectedTokenId === cloudToken
  )
}

const resetFileState = () => {
  fileState.loading = false
  fileState.treeLoading = false
  fileState.files = []
  fileState.selectedFile = null
  fileState.currentParentId = '-11'
  fileState.breadcrumbs = []
  fileState.selectedKeys = []
  fileState.expandedKeys = []
  fileState.treeData.clear()
}

// 获取令牌列表
const fetchTokenList = () => {
  if (!isComponentMounted) {
    return
  }

  const currentOperation = operationVersion
  const currentRequestId = ++tokenRequestId
  tokenState.loading = true
  getCloudTokenList({ noPaginate: true })
    .then((response) => {
      if (!isCurrentTokenRequest(currentRequestId, currentOperation)) {
        return
      }

      if (response.code === 200 && response.data) {
        tokenState.tokens = response.data.data || []
      } else {
        message.error(response.msg || '获取令牌列表失败')
      }
    })
    .catch((error) => {
      if (!isCurrentTokenRequest(currentRequestId, currentOperation)) {
        return
      }

      console.error('获取令牌列表失败:', error)
      message.error('获取令牌列表失败')
    })
    .finally(() => {
      if (isCurrentTokenRequest(currentRequestId, currentOperation)) {
        tokenState.loading = false
      }
    })
}

// 选择令牌
const handleSelectToken = (tokenId: number) => {
  if (bindLoading.value) {
    return
  }

  if (tokenState.selectedTokenId !== tokenId) {
    invalidatePendingWork()
    resetFileState()
  }
  tokenState.selectedTokenId = tokenId
}

// 下一步到文件选择
const handleNextToFileSelection = () => {
  if (bindLoading.value) {
    return
  }

  if (!tokenState.selectedTokenId) {
    message.warning('请选择令牌')
    return
  }
  currentStep.value = 2
  fetchPersonFiles()
}

// 获取个人文件列表
const fetchPersonFiles = (parentId: string = '-11') => {
  if (!tokenState.selectedTokenId) return

  const currentOperation = operationVersion
  const currentRequestId = ++fileRequestId
  const currentCloudToken = tokenState.selectedTokenId
  activeFileRequestIds.set(parentId, currentRequestId)

  if (parentId === '-11') {
    fileState.loading = true
  } else {
    fileState.treeLoading = true
  }

  const params: GetPersonFilesQuery = {
    pageNum: 1,
    pageSize: 100,
    cloudToken: currentCloudToken,
    parentId: parentId,
  }

  getPersonFiles(params)
    .then((response) => {
      if (!isCurrentFileRequest(parentId, currentRequestId, currentOperation, currentCloudToken)) {
        return
      }

      if (response.code === 200 && response.data) {
        const files = response.data.data || []
        if (parentId === '-11') {
          fileState.files = files
          fileState.currentParentId = parentId
        }
        fileState.treeData.set(parentId, files)
      } else {
        message.error(response.msg || '获取文件列表失败')
      }
    })
    .catch((error) => {
      if (!isCurrentFileRequest(parentId, currentRequestId, currentOperation, currentCloudToken)) {
        return
      }

      console.error('获取文件列表失败:', error)
      message.error('获取文件列表失败')
    })
    .finally(() => {
      if (isCurrentFileRequest(parentId, currentRequestId, currentOperation, currentCloudToken)) {
        if (parentId === '-11') {
          fileState.loading = false
        } else {
          fileState.treeLoading = false
        }
      }
    })
}

// 构建树形数据
const fileTreeData = computed(() => {
  const buildTreeNode = (file: FileNode): Record<string, unknown> => {
    const isFolder = file.isFolder === 1
    const hasChildren = isFolder && fileState.treeData.has(file.id)
    return {
      key: file.id,
      label: file.name,
      isLeaf: !isFolder,
      disabled: !isFolder,
      file: file,
      children: isFolder
        ? hasChildren
          ? fileState.treeData.get(file.id)?.map(buildTreeNode)
          : []
        : undefined,
    }
  }
  return fileState.files.map(buildTreeNode)
})

// 树形组件渲染
const renderTreeLabel = ({ option }: { option: Record<string, unknown> }) =>
  h('span', { class: 'tree-label' }, option.label as string)
const renderTreePrefix = ({ option }: { option: Record<string, unknown> }) => {
  const file = option.file as FileNode
  const isFolder = file.isFolder === 1
  return h(
    NIcon,
    { size: 18, color: isFolder ? '#ff9800' : '#2196f3' },
    { default: () => h(isFolder ? FolderOutline : DocumentOutline) }
  )
}
const renderTreeSuffix = ({ option }: { option: Record<string, unknown> }) => {
  const isSelected = fileState.selectedKeys.includes(option.key as string)
  const file = option.file as FileNode
  const isFolder = file.isFolder === 1
  if (isSelected && isFolder) {
    return h(NIcon, { size: 16, color: '#18a058' }, { default: () => h(CheckmarkCircleOutline) })
  }
  return null
}

// 树形选择处理
const handleTreeSelect = (keys: string[]) => {
  if (bindLoading.value) {
    return
  }

  fileState.selectedKeys = keys
  if (keys.length > 0) {
    const selectedKey = keys[0]
    const findFileById = (files: FileNode[], id: string): FileNode | null => {
      for (const file of files) {
        if (file.id === id) return file
        const childFiles = fileState.treeData.get(file.id)
        if (childFiles) {
          const found = findFileById(childFiles, id)
          if (found) return found
        }
      }
      return null
    }
    let selectedFile: FileNode | null = null
    for (const [, files] of fileState.treeData) {
      selectedFile = findFileById(files, selectedKey)
      if (selectedFile) break
    }
    if (!selectedFile) {
      selectedFile = findFileById(fileState.files, selectedKey)
    }
    if (selectedFile && selectedFile.isFolder === 1) {
      fileState.selectedFile = selectedFile
    }
  } else {
    fileState.selectedFile = null
  }
}

// 树形展开处理
const handleTreeExpand = (keys: string[]) => {
  if (bindLoading.value) {
    return
  }

  const newExpandedKeys = keys.filter((key) => !fileState.expandedKeys.includes(key))
  fileState.expandedKeys = keys
  newExpandedKeys.forEach((key) => {
    if (!fileState.treeData.has(key)) {
      fetchPersonFiles(key)
    }
  })
}

// 导航处理
const handleNavigateToRoot = () => {
  if (bindLoading.value) {
    return
  }

  fileState.breadcrumbs = []
  fileState.selectedFile = null
  fetchPersonFiles('-11')
}
const handleNavigateToBreadcrumb = (index: number) => {
  if (bindLoading.value) {
    return
  }

  const targetBreadcrumb = fileState.breadcrumbs[index]
  fileState.breadcrumbs = fileState.breadcrumbs.slice(0, index + 1)
  fileState.selectedFile = null
  fetchPersonFiles(targetBreadcrumb.id)
}

// 获取选中文件路径
const getSelectedFilePath = () => {
  if (!fileState.selectedFile) return ''
  const pathParts = [
    '根目录',
    ...fileState.breadcrumbs.map((b) => b.name),
    fileState.selectedFile.name,
  ]
  return pathParts.join(' / ')
}

// 返回上一步
const handleBackToTokenSelection = () => {
  if (bindLoading.value) {
    return
  }

  currentStep.value = 1
}

// 取消
const handleCancel = () => {
  if (bindLoading.value) {
    return
  }

  invalidatePendingWork()
  emit('cancel')
}

// 确认挂载
const handleConfirm = () => {
  if (bindLoading.value) {
    return
  }

  if (!fileState.selectedFile || !tokenState.selectedTokenId) {
    message.warning('请选择文件夹和令牌')
    return
  }
  const itemsToMount = [
    {
      name: fileState.selectedFile.name,
      osType: OS_TYPES.PERSON_FOLDER,
      cloudToken: tokenState.selectedTokenId,
      disableSwitchCloudToken: true,
      fileId: fileState.selectedFile.id,
    },
  ]
  const currentOperation = operationVersion
  bindLoading.value = true
  mountPointBind
    .show(itemsToMount, { defaultCloudToken: tokenState.selectedTokenId || undefined })
    .then((payload) => {
      if (!isCurrentOperation(currentOperation)) {
        return
      }

      if (payload && payload.length > 0) {
        handleMountBindSuccess()
      }
    })
    .finally(() => {
      if (isCurrentOperation(currentOperation)) {
        bindLoading.value = false
      }
    })
}

// 挂载成功回调
const handleMountBindSuccess = () => {
  emit('confirm', { success: true })
}
onMounted(() => {
  isComponentMounted = true
  fetchTokenList()
})

onUnmounted(() => {
  isComponentMounted = false
  invalidatePendingWork()
})
</script>

<style scoped>
.person-mount-container {
  min-width: 800px;
  width: 100%;
}

.person-mount-content {
  padding: 16px 0;
}

.step-content {
  min-height: 400px;
}

.step-header {
  margin-bottom: 24px;
  text-align: center;
}

.step-header .n-text:first-child {
  display: block;
  margin-bottom: 8px;
  font-size: 18px;
}

.loading-container,
.empty-state {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 200px;
}

.token-section {
  max-width: 600px;
  margin: 0 auto;
}

.token-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 24px;
}

.token-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.3s ease;
  background: var(--n-card-color);
}

.token-card:hover {
  border-color: var(--n-primary-color);
  background: var(--n-color-target);
}

.token-card.active {
  border-color: var(--n-primary-color);
  background: var(--n-primary-color-suppl);
}

.token-card.disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.token-info {
  flex: 1;
}

.token-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.token-name {
  font-size: 16px;
}

.token-id-tag {
  font-size: 11px;
}

.token-username {
  font-size: 13px;
}

.step-actions {
  margin-top: 24px;
}

.file-section {
  max-width: 800px;
  margin: 0 auto;
}

.file-tree-container {
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  background: var(--n-card-color);
  overflow: hidden;
}

.breadcrumb-container {
  padding: 12px 16px;
  border-bottom: 1px solid var(--n-border-color);
  background: var(--n-color-target);
}

.file-tree {
  max-height: 400px;
  overflow-y: auto;
}

.selected-info {
  margin-top: 16px;
}

.selected-card {
  padding: 16px;
  background: var(--n-success-color-suppl);
  border: 1px solid var(--n-success-color);
  border-radius: 8px;
}

.selected-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.selected-name {
  display: block;
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 4px;
}

.selected-path {
  font-size: 13px;
}

.modal-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  padding-top: 16px;
  border-top: 1px solid var(--n-border-color);
}

@media (width <= 768px) {
  .token-card {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .token-actions {
    align-self: flex-end;
  }

  .modal-actions {
    flex-direction: column;
  }
}
</style>
