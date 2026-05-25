<template>
  <n-spin :show="loading">
    <div class="media-settings-page">
      <!-- 未初始化提示，使用 NAlert -->
      <template v-if="!initialized">
        <n-alert type="warning" title="初始化 STRM 能力" :bordered="false">
          <n-space vertical size="small">
            <n-text>媒体服务尚未初始化，请先完成初始化以启用 STRM 生成能力</n-text>
            <n-space>
              <n-button type="primary" @click="openInitModal">开始初始化</n-button>
            </n-space>
          </n-space>
        </n-alert>
      </template>

      <template v-else>
        <n-space vertical size="large">
          <n-descriptions
            bordered
            size="small"
            :column="1"
            label-placement="left"
            :label-style="{ width: '220px' }"
          >
            <n-descriptions-item label="启用媒体服务">
              <n-switch
                :value="!!config?.enable"
                :loading="savingEnable"
                @update:value="handleToggleEnable"
              />
              <div class="desc-sub">
                开启后，当入库文件时，会自动生成 STRM 文件，删除时也会自动删除相关文件
              </div>
            </n-descriptions-item>

            <n-descriptions-item label="存储根路径">
              <n-text>{{ config?.storagePath || '-' }}</n-text>
              <div class="desc-sub">STRM 与相关文件输出的根目录</div>
            </n-descriptions-item>

            <n-descriptions-item label="自动清理空文件夹">
              <n-tag :type="config?.autoClean ? 'success' : 'default'">
                {{ config?.autoClean ? '已启用' : '未启用' }}
              </n-tag>
              <div class="desc-sub">启用后，生成或删除文件后将自动清理空文件夹</div>
            </n-descriptions-item>

            <n-descriptions-item label="冲突策略">
              <n-text>{{
                config?.conflictPolicy === 'replace' ? 'replace（替换）' : 'skip（跳过）'
              }}</n-text>
              <div class="desc-sub">处理已存在的目标文件时的策略（跳过或替换）</div>
            </n-descriptions-item>

            <n-descriptions-item label="媒体基础 URL">
              <n-text>{{ config?.baseURL || '-' }}</n-text>
              <div class="desc-sub">用于生成外部链接等场景</div>
            </n-descriptions-item>

            <n-descriptions-item label="包括的后缀格式">
              <n-text>{{ config?.includedSuffixes?.join(', ') || '-' }}</n-text>
              <div class="desc-sub">仅支持这些后缀的文件生成 STRM</div>
            </n-descriptions-item>

            <n-descriptions-item label="定时重建strm">
              <n-tag :type="config?.autoRebuildEnable ? 'success' : 'default'">
                {{ config?.autoRebuildEnable ? '已启用' : '未启用' }}
              </n-tag>
              <div class="desc-sub" v-if="config?.autoRebuildEnable">
                每天 {{ config?.autoRebuildCron || '0 2 * * *' }} 自动重建STRM文件
              </div>
              <div class="desc-sub" v-else>启用后将按设定时间自动重建STRM文件</div>
            </n-descriptions-item>
          </n-descriptions>

          <n-space justify="space-between" align="center">
            <n-space>
              <n-popover trigger="hover" placement="top">
                <template #trigger>
                  <n-button
                    size="small"
                    type="error"
                    :loading="clearingMedia"
                    :disabled="mediaDangerActionBusy && !clearingMedia"
                    @click="handleClearMedia"
                  >
                    清理媒体文件
                  </n-button>
                </template>
                <n-text style="font-size: 12px">
                  警告：会把整个媒体目录文件全部清空，包括自己创建的文件，请谨慎操作
                </n-text>
              </n-popover>
              <n-popover trigger="hover" placement="top">
                <template #trigger>
                  <n-button
                    size="small"
                    type="warning"
                    :loading="rebuildingStrm"
                    :disabled="mediaDangerActionBusy && !rebuildingStrm"
                    @click="handleRebuildStrm"
                  >
                    重建strm文件
                  </n-button>
                </template>
                <n-text style="font-size: 12px">
                  只会给还没有创建strm的创建，已创建的不会影响。如需重新构建，请先清空再重建
                </n-text>
              </n-popover>
            </n-space>
            <n-space>
              <n-button size="small" type="primary" @click="openEditModal">编辑配置</n-button>
              <n-button size="small" type="info" @click="reload">刷新配置</n-button>
            </n-space>
          </n-space>
        </n-space>
      </template>

      <!-- 初始化弹窗 -->
      <n-modal
        v-model:show="showInitModal"
        preset="card"
        title="初始化媒体配置"
        style="width: 640px"
        :closable="!initSubmitting"
        :mask-closable="!initSubmitting"
        :close-on-esc="!initSubmitting"
      >
        <n-form :model="initForm" label-placement="left" label-width="130px">
          <n-form-item label="是否启用">
            <n-switch v-model:value="initForm.enable" />
          </n-form-item>

          <n-form-item label="存储根路径">
            <n-input v-model:value="initForm.storagePath" placeholder="media_dir" />
            <div class="desc-sub">
              <n-text type="warning"
                >⚠️ 必须配置为 "media_dir" 才能挂载成功并正常生成strm文件</n-text
              >
            </div>
          </n-form-item>

          <n-form-item label="自动清理空文件夹">
            <n-switch v-model:value="initForm.autoClean" />
          </n-form-item>

          <n-form-item label="冲突策略">
            <n-select
              v-model:value="initForm.conflictPolicy"
              :options="conflictPolicyOptions"
              placeholder="请选择冲突策略"
            />
          </n-form-item>

          <n-form-item label="媒体基础 URL">
            <n-space>
              <n-input
                v-model:value="initForm.baseURL"
                placeholder="http://localhost:12395"
                clearable
              />
              <n-button size="small" @click="autoDetectBaseURL">自动获取</n-button>
            </n-space>
          </n-form-item>

          <n-form-item label="包括的后缀格式">
            <n-dynamic-tags
              v-model:value="initForm.includedSuffixes"
              input-placeholder="添加后缀..."
              @create="handleSuffixCreate"
            />
            <div class="desc-sub">输入以 . 开头的后缀名后按回车添加，留空将支持所有类型。</div>
          </n-form-item>

          <n-form-item label="定时重建strm">
            <n-space vertical>
              <n-switch v-model:value="initForm.autoRebuildEnable" />
              <div class="desc-sub">启用后将按设定时间自动重建STRM文件（强制覆盖）</div>
            </n-space>
          </n-form-item>

          <n-form-item v-if="initForm.autoRebuildEnable" label="重建时间">
            <n-space vertical>
              <n-input
                v-model:value="initForm.autoRebuildCron"
                placeholder="0 2 * * *"
                style="width: 200px"
              />
              <div class="desc-sub">cron表达式，默认每天凌晨2点 (0 2 * * *)</div>
            </n-space>
          </n-form-item>

          <n-space justify="end">
            <n-button @click="handleCloseInitModal" :disabled="initSubmitting">取消</n-button>
            <n-button
              type="primary"
              :loading="initSubmitting"
              :disabled="initSubmitting"
              @click="handleInit"
            >
              完成初始化
            </n-button>
          </n-space>
        </n-form>
      </n-modal>

      <!-- 编辑弹窗 -->
      <n-modal
        v-model:show="showEditModal"
        preset="card"
        title="编辑媒体配置"
        style="width: 680px"
        :closable="!editSubmitting"
        :mask-closable="!editSubmitting"
        :close-on-esc="!editSubmitting"
      >
        <n-form :model="editForm" label-placement="left" label-width="130px">
          <n-form-item label="存储根路径">
            <n-input
              v-model:value="editForm.storagePath"
              placeholder="media_dir（必须配置为 media_dir 才能挂载成功）"
            />
          </n-form-item>

          <n-form-item label="自动清理空文件夹">
            <n-switch v-model:value="editForm.autoClean" />
          </n-form-item>

          <n-form-item label="冲突策略">
            <n-select
              v-model:value="editForm.conflictPolicy"
              :options="conflictPolicyOptions"
              placeholder="请选择冲突策略"
            />
          </n-form-item>

          <n-form-item label="媒体基础 URL">
            <n-space>
              <n-input
                v-model:value="editForm.baseURL"
                placeholder="http://localhost:12395"
                clearable
              />
              <n-button size="small" @click="autoDetectConfigBaseURL">自动获取</n-button>
            </n-space>
          </n-form-item>

          <n-form-item label="包括的后缀格式">
            <n-dynamic-tags
              v-model:value="editForm.includedSuffixes"
              input-placeholder="添加后缀..."
              @create="handleSuffixCreate"
            />
            <div class="desc-sub">输入以 . 开头的后缀名后按回车添加，留空将支持所有类型。</div>
          </n-form-item>

          <n-form-item label="定时重建strm">
            <n-space vertical>
              <n-switch v-model:value="editForm.autoRebuildEnable" />
              <div class="desc-sub">启用后将按设定时间自动重建STRM文件（强制覆盖）</div>
            </n-space>
          </n-form-item>

          <n-form-item v-if="editForm.autoRebuildEnable" label="重建时间">
            <n-space vertical>
              <n-input
                v-model:value="editForm.autoRebuildCron"
                placeholder="0 2 * * *"
                style="width: 200px"
              />
              <div class="desc-sub">cron表达式，默认每天凌晨2点 (0 2 * * *)</div>
            </n-space>
          </n-form-item>

          <n-space justify="end">
            <n-button @click="handleCloseEditModal" :disabled="editSubmitting">取消</n-button>
            <n-button
              type="primary"
              :loading="editSubmitting"
              :disabled="editSubmitting"
              @click="handleSaveEdit"
            >
              保存
            </n-button>
          </n-space>
        </n-form>
      </n-modal>
    </div>
  </n-spin>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, reactive } from 'vue'
