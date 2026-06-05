<template>
  <n-modal
    :show="show"
    preset="card"
    title="文件搜索"
    style="width: min(1000px, calc(100vw - 32px))"
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
        @keyup.enter="handleNewSearch"
        class="keyword-input"
      />
      <n-button
        type="primary"
        secondary
        :loading="searching"
        :disabled="searching"
        @click="handleNewSearch"
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
          class="desktop-search-table"
          :columns="columns"
          :data="list"
          :loading="searching"
          :bordered="false"
          :single-line="false"
          :row-props="rowProps"
          :scroll-x="720"
        />
        <div v-if="list.length" class="mobile-search-results">
          <div
            v-for="row in list"
            :key="row.id"
            class="search-result-card"
            role="button"
            tabindex="0"
            @click="selectResult(row)"
            @keydown.enter.prevent="selectResult(row)"
            @keydown.space.prevent="selectResult(row)"
          >
            <n-icon
              class="search-result-card__icon"
              :color="row.isDir ? 'var(--n-primary-color)' : undefined"
            >
              <FolderOutline v-if="row.isDir" />
              <DocumentOutline v-else />
            </n-icon>
            <div class="search-result-card__body">
              <div class="search-result-card__name" :title="row.name || '-'">
                {{ row.name || '-' }}
              </div>
              <div class="search-result-card__meta">
                <span>{{ row.isDir ? '目录' : '文件' }}</span>
                <span v-if="!row.isDir">{{ formatFileSize(row.size || 0) }}</span>
              </div>
              <div class="search-result-card__path" :title="row.fullPath || '-'">
                {{ row.fullPath || '-' }}
              </div>
            </div>
          </div>
        </div>
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
  useMessage,
} from 'naive-ui'
import { DocumentOutline, FolderOutline, SearchOutline } from '@vicons/ionicons5'
import { searchFiles, type FileSearchItem } from '@/api/file'
import { getErrorMessage } from '@/utils/api'
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

const toSafeNonNegativeInteger = (value: unknown): number | null => {
  if (typeof value === 'number') {
    return Number.isSafeInteger(value) && value >= 0 ? value : null
  }

  if (typeof value !== 'string') {
    return null
  }

  const normalizedValue = value.trim()
  if (!/^\d+$/.test(normalizedValue)) {
    return null
  }

  const parsedValue = Number(normalizedValue)

  return Number.isSafeInteger(parsedValue) ? parsedValue : null
}

const resetSearchState = () => {
  currentPage.value = 1
  list.value = []
  total.value = 0
  searching.value = false
}

const resetDialogState = () => {
  invalidateSearchRequests()
  keyword.value = ''
  globalSearch.value = false
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

watch(
  () => props.currentDirId,
  () => {
    if (!show.value || globalSearch.value) {
      return
    }

    invalidateSearchRequests()
    resetSearchState()
  }
)

watch(globalSearch, () => {
  if (!show.value) {
    return
  }

  invalidateSearchRequests()
  resetSearchState()
})

const onUpdateShow = (val: boolean) => {
  show.value = val
  if (val) {
    resetDialogState()
  } else {
    closeDialogState()
  }
  emits('update:show', val)
}

const renderEllipsisText = (text: string, className = 'search-result-text') => {
  return h(
    'span',
    {
      class: className,
      title: text,
    },
    text
  )
}

const columns = [
  {
    title: '名称',
    key: 'name',
    render(row: FileSearchItem) {
      return renderEllipsisText(row.name || '-')
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

      return renderEllipsisText(text, 'search-path-text')
    },
  },
]

const rowProps = (row: FileSearchItem) => {
  return {
    style: 'cursor: pointer;',
    onClick: () => {
      selectResult(row)
    },
  }
}

const selectResult = (row: FileSearchItem) => {
  emits('select', row)
  show.value = false
  closeDialogState()
  emits('update:show', false)
}

const buildQuery = (requestedPage = currentPage.value) => {
  const normalizedPage = isPositiveSafeInteger(requestedPage) ? requestedPage : 1
  const normalizedPageSize = isPositiveSafeInteger(pageSize.value) ? pageSize.value : 15
  const q: {
    keyword?: string
    pid?: number
    global?: boolean
    pageSize: number
    currentPage: number
  } = {
    pageSize: normalizedPageSize,
    currentPage: normalizedPage,
  }
  if (keyword.value?.trim()) {
    q.keyword = keyword.value.trim()
  }
  if (globalSearch.value) {
    q.global = true
  } else {
    const pid = toSafeNonNegativeInteger(props.currentDirId)
    if (pid !== null) {
      q.pid = pid
    }
  }
  return q
}

const doSearch = (requestedPage = currentPage.value) => {
  const query = buildQuery(requestedPage)
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

      const errorMessage = getErrorMessage(err, '搜索失败')

      console.error('搜索失败:', errorMessage)
      message.error(errorMessage)
    })
    .finally(() => {
      if (!isLatestSearch(currentDialogVersion, currentSearchRequestId)) return

      searching.value = false
    })
}

const handleNewSearch = () => {
  currentPage.value = 1
  total.value = 0
  list.value = []
  doSearch(1)
}

const prevPage = () => {
  if (!searching.value && currentPage.value > 1) {
    doSearch(currentPage.value - 1)
  }
}

const nextPage = () => {
  if (!searching.value && currentPage.value < totalPages.value) {
    doSearch(currentPage.value + 1)
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
  flex-wrap: wrap;
  margin-bottom: 16px;
  padding: 10px;
  background: #f7f8fa;
  border: 1px solid #eef0f3;
  border-radius: 8px;
}

.keyword-input {
  flex: 1 1 240px;
  min-width: 0;
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

.mobile-search-results {
  display: none;
}

.search-result-text,
.search-path-text {
  display: block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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

@media (width <= 640px) {
  .search-toolbar,
  .pagination,
  .pager {
    align-items: stretch;
    flex-direction: column;
  }

  .search-toolbar :deep(.n-button),
  .keyword-input,
  .pager :deep(.n-button) {
    width: 100%;
  }

  .keyword-input {
    flex: 0 0 auto;
  }

  .dialog-section {
    max-height: 54vh;
  }

  .desktop-search-table {
    display: none;
  }

  .mobile-search-results {
    display: flex;
    flex-direction: column;
  }

  .search-result-card {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 10px;
    padding: 12px;
    border-bottom: 1px solid #eef0f3;
    cursor: pointer;
  }

  .search-result-card:last-child {
    border-bottom: 0;
  }

  .search-result-card:focus-visible {
    outline: 2px solid var(--n-primary-color);
    outline-offset: -2px;
  }

  .search-result-card__icon {
    margin-top: 2px;
    font-size: 22px;
  }

  .search-result-card__body {
    min-width: 0;
  }

  .search-result-card__name {
    overflow: hidden;
    color: var(--n-text-color);
    font-weight: 500;
    line-height: 1.35;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .search-result-card__meta {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 10px;
    margin-top: 4px;
    color: var(--n-text-color-3);
    font-size: 12px;
    line-height: 1.4;
  }

  .search-result-card__path {
    overflow: hidden;
    margin-top: 5px;
    color: var(--n-text-color-3);
    font-size: 12px;
    line-height: 1.4;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}
</style>
