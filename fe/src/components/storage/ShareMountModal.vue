<template>
  <div class="share-mount-container">
    <div class="share-mount-content">
      <!-- 第一步：输入分享码和访问码 -->
      <div v-if="currentStep === 1" class="step-content">
        <div class="step-header">
          <n-text strong>输入分享信息</n-text>
          <n-text depth="3">请输入天翼云盘分享码和访问码（如有）</n-text>
        </div>

        <div class="input-section">
          <div class="input-group">
            <n-text strong class="input-label">分享码 *</n-text>
            <n-input
              v-model:value="shareState.shareCode"
              placeholder="请输入分享码"
              clearable
              size="large"
              :disabled="shareState.loading"
              @keyup.enter="handleGetShareInfo"
            >
              <template #prefix>
                <n-icon :size="16"><LinkOutline /></n-icon>
              </template>
            </n-input>
          </div>

          <div class="input-group">
            <n-text strong class="input-label">访问码（可选）</n-text>
            <n-input
              v-model:value="shareState.shareAccessCode"
              placeholder="请输入访问码（如果分享设置了访问码）"
              clearable
              size="large"
              :disabled="shareState.loading"
              @keyup.enter="handleGetShareInfo"
            >
              <template #prefix>
                <n-icon :size="16"><LockClosedOutline /></n-icon>
              </template>
            </n-input>
          </div>

          <n-button
            type="primary"
            size="large"
            :loading="shareState.loading"
            :disabled="!isValidShareCode || shareState.loading"
            @click="handleGetShareInfo"
            style="margin-top: 24px; width: 100%"
          >
            <template #icon
              ><n-icon><SearchOutline /></n-icon
            ></template>
            获取分享信息
          </n-button>
        </div>
      </div>

      <!-- 第二步：展示分享信息 -->
      <div v-if="currentStep === 2" class="step-content">
        <div class="step-header">
          <n-text strong>确认分享信息</n-text>
          <n-text depth="3">请确认要挂载的分享资源信息</n-text>
        </div>

        <div v-if="shareState.shareInfo" class="share-info-card">
          <div class="share-info-header">
            <n-icon :size="24" :color="shareState.shareInfo.isFolder ? '#ff9800' : '#2196f3'">
              <FolderOutline v-if="shareState.shareInfo.isFolder" />
              <DocumentOutline v-else />
            </n-icon>
            <div class="share-info-details">
              <n-text strong class="share-name">{{ shareState.shareInfo.name }}</n-text>
              <div class="share-meta">
                <n-text depth="3" class="share-type">
                  {{ shareState.shareInfo.isFolder ? '文件夹' : '单文件' }}
                </n-text>
                <n-text depth="3" class="share-time">
                  分享时间：{{ formatDateTime(shareState.shareInfo.shareTime) }}
                </n-text>
              </div>
            </div>
          </div>

          <div class="share-codes-info">
            <div class="code-item">
              <n-text depth="2">分享码：</n-text>
              <n-text code>{{ shareState.shareCode }}</n-text>
            </div>
            <div v-if="shareState.shareAccessCode" class="code-item">
              <n-text depth="2">访问码：</n-text>
              <n-text code>{{ shareState.shareAccessCode }}</n-text>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="modal-actions">
      <n-button v-if="currentStep === 2" :disabled="bindLoading" @click="handleBackToStep1">
        <template #icon
          ><n-icon><ArrowBackOutline /></n-icon
        ></template>
        返回上一步
      </n-button>
      <n-button :disabled="bindLoading" @click="handleCancel">取消</n-button>
      <n-button
        v-if="currentStep === 2"
        type="primary"
        :loading="bindLoading"
        :disabled="!shareState.shareInfo || bindLoading"
        @click="handleConfirm"
      >
        绑定挂载点
      </n-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { NIcon, NText, NInput, NButton, useMessage } from 'naive-ui'
import {
  FolderOutline,
  DocumentOutline,
  LinkOutline,
  LockClosedOutline,
  SearchOutline,
  ArrowBackOutline,
} from '@vicons/ionicons5'
import { getShareInfo } from '@/api/storage/advance'
import type { ShareInfo, GetShareInfoQuery } from '@/api/storage/advance'
import { getErrorMessage, type ApiResponse } from '@/utils/api'
import { formatDateTime } from '@/utils/time'
import { OS_TYPES } from '@/utils/osType'
import { useMountPointBind } from '@/composables/useMountPointBind'
import { normalizeShareInfo } from '@/utils/responseGuards'

// Emits
interface Emits {
  (e: 'confirm', data: { success: boolean }): void
  (e: 'cancel'): void
}
const emit = defineEmits<Emits>()

// Hooks
const message = useMessage()
const mountPointBind = useMountPointBind()

// 步骤控制
const currentStep = ref(1)

// 分享状态
const shareState = reactive({
  shareCode: '',
  shareAccessCode: '',
  loading: false,
  shareInfo: null as ShareInfo | null,
})