import {
  NForm,
  NFormItem,
  NInput,
  NSelect,
  NSwitch,
  NSpace,
  NButton,
  NText,
  NModal,
  NSpin,
  NDescriptions,
  NDescriptionsItem,
  NTag,
  NAlert,
  NDynamicTags,
  useMessage,
  useDialog,
} from 'naive-ui'
import type { ConfigInitRequest, ConfigUpdateRequest, RebuildStrmFilesResponse } from '@/api/media'
import {
  getMediaConfigInfo,
  initMediaConfig,
  toggleMediaConfig,
  updateMediaConfig,
  clearMediaFiles,
  rebuildStrmFiles,
} from '@/api/media'
import { normalizeMediaConfigInfoResponse } from '@/utils/responseGuards'
import { getErrorMessage } from '@/utils/api'

const message = useMessage()
const dialog = useDialog()

const loading = ref<boolean>(true)
const initialized = ref<boolean>(true)
const config = ref<Models.MediaConfig | undefined>(undefined)
const showInitModal = ref(false)
const showEditModal = ref(false)
const savingEnable = ref(false)
const clearingMedia = ref(false)
const rebuildingStrm = ref(false)
const initSubmitting = ref(false)
const editSubmitting = ref(false)
const clearMediaDialogOpen = ref(false)
const rebuildStrmDialogOpen = ref(false)
const mediaDangerActionBusy = computed(
  () =>
    clearingMedia.value ||
    rebuildingStrm.value ||
    clearMediaDialogOpen.value ||
    rebuildStrmDialogOpen.value
)
let isComponentMounted = false
let configRequestId = 0
let initRequestId = 0
let editRequestId = 0

