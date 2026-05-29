<template>
  <div class="settings-page">
    <div class="page-title">站点信息</div>
    <section class="settings-section">
      <div class="setting-item">
        <div class="item-left">
          <div class="item-title">网站名称</div>
          <div class="item-desc">用于站点标题展示</div>
        </div>
        <div class="item-right">
          <div class="right-inline">
            <n-input
              v-model:value="form.title"
              placeholder="请输入网站名称"
              clearable
              maxlength="50"
              show-count
              :disabled="savingTitle"
              style="width: 320px"
              @keyup.enter="handleSaveTitle"
            />
            <n-button
              type="primary"
              size="small"
              :loading="savingTitle"
              :disabled="savingTitle || !isTitleChanged"
              @click="handleSaveTitle"
            >
              保存
            </n-button>
          </div>
        </div>
      </div>

      <div class="setting-item">
        <div class="item-left">
          <div class="item-title">基础 URL</div>
          <div class="item-desc">用于生成外部链接等场景</div>
        </div>
        <div class="item-right">
          <div class="right-inline">
            <n-button size="small" :disabled="savingBaseURL" @click="autoDetectBaseURL">
              自动获取
            </n-button>
            <n-input
              v-model:value="form.baseURL"
              placeholder="http://example.com"
              clearable
              :disabled="savingBaseURL"
              style="width: 320px"
            />
            <n-button
              size="small"
              type="primary"
              :loading="savingBaseURL"
              :disabled="savingBaseURL || !isBaseURLChanged"
              @click="handleSaveBaseURL"
            >
              保存
            </n-button>
          </div>
        </div>
      </div>

      <div class="setting-item">
        <div class="item-left">
          <div class="item-title">系统运行时间</div>
          <div class="item-desc">服务已运行时长</div>
        </div>
        <div class="item-right">
          <n-text>{{ systemInfo.runTimeHuman || '-' }}</n-text>
        </div>
      </div>
    </section>

    <div class="page-title">系统设置</div>
    <section class="settings-section">
      <!-- 用户认证 -->
      <div class="setting-item">
        <div class="item-left">
          <div class="item-title">用户认证</div>
          <div class="item-desc">开启后访问 WebDAV 需要用户登录认证</div>
        </div>
        <div class="item-right">
          <n-switch
            v-model:value="enableAuth"
            :loading="savingEnableAuth"
            :disabled="savingEnableAuth"
            @update:value="handleToggleEnableAuth"
          />
        </div>
      </div>

      <!-- 本地代理 -->
      <div class="setting-item">
        <div class="item-left">
          <div class="item-title">本地代理</div>
          <div class="item-desc">
            系统默认通过 302 跳转方式提供资源，开启后服务器代理获取资源再转发给用户
          </div>
        </div>
        <div class="item-right">
          <n-switch
            v-model:value="additionForm.localProxy"
            :loading="savingLocalProxy"
            :disabled="savingLocalProxy"
            @update:value="handleToggleLocalProxy"
          />
        </div>
      </div>

      <!-- 多线程流式下载 -->
      <div class="setting-item">
        <div class="item-left">
          <div class="item-title">多线程流式下载</div>
          <div class="item-desc">
            通过多连接分片下载大幅提升下载速度，为本地代理的增强版本。优先级大于本地代理（如果同时开启）
          </div>
        </div>
        <div class="item-right">
          <n-switch
            v-model:value="additionForm.multipleStream"
            :loading="savingMultipleStream"
            :disabled="savingMultipleStream"
            @update:value="handleToggleMultipleStream"
          />
        </div>
      </div>

      <!-- 多线程数量 -->
      <div v-if="additionForm.multipleStream" class="setting-item">
        <div class="item-left">
          <div class="item-title">多线程数量</div>
          <div class="item-desc">建议 4-8 线程，过多可能受限于网络或服务端限制</div>
        </div>
        <div class="item-right">
          <div class="right-inline">
            <n-slider
              v-model:value="additionForm.multipleStreamThreadCount"
              :min="1"
              :max="64"
              :step="1"
              :marks="threadCountMarks"
              :format-tooltip="formatThreadTooltip"
              :disabled="savingThreadCount"
              style="width: 340px"
              @change="handleThreadCountChange"
            />
            <n-button
              size="small"
              type="primary"
              :loading="savingThreadCount"
              :disabled="savingThreadCount"
              @click="handleSaveThreadCount"
            >
              保存
            </n-button>
          </div>
        </div>
      </div>

      <!-- 分片大小 -->
      <div v-if="additionForm.multipleStream" class="setting-item">
        <div class="item-left">
          <div class="item-title">分片大小</div>
          <div class="item-desc">步长 512 KiB，推荐 ≥ 1 MB。分片越大，对网络稳定性要求越高</div>
        </div>
        <div class="item-right">
          <div class="right-inline">
            <n-slider
              v-model:value="additionForm.multipleStreamChunkSize"
              :min="1048576"
              :max="67108864"
              :step="524288"
              :marks="chunkSizeMarks"
              :format-tooltip="formatChunkTooltip"
              :disabled="savingChunkSize"
              style="width: 340px"
              @change="handleChunkSizeChange"
            />
            <n-button
              size="small"
              type="primary"
              :loading="savingChunkSize"
              :disabled="savingChunkSize"
              @click="handleSaveChunkSize"
            >
              保存
            </n-button>
          </div>
        </div>
      </div>

      <!-- 任务线程数 -->
      <div v-if="additionForm.multipleStream" class="setting-item">
        <div class="item-left">
          <div class="item-title">任务线程数</div>
          <div class="item-desc">同时执行的下载任务数，建议按设备性能与带宽适度调整</div>
        </div>
        <div class="item-right">
          <div class="right-inline">
            <n-slider
              v-model:value="additionForm.taskThreadCount"
              :min="1"
              :max="32"
              :step="1"
              :marks="taskThreadMarks"
              :format-tooltip="formatTaskThreadTooltip"
              :disabled="savingTaskThreads"
              style="width: 340px"
              @change="handleTaskThreadChange"
            />
            <n-button
              size="small"
              type="primary"
              :loading="savingTaskThreads"
              :disabled="savingTaskThreads"
              @click="handleSaveTaskThreads"
            >
              保存
            </n-button>
          </div>
        </div>
      </div>

      <!-- 工作流数 -->
      <div class="setting-item">
        <div class="item-left">
          <div class="item-title">工作流数</div>
          <div class="item-desc">任务引擎的工作线程数，建议 4-8</div>
        </div>
        <div class="item-right">
          <div class="right-inline">
            <n-slider
              v-model:value="additionForm.workerCount"
              :min="1"
              :max="32"
              :step="1"
              :disabled="savingWorkerCount"
              style="width: 340px"
              @change="handleWorkerCountChange"
            />
            <n-button
              size="small"
              type="primary"
              :loading="savingWorkerCount"
              :disabled="savingWorkerCount"
              @click="handleWorkerCountChange"
            >
              保存
            </n-button>
          </div>
        </div>
      </div>

      <!-- 存储自动刷新 -->
      <div class="setting-item">
        <div class="item-left">
          <div class="item-title">存储自动刷新</div>
          <div class="item-desc">开启后，挂载的存储将自动定时刷新文件列表</div>
        </div>
        <div class="item-right">
          <n-switch
            v-model:value="additionForm.enableStorageAutoRefresh"
            :loading="savingStorageAutoRefresh"
            :disabled="savingStorageAutoRefresh"
            @update:value="handleToggleStorageAutoRefresh"
          />
        </div>
      </div>

      <div class="setting-item">
        <div class="item-left">
          <div class="item-title">普通用户 WebDAV 媒体格式限制</div>
          <div class="item-desc">
            开启后，普通用户通过 WebDAV 只能看到允许后缀列表中的文件；管理员不受影响，目录仍正常显示
          </div>
        </div>
        <div class="item-right">
          <n-switch
            v-model:value="additionForm.webdavUserStrmOnly"
            :loading="savingWebdavUserStrmOnly"
            :disabled="savingWebdavUserStrmOnly"
            @update:value="handleToggleWebdavUserStrmOnly"
          />
        </div>
      </div>

      <div class="setting-item">
        <div class="item-left">
          <div class="item-title">WebDAV 允许后缀列表</div>
          <div class="item-desc">
            仅对普通用户 WebDAV 限制生效。使用英文逗号分隔，支持不带点输入，例如 mp4, mkv, avi
          </div>
        </div>
        <div class="item-right item-right-column">
          <n-input
            v-model:value="webdavAllowedSuffixesText"
            type="textarea"
            :autosize="{ minRows: 3, maxRows: 6 }"
            placeholder=".mp4, .mkv, .avi"
            :disabled="savingWebdavAllowedSuffixes"
            style="width: 420px"
          />
          <div class="right-inline suffix-actions">
            <n-button
              size="small"
              :disabled="savingWebdavAllowedSuffixes"
              @click="resetWebdavAllowedSuffixes"
            >
              恢复默认
            </n-button>
            <n-button
              size="small"
              type="primary"
              :loading="savingWebdavAllowedSuffixes"
              :disabled="savingWebdavAllowedSuffixes"
              @click="handleSaveWebdavAllowedSuffixes"
            >
              保存
            </n-button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watchEffect, onMounted, onUnmounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { NInput, NButton, NText, useMessage, NSlider, NSwitch } from 'naive-ui'