const bindLoading = ref(false)
let operationVersion = 0
let isComponentMounted = false

// 计算属性
const isValidShareCode = computed(() => shareState.shareCode.trim().length > 0)

const isCurrentOperation = (version: number) => {
  return isComponentMounted && operationVersion === version
}

const invalidatePendingWork = () => {
  operationVersion++
  shareState.loading = false
  bindLoading.value = false
}

// 获取分享信息
const handleGetShareInfo = () => {
  if (!isComponentMounted) {
    return
  }

  if (shareState.loading) {
    return
  }

  if (!isValidShareCode.value) {
    message.warning('请输入分享码')
    return
  }

  operationVersion++
  const currentOperation = operationVersion

  shareState.loading = true
  shareState.shareInfo = null

  const params: GetShareInfoQuery = {
    shareCode: shareState.shareCode.trim(),
  }

  if (shareState.shareAccessCode.trim()) {
    params.shareAccessCode = shareState.shareAccessCode.trim()
  }

  getShareInfo(params)
    .then((response: ApiResponse<ShareInfo>) => {
      if (!isCurrentOperation(currentOperation)) {
        return
      }

      if (response.code === 200) {
        const shareInfo = normalizeShareInfo(response.data)

        if (!shareInfo) {
          message.error('响应数据格式异常')
          return
        }

        shareState.shareInfo = shareInfo
        currentStep.value = 2
        message.success('获取分享信息成功')
      } else {
        message.error(response.msg || '获取分享信息失败')
      }
    })
    .catch((error) => {
      if (!isCurrentOperation(currentOperation)) {
        return
      }

      console.error('获取分享信息失败:', error)
      message.error(getErrorMessage(error, '获取分享信息失败'))
    })
    .finally(() => {
      if (isCurrentOperation(currentOperation)) {
        shareState.loading = false
      }
    })
}

// 返回上一步
const handleBackToStep1 = () => {
  currentStep.value = 1
}

// 取消
const handleCancel = () => {
  invalidatePendingWork()
  emit('cancel')
}

// 确认挂载
const handleConfirm = () => {
  if (bindLoading.value) {
    return
  }

  if (!shareState.shareInfo) {
    message.warning('分享信息不完整')
    return
  }

  const itemsToMount = [
    {
      name: shareState.shareInfo.name,
      osType: OS_TYPES.SHARE_FOLDER,
      shareCode: shareState.shareCode.trim(),
      shareAccessCode: shareState.shareAccessCode.trim() || undefined,
    },
  ]

  const currentOperation = ++operationVersion
  bindLoading.value = true

  mountPointBind
    .show(itemsToMount)
    .then((payload) => {
      if (isCurrentOperation(currentOperation) && payload && payload.length > 0) {
        handleMountBindSuccess()
      }
    })
    .finally(() => {
      if (isCurrentOperation(currentOperation)) {
        bindLoading.value = false
      }
    })
}

// 绑定挂载点成功回调
const handleMountBindSuccess = () => {
  emit('confirm', { success: true })
}

// 重置所有状态
const resetAllState = () => {
  currentStep.value = 1
  shareState.shareCode = ''
  shareState.shareAccessCode = ''
  shareState.loading = false
  bindLoading.value = false
  shareState.shareInfo = null
}

onMounted(() => {
  isComponentMounted = true
  operationVersion++
  resetAllState()
})

onUnmounted(() => {
  isComponentMounted = false
  invalidatePendingWork()
})
</script>

<style scoped>
.share-mount-container {
  min-width: 800px;
  width: 100%;
}

.share-mount-content {
  padding: 16px 0;
}

.step-content {
  min-height: 300px;
}

.step-header {
  margin-bottom: 24px;
  text-align: center;
}

.step-header .n-text:first-child {
  display: block;
  margin-bottom: 8px;
  font-size: 18px;
}

.input-section {
  max-width: 500px;
  margin: 0 auto;
}

.input-group {
  margin-bottom: 20px;
}

.input-label {
  display: block;
  margin-bottom: 8px;
  font-size: 14px;
}

.share-info-card {
  max-width: 600px;
  margin: 0 auto;
  padding: 20px;
  background: var(--n-card-color);
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
}

.share-info-header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 16px;
}

.share-info-details {
  flex: 1;
}

.share-name {
  display: block;
  font-size: 16px;
  margin-bottom: 8px;
  word-break: break-all;
}

.share-meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.share-type,
.share-time {
  font-size: 13px;
}

.share-codes-info {
  padding-top: 16px;
  border-top: 1px solid var(--n-border-color);
}

.code-item {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.code-item:last-child {
  margin-bottom: 0;
}

.modal-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  padding-top: 16px;
  border-top: 1px solid var(--n-border-color);
}

@media (width <= 768px) {
  .share-info-header {
    flex-direction: column;
    align-items: center;
    text-align: center;
  }

  .code-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .modal-actions {
    flex-direction: column;
  }
}
</style>
