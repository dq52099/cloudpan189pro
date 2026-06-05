<template>
  <n-modal
    v-model:show="show"
    preset="dialog"
    title="新建入库计划"
    :closable="!submitting"
    :mask-closable="false"
    :close-on-esc="!submitting"
  >
    <div class="create-plan-modal">
      <!-- 第1段：入库类型与订阅号解析 -->
      <n-form
        ref="typeFormRef"
        :model="typeForm"
        :rules="typeRules"
        label-placement="left"
        label-width="100"
        class="section"
      >
        <n-form-item label="入库类型" path="sourceType">
          <n-select
            v-model:value="typeForm.sourceType"
            :options="AUTO_INGEST_SOURCE_TYPE_OPTIONS"
            placeholder="请选择入库类型"
            :disabled="parsing || submitting"
            clearable
          />
        </n-form-item>

        <template v-if="typeForm.sourceType === 'subscribe'">
          <n-form-item label="订阅号ID">
            <div class="row">
              <n-input
                v-model:value="subscribeUserIdInput"
                :disabled="parsing || submitting || (parsedLocked && !!parsedSubscribeUserId)"
                placeholder="请输入订阅号ID或订阅号链接"
                class="subscribe-id-input"
              />
              <!-- 解析/编辑按钮互斥显示 -->
              <n-button
                v-if="!(parsedLocked && !!parsedSubscribeUserId)"
                type="primary"
                size="small"
                :loading="parsing"
                :disabled="parsing || submitting"
                @click="handleParseSubscribe"
              >
                <template #icon>
                  <n-icon>
                    <SearchOutline />
                  </n-icon>
                </template>
                解析
              </n-button>
              <n-button
                v-else
                size="small"
                type="warning"
                secondary
                :disabled="submitting"
                @click="handleEnableEditSubscribe"
              >
                <template #icon>
                  <n-icon>
                    <CreateOutline />
                  </n-icon>
                </template>
                编辑
              </n-button>
            </div>
          </n-form-item>

          <div v-if="parsedSubscribeUserId" class="parsed-summary">
            <n-tag type="success" size="small" bordered>
              已解析：{{ parsedUserName || '-' }}（ID: {{ parsedSubscribeUserId }}）
            </n-tag>
            <n-tag size="small" bordered> 已分享文件数：{{ parsedShareTotal ?? '-' }} </n-tag>
          </div>
        </template>
      </n-form>

      <!-- 第2段：详细信息（必须在解析成功后显示） -->
      <n-form
        v-if="canShowDetailForm"
        ref="detailFormRef"
        :model="detailForm"
        :rules="detailRules"
        label-placement="left"
        label-width="100"
        class="section"
      >
        <n-form-item label="计划名称" path="name">
          <n-input
            v-model:value="detailForm.name"
            placeholder="例如：入库计划A"
            :disabled="submitting"
          />
        </n-form-item>

        <n-form-item label="挂载父目录" path="parentPath">
          <n-input
            v-model:value="detailForm.parentPath"
            placeholder="/Movies"
            :disabled="submitting"
          />
        </n-form-item>

        <n-form-item label="绑定令牌" path="cloudToken">
          <n-select
            v-model:value="detailForm.cloudToken"
            :options="cloudTokenOptions"
            placeholder="可选"
            clearable
            filterable
            :disabled="submitting"
          />
        </n-form-item>

        <n-form-item label="冲突处理策略" path="onConflict">
          <n-radio-group v-model:value="detailForm.onConflict" :disabled="submitting">
            <n-space>
              <n-radio
                v-for="opt in AUTO_INGEST_ON_CONFLICT_OPTIONS"
                :key="opt.value"
                :value="opt.value"
              >
                {{ opt.label }}
              </n-radio>
            </n-space>
          </n-radio-group>
        </n-form-item>

        <n-form-item label="自动扫描间隔(分钟)" path="autoIngestInterval">
          <n-input-number
            v-model:value="detailForm.autoIngestInterval"
            :min="AUTO_INGEST_INTERVAL_MIN"
            :max="REFRESH_INTERVAL_MAX"
            :disabled="submitting"
          />
        </n-form-item>

        <n-form-item label="启用计划" path="enable" :show-feedback="false">
          <n-switch v-model:value="detailForm.enable" :disabled="submitting" />
        </n-form-item>

        <n-form-item label="一键添加历史" path="oneClickAddHistory" :show-feedback="false">
          <div class="row">
            <n-switch v-model:value="detailForm.oneClickAddHistory" :disabled="submitting" />
            <span
              v-if="detailForm.oneClickAddHistory && parsedShareTotal !== undefined"
              class="hint-inline"
            >
              启用后将直接入库约 {{ parsedShareTotal }} 个分享
            </span>
          </div>
        </n-form-item>

        <n-divider title-placement="left">刷新策略（可选）</n-divider>

        <n-form-item label="启用自动刷新" path="refreshStrategy.enableAutoRefresh">
          <n-switch
            v-model:value="detailForm.refreshStrategy.enableAutoRefresh"
            :disabled="submitting"
          />
        </n-form-item>

        <template v-if="detailForm.refreshStrategy.enableAutoRefresh">
          <n-form-item label="刷新间隔(分钟)" path="refreshStrategy.refreshInterval">
            <n-input-number
              v-model:value="detailForm.refreshStrategy.refreshInterval"
              :min="REFRESH_INTERVAL_MIN"
              :max="REFRESH_INTERVAL_MAX"
              :disabled="submitting"
            />
          </n-form-item>
          <n-form-item label="持续天数" path="refreshStrategy.autoRefreshDays">
            <n-input-number
              v-model:value="detailForm.refreshStrategy.autoRefreshDays"
              :min="AUTO_REFRESH_DAYS_MIN"
              :max="AUTO_REFRESH_DAYS_MAX"
              :disabled="submitting"
            />
          </n-form-item>
          <n-form-item label="深度刷新" path="refreshStrategy.enableDeepRefresh">
            <n-switch
              v-model:value="detailForm.refreshStrategy.enableDeepRefresh"
              :disabled="submitting"
            />
          </n-form-item>
        </template>
      </n-form>
    </div>

    <template #action>
      <n-space>
        <n-button :disabled="submitting" @click="handleCancel">取消</n-button>
        <n-button type="primary" :loading="submitting" :disabled="submitting" @click="handleSubmit">
          确认创建
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onUnmounted } from 'vue'
import {
  NModal,
  NForm,
  NFormItem,
  NSelect,
  NInput,
  NInputNumber,
  NRadioGroup,
  NRadio,
  NSwitch,
  NDivider,
  NButton,
  NIcon,
  NTag,
  NSpace,
  useMessage,
  type FormInst,
  type FormRules,
} from 'naive-ui'
import {
  AUTO_INGEST_SOURCE_TYPE_OPTIONS,
  AUTO_INGEST_ON_CONFLICT_OPTIONS,
  AUTO_INGEST_INTERVAL_MIN,
  REFRESH_INTERVAL_MIN,
  REFRESH_INTERVAL_MAX,
  AUTO_REFRESH_DAYS_MIN,
  AUTO_REFRESH_DAYS_MAX,
  type AutoIngestSourceType,
} from '@/constants/autoIngest'
import { SearchOutline, CreateOutline } from '@vicons/ionicons5'
import { getSubscribeUser, type GetSubscribeUserResponse } from '@/api/storage/advance'
import {
  createSubscribePlan,
  type CreateSubscribePlanRequest,
  type CreateSubscribePlanResponse,
} from '@/api/autoingest'
import { getErrorMessage, type ApiResponse } from '@/utils/api'
import {
  normalizeCreateSubscribePlanResponse,
  normalizeGetSubscribeUserResponse,
} from '@/utils/responseGuards'
import { normalizeCloud189SubscribeUserInput } from '@/utils/subscribeUser'