const buildDefaultEditForm = (): ConfigUpdateRequest => ({
  storagePath: '',
  autoClean: false,
  conflictPolicy: 'skip',
  baseURL: '',
  includedSuffixes: [],
  autoRebuildEnable: false,
  autoRebuildCron: '0 2 * * *',
})

const editForm = reactive<ConfigUpdateRequest>(buildDefaultEditForm())

const resetEditForm = () => {
  Object.assign(editForm, buildDefaultEditForm())
}

// 初始化表单
const initForm = reactive<ConfigInitRequest>({
  enable: true,
  storagePath: '',
  autoClean: true,
  conflictPolicy: 'skip',
  baseURL: '',
  includedSuffixes: [],
  autoRebuildEnable: false,
  autoRebuildCron: '0 2 * * *',
})

const conflictPolicyOptions = [
  { label: 'skip（跳过）', value: 'skip' },
  { label: 'replace（替换）', value: 'replace' },
]

const defaultIncludedSuffixes = [
  'mp4',
  'mkv',
  'avi',
  'mov',
  'wmv',
  'flv',
  'webm',
  'm4v',
  'mpg',
  'mpeg',
  'm2v',
  'm4p',
  'm4b',
  'ts',
  'mts',
  'm2ts',
  'm2t',
  'mxf',
  'dv',
  'dvr-ms',
  'asf',
  '3gp',
  '3g2',
  'f4v',
  'f4p',
  'f4a',
  'f4b',
  'vob',
  'ogv',
  'ogg',
  'divx',
  'xvid',
  'rm',
  'rmvb',
  'dat',
  'nsv',
  'qt',
  'amv',
  'mpv',
  'm1v',
  'svi',
  'viv',
  'fli',
  'flc',
].map((s) => `.${s}`)

