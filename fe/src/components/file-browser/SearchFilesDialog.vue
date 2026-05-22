<template>
  <n-modal
    :show="show"
    preset="card"
    title="文件搜索"
    style="width: 1000px; max-width: 92vw"
    :bordered="false"
    @update:show="onUpdateShow"
  >
    <div class="search-toolbar">
      <n-checkbox v-model:checked="globalSearch" :disabled="searching">全局搜索</n-checkbox>
      <n-input
        v-model:value="keyword"
        placeholder="请输入搜索关键词"
        clearable
        :disabled="searching"
        @keyup.enter="doSearch"
        class="keyword-input"
      />
      <n-button
        type="primary"
        secondary
        :loading="searching"
        :disabled="searching"
        @click="doSearch"
      >
        <template #icon>
          <n-icon :component="SearchOutline" />
        </template>
        搜索
      </n-button>
    </div>

    <div v-if="searching" class="loading-container">
      <n-spin />
    </div>

    <template v-else>
      <div class="dialog-section">
        <n-empty v-if="!list.length" description="暂无搜索结果" />
        <n-data-table
          v-else
          :columns="columns"
          :data="list"
          :loading="searching"
          :bordered="false"
          :single-line="false"
          :row-props="rowProps"
        />
      </div>
      <div class="pagination">
        <div class="summary">共 {{ total }} 条结果，第 {{ currentPage }} / {{ totalPages }} 页</div>
        <div class="pager">
          <n-button
            size="small"
            tertiary
            :disabled="searching || currentPage <= 1"
            @click="prevPage"
            >上一页</n-button
          >
          <n-button
            size="small"
            type="primary"
            ghost
            :disabled="searching || currentPage >= totalPages"
            @click="nextPage"
            >下一页</n-button
          >
        </div>
      </div>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, watch, computed, h, onUnmounted } from 'vue'
import {
  NModal,
  NCheckbox,
  NInput,
  NButton,
  NIcon,
  NDataTable,
  NSpin,
  NEmpty,
  NEllipsis,
  NTooltip,
  useMessage,
} from 'naive-ui'
import { SearchOutline } from '@vicons/ionicons5'
import { searchFiles, type FileSearchItem } from '@/api/file'
import { formatFileSize } from '@/utils/format'
import { getListItems, getListTotal } from '@/utils/pagination'
import { normalizeFileSearchItems } from '@/utils/responseGuards'

interface Props {
  show: boolean
  // 当前目录的文件 id（用于局部搜索时作为 pid）
  currentDirId?: number | null
  // 默认页大小
  pageSize?: number
}

const props = defineProps<Props>()
const emits = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'select', row: FileSearchItem): void
}>()

const message = useMessage()

const show = ref<boolean>(props.show)
const keyword = ref<string>('')
const globalSearch = ref<boolean>(false)
const searching = ref<boolean>(false)

const list = ref<FileSearchItem[]>([])
const total = ref<number>(0)
const currentPage = ref<number>(1)
const pageSize = ref<number>(props.pageSize ?? 15)

let dialogVersion = 0
let searchRequestId = 0

const totalPages = computed(() => {
  if (total.value === 0) return 1
  return Math.max(1, Math.ceil(total.value / pageSize.value))
})

const invalidateSearchRequests = () => {
  dialogVersion += 1
  searchRequestId += 1
}

const isLatestSearch = (targetDialogVersion: number, targetSearchRequestId: number) => {
  return targetDialogVersion === dialogVersion && targetSearchRequestId === searchRequestId
}

const isPositiveSafeInteger = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isSafeInteger(value) && value > 0
}

const resetSearchState = () => {
  currentPage.value = 1
  list.value = []
  total.value = 0
  searching.value = false
}

const resetDialogState = () => {
  invalidateSearchRequests()
  resetSearchState()
}

const closeDialogState = () => {
  invalidateSearchRequests()
  searching.value = false
}

watch(
  () => props.show,
  (val) => {
    show.value = val
    if (val) {
      resetDialogState()
    } else {
      closeDialogState()
    }
  }
)

const onUpdateShow = (val: boolean) => {
  show.value = val
  if (val) {
    resetDialogState()
  } else {
    closeDialogState()
  }
  emits('update:show', val)
}

