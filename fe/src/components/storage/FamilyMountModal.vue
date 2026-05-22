<template>
  <div class="family-mount-container">
    <div class="family-mount-content">
      <!-- 第一步：选择令牌 -->
      <div v-if="currentStep === 1" class="step-content">
        <div class="step-header">
          <n-text strong>选择云盘令牌</n-text>
          <n-text depth="3">请选择要使用的天翼云盘令牌</n-text>
        </div>

        <div class="token-section">
          <div v-if="tokenState.loading" class="loading-container">
            <n-spin size="large">
              <template #description><n-text depth="2">正在加载令牌列表...</n-text></template>
            </n-spin>
          </div>

          <div v-else-if="tokenState.tokens.length === 0" class="empty-state">
            <n-empty description="暂无可用令牌" size="large">
              <template #icon
                ><n-icon size="64" :depth="3"><KeyOutline /></n-icon
              ></template>
              <template #extra><n-text depth="3">请先添加天翼云盘令牌</n-text></template>
            </n-empty>
          </div>

          <div v-else class="token-list">
            <div
              v-for="token in tokenState.tokens"
              :key="token.id"
              class="token-card"
              :class="{ active: tokenState.selectedTokenId === token.id }"
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
              :disabled="!tokenState.selectedTokenId"
              @click="handleNextToFamilySelection"
              style="width: 100%"
            >
              <template #icon
                ><n-icon><ArrowForwardOutline /></n-icon
              ></template>
              下一步：选择家庭
            </n-button>
          </div>
        </div>
      </div>

      <!-- 第二步：选择家庭 -->
      <div v-if="currentStep === 2" class="step-content">
        <div class="step-header">
          <n-text strong>选择家庭云</n-text>
          <n-text depth="3">请选择要访问的家庭云</n-text>
        </div>

        <div class="family-section">
          <div v-if="familyState.loading" class="loading-container">
            <n-spin size="large">
              <template #description><n-text depth="2">正在加载家庭列表...</n-text></template>
            </n-spin>
          </div>

          <div v-else-if="familyState.families.length === 0" class="empty-state">
            <n-empty description="暂无可用家庭云" size="large">
              <template #icon
                ><n-icon size="64" :depth="3"><PeopleOutline /></n-icon
              ></template>
              <template #extra><n-text depth="3">当前账号未加入任何家庭云</n-text></template>
            </n-empty>
          </div>

          <div v-else class="family-list">
            <div
              v-for="family in familyState.families"
              :key="family.familyId"
              class="family-card"
              :class="{ active: familyState.selectedFamilyId === family.familyId }"
              @click="handleSelectFamily(family.familyId)"
            >
              <div class="family-info">
                <div class="family-header">
                  <n-text strong class="family-name">{{
                    family.remarkName || '未命名家庭'
                  }}</n-text>
                  <n-tag size="small" type="success" class="family-role-tag">{{
                    getFamilyRoleText(family.userRole)
                  }}</n-tag>
                </div>
                <div class="family-details">
                  <n-text depth="3" class="family-id">家庭ID: {{ family.familyId }}</n-text>
                  <n-text depth="3" class="family-count">成员数: {{ family.count }}</n-text>
                </div>
                <div class="family-time">
                  <n-text depth="3" class="family-create-time"
                    >创建时间: {{ formatTime(family.createTime) }}</n-text
                  >
                  <n-text depth="3" class="family-expire-time"
                    >到期时间: {{ formatTime(family.expireTime) }}</n-text
                  >
                </div>
              </div>
              <div class="family-actions">
                <n-icon
                  v-if="familyState.selectedFamilyId === family.familyId"
                  :size="20"
                  color="#18a058"
                >
                  <CheckmarkCircleOutline />
                </n-icon>
              </div>
            </div>
          </div>

          <div v-if="familyState.families.length > 0" class="step-actions">
            <n-button
              type="primary"
              size="large"
              :disabled="!familyState.selectedFamilyId"
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

      <!-- 第三步：选择文件夹 -->
      <div v-if="currentStep === 3" class="step-content">
        <div class="step-header">
          <n-text strong>选择文件夹</n-text>
          <n-text depth="3">请选择要挂载的家庭文件夹（只能选择文件夹类型）</n-text>
        </div>

        <div class="file-section">
          <div v-if="fileState.loading" class="loading-container">
            <n-spin size="large">
              <template #description><n-text depth="2">正在加载文件列表...</n-text></template>
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
                  <n-text depth="3" class="selected-path">路径：{{ getSelectedFilePath() }}</n-text>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="modal-actions">
      <n-button v-if="currentStep === 2" @click="handleBackToTokenSelection">
        <template #icon
          ><n-icon><ArrowBackOutline /></n-icon
        ></template>
        返回上一步
      </n-button>
      <n-button v-if="currentStep === 3" @click="handleBackToFamilySelection">
        <template #icon
          ><n-icon><ArrowBackOutline /></n-icon
        ></template>
        返回上一步
      </n-button>
      <n-button :disabled="bindLoading" @click="handleCancel">取消</n-button>
      <n-button
        v-if="currentStep === 3"
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
  PeopleOutline,
  KeyOutline,
  CheckmarkCircleOutline,
  ArrowForwardOutline,
  ArrowBackOutline,
  HomeOutline,
  FolderOutline,
  DocumentOutline,
} from '@vicons/ionicons5'
import { getCloudTokenList } from '@/api/cloudtoken'
import { getFamilyList, getFamilyFiles } from '@/api/storage/advance'
import type {
  FileNode,
  GetFamilyFilesQuery,
  FamilyInfo,
  GetFamilyListQuery,
} from '@/api/storage/advance'
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

