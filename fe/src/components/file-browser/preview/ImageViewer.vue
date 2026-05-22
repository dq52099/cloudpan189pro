<template>
  <div class="image-viewer-container">
    <div class="image-wrapper">
      <!-- 图片显示区域 -->
      <div
        class="image-display"
        @wheel="handleWheel"
        @mousedown="handleMouseDown"
        @dblclick="handleDoubleClick"
      >
        <img
          ref="imageRef"
          v-if="imageUrl"
          :src="imageUrl"
          :alt="fileName"
          class="image-element"
          :style="imageStyle"
          @load="handleImageLoad"
          @error="handleImageError"
          @dragstart.prevent
        />

        <!-- 加载状态 -->
        <div v-if="loading" class="loading-overlay">
          <n-spin size="large" />
          <p>图片加载中...</p>
        </div>

        <!-- 错误状态 -->
        <div v-if="error" class="error-overlay">
          <n-icon :component="ImageOutline" size="48" />
          <p>{{ errorMessage }}</p>
          <n-button @click="retry">重试</n-button>
        </div>
      </div>

      <!-- 工具栏 -->
      <div class="toolbar">
        <n-space justify="center" size="small">
          <n-button-group>
            <n-button size="small" @click="zoomOut" :disabled="scale <= 0.1">
              <template #icon>
                <n-icon :component="RemoveOutline" />
              </template>
            </n-button>
            <n-button size="small" @click="resetZoom"> {{ Math.round(scale * 100) }}% </n-button>
            <n-button size="small" @click="zoomIn" :disabled="scale >= 5">
              <template #icon>
                <n-icon :component="AddOutline" />
              </template>
            </n-button>
          </n-button-group>

          <n-button-group>
            <n-button size="small" @click="rotateLeft">
              <template #icon>
                <n-icon :component="RefreshOutline" />
              </template>
              左转
            </n-button>
            <n-button size="small" @click="rotateRight">
              <template #icon>
                <n-icon :component="RefreshOutline" />
              </template>
              右转
            </n-button>
          </n-button-group>

          <n-button size="small" @click="toggleFullscreen">
            <template #icon>
              <n-icon :component="ExpandOutline" />
            </template>
            全屏
          </n-button>

          <n-button size="small" @click="downloadImage">
            <template #icon>
              <n-icon :component="DownloadOutline" />
            </template>
            下载
          </n-button>
        </n-space>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { NIcon, NSpin, NButton, NButtonGroup, NSpace, useMessage } from 'naive-ui'
import {
  ImageOutline,
  RemoveOutline,
  AddOutline,
  RefreshOutline,
  ExpandOutline,
  DownloadOutline,
} from '@vicons/ionicons5'
import { createDownloadUrl, type FileChild } from '@/api/file'
import { normalizeCreateDownloadUrlResponse } from '@/utils/responseGuards'

// Props
const props = defineProps<{
  // 参考 VideoPlayer：可直接传入 file，通过接口获取直链
  file?: FileChild
  // 兼容旧用法：直接传入 URL/名称/大小
  imageUrl?: string
  fileName?: string
  fileSize?: number
}>()

const message = useMessage()

// 响应式数据
const imageRef = ref<HTMLImageElement>()
const loading = ref(true)
const error = ref(false)
const errorMessage = ref('')
// 内部链接：当提供 file 时通过接口生成；否则使用 props.imageUrl
const innerImageUrl = ref<string>('')
// 统一对外暴露给模板的取值
const imageUrl = computed(() => (props.file ? innerImageUrl.value : props.imageUrl || ''))
// 名称/大小统一
const fileName = computed(() => (props.file ? props.file.name : props.fileName || ''))
const scale = ref(1)
const rotation = ref(0)
const translateX = ref(0)
const translateY = ref(0)
const isDragging = ref(false)
const dragStart = ref({ x: 0, y: 0 })
const imageInfo = ref<{
  width: number
  height: number
} | null>(null)
let isComponentMounted = false
let sourceRequestId = 0

// 计算属性
const imageStyle = computed(() => ({
  transform: `scale(${scale.value}) rotate(${rotation.value}deg) translate(${translateX.value}px, ${translateY.value}px)`,
  cursor: isDragging.value ? 'grabbing' : 'grab',
  transition: isDragging.value ? 'none' : 'transform 0.2s ease',
}))

// 方法
const handleImageLoad = () => {
  loading.value = false
  error.value = false

  if (imageRef.value) {
    imageInfo.value = {
      width: imageRef.value.naturalWidth,
      height: imageRef.value.naturalHeight,
    }
  }
}

const handleImageError = () => {
  // 忽略初始空 src 或切换过程中空链接导致的错误提示
  if (!imageUrl.value) {
    return
  }
  loading.value = false
  error.value = true
  errorMessage.value = '图片加载失败'
  message.error(errorMessage.value)
}

const retry = () => {
  loading.value = true
  error.value = false
  if (props.file) {
    initSource()
  } else if (imageRef.value) {
    imageRef.value.src = imageUrl.value
  }
}

const zoomIn = () => {
  scale.value = Math.min(5, scale.value * 1.2)
}

const zoomOut = () => {
  scale.value = Math.max(0.1, scale.value / 1.2)
}

const resetZoom = () => {
  scale.value = 1
  rotation.value = 0
  translateX.value = 0
  translateY.value = 0
}

const rotateLeft = () => {
  rotation.value -= 90
}

const rotateRight = () => {
  rotation.value += 90
}

const toggleFullscreen = () => {
  if (document.fullscreenElement) {
    document.exitFullscreen()
  } else {
    const container = imageRef.value?.closest('.image-viewer-container') as HTMLElement
    if (container) {
      container.requestFullscreen()
    }
  }
}

