<template>
  <n-modal
    v-model:show="visible"
    preset="dialog"
    title="绑定存储"
    style="width: 900px"
    :closable="!submitting"
    :mask-closable="!submitting"
    :close-on-esc="!submitting"
  >
    <div class="bind-files-modal">
      <div class="search-section">
        <n-space justify="space-between" style="width: 100%">
          <n-space>
            <n-input
              v-model:value="searchKeyword"
              placeholder="搜索存储挂载点..."
              clearable
              :disabled="submitting"
              @keyup.enter="handleSearch"
            >
              <template #prefix>
                <n-icon :component="SearchOutline" />
              </template>
            </n-input>
            <n-button type="primary" :disabled="submitting" @click="handleSearch">搜索</n-button>
          </n-space>

          <n-space>
            <n-button @click="selectCurrentPage" :disabled="storageList.length === 0 || submitting">
              全选当前页
            </n-button>
            <n-button
              type="info"
              ghost
              @click="selectAllSearchResults"
              :loading="selectingAll"
              :disabled="submitting"
            >
              全选搜索结果
            </n-button>
          </n-space>
        </n-space>
      </div>

      <div class="file-list-section">
        <n-data-table
          :columns="columns"
          :data="storageList"
          :loading="loading"
          :disabled="submitting"
          :row-key="(row) => row.id"
          :checked-row-keys="selectedStorageIds"
          @update:checked-row-keys="handleSelectionChange"
        />
      </div>

      <div class="pagination-section">
        <n-pagination
          v-model:page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :item-count="pagination.itemCount"
          :page-sizes="pagination.pageSizes"
          show-size-picker
          :disabled="submitting"
          @update:page="handlePageChange"
          @update:page-size="handlePageSizeChange"
        >
          <template #prefix>共 {{ pagination.itemCount }} 条</template>
        </n-pagination>
      </div>

      <div v-if="selectedStorageIds.length > 0" class="selected-section">
        <n-divider style="margin: 12px 0 8px" />
        <div class="selected-header">
          <span>已选择 {{ selectedStorageIds.length }} 个存储挂载点</span>
          <n-button text type="error" :disabled="submitting" @click="clearSelection"
            >清空选择</n-button
          >
        </div>
        <div class="selected-files">
          <n-tag
            v-for="storageId in displayedStorageIds"
            :key="storageId"
            size="small"
            closable
            :disabled="submitting"
            :title="getFullStorageName(storageId)"
            @close="removeSelection(storageId)"
          >
            {{ getStorageName(storageId) }}
          </n-tag>
          <n-tag v-if="selectedStorageIds.length > 10" size="small" type="info">
            +{{ selectedStorageIds.length - 10 }} 更多...
          </n-tag>
        </div>
      </div>
    </div>

    <template #action>
      <n-space>
        <n-button :disabled="submitting" @click="handleCancel">取消</n-button>
        <n-button
          :type="selectedStorageIds.length === 0 ? 'warning' : 'primary'"
          :loading="submitting || loadingBindFiles"
          :disabled="submitting || loadingBindFiles || !bindFilesLoaded"
          @click="handleConfirm"
        >
          {{ selectedStorageIds.length === 0 ? '清空绑定' : '确定绑定' }}
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import {
  NButton,
  NDataTable,
  NDivider,
  NIcon,
  NInput,
  NModal,
  NPagination,
  NSpace,
  NTag,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { SearchOutline } from '@vicons/ionicons5'
import { getStorageSelectList, type StorageSelectItem } from '@/api/storage'
import { batchBindFiles, getBindFiles } from '@/api/usergroup'

interface Props {
  show: boolean
  userGroupInfo?: Models.UserGroup | null
}

interface Emits {
  (e: 'update:show', value: boolean): void
  (e: 'success'): void
}

const props = withDefaults(defineProps<Props>(), {
  show: false,
  userGroupInfo: null,
})

const emit = defineEmits<Emits>()
const message = useMessage()

const visible = computed({
  get: () => props.show,
  set: (value) => {
    if (!value && submitting.value) {
      return
    }

    emit('update:show', value)
  },
})

const searchKeyword = ref('')
const loading = ref(false)
const loadingBindFiles = ref(false)
const bindFilesLoaded = ref(false)
const submitting = ref(false)
const selectingAll = ref(false)
const storageList = ref<StorageSelectItem[]>([])
const selectedStorageIds = ref<number[]>([])
const selectedStorageMap = ref<Record<number, StorageSelectItem>>({})
const operationVersion = ref(0)

const pagination = reactive({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  pageSizes: [10, 20, 50, 100],
})

const displayedStorageIds = computed(() => selectedStorageIds.value.slice(0, 10))

const columns: DataTableColumns<StorageSelectItem> = [
  {
    type: 'selection',
  },
  {
    title: '存储名称',
    key: 'name',
    ellipsis: {
      tooltip: true,
    },
  },
  {
    title: '存储路径',
    key: 'path',
    ellipsis: {
      tooltip: true,
    },
  },
]

const invalidateOperation = () => {
  operationVersion.value += 1
}

const getCurrentUserGroupId = () => props.userGroupInfo?.id ?? null

const isCurrentOperation = (version: number, userGroupId: number | null) =>
  visible.value && operationVersion.value === version && getCurrentUserGroupId() === userGroupId

watch(
  () => [props.show, props.userGroupInfo?.id ?? null] as const,
  ([newShow, userGroupId], [oldShow, oldUserGroupId]) => {
    if (newShow !== oldShow || userGroupId !== oldUserGroupId) {
      invalidateOperation()
    }

    if (!newShow) {
      loading.value = false
      loadingBindFiles.value = false
      bindFilesLoaded.value = false
      submitting.value = false
      selectingAll.value = false
      return
    }

    searchKeyword.value = ''
    storageList.value = []
    selectedStorageIds.value = []
    selectedStorageMap.value = {}
    pagination.page = 1
    pagination.pageSize = 10
    pagination.itemCount = 0
    loading.value = false
    loadingBindFiles.value = false
    bindFilesLoaded.value = false
    submitting.value = false
    selectingAll.value = false

    const version = operationVersion.value

    nextTick(() => {
      Promise.all([fetchStorageList(version, userGroupId), loadBindFiles(version, userGroupId)])
    })
  }
)

const buildParams = (noPaginate = false) => ({
  currentPage: pagination.page,
  pageSize: pagination.pageSize,
  noPaginate,
  name: searchKeyword.value || undefined,
  path: searchKeyword.value || undefined,
})

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return value !== null && typeof value === 'object'
}