// 家庭状态
const familyState = reactive({
  loading: false,
  families: [] as FamilyInfo[],
  selectedFamilyId: null as string | null,
})

// 文件状态
const fileState = reactive({
  loading: false,
  treeLoading: false,
  files: [] as FileNode[],
  selectedFile: null as FileNode | null,
  currentParentId: '', // 家庭云根目录ID为空字符串
  breadcrumbs: [] as Array<{ id: string; name: string }>,
  selectedKeys: [] as string[],
  expandedKeys: [] as string[],
  treeData: new Map<string, FileNode[]>(),
})

const bindLoading = ref(false)
let isComponentMounted = false
let operationVersion = 0
let tokenRequestId = 0
let familyRequestId = 0
let fileRequestId = 0
const activeFileRequestIds = new Map<string, number>()

const isCurrentOperation = (version: number) => {
  return isComponentMounted && operationVersion === version
}

const invalidatePendingWork = () => {
  operationVersion++
  activeFileRequestIds.clear()
  bindLoading.value = false
}

const isCurrentTokenRequest = (requestId: number, currentOperation: number) => {
  return isCurrentOperation(currentOperation) && tokenRequestId === requestId
}

const isCurrentFamilyRequest = (
  requestId: number,
  currentOperation: number,
  cloudToken: number
) => {
  return (
    isCurrentOperation(currentOperation) &&
    familyRequestId === requestId &&
    tokenState.selectedTokenId === cloudToken
  )
}

const isCurrentFileRequest = (
  parentId: string,
  requestId: number,
  currentOperation: number,
  cloudToken: number,
  familyId: string
) => {
  return (
    isCurrentOperation(currentOperation) &&
    activeFileRequestIds.get(parentId) === requestId &&
    tokenState.selectedTokenId === cloudToken &&
    familyState.selectedFamilyId === familyId
  )
}

const resetFamilyState = () => {
  familyState.loading = false
  familyState.families = []
  familyState.selectedFamilyId = null
}

const resetFileState = () => {
  fileState.loading = false
  fileState.treeLoading = false
  fileState.files = []
  fileState.selectedFile = null
  fileState.currentParentId = ''
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
  if (tokenState.selectedTokenId !== tokenId) {
    invalidatePendingWork()
    resetFamilyState()
    resetFileState()
  }
  tokenState.selectedTokenId = tokenId
}

// 下一步到家庭选择
const handleNextToFamilySelection = () => {
  if (!tokenState.selectedTokenId) {
    message.warning('请选择令牌')
    return
  }
  currentStep.value = 2
  fetchFamilyList()
}

// 获取家庭列表
const fetchFamilyList = () => {
  if (!tokenState.selectedTokenId) return
  const currentOperation = operationVersion
  const currentRequestId = ++familyRequestId
  const currentCloudToken = tokenState.selectedTokenId
  familyState.loading = true
  const params: GetFamilyListQuery = { cloudToken: currentCloudToken }
  getFamilyList(params)
    .then((response) => {
      if (!isCurrentFamilyRequest(currentRequestId, currentOperation, currentCloudToken)) {
        return
      }

      if (response.code === 200 && response.data) {
        familyState.families = response.data.familyInfoResp || []
      } else {
        message.error(response.msg || '获取家庭列表失败')
      }
    })
    .catch((error) => {
      if (!isCurrentFamilyRequest(currentRequestId, currentOperation, currentCloudToken)) {
        return
      }

      console.error('获取家庭列表失败:', error)
      message.error('获取家庭列表失败')
    })
    .finally(() => {
      if (isCurrentFamilyRequest(currentRequestId, currentOperation, currentCloudToken)) {
        familyState.loading = false
      }
    })
}

// 选择家庭
const handleSelectFamily = (familyId: string) => {
  if (familyState.selectedFamilyId !== familyId) {
    invalidatePendingWork()
    resetFileState()
  }
  familyState.selectedFamilyId = familyId
}