import { useAuthStore, useSystemStore } from '@/stores'
import {
  modifySystemTitle,
  modifySystemBaseURL,
  getSystemInfo,
  getSettingAddition,
  modifySettingAddition,
  toggleSystemEnableAuth,
  type ModifySettingAdditionRequest,
} from '@/api/setting'
import { getErrorMessage } from '@/utils/api'
import { formatFileSize } from '@/utils/format'

const message = useMessage()
const router = useRouter()

const systemStore = useSystemStore()
const authStore = useAuthStore()
const systemInfo = systemStore.get()
let isSettingsMounted = false

const refreshSystemInfoAfterSave = () => {
  return systemStore.refresh().then((res) => {
    if (!isSettingsMounted) {
      return res
    }

    if (res?.code !== 200) {
      message.warning(res?.msg || '设置已保存，但刷新系统信息失败')
    }

    return res
  })
}

const defaultWebdavAllowedSuffixes = [
  '.mp4',
  '.mkv',
  '.avi',
  '.mov',
  '.wmv',
  '.flv',
  '.webm',
  '.m4v',
  '.mpg',
  '.mpeg',
  '.m2v',
  '.m4p',
  '.m4b',
  '.ts',
  '.mts',
  '.m2ts',
  '.m2t',
  '.mxf',
  '.dv',
  '.dvr-ms',
  '.asf',
  '.3gp',
  '.3g2',
  '.f4v',
  '.f4p',
  '.f4a',
  '.f4b',
  '.vob',
  '.ogv',
  '.ogg',
  '.divx',
  '.xvid',
  '.rm',
  '.rmvb',
  '.dat',
  '.nsv',
  '.qt',
  '.amv',
  '.mpv',
  '.m1v',
  '.svi',
  '.viv',
  '.fli',
  '.flc',
]

