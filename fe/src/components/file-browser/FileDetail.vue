<template>
  <div class="file-detail">
    <!-- 文件名 -->
    <div class="file-name">
      <h3>{{ fileInfo.name || '未知文件' }}</h3>
    </div>

    <!-- 视频播放器区域 - 插槽预留 -->
    <div class="video-player-container" v-if="isVideoFile">
      <slot name="video-player" :file="fileInfo">
        <VideoPlayer :file="fileInfo" preload="metadata" />
      </slot>
    </div>

    <!-- 图片预览区域 - 插槽预留 -->
    <div class="image-preview-container" v-else-if="isImageFile">
      <slot name="image-preview" :file="fileInfo">
        <ImageViewer :file="fileInfo" />
      </slot>
    </div>

    <!-- 其他文件类型预览 -->
    <div class="file-preview-container" v-else>
      <slot name="file-preview" :file="fileInfo">
        <div class="file-placeholder">
          <n-icon size="48"><component :is="getFileTypeIcon(fileInfo.name || '')" /></n-icon>
          <p>{{ fileInfo.name || '未知文件' }}</p>
        </div>
      </slot>
    </div>

    <!-- 推荐播放器 (仅视频文件显示) -->
    <div class="recommended-players" v-if="isVideoFile">
      <span class="label">使用外部播放器：</span>
      <button class="player-btn vlc" :disabled="openPlayerPending" @click="openWithPlayer('vlc')">
        <img class="player-icon" src="./icon/vlc.png" alt="VLC" />
        <span>VLC</span>
      </button>
      <button
        class="player-btn potplayer"
        :disabled="openPlayerPending"
        @click="openWithPlayer('potplayer')"
      >
        <img class="player-icon" src="./icon/potplayer.png" alt="PotPlayer" />
        <span>PotPlayer</span>
      </button>
      <button class="player-btn mpc" :disabled="openPlayerPending" @click="openWithPlayer('mpc')">
        <img class="player-icon" src="./icon/mpc-hc.png" alt="MPC-HC" />
        <span>MPC-HC</span>
      </button>
    </div>

    <!-- 文件信息和操作按钮 -->
    <div class="file-info-actions">
      <div class="file-info">
        <div class="info-item">
          <span class="label">类型：</span>
          <span class="value">{{ getFileTypeLabel(fileInfo.name || '') }}</span>
        </div>
        <div class="info-item">
          <span class="label">大小：</span>
          <span class="value">{{ formatFileSize(fileInfo.size || 0) }}</span>
        </div>
        <div class="info-item">
          <span class="label">修改时间：</span>
          <span class="value">{{ formatDateTime(fileInfo.modifyDate) }}</span>
        </div>
      </div>

      <div class="action-buttons">
        <button class="action-btn copy-link" :disabled="copyPending" @click="copyLink">
          <n-icon><LinkOutline /></n-icon>
          <span>复制链接</span>
        </button>
        <button class="action-btn download" :disabled="downloadPending" @click="downloadFile">
          <n-icon><DownloadOutline /></n-icon>
          <span>下载</span>
        </button>
        <div class="qrcode-container">
          <button class="action-btn qrcode">
            <n-icon><QrCodeOutline /></n-icon>
            <span>二维码</span>
          </button>
          <div class="qrcode-tooltip">
            <n-qr-code
              :value="shareUrl"
              :size="120"
              :margin="8"
              :padding="0"
              color="#000000"
              background-color="#ffffff"
              error-correction-level="M"
            />
            <p class="qr-tip">扫描二维码分享</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { NIcon, NQrCode, useMessage } from 'naive-ui'
import {
  QrCodeOutline,
  LinkOutline,
  DownloadOutline,
  PlayOutline,
  ImageOutline,
  DocumentOutline,
} from '@vicons/ionicons5'
import { type FileChild, createDownloadUrl } from '@/api/file'
import { formatDateTime } from '@/utils/time'
import { normalizeCreateDownloadUrlResponse } from '@/utils/responseGuards'
import VideoPlayer from './preview/VideoPlayer.vue'
import ImageViewer from './preview/ImageViewer.vue'

interface Props {
  fileInfo: FileChild
}

const props = defineProps<Props>()

// 计算文件类型
const isVideoFile = computed(() => {
  if (!props.fileInfo.name) return false
  const ext = props.fileInfo.name.split('.').pop()?.toLowerCase()
  return ['mp4', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'm4v', 'webm'].includes(ext || '')
})