// 获取家庭角色文本
const getFamilyRoleText = (userRole: number) => {
  return userRole === 0 ? '管理员' : userRole === 1 ? '成员' : '未知'
}

// 格式化时间
const formatTime = (timeStr: string) => {
  if (!timeStr) return '未知'
  try {
    return new Date(timeStr).toLocaleString('zh-CN')
  } catch {
    return timeStr
  }
}

// 下一步到文件选择
const handleNextToFileSelection = () => {
  if (!familyState.selectedFamilyId) {
    message.warning('请选择家庭')
    return
  }
  currentStep.value = 3
  fetchFamilyFiles()
}

// 获取家庭文件列表
const fetchFamilyFiles = (parentId: string = '') => {
  if (!tokenState.selectedTokenId || !familyState.selectedFamilyId) return
  const currentOperation = operationVersion
  const currentRequestId = ++fileRequestId
  const currentCloudToken = tokenState.selectedTokenId
  const currentFamilyId = familyState.selectedFamilyId
  activeFileRequestIds.set(parentId, currentRequestId)

  if (parentId === '') {
    fileState.loading = true
  } else {
    fileState.treeLoading = true
  }
  const params: GetFamilyFilesQuery = {
    pageNum: 1,
    pageSize: 100,
    cloudToken: currentCloudToken,
    familyId: currentFamilyId,
    parentId: parentId,
  }
  getFamilyFiles(params)
    .then((response) => {
      if (
        !isCurrentFileRequest(
          parentId,
          currentRequestId,
          currentOperation,
          currentCloudToken,
          currentFamilyId
        )
      ) {
        return
      }

      if (response.code === 200 && response.data) {
        const files = response.data.data || []
        if (parentId === '') {
          fileState.files = files
          fileState.currentParentId = parentId
        }
        fileState.treeData.set(parentId, files)
      } else {
        message.error(response.msg || '获取家庭文件列表失败')
      }
    })
    .catch((error) => {
      if (
        !isCurrentFileRequest(
          parentId,
          currentRequestId,
          currentOperation,
          currentCloudToken,
          currentFamilyId
        )
      ) {
        return
      }

      console.error('获取家庭文件列表失败:', error)
      message.error('获取家庭文件列表失败')
    })
    .finally(() => {
      if (
        isCurrentFileRequest(
          parentId,
          currentRequestId,
          currentOperation,
          currentCloudToken,
          currentFamilyId
        )
      ) {
        if (parentId === '') {
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
  const newExpandedKeys = keys.filter((key) => !fileState.expandedKeys.includes(key))
  fileState.expandedKeys = keys
  newExpandedKeys.forEach((key) => {
    if (!fileState.treeData.has(key)) {
      fetchFamilyFiles(key)
    }
  })
}

// 导航处理
const handleNavigateToRoot = () => {
  fileState.breadcrumbs = []
  fileState.selectedFile = null
  fetchFamilyFiles('')
}
const handleNavigateToBreadcrumb = (index: number) => {
  const targetBreadcrumb = fileState.breadcrumbs[index]
  fileState.breadcrumbs = fileState.breadcrumbs.slice(0, index + 1)
  fileState.selectedFile = null
  fetchFamilyFiles(targetBreadcrumb.id)
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
  currentStep.value = 1
}
const handleBackToFamilySelection = () => {
  currentStep.value = 2
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
      osType: OS_TYPES.FAMILY_FOLDER,
      cloudToken: tokenState.selectedTokenId,
      disableSwitchCloudToken: true,
      fileId: fileState.selectedFile.id,
      familyId: familyState.selectedFamilyId!,
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
.family-mount-container {
  min-width: 800px;
  width: 100%;
}

.family-mount-content {
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

.token-section,
.family-section {
  max-width: 700px;
  margin: 0 auto;
}

.token-list,
.family-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 24px;
}

.token-card,
.family-card {
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

.token-card:hover,
.family-card:hover {
  border-color: var(--n-primary-color);
  background: var(--n-color-target);
}

.token-card.active,
.family-card.active {
  border-color: var(--n-primary-color);
  background: var(--n-primary-color-suppl);
}

.token-info,
.family-info {
  flex: 1;
}

.token-header,
.family-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.token-name,
.family-name {
  font-size: 16px;
}

.token-id-tag,
.family-role-tag {
  font-size: 11px;
}

.token-username {
  font-size: 13px;
}

.family-details {
  display: flex;
  gap: 16px;
  margin-bottom: 4px;
}

.family-id,
.family-count {
  font-size: 12px;
}

.family-time {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.family-create-time,
.family-expire-time {
  font-size: 11px;
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
  .token-card,
  .family-card {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .token-actions,
  .family-actions {
    align-self: flex-end;
  }

  .family-details {
    flex-direction: column;
    gap: 4px;
  }

  .modal-actions {
    flex-direction: column;
  }
}
</style>