const isPositiveInteger = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isSafeInteger(value) && value > 0
}

const isNonNegativeInteger = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

const normalizePositiveIds = (value: unknown): number[] | null => {
  if (!Array.isArray(value)) {
    return null
  }

  const ids: number[] = []
  const seen = new Set<number>()

  for (const item of value) {
    if (!isPositiveInteger(item)) {
      return null
    }

    if (seen.has(item)) {
      continue
    }

    seen.add(item)
    ids.push(item)
  }

  return ids
}

const isStorageSelectItem = (value: unknown): value is StorageSelectItem => {
  if (!isRecord(value)) {
    return false
  }

  return (
    isPositiveInteger(value.id) &&
    typeof value.name === 'string' &&
    value.name.trim().length > 0 &&
    typeof value.path === 'string'
  )
}

const normalizeStorageSelectItems = (value: unknown): StorageSelectItem[] | null => {
  if (!Array.isArray(value)) {
    return null
  }

  const items: StorageSelectItem[] = []
  const seen = new Set<number>()

  for (const item of value) {
    if (!isStorageSelectItem(item)) {
      return null
    }

    if (seen.has(item.id)) {
      continue
    }

    seen.add(item.id)
    items.push(item)
  }

  return items
}

const normalizeStoragePage = (
  value: unknown
): Models.PaginationResponse<StorageSelectItem> | null => {
  if (!isRecord(value) || !isNonNegativeInteger(value.total)) {
    return null
  }

  const items = normalizeStorageSelectItems(value.data)
  if (!items) {
    return null
  }

  return {
    currentPage: isPositiveInteger(value.currentPage) ? value.currentPage : pagination.page,
    pageSize: isPositiveInteger(value.pageSize) ? value.pageSize : pagination.pageSize,
    total: value.total,
    data: items,
  }
}