const normalizeSuffixes = (input: string): string[] => {
  const items = input
    .split(',')
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean)
    .map((item) => (item.startsWith('.') ? item : `.${item}`))

  return Array.from(new Set(items))
}

const createDefaultAddition = (): Models.SettingAddition => ({
  localProxy: false,
  localProxyURL: '',
  multipleStream: false,
  multipleStreamThreadCount: 4,
  multipleStreamChunkSize: 4 * 1024 * 1024,
  taskThreadCount: 1,
  workerCount: 5,
  enableStorageAutoRefresh: true,
  webdavUserStrmOnly: false,
  webdavAllowedSuffixes: [...defaultWebdavAllowedSuffixes],
})

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return value !== null && typeof value === 'object'
}

const normalizeBoolean = (value: unknown, fallback: boolean): boolean => {
  return typeof value === 'boolean' ? value : fallback
}

const normalizeInteger = (value: unknown, fallback: number, min: number, max?: number): number => {
  if (
    typeof value !== 'number' ||
    !Number.isSafeInteger(value) ||
    value < min ||
    (max !== undefined && value > max)
  ) {
    return fallback
  }

  return value
}

const normalizeSettingAddition = (value: unknown): Models.SettingAddition => {
  const defaults = createDefaultAddition()
  if (!isRecord(value)) {
    return defaults
  }

  const suffixes = Array.isArray(value.webdavAllowedSuffixes)
    ? normalizeSuffixes(
        value.webdavAllowedSuffixes.filter((item) => typeof item === 'string').join(',')
      )
    : defaults.webdavAllowedSuffixes

  return {
    localProxy: normalizeBoolean(value.localProxy, defaults.localProxy),
    localProxyURL:
      typeof value.localProxyURL === 'string' ? value.localProxyURL : defaults.localProxyURL,
    multipleStream: normalizeBoolean(value.multipleStream, defaults.multipleStream),
    multipleStreamThreadCount: normalizeInteger(
      value.multipleStreamThreadCount,
      defaults.multipleStreamThreadCount,
      1,
      64
    ),
    multipleStreamChunkSize: normalizeInteger(
      value.multipleStreamChunkSize,
      defaults.multipleStreamChunkSize,
      1048576,
      67108864
    ),
    taskThreadCount: normalizeInteger(value.taskThreadCount, defaults.taskThreadCount, 1, 32),
    workerCount: normalizeInteger(value.workerCount, defaults.workerCount, 1, 32),
    enableStorageAutoRefresh: normalizeBoolean(
      value.enableStorageAutoRefresh,
      defaults.enableStorageAutoRefresh
    ),
    webdavUserStrmOnly: normalizeBoolean(value.webdavUserStrmOnly, defaults.webdavUserStrmOnly),
    webdavAllowedSuffixes: suffixes.length ? suffixes : [...defaultWebdavAllowedSuffixes],
  }
}