const isImageFile = computed(() => {
  if (!props.fileInfo.name) return false
  const ext = props.fileInfo.name.split('.').pop()?.toLowerCase()
  return ['jpg', 'jpeg', 'png', 'gif', 'bmp', 'webp', 'svg'].includes(ext || '')
})

// 获取文件类型图标
const getFileTypeIcon = (fileName: string) => {
  const ext = fileName.split('.').pop()?.toLowerCase()

  if (['mp4', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'm4v', 'webm'].includes(ext || '')) {
    return PlayOutline
  }
  if (['jpg', 'jpeg', 'png', 'gif', 'bmp', 'webp', 'svg'].includes(ext || '')) {
    return ImageOutline
  }

  return DocumentOutline
}

// 获取文件类型标签
const getFileTypeLabel = (fileName: string): string => {
  const ext = fileName.split('.').pop()?.toLowerCase()

  if (['mp4', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'm4v', 'webm'].includes(ext || '')) {
    return 'Video'
  }
  if (['jpg', 'jpeg', 'png', 'gif', 'bmp', 'webp', 'svg'].includes(ext || '')) {
    return 'Image'
  }
  if (['pdf'].includes(ext || '')) {
    return 'PDF'
  }
  if (['doc', 'docx'].includes(ext || '')) {
    return 'Word'
  }
  if (['xls', 'xlsx'].includes(ext || '')) {
    return 'Excel'
  }
  if (['ppt', 'pptx'].includes(ext || '')) {
    return 'PowerPoint'
  }
  if (['zip', 'rar', '7z', 'tar', 'gz'].includes(ext || '')) {
    return 'Archive'
  }

  return 'File'
}

// 格式化文件大小
const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 B'

  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// 消息提示
const message = useMessage()

// 二维码相关
const shareUrl = computed(() => window.location.href)

const isMounted = ref(true)
const openPlayerPending = ref(false)
const downloadPending = ref(false)
const copyPending = ref(false)
const openPlayerRequestId = ref(0)
const downloadRequestId = ref(0)
const copyRequestId = ref(0)

const isActiveRequest = (requestId: number, currentRequestId: number, fileId?: number) => {
  return (
    isMounted.value &&
    requestId === currentRequestId &&
    (fileId === undefined || props.fileInfo.id === fileId)
  )
}

const isActiveCopyRequest = (requestId: number, currentRequestId: number, href: string) => {
  return isActiveRequest(requestId, currentRequestId) && window.location.href === href
}

watch(
  () => props.fileInfo.id,
  () => {
    openPlayerRequestId.value += 1
    downloadRequestId.value += 1
    copyRequestId.value += 1
    openPlayerPending.value = false
    downloadPending.value = false
    copyPending.value = false
  }
)

onUnmounted(() => {
  isMounted.value = false
  openPlayerRequestId.value += 1
  downloadRequestId.value += 1
  copyRequestId.value += 1
})

// 功能实现
const openWithPlayer = (player: string) => {
  if (openPlayerPending.value) {
    return
  }

  const fileId = props.fileInfo.id
  const requestId = openPlayerRequestId.value + 1
  openPlayerRequestId.value = requestId
  openPlayerPending.value = true

  // 获取当前文件的下载链接，然后用指定播放器打开
  createDownloadUrl({ fileId })
    .then((response) => {
      if (!isActiveRequest(requestId, openPlayerRequestId.value, fileId)) {
        return
      }

      if (response.code === 200) {
        const data = normalizeCreateDownloadUrlResponse(response.data)
        if (!data) {
          message.error('获取播放链接失败：响应数据格式异常')

          return
        }

        const url = data.downloadUrl

        // 根据不同播放器生成对应的协议链接
        let playerUrl = ''
        switch (player) {
          case 'vlc':
            playerUrl = `vlc://${url}`
            break
          case 'potplayer':
            playerUrl = `potplayer://${url}`
            break
          case 'mpc':
            playerUrl = `mpc-hc://${url}`
            break
          default:
            playerUrl = url
        }

        // 尝试打开播放器
        const link = document.createElement('a')
        link.href = playerUrl
        link.click()

        message.success(`正在使用 ${player.toUpperCase()} 播放器打开文件`)
      } else {
        message.error(response.msg || '获取播放链接失败')
      }
    })
    .catch((error) => {
      if (!isActiveRequest(requestId, openPlayerRequestId.value, fileId)) {
        return
      }

      console.error('获取播放链接失败:', error)
      message.error('获取播放链接失败')
    })
    .finally(() => {
      if (isActiveRequest(requestId, openPlayerRequestId.value, fileId)) {
        openPlayerPending.value = false
      }
    })
}