const mergeStorageMeta = (items: StorageSelectItem[]) => {
  const nextMap = { ...selectedStorageMap.value }
  items.forEach((item) => {
    nextMap[item.id] = item
  })
  selectedStorageMap.value = nextMap
}

const loadBindFiles = async (
  version = operationVersion.value,
  userGroupId = getCurrentUserGroupId()
) => {
  if (userGroupId == null || !isCurrentOperation(version, userGroupId)) return

  loadingBindFiles.value = true
  bindFilesLoaded.value = false

  try {
    const response = await getBindFiles(userGroupId)
    if (!isCurrentOperation(version, userGroupId)) return

    if (response.code === 200 && response.data) {
      const fileIds = normalizePositiveIds(response.data.fileIds)
      if (!fileIds) {
        message.error('已绑定存储响应格式异常')

        return
      }

      selectedStorageIds.value = fileIds
      bindFilesLoaded.value = true
    } else {
      message.error(response.msg || '获取已绑定存储失败')
    }
  } catch (error) {
    if (!isCurrentOperation(version, userGroupId)) return

    console.error('获取已绑定文件失败:', error)
    message.error('获取已绑定存储失败')
  } finally {
    if (isCurrentOperation(version, userGroupId)) {
      loadingBindFiles.value = false
    }
  }
}

const fetchStorageList = async (
  version = operationVersion.value,
  userGroupId = getCurrentUserGroupId()
) => {
  if (!isCurrentOperation(version, userGroupId)) return

  loading.value = true

  try {
    const response = await getStorageSelectList(buildParams())
    if (!isCurrentOperation(version, userGroupId)) return

    if (response.code === 200 && response.data) {
      const page = normalizeStoragePage(response.data)
      if (!page) {
        message.error('存储列表响应格式异常')

        return
      }

      storageList.value = page.data
      pagination.itemCount = page.total
      mergeStorageMeta(storageList.value)
    } else {
      message.error(response.msg || '获取存储列表失败')
    }
  } catch (error) {
    if (!isCurrentOperation(version, userGroupId)) return

    console.error('获取存储列表失败:', error)
    message.error('获取存储列表失败')
  } finally {
    if (isCurrentOperation(version, userGroupId)) {
      loading.value = false
    }
  }
}

const handleSearch = async () => {
  if (submitting.value) return

  pagination.page = 1
  await fetchStorageList()
}

const handlePageChange = async (page: number) => {
  if (submitting.value) return

  pagination.page = page
  await fetchStorageList()
}

const handlePageSizeChange = async (pageSize: number) => {
  if (submitting.value) return

  pagination.pageSize = pageSize
  pagination.page = 1
  await fetchStorageList()
}

const handleSelectionChange = (keys: Array<string | number>) => {
  if (submitting.value) return

  const pageIDs = new Set(storageList.value.map((item) => item.id))
  const pageSelectedIDs = normalizePositiveIds(keys.map((key) => Number(key)))
  if (!pageSelectedIDs) {
    return
  }

  const reservedIDs = selectedStorageIds.value.filter((id) => !pageIDs.has(id))

  selectedStorageIds.value = [...reservedIDs, ...pageSelectedIDs]

  storageList.value.forEach((item) => {
    if (pageSelectedIDs.includes(item.id)) {
      selectedStorageMap.value[item.id] = item
    }
  })
}

const selectCurrentPage = () => {
  if (submitting.value) return

  const next = new Set(selectedStorageIds.value)
  storageList.value.forEach((item) => {
    next.add(item.id)
    selectedStorageMap.value[item.id] = item
  })
  selectedStorageIds.value = Array.from(next)
}