// 表单状态
const form = reactive({
  title: systemInfo.title || '',
  baseURL: systemInfo.baseURL || '',
})
const lastSyncedTitle = ref(systemInfo.title || '')
const lastSyncedBaseURL = ref(systemInfo.baseURL || '')

watchEffect(() => {
  const nextTitle = systemInfo.title || ''
  const nextBaseURL = systemInfo.baseURL || ''

  if ((form.title || '') === lastSyncedTitle.value) {
    form.title = nextTitle
  }

  if ((form.baseURL || '') === lastSyncedBaseURL.value) {
    form.baseURL = nextBaseURL
  }

  lastSyncedTitle.value = nextTitle
  lastSyncedBaseURL.value = nextBaseURL
})

// ===== 用户认证开关 =====
const enableAuth = ref<boolean>(systemInfo.enableAuth || false)
const savingEnableAuth = ref(false)
const handleToggleEnableAuth = async (val: boolean) => {
  if (savingEnableAuth.value || !isSettingsMounted) return

  savingEnableAuth.value = true
  try {
    const res = await toggleSystemEnableAuth(val)
    if (!isSettingsMounted) return

    if (res.code !== 200) {
      message.error(res.msg || '保存失败')
      enableAuth.value = !val

      return
    }

    message.success('设置已保存')
    await refreshSystemInfoAfterSave()
    if (!isSettingsMounted) return

    const nextEnableAuth = systemStore.get().enableAuth
    enableAuth.value = nextEnableAuth

    if (!authStore.isLogin) {
      router.replace('/@login')
    }
  } catch (err) {
    if (!isSettingsMounted) return

    message.error(getErrorMessage(err, '网络错误'))
    enableAuth.value = !val
  } finally {
    if (isSettingsMounted) {
      savingEnableAuth.value = false
    }
  }
}

// ===== 附加设置表单 =====
const originalAddition = ref<Models.SettingAddition | null>(null)
const additionForm = reactive<Models.SettingAddition>(createDefaultAddition())
const webdavAllowedSuffixesText = ref(defaultWebdavAllowedSuffixes.join(', '))

// 初始化完成标记，防止初始渲染触发自动保存
const additionLoaded = ref(false)

// 单字段保存的 loading 状态
const savingLocalProxy = ref(false)
const savingMultipleStream = ref(false)
const savingThreadCount = ref(false)
const savingChunkSize = ref(false)
const savingTaskThreads = ref(false)
const savingWorkerCount = ref(false)
const savingStorageAutoRefresh = ref(false)
const savingWebdavUserStrmOnly = ref(false)
const savingWebdavAllowedSuffixes = ref(false)
const inFlightAdditionPayloads = new Map<string, ModifySettingAdditionRequest>()
const pendingAdditionPayloads = new Map<string, ModifySettingAdditionRequest>()

const cloneAddition = (addition: Models.SettingAddition): Models.SettingAddition => ({
  ...addition,
  webdavAllowedSuffixes: [...addition.webdavAllowedSuffixes],
})

const cloneAdditionPayload = (
  payload: ModifySettingAdditionRequest
): ModifySettingAdditionRequest => ({
  ...payload,
  webdavAllowedSuffixes: payload.webdavAllowedSuffixes
    ? [...payload.webdavAllowedSuffixes]
    : undefined,
})

const getAdditionPayloadKey = (payload: ModifySettingAdditionRequest) =>
  Object.keys(payload).sort().join(',')

const areStringArraysEqual = (a: string[] | undefined, b: string[] | undefined) => {
  if (a === b) return true
  if (!a || !b || a.length !== b.length) return false

  return a.every((item, index) => item === b[index])
}

