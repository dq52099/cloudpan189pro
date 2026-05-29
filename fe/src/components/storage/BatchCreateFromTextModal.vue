<template>
  <div class="batch-text-container">
    <div class="batch-text-content">
      <div class="header-section">
        <n-alert type="info" show-icon title="使用说明" class="mb-4">
          <template #default>
            <div class="usage-guide">
              <p>输入分享链接、分享码、文件夹ID或订阅号链接，系统将自动识别名称。</p>
              <p>支持格式：</p>
              <p>• 分享链接：https://cloud.189.cn/t/xxxxx</p>
              <p>• 文件夹ID：123456789</p>
              <p>• 订阅号链接：https://content.21cn.com/h5/subscrip/... ?uuid=xxx</p>
              <p>点击"下一步"后，您可以批量设置挂载路径前缀。</p>
            </div>
          </template>
        </n-alert>
      </div>

      <n-form
        ref="formRef"
        :model="formModel"
        :rules="rules"
        label-placement="left"
        label-width="100px"
      >
        <!-- 云盘账号选择 -->
        <n-form-item label="云盘账号" path="cloudToken">
          <n-select
            v-model:value="formModel.cloudToken"
            :options="cloudTokenOptions"
            :loading="state.loadingTokens"
            :disabled="isBusy"
            placeholder="请选择用于解析的账号"
            clearable
          />
        </n-form-item>

        <!-- 文本内容 -->
        <n-form-item label="资源列表" path="content">
          <n-input
            v-model:value="formModel.content"
            type="textarea"
            placeholder="示例：&#10;https://cloud.189.cn/t/code&#10;123456789 (文件夹ID)"
            :autosize="{ minRows: 8, maxRows: 15 }"
            :disabled="isBusy"
            class="resource-textarea"
          />
        </n-form-item>
      </n-form>
    </div>

    <!-- 底部操作栏 -->
    <div class="modal-actions">
      <n-button :disabled="state.submitting" @click="handleCancel">取消</n-button>
      <n-button type="primary" :loading="state.submitting" :disabled="isBusy" @click="handleNext">
        下一步：解析并设置路径
      </n-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, computed, onMounted, onUnmounted } from 'vue'
import {
  NForm,
  NFormItem,
  NInput,
  NSelect,
  NButton,
  NAlert,
  useMessage,
  type FormInst,
  type FormRules,
} from 'naive-ui'
import { getCloudTokenList } from '@/api/cloudtoken'
import { batchParseStorageText, type BatchParseItem } from '@/api/storage'
import { getErrorMessage } from '@/utils/api'
import { getListItems } from '@/utils/pagination'
import { normalizeCloudTokens } from '@/utils/responseGuards'

interface Emits {
  (e: 'parsed', payload: { items: BatchParseItem[]; token: number }): void
  (e: 'cancel'): void
}
const emit = defineEmits<Emits>()

const message = useMessage()
const formRef = ref<FormInst | null>(null)

const state = reactive({
  loadingTokens: false,
  submitting: false,
  cloudTokens: [] as Models.CloudToken[],
})

const formModel = reactive({
  cloudToken: null as number | null,
  content: '',
})

let operationVersion = 0
let isComponentMounted = false

const rules: FormRules = {
  cloudToken: [
    {
      type: 'number',
      required: true,
      message: '请选择云盘账号',
      trigger: ['blur', 'change'],
    },
  ],
  content: [
    {
      required: true,
      message: '请输入内容',
      trigger: 'blur',
    },
  ],
}

const cloudTokenOptions = computed(() => {
  return state.cloudTokens.map((token) => ({
    label: token.name,
    value: token.id,
  }))
})

const isBusy = computed(() => state.loadingTokens || state.submitting)

const isCurrentOperation = (version: number) => {
  return isComponentMounted && operationVersion === version
}

const isOptionalString = (value: unknown): value is string | undefined => {
  return value === undefined || typeof value === 'string'
}

