<template>
  <n-modal v-model:show="visible" preset="dialog" title="绑定存储" style="width: 900px">
    <div class="bind-files-modal">
      <div class="search-section">
        <n-space justify="space-between" style="width: 100%">
          <n-space>
            <n-input
              v-model:value="searchKeyword"
              placeholder="搜索存储挂载点..."
              clearable
              @keyup.enter="handleSearch"
            >
              <template #prefix>
                <n-icon :component="SearchOutline" />
              </template>
            </n-input>
            <n-button type="primary" @click="handleSearch">搜索</n-button>
          </n-space>

          <n-space>
            <n-button @click="selectCurrentPage" :disabled="storageList.length === 0">全选当前页</n-button>
            <n-button type="info" ghost @click="selectAllSearchResults" :loading="selectingAll">
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
          <n-button text type="error" @click="clearSelection">清空选择</n-button>
        </div>
        <div class="selected-files">
          <n-tag
            v-for="storageId in displayedStorageIds"
            :key="storageId"
            size="small"
            closable
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
        <n-button @click="handleCancel">取消</n-button>
        <n-button
          type="primary"
          :loading="submitting"
          :disabled="selectedStorageIds.length === 0"
          @click="handleConfirm"
        >
          确定绑定
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
  set: (value) => emit('update:show', value),
})

const searchKeyword = ref('')
const loading = ref(false)
const submitting = ref(false)
const selectingAll = ref(false)
const storageList = ref<StorageSelectItem[]>([])
const selectedStorageIds = ref<number[]>([])
const selectedStorageMap = ref<Record<number, StorageSelectItem>>({})

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

watch(
  () => props.show,
  (newShow) => {
    if (!newShow) {
      return
    }

    searchKeyword.value = ''
    storageList.value = []
    selectedStorageIds.value = []
    selectedStorageMap.value = {}
    pagination.page = 1
    pagination.pageSize = 10
    pagination.itemCount = 0

    nextTick(() => {
      Promise.all([fetchStorageList(), loadBindFiles()])
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

const mergeStorageMeta = (items: StorageSelectItem[]) => {
  const nextMap = { ...selectedStorageMap.value }
  items.forEach((item) => {
    nextMap[item.id] = item
  })
  selectedStorageMap.value = nextMap
}

const loadBindFiles = async () => {
  if (!props.userGroupInfo?.id) return

  try {
    const response = await getBindFiles(props.userGroupInfo.id)
    if (response.code === 200 && response.data) {
      selectedStorageIds.value = response.data.fileIds || []
    }
  } catch (error) {
    console.error('获取已绑定文件失败:', error)
  }
}

const fetchStorageList = async () => {
  if (!visible.value) return

  loading.value = true

  try {
    const response = await getStorageSelectList(buildParams())
    if (response.code === 200 && response.data) {
      storageList.value = response.data.data || []
      pagination.itemCount = response.data.total || 0
      mergeStorageMeta(storageList.value)
    } else {
      message.error(response.msg || '获取存储列表失败')
    }
  } catch (error) {
    console.error('获取存储列表失败:', error)
    message.error('获取存储列表失败')
  } finally {
    loading.value = false
  }
}

const handleSearch = async () => {
  pagination.page = 1
  await fetchStorageList()
}

const handlePageChange = async (page: number) => {
  pagination.page = page
  await fetchStorageList()
}

const handlePageSizeChange = async (pageSize: number) => {
  pagination.pageSize = pageSize
  pagination.page = 1
  await fetchStorageList()
}

const handleSelectionChange = (keys: Array<string | number>) => {
  const pageIDs = new Set(storageList.value.map((item) => item.id))
  const pageSelectedIDs = keys.map((key) => Number(key))
  const reservedIDs = selectedStorageIds.value.filter((id) => !pageIDs.has(id))

  selectedStorageIds.value = [...reservedIDs, ...pageSelectedIDs]

  storageList.value.forEach((item) => {
    if (pageSelectedIDs.includes(item.id)) {
      selectedStorageMap.value[item.id] = item
    }
  })
}

const selectCurrentPage = () => {
  const next = new Set(selectedStorageIds.value)
  storageList.value.forEach((item) => {
    next.add(item.id)
    selectedStorageMap.value[item.id] = item
  })
  selectedStorageIds.value = Array.from(next)
}

const selectAllSearchResults = async () => {
  selectingAll.value = true

  try {
    const response = await getStorageSelectList({
      ...buildParams(true),
      currentPage: 1,
    })

    if (response.code !== 200 || !response.data) {
      message.error(response.msg || '获取全部存储失败')
      return
    }

    const items = response.data.data || []
    mergeStorageMeta(items)
    selectedStorageIds.value = items.map((item) => item.id)
    message.success(`已选择 ${selectedStorageIds.value.length} 个存储挂载点`)
  } catch (error) {
    console.error('获取全部存储失败:', error)
    message.error('获取全部存储失败')
  } finally {
    selectingAll.value = false
  }
}

const clearSelection = () => {
  selectedStorageIds.value = []
}

const removeSelection = (storageId: number) => {
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
  visible.value = false
}

const handleConfirm = async () => {
  if (!props.userGroupInfo?.id || selectedStorageIds.value.length === 0) {
    message.warning('请选择要绑定的存储')
    return
  }

  submitting.value = true

  try {
    const response = await batchBindFiles({
      groupId: props.userGroupInfo.id,
      fileIds: selectedStorageIds.value,
    })

    if (response.code === 200) {
      message.success('存储绑定成功')
      visible.value = false
      emit('success')
    } else {
      message.error(response.msg || '存储绑定失败')
    }
  } catch (error) {
    console.error('存储绑定失败:', error)
    message.error('存储绑定失败')
  } finally {
    submitting.value = false
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