const areAdditionPayloadsEqual = (
  a: ModifySettingAdditionRequest,
  b: ModifySettingAdditionRequest
) => {
  return (
    a.localProxy === b.localProxy &&
    a.multipleStream === b.multipleStream &&
    a.multipleStreamThreadCount === b.multipleStreamThreadCount &&
    a.multipleStreamChunkSize === b.multipleStreamChunkSize &&
    a.taskThreadCount === b.taskThreadCount &&
    a.workerCount === b.workerCount &&
    a.enableStorageAutoRefresh === b.enableStorageAutoRefresh &&
    a.webdavUserStrmOnly === b.webdavUserStrmOnly &&
    areStringArraysEqual(a.webdavAllowedSuffixes, b.webdavAllowedSuffixes)
  )
}

const isAdditionPayloadUnchanged = (payload: ModifySettingAdditionRequest) => {
  const saved = originalAddition.value
  if (!saved) return false

  return (
    (payload.localProxy === undefined || payload.localProxy === saved.localProxy) &&
    (payload.multipleStream === undefined || payload.multipleStream === saved.multipleStream) &&
    (payload.multipleStreamThreadCount === undefined ||
      payload.multipleStreamThreadCount === saved.multipleStreamThreadCount) &&
    (payload.multipleStreamChunkSize === undefined ||
      payload.multipleStreamChunkSize === saved.multipleStreamChunkSize) &&
    (payload.taskThreadCount === undefined || payload.taskThreadCount === saved.taskThreadCount) &&
    (payload.workerCount === undefined || payload.workerCount === saved.workerCount) &&
    (payload.enableStorageAutoRefresh === undefined ||
      payload.enableStorageAutoRefresh === saved.enableStorageAutoRefresh) &&
    (payload.webdavUserStrmOnly === undefined ||
      payload.webdavUserStrmOnly === saved.webdavUserStrmOnly) &&
    (payload.webdavAllowedSuffixes === undefined ||
      areStringArraysEqual(payload.webdavAllowedSuffixes, saved.webdavAllowedSuffixes))
  )
}

const commitAdditionPayload = (payload: ModifySettingAdditionRequest) => {
  const saved = originalAddition.value
    ? cloneAddition(originalAddition.value)
    : cloneAddition(additionForm)

  if (payload.localProxy !== undefined) saved.localProxy = payload.localProxy
  if (payload.multipleStream !== undefined) saved.multipleStream = payload.multipleStream
  if (payload.multipleStreamThreadCount !== undefined) {
    saved.multipleStreamThreadCount = payload.multipleStreamThreadCount
  }
  if (payload.multipleStreamChunkSize !== undefined) {
    saved.multipleStreamChunkSize = payload.multipleStreamChunkSize
  }
  if (payload.taskThreadCount !== undefined) saved.taskThreadCount = payload.taskThreadCount
  if (payload.workerCount !== undefined) saved.workerCount = payload.workerCount
  if (payload.enableStorageAutoRefresh !== undefined) {
    saved.enableStorageAutoRefresh = payload.enableStorageAutoRefresh
  }
  if (payload.webdavUserStrmOnly !== undefined) {
    saved.webdavUserStrmOnly = payload.webdavUserStrmOnly
  }
  if (payload.webdavAllowedSuffixes !== undefined) {
    saved.webdavAllowedSuffixes = [...payload.webdavAllowedSuffixes]
  }

  originalAddition.value = saved
}

const rollbackAdditionPayload = (payload: ModifySettingAdditionRequest) => {
  const saved = originalAddition.value
  if (!saved) return

  if (payload.localProxy !== undefined) additionForm.localProxy = saved.localProxy
  if (payload.multipleStream !== undefined) additionForm.multipleStream = saved.multipleStream
  if (payload.multipleStreamThreadCount !== undefined) {
    additionForm.multipleStreamThreadCount = saved.multipleStreamThreadCount
  }
  if (payload.multipleStreamChunkSize !== undefined) {
    additionForm.multipleStreamChunkSize = saved.multipleStreamChunkSize
  }
  if (payload.taskThreadCount !== undefined) additionForm.taskThreadCount = saved.taskThreadCount
  if (payload.workerCount !== undefined) additionForm.workerCount = saved.workerCount
  if (payload.enableStorageAutoRefresh !== undefined) {
    additionForm.enableStorageAutoRefresh = saved.enableStorageAutoRefresh
  }
  if (payload.webdavUserStrmOnly !== undefined) {
    additionForm.webdavUserStrmOnly = saved.webdavUserStrmOnly
  }
  if (payload.webdavAllowedSuffixes !== undefined) {
    additionForm.webdavAllowedSuffixes = [...saved.webdavAllowedSuffixes]
    webdavAllowedSuffixesText.value = additionForm.webdavAllowedSuffixes.join(', ')
  }
}