const isBusinessSuccess = (response: { code: number }) => response.code === 200
const isNonNegativeSafeInteger = (value: unknown): value is number =>
  typeof value === 'number' && Number.isSafeInteger(value) && value >= 0

const isRebuildStrmFilesResponse = (result: unknown): result is RebuildStrmFilesResponse => {
  if (!result || typeof result !== 'object') {
    return false
  }

  const data = result as Partial<RebuildStrmFilesResponse>

  return (
    isNonNegativeSafeInteger(data.total) &&
    isNonNegativeSafeInteger(data.success) &&
    isNonNegativeSafeInteger(data.failed)
  )
}

const buildInitPayload = (): ConfigInitRequest => ({
  ...initForm,
  includedSuffixes: normalizeSuffixes(initForm.includedSuffixes || []),
})

const buildEditPayload = (): ConfigUpdateRequest => ({
  ...editForm,
  includedSuffixes: normalizeSuffixes(editForm.includedSuffixes || []),
})

const isCurrentConfigRequest = (requestId: number) => {
  return isComponentMounted && configRequestId === requestId
}

const isCurrentInitRequest = (requestId: number) => {
  return isComponentMounted && initRequestId === requestId
}

const isCurrentEditRequest = (requestId: number) => {
  return isComponentMounted && editRequestId === requestId
}

const reload = () => {
  if (!isComponentMounted) return

  const requestId = ++configRequestId

  loading.value = true
  getMediaConfigInfo()
    .then((res) => {
      if (!isCurrentConfigRequest(requestId)) return

      if (!isBusinessSuccess(res) || !res.data) {
        message.error(res.msg || '获取媒体配置失败')

        return
      }

      const configInfo = normalizeMediaConfigInfoResponse(res.data)
      if (!configInfo) {
        message.error('获取媒体配置失败：响应数据格式异常')

        return
      }

      initialized.value = configInfo.initialized
      if (!configInfo.initialized) {
        config.value = undefined
        resetEditForm()

        return
      }

      config.value = configInfo.config
      Object.assign(editForm, configInfo.config)
    })
    .catch((err) => {
      if (!isCurrentConfigRequest(requestId)) return

      message.error(getErrorMessage(err, '获取媒体配置失败'))
    })
    .finally(() => {
      if (isCurrentConfigRequest(requestId)) {
        loading.value = false
      }
    })
}