const isBatchParseItem = (value: unknown): value is BatchParseItem => {
  if (!value || typeof value !== 'object') {
    return false
  }

  const item = value as Record<string, unknown>

  return (
    typeof item.name === 'string' &&
    item.name.trim().length > 0 &&
    typeof item.osType === 'string' &&
    item.osType.trim().length > 0 &&
    isOptionalString(item.shareCode) &&
    isOptionalString(item.shareAccessCode) &&
    isOptionalString(item.fileId) &&
    isOptionalString(item.subscribeUser)
  )
}

const isBatchParseItemArray = (value: unknown): value is BatchParseItem[] => {
  return Array.isArray(value) && value.every(isBatchParseItem)
}

const invalidatePendingWork = () => {
  operationVersion++
  state.loadingTokens = false
  state.submitting = false
}

const fetchCloudTokens = async (currentOperation = operationVersion) => {
  if (!isComponentMounted) {
    return
  }

  state.loadingTokens = true
  try {
    const res = await getCloudTokenList({ noPaginate: true })

    if (!isCurrentOperation(currentOperation)) {
      return
    }

    if (res.code === 200 && res.data) {
      const tokenItems = getListItems<Models.CloudToken>(res.data)
      const tokens = normalizeCloudTokens(tokenItems)
      if (!tokens) {
        message.error('获取云盘账号失败：响应数据格式异常')

        return
      }

      state.cloudTokens = tokens
      if (state.cloudTokens.length === 1) formModel.cloudToken = state.cloudTokens[0].id

      return
    }
    message.error(res.msg || '获取云盘账号失败')
  } catch (err) {
    if (!isCurrentOperation(currentOperation)) {
      return
    }

    message.error(getErrorMessage(err, '获取云盘账号失败'))
  } finally {
    if (isCurrentOperation(currentOperation)) {
      state.loadingTokens = false
    }
  }
}

const handleNext = async () => {
  if (!isComponentMounted || state.submitting) {
    return
  }

  operationVersion++
  const currentOperation = operationVersion

  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  if (!isCurrentOperation(currentOperation)) {
    return
  }

  if (!formModel.cloudToken) return

  const cloudToken = formModel.cloudToken

  state.submitting = true

  batchParseStorageText({
    content: formModel.content,
    cloudToken,
  })
    .then((res) => {
      if (!isCurrentOperation(currentOperation)) {
        return
      }

      if (res.code !== 200) {
        message.error(res.msg || '解析失败')

        return
      }

      if (!isBatchParseItemArray(res.data)) {
        message.error('解析响应格式异常')

        return
      }

      if (res.data.length > 0) {
        message.success(`成功解析 ${res.data.length} 个资源`)
        emit('parsed', {
          items: res.data,
          token: cloudToken,
        })

        return
      }

      message.warning('未能解析出有效资源，请检查格式或账号权限')
    })
    .catch((err: unknown) => {
      if (!isCurrentOperation(currentOperation)) {
        return
      }

      message.error(getErrorMessage(err, '解析失败'))
    })
    .finally(() => {
      if (isCurrentOperation(currentOperation)) {
        state.submitting = false
      }
    })
}

const handleCancel = () => {
  if (state.submitting) {
    return
  }

  invalidatePendingWork()
  emit('cancel')
}

onMounted(() => {
  isComponentMounted = true
  operationVersion++
  fetchCloudTokens(operationVersion)
})

onUnmounted(() => {
  isComponentMounted = false
  invalidatePendingWork()
})
</script>

<style scoped>
.batch-text-container {
  width: 100%;
}

.batch-text-content {
  padding: 0 4px;
}

.header-section {
  margin-bottom: 20px;
}

.usage-guide {
  font-size: 13px;
  line-height: 1.6;
}

.resource-textarea {
  font-family: monospace;
}

.modal-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  padding-top: 20px;
  margin-top: 10px;
  border-top: 1px solid var(--n-border-color);
}
</style>
