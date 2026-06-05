<template>
  <div class="file-list-container">
    <n-data-table
      :columns="columns"
      :data="fileList"
      :loading="loading"
      :row-key="(row) => row.id"
      :checked-row-keys="checkedRowKeys"
      @update:checked-row-keys="handleCheck"
      :row-props="rowProps"
      :scroll-x="640"
      :bordered="false"
      class="custom-table desktop-file-table"
    />

    <div class="mobile-file-list">
      <n-empty
        v-if="!loading && fileList.length === 0"
        description="暂无文件"
        class="mobile-empty"
      />
      <div
        v-for="file in fileList"
        :key="file.id"
        class="file-card"
        :class="{
          'is-directory': file.isDir,
          'is-pending': isRowPendingDelete(file.id),
        }"
        role="button"
        tabindex="0"
        @click="handleRowClick(file, $event)"
        @keydown.enter.prevent="handleKeyboardOpen(file)"
        @keydown.space.prevent="handleKeyboardOpen(file)"
      >
        <div class="file-card__select">
          <n-checkbox
            :checked="isRowChecked(file.id)"
            :disabled="isRowActionBlocked(file.id)"
            @update:checked="(checked) => handleCardCheck(file.id, checked)"
          />
        </div>
        <n-icon class="file-card__icon" :color="file.isDir ? 'var(--n-primary-color)' : undefined">
          <component :is="getFileIcon(file.name, file.isDir)" />
        </n-icon>
        <div class="file-card__body">
          <div class="file-card__name" :title="file.name">
            {{ file.name }}
          </div>
          <div class="file-card__meta">
            <span>{{ file.isDir ? '文件夹' : formatFileSize(file.size) }}</span>
            <span>{{ formatDate(file.modifyDate || file.updatedAt) }}</span>
          </div>
        </div>
        <n-tag
          v-if="isRowPendingDelete(file.id)"
          size="small"
          type="warning"
          :bordered="false"
          class="file-card__tag"
        >
          删除中
        </n-tag>
        <n-button
          class="file-card__action"
          size="small"
          quaternary
          circle
          :type="isRowActionBlocked(file.id) ? 'warning' : 'primary'"
          :loading="isRowDownloading(file.id)"
          :disabled="isRowActionBlocked(file.id)"
          :aria-label="file.isDir ? '打开' : '下载'"
          @click.stop="handleCardAction(file)"
        >
          <template #icon>
            <n-icon>
              <OpenOutline v-if="file.isDir" />
              <DownloadOutline v-else />
            </n-icon>
          </template>
        </n-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { h, computed, type Component } from 'vue'
import {
  NDataTable,
  NIcon,
  NButton,
  NTag,
  NTooltip,
  NCheckbox,
  NEmpty,
  type DataTableColumns,
} from 'naive-ui'
import {
  FolderOutline,
  DocumentOutline,
  VideocamOutline,
  MusicalNotesOutline,
  ImageOutline,
  ArchiveOutline,
  DownloadOutline,
  OpenOutline,
} from '@vicons/ionicons5'
import type { FileChild } from '@/api/file'
import { formatFileSize, formatDate } from '@/utils/format'

const props = defineProps<{
  fileList: FileChild[]
  loading: boolean
  checkedRowKeys?: number[] // 接收父组件的选中状态
  downloadingRowKeys?: number[]
  pendingDeleteRowKeys?: number[]
}>()

// Emits 定义
const emit = defineEmits<{
  (e: 'update:checkedRowKeys', keys: number[]): void
  (e: 'fileClick', file: FileChild): void
  (e: 'download', file: FileChild): void
}>()

