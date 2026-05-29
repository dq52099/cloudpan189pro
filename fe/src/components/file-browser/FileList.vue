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
      :bordered="false"
      class="custom-table"
    />
  </div>
</template>

<script setup lang="ts">
import { h, computed } from 'vue'
import { NDataTable, NIcon, NButton, type DataTableColumns } from 'naive-ui'
import {
  FolderOutline,
  DocumentOutline,
  VideocamOutline,
  MusicalNotesOutline,
  ImageOutline,
  ArchiveOutline,
  DownloadOutline, // <--- 修改：换成 DownloadOutline，更通用
} from '@vicons/ionicons5'
import type { FileChild } from '@/api/file'
import { formatFileSize, formatDate } from '@/utils/format'

const props = defineProps<{
  fileList: FileChild[]
  loading: boolean
  checkedRowKeys?: number[] // 接收父组件的选中状态
  downloadingRowKeys?: number[]
}>()

// Emits 定义
const emit = defineEmits<{
  (e: 'update:checkedRowKeys', keys: number[]): void
  (e: 'fileClick', file: FileChild): void
  (e: 'download', file: FileChild): void
}>()

// 处理选中事件
const downloadingRowKeySet = computed(() => new Set(props.downloadingRowKeys ?? []))
const isRowDownloading = (rowId: number) => downloadingRowKeySet.value.has(rowId)

const handleCheck = (keys: Array<string | number>) => {
  const selectableKeys = keys.filter(
    (key): key is number => typeof key === 'number' && !isRowDownloading(key)
  )

  emit('update:checkedRowKeys', selectableKeys)
}

// 处理行点击（点击行进入目录）
const rowProps = (row: FileChild) => {
  return {
    style: 'cursor: pointer;',
    onClick: (e: MouseEvent) => {
      // 获取点击的目标元素
      const target = e.target as HTMLElement
      // 如果点击的是复选框、按钮或其内部元素，不触发进入目录操作
      if (target.closest('.n-checkbox') || target.closest('.n-button') || target.tagName === 'A') {
        return
      }
      emit('fileClick', row)
    },
  }
}

// 图标判断逻辑
const getFileIcon = (fileName: string, isDir?: boolean) => {
  if (isDir) return FolderOutline
  const ext = fileName.split('.').pop()?.toLowerCase()
  if (!ext) return DocumentOutline
  if (['mp4', 'avi', 'mkv', 'mov', 'wmv', 'flv', 'webm'].includes(ext)) return VideocamOutline
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
    disabled: (row) => isRowDownloading(row.id),
  },
  {
    title: '名称',
    key: 'name',
    render(row) {
      const IconComponent = getFileIcon(row.name, row.isDir)
      const iconColor = row.isDir ? 'var(--n-primary-color)' : undefined

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
              style: 'white-space: nowrap; overflow: hidden; text-overflow: ellipsis;',
            },
            row.name
          ),
        ]
      )
    },
  },
  {
    title: '大小',
    key: 'size',
    width: 120,
    align: 'center',
    render(row) {
      return row.isDir ? '-' : formatFileSize(row.size)
    },
  },
  {
    title: '修改时间',
    key: 'updatedAt', // 这里确认一下你的 API 返回的是 modifyDate 还是 updatedAt
    width: 180,
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
    width: 100,
    align: 'center',
    fixed: 'right',
    render(row) {
      const isDownloading = isRowDownloading(row.id)

      if (row.isDir) {
        return h(
          NButton,
          {
            size: 'small',
            text: true,
            type: 'primary',
            onClick: () => emit('fileClick', row),
          },
          { default: () => '打开' }
        )
      } else {
        return h(
          NButton,
          {
            size: 'small',
            text: true,
            type: 'error',
            loading: isDownloading,
            disabled: isDownloading,
            onClick: () => {
              if (!isDownloading) {
                emit('download', row)
              }
            },
          },
          {
            icon: () => h(NIcon, null, { default: () => h(DownloadOutline) }),
            default: () => '下载',
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
</style>