const selectAllSearchResults = async () => {
  if (submitting.value) return

  const version = operationVersion.value
  const userGroupId = getCurrentUserGroupId()
  if (!isCurrentOperation(version, userGroupId)) return

  selectingAll.value = true

  try {
    const response = await getStorageSelectList({
      ...buildParams(true),
      currentPage: 1,
    })
    if (!isCurrentOperation(version, userGroupId)) return

    if (response.code !== 200 || !response.data) {
      message.error(response.msg || '获取全部存储失败')
      return
    }

    const page = normalizeStoragePage(response.data)
    if (!page) {
      message.error('存储列表响应格式异常')

      return
    }

    const items = page.data
    mergeStorageMeta(items)
    selectedStorageIds.value = items.map((item) => item.id)
    message.success(`已选择 ${selectedStorageIds.value.length} 个存储挂载点`)
  } catch (error) {
    if (!isCurrentOperation(version, userGroupId)) return

    console.error('获取全部存储失败:', error)
    message.error('获取全部存储失败')
  } finally {
    if (isCurrentOperation(version, userGroupId)) {
      selectingAll.value = false
    }
  }
}

const clearSelection = () => {
  if (submitting.value) return

  selectedStorageIds.value = []
}

const removeSelection = (storageId: number) => {
  if (submitting.value) return

  selectedStorageIds.value = selectedStorageIds.value.filter((id) => id !== storageId)
}

const getStorageName = (storageId: number) => {
  const storage = selectedStorageMap.value[storageId]
  const name = storage ? storage.name : `存储ID: ${storageId}`
  return name.length > 20 ? `${name.slice(0, 20)}...` : name
}

const getFullStorageName = (storageId: number) => {
  const storage = selectedStorageMap.value[storageId]
  return storage ? `${storage.name} (${storage.path})` : `存储ID: ${storageId}`
}

const handleCancel = () => {
  if (submitting.value) return

  invalidateOperation()
  loading.value = false
  submitting.value = false
  selectingAll.value = false
  visible.value = false
}

const handleConfirm = async () => {
  if (submitting.value) return

  const version = operationVersion.value
  const userGroupId = getCurrentUserGroupId()
  const fileIds = [...selectedStorageIds.value]

  if (
    userGroupId == null ||
    loadingBindFiles.value ||
    !bindFilesLoaded.value ||
    !isCurrentOperation(version, userGroupId)
  ) {
    return
  }

  submitting.value = true

  try {
    const response = await batchBindFiles({
      groupId: userGroupId,
      fileIds,
    })
    if (!isCurrentOperation(version, userGroupId)) return

    if (response.code === 200) {
      message.success(fileIds.length === 0 ? '存储绑定已清空' : '存储绑定成功')
      invalidateOperation()
      visible.value = false
      emit('success')
    } else {
      message.error(response.msg || '存储绑定失败')
    }
  } catch (error) {
    if (!isCurrentOperation(version, userGroupId)) return

    console.error('存储绑定失败:', error)
    message.error('存储绑定失败')
  } finally {
    if (isCurrentOperation(version, userGroupId)) {
      submitting.value = false
    }
  }
}
</script>

<style scoped>
.bind-files-modal {
  max-height: 680px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.search-section {
  margin-bottom: 16px;
  flex-shrink: 0;
}

.file-list-section {
  flex: 1;
  min-height: 300px;
  max-height: 420px;
  overflow: auto;
}

.pagination-section {
  display: flex;
  justify-content: center;
  margin-top: 16px;
  flex-shrink: 0;
}

.selected-section {
  margin-top: 16px;
  flex-shrink: 0;
  max-height: 150px;
  overflow: hidden;
}

.selected-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  font-weight: 500;
}

.selected-files {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  max-height: 120px;
  overflow-y: auto;
  padding: 4px;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  background-color: var(--n-color-target);
}

.file-list-section::-webkit-scrollbar,
.selected-files::-webkit-scrollbar {
  width: 6px;
}

.file-list-section::-webkit-scrollbar-track,
.selected-files::-webkit-scrollbar-track {
  background: var(--n-scrollbar-color);
  border-radius: 3px;
}

.file-list-section::-webkit-scrollbar-thumb,
.selected-files::-webkit-scrollbar-thumb {
  background: var(--n-scrollbar-color-hover);
  border-radius: 3px;
}

.file-list-section::-webkit-scrollbar-thumb:hover,
.selected-files::-webkit-scrollbar-thumb:hover {
  background: var(--n-scrollbar-color-pressed);
}
</style>