const message = useMessage()

// v-model:show
const props = defineProps<{
  show: boolean
  cloudTokenOptions: { label: string; value: number }[]
}>()
const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'created'): void
}>()

const show = ref(props.show)

let isComponentMounted = true
let operationVersion = 0
let parseRequestId = 0
let submitRequestId = 0

const isCurrentOperation = (version: number) => {
  return isComponentMounted && show.value && operationVersion === version
}

const invalidatePendingWork = () => {
  operationVersion++
}

const isCurrentParseRequest = (requestId: number, currentOperation: number) => {
  return isCurrentOperation(currentOperation) && parseRequestId === requestId
}

const isCurrentSubmitRequest = (requestId: number, currentOperation: number) => {
  return isCurrentOperation(currentOperation) && submitRequestId === requestId
}

watch(
  () => props.show,
  (v) => (show.value = v)
)
watch(show, (v, ov) => {
  emit('update:show', v)
  if (v && !ov) {
    invalidatePendingWork()
    applyDefaultCloudToken()

    return
  }

  // 当用户通过右上角关闭或父组件收起时，自动重置表单，避免再次打开保留上次内容
  if (!v && ov) {
    invalidatePendingWork()
    parsing.value = false
    submitting.value = false
    resetAll()
  }
})

// 顶部类型与订阅解析
const typeFormRef = ref<FormInst | null>(null)
const typeForm = reactive<{
  sourceType?: AutoIngestSourceType
}>({
  sourceType: 'subscribe',
})