const columns = [
  {
    title: '名称',
    key: 'name',
    render(row: FileSearchItem) {
      return row.name
    },
  },
  {
    title: '类型',
    key: 'type',
    render(row: FileSearchItem) {
      return row.isDir ? '目录' : '文件'
    },
    width: 100,
  },
  {
    title: '大小',
    key: 'size',
    render(row: FileSearchItem) {
      return row.isDir ? '-' : formatFileSize(row.size || 0)
    },
    width: 140,
  },
  {
    title: '路径',
    key: 'fullPath',
    render(row: FileSearchItem) {
      const text = row.fullPath || '-'
      return h(
        NTooltip,
        { placement: 'top' },
        {
          default: () => text,
          trigger: () =>
            h(NEllipsis, { style: 'max-width: 100%;', lineClamp: 1 }, { default: () => text }),
        }
      )
    },
  },
]

const rowProps = (row: FileSearchItem) => {
  return {
    style: 'cursor: pointer;',
    onClick: () => {
      emits('select', row)
      show.value = false
      closeDialogState()
      emits('update:show', false)
    },
  }
}

const buildQuery = () => {
  const q: {
    keyword?: string
    pid?: number
    global?: boolean
    pageSize: number
    currentPage: number
  } = {
    pageSize: pageSize.value,
    currentPage: currentPage.value,
  }
  if (keyword.value?.trim()) {
    q.keyword = keyword.value.trim()
  }
  if (globalSearch.value) {
    q.global = true
  } else if (props.currentDirId != null) {
    q.pid = Number(props.currentDirId)
  }
  return q
}

const doSearch = () => {
  const query = buildQuery()
  const currentDialogVersion = dialogVersion
  const currentSearchRequestId = searchRequestId + 1
  searchRequestId = currentSearchRequestId
  searching.value = true

  searchFiles(query)
    .then((res) => {
      if (!isLatestSearch(currentDialogVersion, currentSearchRequestId)) return

      if (res.code === 200 && res.data) {
        const rawItems = getListItems<FileSearchItem>(res.data)
        const items = normalizeFileSearchItems(rawItems)
        const itemCount = getListTotal(res.data)
        const responseCurrentPage = (res.data as { currentPage?: unknown }).currentPage
        const responsePageSize = (res.data as { pageSize?: unknown }).pageSize
        if (
          !items ||
          itemCount === null ||
          !isPositiveSafeInteger(responseCurrentPage) ||
          !isPositiveSafeInteger(responsePageSize)
        ) {
          message.error('搜索失败：响应数据格式异常')

          return
        }

        list.value = items
        total.value = itemCount
        currentPage.value = responseCurrentPage
        pageSize.value = responsePageSize
      } else {
        message.error(res.msg || '搜索失败')
      }
    })
    .catch((err) => {
      if (!isLatestSearch(currentDialogVersion, currentSearchRequestId)) return

      console.error('搜索失败: ', err)
      message.error('搜索失败')
    })
    .finally(() => {
      if (!isLatestSearch(currentDialogVersion, currentSearchRequestId)) return

      searching.value = false
    })
}

const prevPage = () => {
  if (!searching.value && currentPage.value > 1) {
    currentPage.value -= 1
    doSearch()
  }
}

const nextPage = () => {
  if (!searching.value && currentPage.value < totalPages.value) {
    currentPage.value += 1
    doSearch()
  }
}

onUnmounted(() => {
  closeDialogState()
})
</script>

<style scoped>
.search-toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-bottom: 16px;
  padding: 10px;
  background: #f7f8fa;
  border: 1px solid #eef0f3;
  border-radius: 8px;
}

.keyword-input {
  flex: 1;
}

.dialog-section {
  max-height: 60vh;
  overflow: auto;
  border: 1px solid #eef0f3;
  border-radius: 8px;
}

:deep(.n-data-table) {
  --td-padding: 10px 12px;
}

:deep(.n-data-table .n-data-table-th) {
  background: #fafafa;
}

:deep(.n-data-table .n-data-table-tr:hover) {
  background: #f7f9fc;
}

.loading-container {
  display: flex;
  justify-content: center;
  padding: 24px 0;
}

.pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 12px;
  padding-top: 8px;
}

.summary {
  font-size: 12px;
  color: #666;
}

.pager {
  display: flex;
  gap: 8px;
}
</style>
