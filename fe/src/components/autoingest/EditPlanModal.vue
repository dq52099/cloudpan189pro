<template>
  <n-modal
    v-model:show="show"
    preset="dialog"
    title="修改入库计划"
    :closable="!submitting"
    :mask-closable="false"
    :close-on-esc="!submitting"
  >
    <div class="edit-plan-modal">
      <n-form
        ref="detailFormRef"
        :model="form"
        :rules="detailRules"
        label-placement="left"
        label-width="100"
        class="section"
      >
        <n-form-item label="计划名称" path="name">
          <n-input v-model:value="form.name" placeholder="例如：入库计划A" :disabled="submitting" />
        </n-form-item>

        <n-form-item label="挂载父目录" path="parentPath">
          <n-input v-model:value="form.parentPath" placeholder="/Movies" :disabled="submitting" />
        </n-form-item>

        <n-form-item label="绑定令牌" path="tokenId">
          <n-select
            v-model:value="form.tokenId"
            :options="cloudTokenOptionsWithUnbound"
            placeholder="请选择令牌"
            filterable
            :disabled="submitting"
          />
        </n-form-item>

        <n-form-item label="冲突处理策略" path="onConflict">
          <n-radio-group v-model:value="form.onConflict" :disabled="submitting">
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
            v-model:value="form.autoIngestInterval"
            :min="AUTO_INGEST_INTERVAL_MIN"
            :max="REFRESH_INTERVAL_MAX"
            :disabled="submitting"
          />
        </n-form-item>

        <n-divider title-placement="left">刷新策略（可选）</n-divider>

        <n-form-item label="启用自动刷新" path="refreshStrategy.enableAutoRefresh">
          <n-switch v-model:value="form.refreshStrategy.enableAutoRefresh" :disabled="submitting" />
        </n-form-item>

        <template v-if="form.refreshStrategy.enableAutoRefresh">
          <n-form-item label="刷新间隔(分钟)" path="refreshStrategy.refreshInterval">
            <n-input-number
              v-model:value="form.refreshStrategy.refreshInterval"
              :min="REFRESH_INTERVAL_MIN"
              :max="REFRESH_INTERVAL_MAX"
              :disabled="submitting"
            />
          </n-form-item>
          <n-form-item label="持续天数" path="refreshStrategy.autoRefreshDays">
            <n-input-number
              v-model:value="form.refreshStrategy.autoRefreshDays"
              :min="AUTO_REFRESH_DAYS_MIN"
              :max="AUTO_REFRESH_DAYS_MAX"
              :disabled="submitting"
            />
          </n-form-item>
          <n-form-item label="深度刷新" path="refreshStrategy.enableDeepRefresh">
            <n-switch
              v-model:value="form.refreshStrategy.enableDeepRefresh"
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
          保存修改
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, ref, reactive, watch, onUnmounted } from 'vue'
import {
  NModal,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NSelect,
  NRadioGroup,
  NRadio,
  NSwitch,
  NDivider,
  NButton,
  NSpace,
  useMessage,
  type FormInst,
  type FormRules,
} from 'naive-ui'
import {
  AUTO_INGEST_ON_CONFLICT_OPTIONS,
  AUTO_INGEST_INTERVAL_MIN,
  REFRESH_INTERVAL_MIN,
  REFRESH_INTERVAL_MAX,
  AUTO_REFRESH_DAYS_MIN,
  AUTO_REFRESH_DAYS_MAX,
} from '@/constants/autoIngest'
import { updateAutoIngestPlan, type UpdatePlanRequest } from '@/api/autoingest'
import { type ApiResponse } from '@/utils/api'

type CloudTokenOption = { label: string; value: number }

const message = useMessage()

// props / emits
const props = defineProps<{
  show: boolean
  plan: Models.AutoIngestPlan | null
  cloudTokenOptions: CloudTokenOption[]
}>()

const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'saved'): void
}>()

const cloudTokenOptionsWithUnbound = computed<CloudTokenOption[]>(() => [
  { label: '未绑定令牌', value: 0 },
  ...props.cloudTokenOptions,
])

// v-model:show 与关闭重置
const show = ref(props.show)
let isComponentMounted = true
let operationVersion = 0
let submitRequestId = 0

const invalidatePendingWork = () => {
  operationVersion++
}

const isCurrentOperation = (version: number) => {
  return isComponentMounted && show.value && operationVersion === version
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
    // 每次打开时，重新从 props.plan 注入，避免因对象引用未变化导致 watch 不触发
    applyPlanToForm(props.plan)

    return
  }

  if (!v && ov) {
    invalidatePendingWork()
    submitting.value = false
    resetAll()
  }
})