// 处理选中事件
const downloadingRowKeySet = computed(() => new Set(props.downloadingRowKeys ?? []))
const pendingDeleteRowKeySet = computed(() => new Set(props.pendingDeleteRowKeys ?? []))
const isRowDownloading = (rowId: number) => downloadingRowKeySet.value.has(rowId)
const isRowPendingDelete = (rowId: number) => pendingDeleteRowKeySet.value.has(rowId)
const isRowActionBlocked = (rowId: number) => isRowDownloading(rowId) || isRowPendingDelete(rowId)
const checkedRowKeySet = computed(() => new Set(props.checkedRowKeys ?? []))
const isRowChecked = (rowId: number) => checkedRowKeySet.value.has(rowId)
const renderIconButton = (
  label: string,
  icon: Component,
  disabled: boolean,
  loading: boolean,
  onClick: () => void
) => {
  const button = h(
    NButton,
    {
      size: 'small',
      quaternary: true,
      circle: true,
      type: disabled ? 'warning' : 'primary',
      loading,
      disabled,
      'aria-label': label,
      onClick,
    },
    { icon: () => h(NIcon, null, { default: () => h(icon) }) }
  )

  return h(NTooltip, { trigger: 'hover' }, { trigger: () => button, default: () => label })
}

const handleCheck = (keys: Array<string | number>) => {
  const selectableKeys = keys.filter(
    (key): key is number => typeof key === 'number' && !isRowActionBlocked(key)
  )

  emit('update:checkedRowKeys', selectableKeys)
}

const isInteractiveTarget = (target: EventTarget | null) => {
  return (
    target instanceof HTMLElement &&
    (target.closest('.n-checkbox') || target.closest('.n-button') || target.tagName === 'A')
  )
}

const handleRowClick = (row: FileChild, e: MouseEvent) => {
  if (isRowPendingDelete(row.id) || isInteractiveTarget(e.target)) {
    return
  }

  emit('fileClick', row)
}

const handleKeyboardOpen = (row: FileChild) => {
  if (!isRowPendingDelete(row.id)) {
    emit('fileClick', row)
  }
}

const handleCardCheck = (rowId: number, checked: boolean) => {
  if (isRowActionBlocked(rowId)) {
    return
  }

  const keys = new Set(props.checkedRowKeys ?? [])
  if (checked) {
    keys.add(rowId)
  } else {
    keys.delete(rowId)
  }

  handleCheck([...keys])
}

const handleCardAction = (row: FileChild) => {
  if (isRowActionBlocked(row.id)) {
    return
  }

  if (row.isDir) {
    emit('fileClick', row)
  } else {
    emit('download', row)
  }
}

// 处理行点击（点击行进入目录）
const rowProps = (row: FileChild) => {
  return {
    style: isRowPendingDelete(row.id) ? 'cursor: not-allowed; opacity: 0.62;' : 'cursor: pointer;',
    onClick: (e: MouseEvent) => {
      handleRowClick(row, e)
    },
  }
}

// 图标判断逻辑
const getFileIcon = (fileName: string, isDir?: boolean) => {
  if (isDir) return FolderOutline
  const ext = fileName.split('.').pop()?.toLowerCase()
  if (!ext) return DocumentOutline
  if (['mp4', 'avi', 'mkv', 'mov', 'wmv', 'flv', 'webm', 'm4v', 'm3u8'].includes(ext)) {
    return VideocamOutline
  }
  if (['mp3', 'wav', 'flac', 'aac', 'ogg', 'wma'].includes(ext)) return MusicalNotesOutline
  if (['jpg', 'jpeg', 'png', 'gif', 'bmp', 'webp', 'svg'].includes(ext)) return ImageOutline
  if (['zip', 'rar', '7z', 'tar', 'gz', 'bz2'].includes(ext)) return ArchiveOutline
  return DocumentOutline
}