const handleInit = () => {
  if (initSubmitting.value) return

  // 基础校验
  if (!initForm.storagePath) {
    message.warning('请填写存储根路径')
    return
  }
  if (!initForm.baseURL) {
    message.warning('请填写媒体基础URL')
    return
  }

  const payload = buildInitPayload()
  const requestId = ++initRequestId

  initSubmitting.value = true
  initMediaConfig(payload)
    .then((res) => {
      if (!isCurrentInitRequest(requestId)) return

      if (!isBusinessSuccess(res)) {
        message.error(res.msg || '初始化媒体配置失败')

        return
      }
      message.success('初始化成功')
      showInitModal.value = false
      reload()
    })
    .catch((err) => {
      if (!isCurrentInitRequest(requestId)) return

      message.error(getErrorMessage(err, '初始化媒体配置失败'))
    })
    .finally(() => {
      if (isCurrentInitRequest(requestId)) {
        initSubmitting.value = false
      }
    })
}

const handleCloseInitModal = () => {
  if (initSubmitting.value) return

  showInitModal.value = false
}

const autoDetectBaseURL = () => {
  if (initSubmitting.value) return

  initForm.baseURL = window.location.origin
}
const autoDetectConfigBaseURL = () => {
  if (editSubmitting.value) return

  editForm.baseURL = window.location.origin
}

// 规范化后缀数组：去空、去重、统一小写
const normalizeSuffixes = (arr: string[]): string[] => {
  const out: string[] = []
  const seen = new Set<string>()
  for (const raw of arr || []) {
    const s = (raw || '').trim().toLowerCase()
    if (!s || !s.startsWith('.')) continue // 过滤无效或非 . 开头的
    if (!seen.has(s)) {
      seen.add(s)
      out.push(s)
    }
  }
  return out
}

const handleSuffixCreate = (label: string): string => {
  let s = (label || '').trim()
  if (!s.startsWith('.')) {
    s = `.${s}`
  }
  return s.toLowerCase()
}

const openInitModal = () => {
  if (initSubmitting.value) return

  if (!initForm.baseURL) {
    initForm.baseURL = window.location.origin
  }
  // 默认设置为 media_dir
  if (!initForm.storagePath) {
    initForm.storagePath = 'media_dir'
  }
  // 如果是首次初始化且后缀列表为空，则提供一组默认值
  if (!initForm.includedSuffixes || initForm.includedSuffixes.length === 0) {
    initForm.includedSuffixes = [...defaultIncludedSuffixes]
  }
  showInitModal.value = true
}

const openEditModal = () => {
  if (editSubmitting.value) return

  // 同步当前配置到编辑表单
  if (config.value) {
    Object.assign(editForm, config.value)
  }
  showEditModal.value = true
}

const handleCloseEditModal = () => {
  if (editSubmitting.value) return

  showEditModal.value = false
}

const handleSaveEdit = () => {
  if (editSubmitting.value) return

  const v = (editForm.baseURL || '').trim()
  if (v.length === 0) {
    message.warning('请输入基础 URL')
    return
  }
  if (!/^https?:\/\/.+/.test(v)) {
    message.warning('基础 URL 必须以 http:// 或 https:// 开头')
    return
  }

  const payload = buildEditPayload()
  const requestId = ++editRequestId

  editSubmitting.value = true
  updateMediaConfig(payload)
    .then((res) => {
      if (!isCurrentEditRequest(requestId)) return

      if (!isBusinessSuccess(res)) {
        message.error(res.msg || '更新媒体配置失败')

        return
      }
      message.success('更新媒体配置成功')
      showEditModal.value = false
      reload()
    })
    .catch((err) => {
      if (!isCurrentEditRequest(requestId)) return

      message.error(getErrorMessage(err, '更新媒体配置失败'))
    })
    .finally(() => {
      if (isCurrentEditRequest(requestId)) {
        editSubmitting.value = false
      }
    })
}