// 通用保存函数：仅提交传入字段
const saveAdditionField = (
  payload: ModifySettingAdditionRequest,
  setLoading: (v: boolean) => void,
  isSaving: () => boolean
) => {
  if (!isSettingsMounted) return
  if (!additionLoaded.value) {
    message.warning('附加设置尚未加载，请刷新后重试')

    return
  }

  const payloadKey = getAdditionPayloadKey(payload)
  const nextPayload = cloneAdditionPayload(payload)
  if (isSaving()) {
    const inFlightPayload = inFlightAdditionPayloads.get(payloadKey)
    const pendingPayload = pendingAdditionPayloads.get(payloadKey)
    if (pendingPayload && areAdditionPayloadsEqual(nextPayload, pendingPayload)) {
      return
    }

    if (inFlightPayload && areAdditionPayloadsEqual(nextPayload, inFlightPayload)) {
      pendingAdditionPayloads.delete(payloadKey)

      return
    }

    pendingAdditionPayloads.set(payloadKey, nextPayload)

    return
  }

  if (isAdditionPayloadUnchanged(nextPayload)) {
    pendingAdditionPayloads.delete(payloadKey)

    return
  }

  const requestPayload = nextPayload
  inFlightAdditionPayloads.set(payloadKey, cloneAdditionPayload(requestPayload))
  setLoading(true)
  modifySettingAddition(requestPayload)
    .then((res) => {
      if (!isSettingsMounted) return

      if (res.code === 200) {
        message.success('已保存')
        commitAdditionPayload(requestPayload)
      } else {
        message.error(res.msg || '保存失败')
        if (!pendingAdditionPayloads.has(payloadKey)) {
          rollbackAdditionPayload(requestPayload)
        }
      }
    })
    .catch((err) => {
      if (!isSettingsMounted) return

      message.error(getErrorMessage(err, '网络错误'))
      if (!pendingAdditionPayloads.has(payloadKey)) {
        rollbackAdditionPayload(requestPayload)
      }
    })
    .finally(() => {
      if (!isSettingsMounted) return

      inFlightAdditionPayloads.delete(payloadKey)
      setLoading(false)
      const pendingPayload = pendingAdditionPayloads.get(payloadKey)
      if (pendingPayload) {
        pendingAdditionPayloads.delete(payloadKey)
        saveAdditionField(pendingPayload, setLoading, isSaving)
      }
    })
}

// 工作流数量保存
const handleWorkerCountChange = () => {
  saveAdditionField(
    { workerCount: additionForm.workerCount },
    (v) => (savingWorkerCount.value = v),
    () => savingWorkerCount.value
  )
}

// 存储自动刷新保存
const handleToggleStorageAutoRefresh = (val: boolean) => {
  saveAdditionField(
    { enableStorageAutoRefresh: val },
    (v) => (savingStorageAutoRefresh.value = v),
    () => savingStorageAutoRefresh.value
  )
}

const handleToggleWebdavUserStrmOnly = (val: boolean) => {
  saveAdditionField(
    { webdavUserStrmOnly: val },
    (v) => (savingWebdavUserStrmOnly.value = v),
    () => savingWebdavUserStrmOnly.value
  )
}

const handleSaveWebdavAllowedSuffixes = () => {
  const normalized = normalizeSuffixes(webdavAllowedSuffixesText.value)
  additionForm.webdavAllowedSuffixes = normalized.length
    ? normalized
    : [...defaultWebdavAllowedSuffixes]
  webdavAllowedSuffixesText.value = additionForm.webdavAllowedSuffixes.join(', ')
  saveAdditionField(
    { webdavAllowedSuffixes: additionForm.webdavAllowedSuffixes },
    (v) => (savingWebdavAllowedSuffixes.value = v),
    () => savingWebdavAllowedSuffixes.value
  )
}

const resetWebdavAllowedSuffixes = () => {
  additionForm.webdavAllowedSuffixes = [...defaultWebdavAllowedSuffixes]
  webdavAllowedSuffixesText.value = additionForm.webdavAllowedSuffixes.join(', ')
}