// 表格列定义
const columns = computed<DataTableColumns<FileChild>>(() => [
  {
    type: 'selection', // 开启复选框列
    width: 40,
    fixed: 'left',
    disabled: (row) => isRowActionBlocked(row.id),
  },
  {
    title: '名称',
    key: 'name',
    render(row) {
      const IconComponent = getFileIcon(row.name, row.isDir)
      const iconColor = row.isDir ? 'var(--n-primary-color)' : undefined
      const isPendingDelete = isRowPendingDelete(row.id)

      return h(
        'div',
        {
          style: 'display: flex; align-items: center; gap: 12px; min-width: 0;',
        },
        [
          h(NIcon, { size: 22, color: iconColor }, { default: () => h(IconComponent) }),
          h(
            'span',
            {
              style:
                'min-width: 0; flex: 1 1 auto; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;',
              title: row.name,
            },
            row.name
          ),
          isPendingDelete
            ? h(
                NTag,
                { size: 'small', type: 'warning', bordered: false },
                { default: () => '删除中' }
              )
            : null,
        ]
      )
    },
  },
  {
    title: '大小',
    key: 'size',
    width: 110,
    align: 'center',
    render(row) {
      return row.isDir ? '-' : formatFileSize(row.size)
    },
  },
  {
    title: '修改时间',
    key: 'updatedAt', // 这里确认一下你的 API 返回的是 modifyDate 还是 updatedAt
    width: 160,
    align: 'center',
    render(row) {
      // 优先使用 modifyDate，如果没有则尝试 updatedAt
      const dateStr = row.modifyDate || row.updatedAt
      return formatDate(dateStr)
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 72,
    align: 'center',
    fixed: 'right',
    render(row) {
      const isDownloading = isRowDownloading(row.id)
      const isPendingDelete = isRowPendingDelete(row.id)

      if (row.isDir) {
        return renderIconButton(
          isPendingDelete ? '删除中' : '打开',
          OpenOutline,
          isPendingDelete,
          false,
          () => {
            if (!isPendingDelete) {
              emit('fileClick', row)
            }
          }
        )
      } else {
        return renderIconButton(
          isPendingDelete ? '删除中' : isDownloading ? '下载中' : '下载',
          DownloadOutline,
          isDownloading || isPendingDelete,
          isDownloading,
          () => {
            if (!isDownloading && !isPendingDelete) {
              emit('download', row)
            }
          }
        )
      }
    },
  },
])
</script>

<style scoped>
.file-list-container {
  background: var(--n-card-color);
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  min-height: 400px;
}

/* 覆盖 Naive UI 默认样式，使其更紧凑美观 */
:deep(.custom-table .n-data-table-td) {
  padding: 12px 16px;
  vertical-align: middle;
}

:deep(.custom-table .n-data-table-th) {
  background-color: var(--n-color-hover);
  font-weight: 500;
}

.mobile-file-list {
  display: none;
}

@media (width <= 640px) {
  .file-list-container {
    min-height: 240px;
  }

  .desktop-file-table {
    display: none;
  }

  .mobile-file-list {
    display: flex;
    flex-direction: column;
  }

  .file-card {
    display: grid;
    grid-template-columns: auto auto minmax(0, 1fr) auto auto;
    align-items: center;
    gap: 10px;
    min-height: 68px;
    padding: 12px;
    border-bottom: 1px solid var(--n-border-color);
    cursor: pointer;
  }

  .file-card:last-child {
    border-bottom: 0;
  }

  .file-card.is-pending {
    cursor: not-allowed;
    opacity: 0.62;
  }

  .file-card:focus-visible {
    outline: 2px solid var(--n-primary-color);
    outline-offset: -2px;
  }

  .file-card__select {
    display: flex;
    align-items: center;
  }

  .file-card__icon {
    font-size: 22px;
  }

  .file-card__body {
    min-width: 0;
  }

  .file-card__name {
    overflow: hidden;
    color: var(--n-text-color);
    font-weight: 500;
    line-height: 1.35;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .file-card__meta {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 10px;
    margin-top: 4px;
    color: var(--n-text-color-3);
    font-size: 12px;
    line-height: 1.4;
  }

  .file-card__tag {
    justify-self: end;
  }

  .file-card__action {
    justify-self: end;
  }

  .mobile-empty {
    padding: 40px 16px;
  }
}
</style>