const subscribeUserIdInput = ref<string>('')
const parsedSubscribeUserId = ref<string | undefined>(undefined)
const parsedUserName = ref<string | undefined>(undefined)
const parsedShareTotal = ref<number | undefined>(undefined)
const parsedLocked = ref<boolean>(false)
const parsing = ref<boolean>(false)

const clearParsedSubscribe = () => {
  parsedSubscribeUserId.value = undefined
  parsedUserName.value = undefined
  parsedShareTotal.value = undefined
  parsedLocked.value = false
}

const typeRules: FormRules = {
  sourceType: [{ required: true, message: '请选择入库类型', trigger: 'change' }],
}

const canShowDetailForm = computed(
  () => typeForm.sourceType === 'subscribe' && !!parsedSubscribeUserId.value
)

// 详细表单
const detailFormRef = ref<FormInst | null>(null)
const detailForm = reactive<
  CreateSubscribePlanRequest & {
    enable: boolean
    refreshStrategy: NonNullable<CreateSubscribePlanRequest['refreshStrategy']>
  }
>({
  name: '',
  parentPath: '',
  upUserId: '', // 注意：提交时使用 parsedSubscribeUserId
  enable: true,
  cloudToken: undefined,
  onConflict: 'abandon',
  autoIngestInterval: 30,
  oneClickAddHistory: false,
  refreshStrategy: {
    enableAutoRefresh: false,
    autoRefreshDays: 7,
    refreshInterval: 30,
    enableDeepRefresh: false,
  },
})

function applyDefaultCloudToken() {
  if (!show.value || detailForm.cloudToken || props.cloudTokenOptions.length === 0) {
    return
  }

  detailForm.cloudToken = props.cloudTokenOptions[0].value
}

watch(
  () => props.cloudTokenOptions,
  () => {
    applyDefaultCloudToken()
  },
  { immediate: true }
)

watch(subscribeUserIdInput, (value) => {
  if (parsedLocked.value || !parsedSubscribeUserId.value || value === parsedSubscribeUserId.value) {
    return
  }

  clearParsedSubscribe()
})

watch(
  () => typeForm.sourceType,
  (sourceType) => {
    if (sourceType !== 'subscribe') {
      clearParsedSubscribe()
    }
  }
)

const detailRules: FormRules = {
  name: [{ required: true, message: '请输入计划名称', trigger: 'blur' }],
  parentPath: [{ required: true, message: '请输入父目录路径', trigger: 'blur' }],
  autoIngestInterval: [
    {
      type: 'number',
      min: AUTO_INGEST_INTERVAL_MIN,
      message: `间隔不能小于${AUTO_INGEST_INTERVAL_MIN}分钟`,
      trigger: 'blur',
    },
  ],
  'refreshStrategy.refreshInterval': [
    {
      type: 'number',
      min: REFRESH_INTERVAL_MIN,
      message: `刷新间隔不能小于${REFRESH_INTERVAL_MIN}分钟`,
      trigger: 'blur',
    },
  ],
  'refreshStrategy.autoRefreshDays': [
    {
      type: 'number',
      min: AUTO_REFRESH_DAYS_MIN,
      message: `持续天数不能小于${AUTO_REFRESH_DAYS_MIN}`,
      trigger: 'blur',
    },
  ],
}

// 解析逻辑
const handleParseSubscribe = () => {
  if (parsing.value || submitting.value) {
    return
  }

  const subscribeUserId = normalizeCloud189SubscribeUserInput(subscribeUserIdInput.value)
  if (!subscribeUserId) {
    message.error('请输入订阅号ID')
    return
  }

  if (!show.value) {
    return
  }

  const currentOperation = operationVersion
  const currentRequestId = ++parseRequestId
  subscribeUserIdInput.value = subscribeUserId
  parsing.value = true
  getSubscribeUser({
    subscribeUser: subscribeUserId,
    currentPage: 1,
    pageSize: 1,
  })
    .then((res: ApiResponse<GetSubscribeUserResponse>) => {
      if (!isCurrentParseRequest(currentRequestId, currentOperation)) {
        return
      }

      if (typeForm.sourceType !== 'subscribe' || subscribeUserIdInput.value !== subscribeUserId) {
        return
      }

      if (res.code === 200 && res.data) {
        const data = normalizeGetSubscribeUserResponse(res.data)
        if (!data) {
          message.error('解析失败：响应数据格式异常')

          return
        }

        parsedSubscribeUserId.value = subscribeUserId
        parsedUserName.value = data.name
        parsedShareTotal.value = data.total
        parsedLocked.value = true
        // 建议预填名称
        if (!detailForm.name) {
          detailForm.name = `订阅：${parsedUserName.value}`
        }
        // 默认挂载父目录
        if (!detailForm.parentPath) {
          detailForm.parentPath = `/电影/${parsedUserName.value}`
        }
        message.success('解析成功')
      } else {
        message.error(res.msg || '解析失败')
      }
    })
    .catch((err: unknown) => {
      if (!isCurrentParseRequest(currentRequestId, currentOperation)) {
        return
      }

      const errorMessage = getErrorMessage(err, '解析失败')

      console.error('解析订阅号失败:', errorMessage)
      message.error(errorMessage)
    })
    .finally(() => {
      if (isCurrentParseRequest(currentRequestId, currentOperation)) {
        parsing.value = false
      }
    })
}