// 开机关联保存
const handleToggleLocalProxy = (val: boolean) => {
  saveAdditionField(
    { localProxy: val },
    (v) => (savingLocalProxy.value = v),
    () => savingLocalProxy.value
  )
}
const handleToggleMultipleStream = (val: boolean) => {
  saveAdditionField(
    { multipleStream: val },
    (v) => (savingMultipleStream.value = v),
    () => savingMultipleStream.value
  )
}

// Slider change事件处理
const handleThreadCountChange = () => {
  // 当slider值改变时自动保存
  if (additionLoaded.value) {
    handleSaveThreadCount()
  }
}
const handleChunkSizeChange = () => {
  // 当slider值改变时自动保存
  if (additionLoaded.value) {
    handleSaveChunkSize()
  }
}
const handleTaskThreadChange = () => {
  // 当slider值改变时自动保存
  if (additionLoaded.value) {
    handleSaveTaskThreads()
  }
}

// 数值保存函数
const handleSaveThreadCount = () => {
  saveAdditionField(
    { multipleStreamThreadCount: additionForm.multipleStreamThreadCount },
    (v) => (savingThreadCount.value = v),
    () => savingThreadCount.value
  )
}
const handleSaveChunkSize = () => {
  saveAdditionField(
    { multipleStreamChunkSize: additionForm.multipleStreamChunkSize },
    (v) => (savingChunkSize.value = v),
    () => savingChunkSize.value
  )
}
const handleSaveTaskThreads = () => {
  saveAdditionField(
    { taskThreadCount: additionForm.taskThreadCount },
    (v) => (savingTaskThreads.value = v),
    () => savingTaskThreads.value
  )
}

// Slider 标记
const threadCountMarks: Record<number, string> = {
  1: '1',
  4: '4',
  8: '8',
  16: '16',
  32: '32',
  64: '64',
}
const formatThreadTooltip = (val: number) => `${val} 线程`

const chunkSizeMarks: Record<number, string> = {
  1048576: '1 MB',
  4194304: '4 MB',
  8388608: '8 MB',
  16777216: '16 MB',
  33554432: '32 MB',
  67108864: '64 MB',
}
const formatChunkTooltip = (val: number) => formatFileSize(val)

const taskThreadMarks: Record<number, string> = {
  1: '1',
  4: '4',
  8: '8',
  16: '16',
  24: '24',
  32: '32',
}
const formatTaskThreadTooltip = (val: number) => `${val} 任务`

// 标题/URL修改
const savingTitle = ref(false)
const savingBaseURL = ref(false)
const isTitleChanged = computed(() => (form.title || '').trim() !== (systemInfo.title || '').trim())
const isBaseURLChanged = computed(
  () => (form.baseURL || '').trim() !== (systemInfo.baseURL || '').trim()
)

const handleSaveTitle = async () => {
  if (savingTitle.value) return

  const newTitle = (form.title || '').trim()
  if (newTitle.length === 0) {
    message.warning('请输入网站名称')
    return
  }

  const previousTitle = systemInfo.title || ''

  savingTitle.value = true
  try {
    const res = await modifySystemTitle(newTitle)
    if (!isSettingsMounted) return

    if (res.code === 200) {
      message.success('网站名称已更新')
      await refreshSystemInfoAfterSave()
    } else {
      message.error(res.msg || '更新失败')
      form.title = previousTitle
    }
  } catch (err) {
    if (!isSettingsMounted) return

    message.error(getErrorMessage(err, '网络错误'))
    form.title = previousTitle
  } finally {
    if (isSettingsMounted) {
      savingTitle.value = false
    }
  }
}

const autoDetectBaseURL = () => {
  if (savingBaseURL.value) return

  form.baseURL = window.location.origin
}
const handleSaveBaseURL = async () => {
  if (savingBaseURL.value) return

  const newBaseURL = (form.baseURL || '').trim()
  if (newBaseURL.length === 0) {
    message.warning('请输入基础 URL')
    return
  }
  if (!/^https?:\/\/.+/.test(newBaseURL)) {
    message.warning('基础 URL 必须以 http:// 或 https:// 开头')
    return
  }

  const previousBaseURL = systemInfo.baseURL || ''

  savingBaseURL.value = true
  try {
    const res = await modifySystemBaseURL(newBaseURL)
    if (!isSettingsMounted) return

    if (res.code === 200) {
      message.success('基础 URL 已更新')
      await refreshSystemInfoAfterSave()
    } else {
      message.error(res.msg || '更新失败')
      form.baseURL = previousBaseURL
    }
  } catch (err) {
    if (!isSettingsMounted) return

    message.error(getErrorMessage(err, '网络错误'))
    form.baseURL = previousBaseURL
  } finally {
    if (isSettingsMounted) {
      savingBaseURL.value = false
    }
  }
}

