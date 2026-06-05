<template>
  <div
    class="plyr-player-wrapper"
    :class="{ 'has-overlay': isLoading || errorText }"
    :aria-busy="isLoading"
  >
    <div ref="playerHostRef" class="player-host" v-once>
      <video ref="videoRef" class="plyr-video" playsinline></video>
    </div>
    <div v-if="isLoading" class="player-overlay" role="status">
      <n-spin size="medium" />
      <span>正在加载播放链接...</span>
    </div>
    <div v-else-if="errorText" class="player-overlay player-error" role="alert">
      <p>{{ errorText }}</p>
      <n-button size="small" type="primary" ghost @click="retryLoad">
        <template #icon>
          <n-icon :component="RefreshOutline" />
        </template>
        重新加载
      </n-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import 'plyr/dist/plyr.css'
import { NButton, NIcon, NSpin, useMessage } from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'
import { createDownloadUrl, type FileChild } from '@/api/file'
import { normalizeCreateDownloadUrlResponse } from '@/utils/responseGuards'
import { getErrorMessage } from '@/utils/api'
import type Plyr from 'plyr'
import type Hls from 'hls.js'

const props = defineProps<{
  file: FileChild
  autoplay?: boolean
  muted?: boolean
  preload?: 'auto' | 'metadata' | 'none'
}>()

const message = useMessage()

const playerHostRef = ref<HTMLElement | null>(null)
const videoRef = ref<HTMLVideoElement | null>(null)
const plyr = ref<Plyr | null>(null)
const hls = ref<Hls | null>(null)
const sourceUrl = ref('')
const isLoading = ref(false)
const errorText = ref('')
let sourceRequestId = 0
let videoErrorHandler: (() => void) | null = null
let videoReadyHandler: (() => void) | null = null
let isComponentMounted = false

const autoplay = computed(() => !!props.autoplay)
const muted = computed(() => !!props.muted)
const preload = computed(() => props.preload || 'metadata')

const isHlsByName = (name: string) => name.toLowerCase().endsWith('.m3u8')
const isHlsByUrl = (url: string) => url.toLowerCase().includes('.m3u8')
const isCurrentSource = (requestId: number) => isComponentMounted && requestId === sourceRequestId
const isExpectedVideoSource = (video: HTMLVideoElement, expectedSourceUrl?: string) => {
  if (!expectedSourceUrl) {
    return true
  }

  return (
    video.getAttribute('src') === expectedSourceUrl ||
    video.currentSrc === expectedSourceUrl ||
    video.src === expectedSourceUrl
  )
}

const showPlayerError = (text: string) => {
  errorText.value = text
  isLoading.value = false
  message.error(text)
}

const syncVideoRefFromHost = () => {
  const hostVideo = playerHostRef.value?.querySelector('video')
  if (hostVideo) {
    videoRef.value = hostVideo
  }

  return videoRef.value
}

const clearVideoReadyHandler = () => {
  const video = videoRef.value
  if (video && videoReadyHandler) {
    video.removeEventListener('loadedmetadata', videoReadyHandler)
    video.removeEventListener('canplay', videoReadyHandler)
  }

  videoReadyHandler = null
}

const clearVideoErrorHandler = () => {
  const video = videoRef.value
  if (video && videoErrorHandler) {
    video.removeEventListener('error', videoErrorHandler)
  }

  videoErrorHandler = null
}

const clearVideoEventHandlers = () => {
  clearVideoReadyHandler()
  clearVideoErrorHandler()
}

// 清空原生 video 状态，避免销毁、切换或失败后残留旧画面/播放状态。
const resetVideoSource = () => {
  const video = syncVideoRefFromHost()
  if (!video) return

  video.pause()
  video.removeAttribute('src')
  video.load()
}

// 初始化播放源并构建播放器
const initSource = async () => {
  if (!isComponentMounted) return

  const requestId = ++sourceRequestId
  sourceUrl.value = ''
  errorText.value = ''
  isLoading.value = true
  destroyPlayer()

  try {
    const res = await createDownloadUrl({ fileId: props.file.id })
    if (!isCurrentSource(requestId)) return

    if (res.code === 200) {
      const data = normalizeCreateDownloadUrlResponse(res.data)
      if (!data) {
        destroyPlayer()
        showPlayerError('获取播放链接失败：响应数据格式异常')

        return
      }

      sourceUrl.value = data.downloadUrl
      await setupPlayer(requestId)
    } else {
      destroyPlayer()
      showPlayerError(res.msg || '获取播放链接失败')
    }
  } catch (e) {
    if (!isCurrentSource(requestId)) return

    destroyPlayer()
    const errorMessage = getErrorMessage(e, '获取播放链接失败')

    console.error('createDownloadUrl error:', errorMessage)
    showPlayerError(errorMessage)
  }
}