const copyLink = () => {
  if (copyPending.value) {
    return
  }

  const currentUrl = window.location.href
  const requestId = copyRequestId.value + 1
  copyRequestId.value = requestId
  copyPending.value = true

  // 使用现代剪贴板 API
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard
      .writeText(currentUrl)
      .then(() => {
        if (!isActiveCopyRequest(requestId, copyRequestId.value, currentUrl)) {
          return
        }

        message.success('链接已复制到剪贴板')
      })
      .catch((error) => {
        if (!isActiveCopyRequest(requestId, copyRequestId.value, currentUrl)) {
          return
        }

        console.error('复制链接失败:', error)
        // 降级方案：使用传统方法复制
        if (fallbackCopyToClipboard(currentUrl)) {
          message.success('链接已复制到剪贴板')
        } else {
          message.error('复制链接失败')
        }
      })
      .finally(() => {
        if (isActiveRequest(requestId, copyRequestId.value)) {
          copyPending.value = false
        }
      })
  } else {
    // 直接使用降级方案
    if (fallbackCopyToClipboard(currentUrl)) {
      message.success('链接已复制到剪贴板')
    } else {
      message.error('复制链接失败')
    }
    copyPending.value = false
  }
}

const fallbackCopyToClipboard = (text: string): boolean => {
  const textArea = document.createElement('textarea')
  textArea.value = text
  document.body.appendChild(textArea)
  textArea.select()

  const successful = document.execCommand('copy')
  document.body.removeChild(textArea)

  return successful
}

const downloadFile = () => {
  if (downloadPending.value) {
    return
  }

  const fileId = props.fileInfo.id
  const requestId = downloadRequestId.value + 1
  downloadRequestId.value = requestId
  downloadPending.value = true

  createDownloadUrl({ fileId })
    .then((response) => {
      if (!isActiveRequest(requestId, downloadRequestId.value, fileId)) {
        return
      }

      if (response.code === 200) {
        const data = normalizeCreateDownloadUrlResponse(response.data)
        if (!data) {
          message.error('创建下载链接失败：响应数据格式异常')

          return
        }

        const link = document.createElement('a')
        link.href = data.downloadUrl
        link.download = props.fileInfo.name || 'download'
        document.body.appendChild(link)
        link.click()
        document.body.removeChild(link)
        message.success('开始下载文件')
      } else {
        message.error(response.msg || '创建下载链接失败')
      }
    })
    .catch((error) => {
      if (!isActiveRequest(requestId, downloadRequestId.value, fileId)) {
        return
      }

      console.error('下载失败:', error)
      message.error('下载失败')
    })
    .finally(() => {
      if (isActiveRequest(requestId, downloadRequestId.value, fileId)) {
        downloadPending.value = false
      }
    })
}
</script>

<style scoped>
.file-detail {
  background: var(--n-card-color);
  border-radius: 8px;
  padding: 20px;
  box-shadow: 0 2px 8px rgb(0 0 0 / 10%);
}

/* 文件类型标签 */
.file-type-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #fff3cd;
  color: #856404;
  padding: 4px 12px;
  border-radius: 4px;
  font-size: 14px;
  font-weight: 500;
  margin-bottom: 16px;
}

.file-type-tag .icon {
  font-size: 16px;
}

/* 视频播放器容器 */
.video-player-container {
  width: 100%;
  margin-bottom: 20px;
}

.video-placeholder {
  position: relative;
  width: 100%;
  height: 400px;
  background: #000;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.play-button {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 60px;
  height: 60px;
  background: rgb(59 130 246 / 90%);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.3s ease;
}

.play-button:hover {
  background: rgb(59 130 246 / 100%);
  transform: translate(-50%, -50%) scale(1.1);
}

.play-icon {
  color: white;
  font-size: 24px;
  margin-left: 4px;
}

.video-controls {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  background: linear-gradient(transparent, rgb(0 0 0 / 70%));
  padding: 20px 16px 16px;
}

.control-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  color: white;
}

.play-btn,
.volume-btn,
.settings-btn,
.fullscreen-btn {
  background: none;
  border: none;
  color: white;
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.play-btn:hover,
.volume-btn:hover,
.settings-btn:hover,
.fullscreen-btn:hover {
  background: rgb(255 255 255 / 20%);
}

.progress-bar {
  flex: 1;
  height: 4px;
  background: rgb(255 255 255 / 30%);
  border-radius: 2px;
  position: relative;
  cursor: pointer;
}

.progress-track {
  height: 100%;
  background: #3b82f6;
  border-radius: 2px;
  width: 0%;
}

.time {
  font-size: 12px;
  white-space: nowrap;
}

.volume-bar {
  width: 60px;
  height: 4px;
  background: rgb(255 255 255 / 30%);
  border-radius: 2px;
  position: relative;
  cursor: pointer;
}

.volume-track {
  height: 100%;
  background: #3b82f6;
  border-radius: 2px;
  width: 80%;
}

/* 图片预览容器 */
.image-preview-container {
  width: 100%;
  margin-bottom: 20px;
}

.image-placeholder {
  width: 100%;
  height: 300px;
  background: var(--n-color-hover);
  border: 2px dashed var(--n-border-color);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--n-text-color-2);
}