// 表单
const detailFormRef = ref<FormInst | null>(null)
const form = reactive<
  Required<
    Pick<UpdatePlanRequest, 'id' | 'name' | 'parentPath' | 'onConflict' | 'autoIngestInterval'>
  > & {
    tokenId: number
    refreshStrategy: {
      enableAutoRefresh: boolean
      autoRefreshDays: number
      refreshInterval: number
      enableDeepRefresh: boolean
    }
  }
>({
  id: 0,
  name: '',
  parentPath: '',
  onConflict: 'abandon',
  autoIngestInterval: 30,
  tokenId: 0,
  refreshStrategy: {
    enableAutoRefresh: false,
    autoRefreshDays: 7,
    refreshInterval: 30,
    enableDeepRefresh: false,
  },
})

const applyPlanToForm = (p: Models.AutoIngestPlan | null | undefined) => {
  if (!p) return
  form.id = p.id
  form.name = p.name || ''
  form.parentPath = p.parentPath || ''
  form.onConflict = (p.onConflict as 'rename' | 'abandon') || 'rename'
  form.autoIngestInterval = p.autoIngestInterval ?? 30
  form.tokenId = p.tokenId ?? 0
  form.refreshStrategy.enableAutoRefresh = !!p.refreshStrategy?.enableAutoRefresh
  form.refreshStrategy.autoRefreshDays = p.refreshStrategy?.autoRefreshDays ?? 7
  form.refreshStrategy.refreshInterval = p.refreshStrategy?.refreshInterval ?? 30
  form.refreshStrategy.enableDeepRefresh = !!p.refreshStrategy?.enableDeepRefresh
}

// 同步 props.plan 到表单（首次加载 & 对象更换时）
watch(
  () => props.plan,
  (p) => {
    if (!show.value) {
      return
    }

    invalidatePendingWork()
    applyPlanToForm(p)
  },
  { immediate: true }
)

// 校验规则（对齐创建）
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

const submitting = ref(false)

const handleCancel = () => {
  if (submitting.value) {
    return
  }

  resetAll()
  show.value = false
}

const handleSubmit = () => {
  if (submitting.value) {
    return
  }

  if (!form.id) {
    message.error('计划ID缺失')
    return
  }

  const validateOperation = operationVersion
  submitting.value = true

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

    const payload: UpdatePlanRequest = {
      id: form.id,
      name: form.name || undefined,
      parentPath: form.parentPath || undefined,
      autoIngestInterval: form.autoIngestInterval || undefined,
      onConflict: form.onConflict || undefined,
      tokenId: form.tokenId,
      refreshStrategy: form.refreshStrategy.enableAutoRefresh
        ? {
            enableAutoRefresh: form.refreshStrategy.enableAutoRefresh,
            autoRefreshDays: form.refreshStrategy.autoRefreshDays,
            refreshInterval: form.refreshStrategy.refreshInterval,
            enableDeepRefresh: form.refreshStrategy.enableDeepRefresh,
          }
        : {
            enableAutoRefresh: false,
            autoRefreshDays: undefined,
            refreshInterval: undefined,
            enableDeepRefresh: undefined,
          },
    }

    const currentOperation = validateOperation
    const currentRequestId = ++submitRequestId

    updateAutoIngestPlan(payload)
      .then((res: ApiResponse) => {
        if (!isCurrentSubmitRequest(currentRequestId, currentOperation)) {
          return
        }

        if (res.code === 200) {
          message.success('修改成功')
          resetAll()
          show.value = false
          emit('saved')
        } else {
          message.error(res.msg || '修改失败')
        }
      })
      .catch((err: unknown) => {
        if (!isCurrentSubmitRequest(currentRequestId, currentOperation)) {
          return
        }

        console.error('修改失败', err)
        message.error('修改失败')
      })
      .finally(() => {
        if (isCurrentSubmitRequest(currentRequestId, currentOperation)) {
          submitting.value = false
        }
      })
  })
}

const resetAll = () => {
  // 下次打开由 props.plan 再次填充
  form.id = 0
  form.name = ''
  form.parentPath = ''
  form.onConflict = 'abandon'
  form.autoIngestInterval = 30
  form.tokenId = 0
  form.refreshStrategy.enableAutoRefresh = false
  form.refreshStrategy.autoRefreshDays = 7
  form.refreshStrategy.refreshInterval = 30
  form.refreshStrategy.enableDeepRefresh = false
}

onUnmounted(() => {
  isComponentMounted = false
  invalidatePendingWork()
})
</script>

<style scoped>
.edit-plan-modal {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section {
  padding: 4px 0;
}
</style>