// 初始化
onMounted(() => {
  isSettingsMounted = true

  systemStore
    .refresh()
    .then((res) => {
      if (!isSettingsMounted) return

      enableAuth.value = systemStore.get().enableAuth || false
      if (res?.code !== 200) {
        getSystemInfo()
          .then((r) => {
            if (!isSettingsMounted) return

            if (r.data) {
              enableAuth.value = r.data.enableAuth || false
            }
          })
          .catch((err) => {
            if (!isSettingsMounted) return

            message.error(getErrorMessage(err, '获取系统信息失败'))
          })
      }
    })
    .catch((err) => {
      if (!isSettingsMounted) return

      message.error(getErrorMessage(err, '获取系统信息失败'))
    })

  let loadedAddition = false

  getSettingAddition()
    .then((res) => {
      if (!isSettingsMounted) return

      if (res.code === 200 && res.data) {
        const normalizedAddition = normalizeSettingAddition(res.data)
        Object.assign(additionForm, normalizedAddition)
        webdavAllowedSuffixesText.value = additionForm.webdavAllowedSuffixes.join(', ')
        originalAddition.value = cloneAddition(additionForm)
        loadedAddition = true
      } else {
        message.error(res.msg || '获取附加设置失败')
      }
    })
    .catch((err) => {
      if (!isSettingsMounted) return

      message.error(getErrorMessage(err, '获取附加设置失败'))
    })
    .finally(() => {
      if (!isSettingsMounted) return

      additionLoaded.value = loadedAddition
    })
})

onUnmounted(() => {
  isSettingsMounted = false
  inFlightAdditionPayloads.clear()
  pendingAdditionPayloads.clear()
})
</script>

<style scoped>
.settings-page {
  padding: 16px 24px 32px;
  background: var(--n-color-target);
}

/* 扁平化页面标题 */
.page-title {
  font-size: 18px;
  font-weight: 600;
  margin: 8px 0 12px;
  color: var(--n-text-color);
}

/* 区块容器（无卡片边框与重阴影，更贴近参考） */
.settings-section {
  background: var(--n-card-color);
  border: 1px solid var(--n-border-color);
  border-radius: 10px;
  box-shadow: 0 1px 6px rgb(0 0 0 / 8%);
  margin-bottom: 20px;
  padding: 8px 16px;
}

/* 单行项目 */
.setting-item {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px 4px;
  border-bottom: 1px solid var(--n-border-color);
}

.setting-item:last-child {
  border-bottom: none;
}

/* 左侧文案 */
.item-left {
  flex: 0 0 560px;
  display: flex;
  flex-direction: column;
}

.item-title {
  font-weight: 600;
  color: var(--n-text-color);
}

.item-desc {
  color: var(--n-text-color-3);
  font-size: 12px;
  margin-top: 6px;
  line-height: 1.6;
}

/* 右侧控件区 */
.item-right {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: flex-end;
}

.item-right-column {
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
}

.right-inline {
  display: flex;
  align-items: center;
  gap: 8px;
}

.suffix-actions {
  justify-content: flex-end;
}

/* Switch/Slider 细节优化 */
:deep(.n-switch) {
  box-shadow: 0 1px 4px rgb(0 0 0 / 10%);
}

:deep(.n-slider-rail) {
  height: 6px;
}

:deep(.n-slider-handle) {
  width: 12px;
  height: 12px;
  box-shadow: 0 2px 8px rgb(0 0 0 / 12%);
}

:deep(.n-slider-marks) {
  font-size: 12px;
  color: var(--n-text-color-3);
}

/* 响应式 */
@media (width <= 1024px) {
  .item-left {
    flex: 1 1 auto;
  }
}

@media (width <= 768px) {
  .settings-section {
    padding: 8px 12px;
    border-radius: 8px;
  }

  .setting-item {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }

  .item-right {
    justify-content: flex-start;
  }

  .item-right-column {
    align-items: stretch;
  }

  .suffix-actions {
    justify-content: flex-start;
  }
}
</style>
