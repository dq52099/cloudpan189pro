<template>
  <div class="plyr-player-wrapper">
    <video ref="videoRef" class="plyr-video" playsinline></video>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import 'plyr/dist/plyr.css'
import { useMessage } from 'naive-ui'
import { createDownloadUrl, type FileChild } from '@/api/file'
import { normalizeCreateDownloadUrlResponse } from '@/utils/responseGuards'
import type Plyr from 'plyr'
import type Hls from 'hls.js'

const props = defineProps<{
  file: FileChild
  autoplay?: boolean
  muted?: boolean
  preload?: 'auto' | 'metadata' | 'none'
}>()

const message = useMessage()

const videoRef = ref<HTMLVideoElement | null>(null)
const plyr = ref<Plyr | null>(null)
const hls = ref<Hls | null>(null)
const sourceUrl = ref('')
let sourceRequestId = 0
let videoErrorHandler: (() => void) | null = null
let isComponentMounted = false

const autoplay = computed(() => !!props.autoplay)
const muted = computed(() => !!props.muted)
const preload = computed(() => props.preload || 'metadata')

const isHlsByName = (name: string) => name.toLowerCase().endsWith('.m3u8')
const isHlsByUrl = (url: string) => url.toLowerCase().includes('.m3u8')
const isCurrentSource = (requestId: number) => isComponentMounted && requestId === sourceRequestId

// 清空原生 video 状态，避免销毁、切换或失败后残留旧画面/播放状态。
const resetVideoSource = () => {
  const video = videoRef.value
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
  destroyPlayer()

  try {
    const res = await createDownloadUrl({ fileId: props.file.id })
    if (!isCurrentSource(requestId)) return

    if (res.code === 200) {
      const data = normalizeCreateDownloadUrlResponse(res.data)
      if (!data) {
        destroyPlayer()
        message.error('获取播放链接失败：响应数据格式异常')

        return
      }

      sourceUrl.value = data.downloadUrl
      await setupPlayer(requestId)
    } else {
      destroyPlayer()
      message.error(res.msg || '获取播放链接失败')
    }
  } catch (e) {
    if (!isCurrentSource(requestId)) return

    destroyPlayer()
    console.error('createDownloadUrl error:', e)
    message.error('获取播放链接失败')
  }
}

const setupPlayer = async (requestId: number) => {
  const video = videoRef.value
  if (!video || !sourceUrl.value || !isCurrentSource(requestId)) return

  // 清理已有实例
  destroyPlayer()

  const name = props.file.name || ''
  const useHls = isHlsByUrl(sourceUrl.value) || isHlsByName(name)

  // HLS 优先：hls.js（非 Safari），Safari 原生 HLS
  if (useHls) {
    const HlsCtor = (await import('hls.js/light')).default
    if (!isCurrentSource(requestId)) return

    if (!HlsCtor.isSupported()) {
      video.src = sourceUrl.value
      await buildPlyr(requestId)
      if (!isCurrentSource(requestId)) return

      playIfNeeded(video)
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
            playIfNeeded(video)
          }
        })
        .catch((error: unknown) => {
          if (!isCurrentSource(requestId)) return

          console.error('buildPlyr error:', error)
          message.error('视频播放器初始化失败')
        })
    })
    hls.value.on(HlsCtor.Events.ERROR, (_event, data) => {
      if (isCurrentSource(requestId) && data?.fatal) {
        destroyPlayer()
        message.error('HLS 播放失败')
      }
    })
  } else {
    // 普通视频 / Safari 原生 HLS
    video.src = sourceUrl.value
    await buildPlyr(requestId)
    if (!isCurrentSource(requestId)) return

    playIfNeeded(video)
  }

  if (!isCurrentSource(requestId)) return

  // 初始设置
  video.preload = preload.value
  video.muted = muted.value
}

const buildPlyr = async (requestId: number) => {
  const video = videoRef.value
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

  // 简单错误提示
  videoErrorHandler = () => {
    if (isCurrentSource(requestId)) {
      message.error('视频播放出错')
    }
  }
  video.addEventListener('error', videoErrorHandler)
}

const destroyPlayer = () => {
  const video = videoRef.value
  if (video && videoErrorHandler) {
    video.removeEventListener('error', videoErrorHandler)
    videoErrorHandler = null
  }
  if (plyr.value) {
    plyr.value.destroy()
    plyr.value = null
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

.plyr-video {
  width: 100%;
  aspect-ratio: 16 / 9;
  height: auto;
  display: block;
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