// 切换启用状态（与系统设置风格一致）
const handleToggleEnable = (val: boolean) => {
  if (savingEnable.value || !isComponentMounted) return

  const previousEnable = !!config.value?.enable
  savingEnable.value = true
  toggleMediaConfig({ enable: val })
    .then((res) => {
      if (!isComponentMounted) return

      if (!isBusinessSuccess(res)) {
        message.error(res.msg || '切换媒体配置启用状态失败')
        if (config.value) {
          config.value = { ...config.value, enable: previousEnable }
        }

        return
      }

      if (config.value) {
        config.value = { ...config.value, enable: val }
      }
      message.success(val ? '已启用媒体配置' : '已禁用媒体配置')
      reload()
    })
    .catch((err) => {
      if (!isComponentMounted) return

      message.error(getErrorMessage(err, '切换媒体配置启用状态失败'))
      if (config.value) {
        config.value = { ...config.value, enable: previousEnable }
      }
    })
    .finally(() => {
      if (isComponentMounted) {
        savingEnable.value = false
      }
    })
}

// 清理媒体文件
const handleClearMedia = () => {
  if (mediaDangerActionBusy.value || !isComponentMounted) return

  clearMediaDialogOpen.value = true
  dialog.warning({
    title: '确认清理媒体文件',
    content: '此操作将清空整个媒体目录，包括所有文件以及自己创建的文件，请确认是否继续？',
    positiveText: '确认清理',
    negativeText: '取消',
    onAfterLeave: () => {
      if (!clearingMedia.value) {
        clearMediaDialogOpen.value = false
      }
    },
    onPositiveClick: () => {
      if (clearingMedia.value || rebuildingStrm.value || !isComponentMounted) return false

      clearingMedia.value = true
      return clearMediaFiles()
        .then((res) => {
          if (!isComponentMounted) return

          if (!isBusinessSuccess(res)) {
            message.error(res.msg || '清理媒体文件失败')

            return
          }
          message.success('清理任务已提交，请稍后查看效果')
        })
        .catch((err) => {
          if (!isComponentMounted) return

          message.error(getErrorMessage(err, '清理媒体文件失败'))
        })
        .finally(() => {
          if (isComponentMounted) {
            clearingMedia.value = false
            clearMediaDialogOpen.value = false
          }
        })
    },
  })
}

// 重建strm文件
const handleRebuildStrm = () => {
  if (mediaDangerActionBusy.value || !isComponentMounted) return

  rebuildStrmDialogOpen.value = true
  dialog.info({
    title: '确认重建strm文件',
    content: '确认重新生成strm文件吗？只会给还没有创建strm的创建，已创建的不会影响。',
    positiveText: '确认重建',
    negativeText: '取消',
    onAfterLeave: () => {
      if (!rebuildingStrm.value) {
        rebuildStrmDialogOpen.value = false
      }
    },
    onPositiveClick: () => {
      if (rebuildingStrm.value || clearingMedia.value || !isComponentMounted) return false

      rebuildingStrm.value = true
      return rebuildStrmFiles()
        .then((res) => {
          if (!isComponentMounted) return

          if (!isBusinessSuccess(res)) {
            message.error(res.msg || '重建strm文件失败')

            return
          }

          if (!isRebuildStrmFilesResponse(res.data)) {
            message.error('响应数据格式异常')

            return
          }

          const { total, success, failed } = res.data

          if (failed > 0) {
            message.warning(
              `重建任务已派发：总计 ${total} 个，成功 ${success} 个，失败 ${failed} 个`
            )

            return
          }

          if (total === 0) {
            message.warning('重建任务没有派发：没有可处理的挂载点')

            return
          }

          message.success(`重建任务已派发：成功 ${success} 个`)
        })
        .catch((err) => {
          if (!isComponentMounted) return

          message.error(getErrorMessage(err, '重建strm文件失败'))
        })
        .finally(() => {
          if (isComponentMounted) {
            rebuildingStrm.value = false
            rebuildStrmDialogOpen.value = false
          }
        })
    },
  })
}

onMounted(() => {
  isComponentMounted = true
  reload()
})

onUnmounted(() => {
  isComponentMounted = false
  configRequestId += 1
  initRequestId += 1
  editRequestId += 1
  initSubmitting.value = false
  editSubmitting.value = false
  clearingMedia.value = false
  rebuildingStrm.value = false
  clearMediaDialogOpen.value = false
  rebuildStrmDialogOpen.value = false
})
</script>

<style scoped>
.desc-sub {
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--n-text-color-3);
}
</style>