const downloadImage = () => {
  if (!imageUrl.value) return
  const link = document.createElement('a')
  link.href = imageUrl.value
  link.download = fileName.value || 'image'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  message.success('开始下载图片')
}

const handleWheel = (event: WheelEvent) => {
  event.preventDefault()

  if (event.deltaY < 0) {
    zoomIn()
  } else {
    zoomOut()
  }
}

const handleMouseDown = (event: MouseEvent) => {
  if (event.button === 0) {
    // 左键
    isDragging.value = true
    dragStart.value = {
      x: event.clientX - translateX.value,
      y: event.clientY - translateY.value,
    }

    document.addEventListener('mousemove', handleMouseMove)
    document.addEventListener('mouseup', handleMouseUp)
  }
}

const handleMouseMove = (event: MouseEvent) => {
  if (isDragging.value) {
    translateX.value = event.clientX - dragStart.value.x
    translateY.value = event.clientY - dragStart.value.y
  }
}

const handleMouseUp = () => {
  isDragging.value = false
  document.removeEventListener('mousemove', handleMouseMove)
  document.removeEventListener('mouseup', handleMouseUp)
}

const handleDoubleClick = () => {
  toggleFullscreen()
}

const handleKeydown = (event: KeyboardEvent) => {
  switch (event.code) {
    case 'Equal':
    case 'NumpadAdd':
      event.preventDefault()
      zoomIn()
      break
    case 'Minus':
    case 'NumpadSubtract':
      event.preventDefault()
      zoomOut()
      break
    case 'Digit0':
    case 'Numpad0':
      event.preventDefault()
      resetZoom()
      break
    case 'KeyF':
      event.preventDefault()
      toggleFullscreen()
      break
    case 'KeyR':
      event.preventDefault()
      if (event.shiftKey) {
        rotateLeft()
      } else {
        rotateRight()
      }
      break
  }
}

// 若传入 file，则通过接口生成直链
const initSource = () => {
  if (!props.file) return
  const fileId = props.file.id
  const requestId = ++sourceRequestId

  loading.value = true
  error.value = false
  innerImageUrl.value = ''
  createDownloadUrl({ fileId })
    .then((res) => {
      if (
        !isComponentMounted ||
        requestId !== sourceRequestId ||
        !props.file ||
        props.file.id !== fileId
      ) {
        return
      }

      if (res.code === 200) {
        const data = normalizeCreateDownloadUrlResponse(res.data)
        if (!data) {
          error.value = true
          loading.value = false
          errorMessage.value = '获取图片链接失败：响应数据格式异常'
          message.error(errorMessage.value)

          return
        }

        innerImageUrl.value = data.downloadUrl
        // 加载完成由 img 的 @load/@error 驱动
      } else {
        error.value = true
        loading.value = false
        errorMessage.value = res.msg || '获取图片链接失败'
        message.error(errorMessage.value)
      }
    })
    .catch((e) => {
      if (
        !isComponentMounted ||
        requestId !== sourceRequestId ||
        !props.file ||
        props.file.id !== fileId
      ) {
        return
      }

      console.error('createDownloadUrl error:', e)
      error.value = true
      loading.value = false
      errorMessage.value = '获取图片链接失败'
      message.error(errorMessage.value)
    })
    .finally(() => {
      // 加载态在 img onload/onerror 中最终消除，这里不处理 loading 完结
    })
}

// 生命周期
onMounted(() => {
  isComponentMounted = true
  document.addEventListener('keydown', handleKeydown)
  // 如果传入 file，则初始化直链
  if (props.file) {
    initSource()
  }
})

onUnmounted(() => {
  isComponentMounted = false
  sourceRequestId += 1
  document.removeEventListener('keydown', handleKeydown)
  document.removeEventListener('mousemove', handleMouseMove)
  document.removeEventListener('mouseup', handleMouseUp)
})

// 监听 file 切换时重新拉取直链
watch(
  () => props.file?.id,
  () => {
    if (props.file) {
      // 重置视图并刷新
      resetZoom()
      initSource()
    } else {
      sourceRequestId += 1
      innerImageUrl.value = ''
      loading.value = false
      error.value = false
    }
  }
)
</script>

<style scoped>
.image-viewer-container {
  width: 100%;
  margin: 0 auto;
}

.image-wrapper {
  background: var(--n-card-color);
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 16px;
}

.image-display {
  position: relative;
  width: 100%;
  height: 60vh;
  background: #000;
  display: flex;
  justify-content: center;
  align-items: center;
  overflow: hidden;
}

.image-element {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  user-select: none;
  -webkit-user-drag: none;
}

.loading-overlay,
.error-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  background: rgb(0 0 0 / 80%);
  color: white;
  gap: 16px;
}

.loading-overlay p,
.error-overlay p {
  margin: 0;
  font-size: 16px;
}

.toolbar {
  padding: 12px 16px;
  background: var(--n-card-color);
  border-top: 1px solid var(--n-border-color);
}

/* 全屏样式 */
.image-viewer-container:fullscreen {
  background: #000;
  padding: 0;
}

.image-viewer-container:fullscreen .image-display {
  height: 100vh;
}

.image-viewer-container:fullscreen .toolbar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 1000;
  background: rgb(0 0 0 / 80%);
  color: white;
  border-top: none;
}

/* 响应式设计 */
@media (width <= 768px) {
  .image-display {
    height: 50vh;
  }

  .toolbar .n-space {
    flex-direction: column;
    gap: 8px;
  }
}

@media (width <= 480px) {
  .toolbar {
    padding: 8px 12px;
  }

  .loading-overlay p,
  .error-overlay p {
    font-size: 14px;
  }
}
</style>