.image-icon {
  font-size: 48px;
  margin-bottom: 12px;
}

/* 文件预览容器 */
.file-preview-container {
  width: 100%;
  margin-bottom: 20px;
}

.file-placeholder {
  width: 100%;
  height: 200px;
  background: var(--n-color-hover);
  border: 2px dashed var(--n-border-color);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--n-text-color-2);
}

.file-icon {
  font-size: 48px;
  margin-bottom: 12px;
}

/* 文件名 */
.file-name {
  margin-bottom: 20px;
}

.file-name h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--n-text-color);
  word-break: break-all;
}

/* 推荐播放器 */
.recommended-players {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
  padding: 16px;
  background: var(--n-color-hover);
  border-radius: 8px;
  flex-wrap: wrap;
}

.recommended-players .label {
  font-size: 14px;
  color: var(--n-text-color-2);
  white-space: nowrap;
}

.player-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  background: var(--n-card-color);
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 14px;
}

.player-btn:hover {
  border-color: var(--n-primary-color);
  background: var(--n-color-hover);
  color: var(--n-primary-color);
}

.player-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.player-btn:disabled:hover {
  border-color: var(--n-border-color);
  background: var(--n-card-color);
  color: var(--n-text-color);
}

.player-icon {
  width: 20px;
  height: 20px;
  object-fit: contain;
}

/* 文件信息和操作按钮 */
.file-info-actions {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
  flex-wrap: wrap;
}

.file-info {
  flex: 1;
  min-width: 200px;
}

.info-item {
  display: flex;
  margin-bottom: 8px;
  font-size: 14px;
}

.info-item .label {
  color: var(--n-text-color-2);
  min-width: 80px;
}

.info-item .value {
  color: var(--n-text-color);
  font-weight: 500;
}

.action-buttons {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.action-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  background: var(--n-card-color);
  cursor: pointer;
  transition: all 0.2s;
  font-size: 14px;
  text-decoration: none;
  color: var(--n-text-color);
}

.action-btn:hover {
  border-color: var(--n-primary-color);
  background: var(--n-color-hover);
  color: var(--n-primary-color);
}

.action-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.action-btn:disabled:hover {
  border-color: var(--n-border-color);
  background: var(--n-card-color);
  color: var(--n-text-color);
}

.action-btn.download {
  background: var(--n-primary-color);
  color: white;
  border-color: var(--n-primary-color);
}

.action-btn.download:hover {
  background: var(--n-primary-color-hover);
  border-color: var(--n-primary-color-hover);
  color: white;
}

.action-btn.download:disabled:hover {
  background: var(--n-primary-color);
  border-color: var(--n-primary-color);
  color: white;
}

/* 二维码容器和提示框 */
.qrcode-container {
  position: relative;
  display: inline-block;
  z-index: 99999;
}

.qrcode-tooltip {
  position: absolute;
  bottom: 100%;
  background: var(--n-card-color);
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  padding: 12px;
  box-shadow: 0 4px 12px rgb(0 0 0 / 15%);
  opacity: 0;
  visibility: hidden;
  transition: all 0.2s ease;
  z-index: 1000;
  margin-bottom: 8px;
  text-align: center;
  min-width: 150px;
}

.qrcode-tooltip::after {
  content: '';
  position: absolute;
  top: 100%;
  right: 20px;
  border: 6px solid transparent;
  border-top-color: var(--n-card-color);
}

.qrcode-container:hover .qrcode-tooltip {
  opacity: 1;
  visibility: visible;
}

.qr-tip {
  color: var(--n-text-color-2);
  font-size: 12px;
  margin: 8px 0 0;
}

/* 响应式设计 */
@media (width <= 768px) {
  .file-detail {
    padding: 16px;
  }

  .file-info-actions {
    flex-direction: column;
  }

  .action-buttons {
    width: 100%;
    justify-content: center;
  }

  .players {
    justify-content: center;
  }

  .video-placeholder {
    height: 250px;
  }
}
</style>