const handleEnableEditSubscribe = () => {
  parsedLocked.value = false
  // 不清理 parsedSubscribeUserId，直至再次点击解析成功覆盖
}

// 取消
const handleCancel = () => {
  if (submitting.value) {
    return
  }

  // 关闭前重置，避免下次打开保留上次内容
  resetAll()
  show.value = false
}

// 提交
const submitting = ref(false)
const handleSubmit = () => {
  if (submitting.value) {
    return
  }

  // 校验类型（以及若为订阅必须先解析成功）
  if (!typeForm.sourceType) {
    message.error('请选择入库类型')
    return
  }

  if (typeForm.sourceType === 'subscribe') {
    if (!parsedSubscribeUserId.value) {
      message.error('请先解析订阅号ID')
      return
    }
  } else {
    message.error('暂不支持该入库类型')
    return
  }

  const validateOperation = operationVersion
  submitting.value = true

  // 校验详细表单
  if (!detailFormRef.value) {
    submitting.value = false
    return
  }

  detailFormRef.value.validate((errors) => {
    if (!isCurrentOperation(validateOperation)) {
      return
    }

    if (errors) {
      message.error('请检查表单输入')
      submitting.value = false
      return
    }

    if (!show.value) {
      submitting.value = false
      return
    }

    const currentOperation = validateOperation
    const currentRequestId = ++submitRequestId

    if (typeForm.sourceType === 'subscribe') {
      const payload: CreateSubscribePlanRequest = {
        ...detailForm,
        upUserId: parsedSubscribeUserId.value as string, // 使用已解析的订阅号ID
      }
      createSubscribePlan(payload)
        .then((res: ApiResponse<CreateSubscribePlanResponse>) => {
          if (!isCurrentSubmitRequest(currentRequestId, currentOperation)) {
            return
          }

          if (res.code !== 200) {
            message.error(res.msg || '创建失败')

            return
          }

          const createResult = normalizeCreateSubscribePlanResponse(res.data)
          if (!createResult) {
            message.warning('创建成功，但响应数据格式异常')
          } else if (payload.oneClickAddHistory && createResult.historyError) {
            message.warning(`计划已创建，但历史入库任务未下发：${createResult.historyError}`)
          } else if (payload.oneClickAddHistory && createResult.historyQueued) {
            message.success('创建成功，历史入库任务已下发')
          } else {
            message.success('创建成功')
          }
          resetAll()
          show.value = false
          emit('created')
        })
        .catch((err: unknown) => {
          if (!isCurrentSubmitRequest(currentRequestId, currentOperation)) {
            return
          }

          const errorMessage = getErrorMessage(err, '创建失败')

          console.error('创建入库计划失败:', errorMessage)
          message.error(errorMessage)
        })
        .finally(() => {
          if (isCurrentSubmitRequest(currentRequestId, currentOperation)) {
            submitting.value = false
          }
        })
      return
    }

    submitting.value = false
  })
}

// 重置
const resetAll = () => {
  typeForm.sourceType = 'subscribe'
  subscribeUserIdInput.value = ''
  clearParsedSubscribe()

  detailForm.name = ''
  detailForm.parentPath = ''
  detailForm.upUserId = ''
  detailForm.enable = true
  detailForm.cloudToken = undefined
  detailForm.onConflict = 'abandon'
  detailForm.autoIngestInterval = 30
  detailForm.oneClickAddHistory = false
  detailForm.refreshStrategy.enableAutoRefresh = false
  detailForm.refreshStrategy.autoRefreshDays = 7
  detailForm.refreshStrategy.refreshInterval = 30
  detailForm.refreshStrategy.enableDeepRefresh = false
}

onUnmounted(() => {
  isComponentMounted = false
  invalidatePendingWork()
})
</script>

<style scoped>
.create-plan-modal {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section {
  padding: 4px 0;
}

.row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-width: 0;
  width: 100%;
  box-sizing: border-box;
}

.subscribe-id-input {
  flex: 1 1 180px;
  min-width: 0;
}

.parsed-summary {
  margin: -8px 0 8px;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.hint-inline {
  color: var(--n-text-color-2);
  font-size: 12px;
}
</style>