const setupPlayer = async (requestId: number) => {
  let video = syncVideoRefFromHost()
  if (!video || !sourceUrl.value || !isCurrentSource(requestId)) return

  // 清理已有实例
  destroyPlayer()
  video = syncVideoRefFromHost()
  if (!video || !isCurrentSource(requestId)) return

  video.preload = preload.value
  video.muted = muted.value

  const name = props.file.name || ''
  const useHls = isHlsByUrl(sourceUrl.value) || isHlsByName(name)

  // HLS 优先：hls.js（非 Safari），Safari 原生 HLS
  if (useHls) {
    const HlsCtor = (await import('hls.js/light')).default
    if (!isCurrentSource(requestId)) return

    if (!HlsCtor.isSupported()) {
      await buildPlyr(requestId)
      if (!isCurrentSource(requestId)) return

      attachNativeVideoHandlers(requestId)
      activateNativeVideoSource(video)
      return
    }

    hls.value = new HlsCtor({
      // 可根据需要调整缓冲策略
      maxBufferLength: 30,
      liveDurationInfinity: true,
    })
    hls.value.loadSource(sourceUrl.value)
    hls.value.attachMedia(video)
    hls.value.on(HlsCtor.Events.MANIFEST_PARSED, () => {
      if (!isCurrentSource(requestId)) return

      buildPlyr(requestId)
        .then(() => {
          if (isCurrentSource(requestId)) {
            attachVideoErrorHandler(requestId)
            isLoading.value = false
            playIfNeeded(video)
          }
        })
        .catch((error: unknown) => {
          if (!isCurrentSource(requestId)) return

          const errorMessage = getErrorMessage(error, '视频播放器初始化失败')

          console.error('buildPlyr error:', errorMessage)
          showPlayerError(errorMessage)
        })
    })
    hls.value.on(HlsCtor.Events.ERROR, (_event, data) => {
      if (isCurrentSource(requestId) && data?.fatal) {
        const errorMessage = data?.details ? `HLS 播放失败：${data.details}` : 'HLS 播放失败'

        destroyPlayer()
        showPlayerError(errorMessage)
      }
    })
  } else {
    // 普通视频 / Safari 原生 HLS
    await buildPlyr(requestId)
    if (!isCurrentSource(requestId)) return

    attachNativeVideoHandlers(requestId)
    activateNativeVideoSource(video)
  }
}

const buildPlyr = async (requestId: number) => {
  const video = syncVideoRefFromHost()
  if (!video || !isCurrentSource(requestId)) return

  const PlyrCtor = (await import('plyr')).default
  if (!isCurrentSource(requestId)) return

  plyr.value = new PlyrCtor(video, {
    controls: [
      'play-large',
      'play',
      'progress',
      'current-time',
      'mute',
      'volume',
      'settings',
      'fullscreen',
    ],
    muted: muted.value,
    autoplay: autoplay.value,
    loadSprite: true,
    invertTime: false,
    clickToPlay: true,
    disableContextMenu: true,
  })
}

const attachVideoErrorHandler = (requestId: number, expectedSourceUrl?: string) => {
  const video = syncVideoRefFromHost()
  if (!video) return

  clearVideoErrorHandler()
  videoErrorHandler = () => {
    if (isCurrentSource(requestId) && isExpectedVideoSource(video, expectedSourceUrl)) {
      destroyPlayer()
      showPlayerError('视频播放出错')
    }
  }
  video.addEventListener('error', videoErrorHandler)
}

const attachNativeVideoHandlers = (requestId: number) => {
  const video = syncVideoRefFromHost()
  if (!video) return

  const expectedSourceUrl = sourceUrl.value

  clearVideoEventHandlers()
  videoReadyHandler = () => {
    if (!isCurrentSource(requestId) || !isExpectedVideoSource(video, expectedSourceUrl)) return

    clearVideoReadyHandler()
    isLoading.value = false
    playIfNeeded(video)
  }
  video.addEventListener('loadedmetadata', videoReadyHandler)
  video.addEventListener('canplay', videoReadyHandler)
  attachVideoErrorHandler(requestId, expectedSourceUrl)
}

const markNativeVideoReadyIfLoaded = (video: HTMLVideoElement) => {
  if (video.readyState >= 1 && videoReadyHandler) {
    videoReadyHandler()
  }
}

const activateNativeVideoSource = (video: HTMLVideoElement) => {
  video.src = sourceUrl.value
  if (preload.value === 'none' && !autoplay.value) {
    clearVideoReadyHandler()
    isLoading.value = false

    return
  }

  video.load()
  markNativeVideoReadyIfLoaded(video)
}

const destroyPlayer = () => {
  clearVideoEventHandlers()
  if (plyr.value) {
    plyr.value.destroy()
    plyr.value = null
    syncVideoRefFromHost()
  }
  if (hls.value) {
    hls.value.destroy()
    hls.value = null
  }
  resetVideoSource()
}

const playIfNeeded = (video: HTMLVideoElement) => {
  if (autoplay.value) {
    video.play().catch(() => {})
  }
}

const retryLoad = () => {
  initSource()
}

watch(
  () => props.file.id,
  () => {
    initSource()
  }
)

onMounted(() => {
  isComponentMounted = true
  initSource()
})

onUnmounted(() => {
  isComponentMounted = false
  sourceRequestId++
  destroyPlayer()
})
</script>

<style scoped>
.plyr-player-wrapper {
  position: relative;
  width: 100%;
  background: #000;
  border-radius: 8px;
  overflow: hidden;
}

.player-host {
  width: 100%;
}

.plyr-video {
  width: 100%;
  aspect-ratio: 16 / 9;
  height: auto;
  display: block;
}

.has-overlay .plyr-video {
  opacity: 0.4;
}

.player-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 20px;
  background: rgb(0 0 0 / 70%);
  color: #fff;
  text-align: center;
  z-index: 2;
}

.player-error p {
  max-width: min(520px, 100%);
  margin: 0;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

/* 全屏时扩展高度 */
:fullscreen .plyr-video {
  height: 100vh;
}

/* 响应式 */
@media (width <= 768px) {
  .plyr-video {
    max-height: 320px;
  }
}
</style>
