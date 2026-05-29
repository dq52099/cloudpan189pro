<template>
  <n-modal
    v-model:show="showModal"
    preset="dialog"
    :title="props.updateMode ? '更新扫码令牌' : '扫码登录'"
    :mask-closable="false"
    :closable="!isBusy"
    :close-on-esc="!isBusy"
  >
    <div class="qrcode-login-container">
      <!-- 二维码显示区域 -->
      <div class="qrcode-section">
        <div v-if="loading" class="loading-container">
          <n-spin size="large" />
          <p>正在生成二维码...</p>
        </div>

        <div v-else-if="qrcodeUrl" class="qrcode-container">
          <n-qr-code :value="qrcodeUrl" :size="200" style="width: 220px; height: 220px" />
          <div class="qrcode-info">
            <n-icon size="20" color="#18a058">
              <CheckmarkCircleOutline />
            </n-icon>
            <span>二维码已生成</span>
          </div>
        </div>

        <div v-else-if="qrcodeFormatInvalid" class="format-warning-container">
          <n-icon size="40" color="#f0a020">
            <AlertCircleOutline />
          </n-icon>
          <p>响应数据格式异常</p>
          <n-button type="primary" :disabled="isBusy" @click="initQrcode">重新生成</n-button>
        </div>

        <div v-else class="error-container">
          <n-icon size="40" color="#d03050">
            <CloseCircleOutline />
          </n-icon>
          <p>二维码生成失败</p>
          <n-button type="primary" :disabled="isBusy" @click="initQrcode">重新生成</n-button>
        </div>
      </div>

      <!-- 操作说明 -->
      <div class="instructions">
        <h4>扫码登录步骤：</h4>
        <ol>
          <li>打开天翼云盘APP</li>
          <li>点击右上角"扫一扫"</li>
          <li>扫描上方二维码</li>
          <li>在APP中确认登录</li>
        </ol>

        <!-- 倒计时显示 -->
        <div v-if="countdown > 0" class="countdown">
          <n-icon size="16" color="#f0a020">
            <TimeOutline />
          </n-icon>
          <span>二维码将在 {{ countdown }} 秒后过期</span>
        </div>

        <div v-else-if="qrcodeUrl" class="expired">
          <n-icon size="16" color="#d03050">
            <CloseCircleOutline />
          </n-icon>
          <span>二维码已过期</span>
          <n-button
            type="primary"
            size="small"
            :disabled="isBusy"
            @click="initQrcode"
            style="margin-left: 8px"
          >
            重新生成
          </n-button>
        </div>
      </div>

      <!-- 状态提示 -->
      <div v-if="statusMessage" class="status-message">
        <n-alert :type="statusType" :title="statusMessage" />
      </div>
    </div>

    <template #action>
      <n-space>
        <n-button :disabled="isBusy" @click="handleCancel">取消</n-button>
        <n-button type="primary" :loading="loading" :disabled="checkLoading" @click="initQrcode">
          重新生成二维码
        </n-button>
        <n-button
          v-if="qrcodeUrl && countdown > 0"
          type="success"
          @click="handleCheckLogin"
          :loading="checkLoading"
          :disabled="loading"
        >
          我已扫码登录
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { NModal, NQrCode, NSpin, NButton, NIcon, NSpace, NAlert, useMessage } from 'naive-ui'
import {
  AlertCircleOutline,
  CheckmarkCircleOutline,
  CloseCircleOutline,
  TimeOutline,
} from '@vicons/ionicons5'
import { initQrcode as initQrcodeApi, checkQrcode } from '@/api/cloudtoken'
import { getErrorMessage } from '@/utils/api'

// Props
interface Props {
  show: boolean
  updateMode?: boolean // 是否为更新模式
  tokenId?: number // 更新时的令牌ID
}

// Emits
interface Emits {
  (e: 'update:show', value: boolean): void
  (e: 'success'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

// 响应式数据
const loading = ref(false)
const checkLoading = ref(false)
const qrcodeUuid = ref('')
const qrcodeUrl = ref('')
const qrcodeFormatInvalid = ref(false)
const countdown = ref(0)
const statusMessage = ref('')
const statusType = ref<'success' | 'info' | 'warning' | 'error'>('info')
const successPending = ref(false)

let operationVersion = 0
let countdownTimer: ReturnType<typeof setInterval> | null = null
let successCloseTimer: ReturnType<typeof setTimeout> | null = null

// 消息提示
const message = useMessage()

// 计算属性
const showModal = computed({
  get: () => props.show,
  set: (value) => {
    if (!value && isBusy.value) {
      return
    }

    emit('update:show', value)
  },
})

const isBusy = computed(() => loading.value || checkLoading.value || successPending.value)

// 监听弹窗显示状态
watch(showModal, (newVal) => {
  if (newVal) {
    // 弹窗打开时初始化二维码
    initQrcode()
  } else {
    invalidatePendingWork()
    resetState()
  }
})

const isCurrentOperation = (version: number) => {
  return showModal.value && operationVersion === version
}

const invalidatePendingWork = () => {
  operationVersion++
  clearTimers()
}

const normalizeQrcodeUuid = (value: unknown) => {
  if (typeof value !== 'object' || value === null || !('uuid' in value)) {
    return null
  }

  const uuid = (value as { uuid: unknown }).uuid
  if (typeof uuid !== 'string') {
    return null
  }

  const normalizedUuid = uuid.trim()
  if (!normalizedUuid) {
    return null
  }

  return normalizedUuid
}

// 初始化二维码
const initQrcode = () => {
  if (!showModal.value || isBusy.value) {
    return
  }

  operationVersion++
  const currentOperation = operationVersion

  loading.value = true
  checkLoading.value = false
  successPending.value = false
  qrcodeFormatInvalid.value = false
  qrcodeUuid.value = ''
  qrcodeUrl.value = ''
  countdown.value = 0
  statusMessage.value = ''
  clearTimers()

  initQrcodeApi()
    .then((response) => {
      if (!isCurrentOperation(currentOperation)) {
        return
      }

      if (response.code === 200) {
        const uuid = normalizeQrcodeUuid(response.data)
        if (!uuid) {
          qrcodeFormatInvalid.value = true
          statusMessage.value = '响应数据格式异常'
          statusType.value = 'warning'

          return
        }

        qrcodeUuid.value = uuid
        // 构建二维码URL，这里假设后端返回的是uuid，需要构建完整的登录URL
        qrcodeUrl.value = `https://cloud.189.cn/api/portal/loginUrl.action?redirectURL=https://cloud.189.cn&uuid=${encodeURIComponent(uuid)}`

        // 开始倒计时（120秒）
        startCountdown(120, currentOperation)

        statusMessage.value =
          '二维码生成成功，请使用天翼云盘APP扫码登录，扫码后点击"我已扫码登录"按钮'
        statusType.value = 'success'
      } else {
        throw new Error(response.msg || '初始化二维码失败')
      }
    })
    .catch((error) => {
      if (!isCurrentOperation(currentOperation)) {
        return
      }

      console.error('初始化二维码失败:', error)
      statusMessage.value = getErrorMessage(error, '二维码生成失败，请重试')
      statusType.value = 'error'
      qrcodeFormatInvalid.value = false
      qrcodeUrl.value = ''
    })
    .finally(() => {
      if (isCurrentOperation(currentOperation)) {
        loading.value = false
      }
    })
}

// 开始倒计时
const startCountdown = (seconds: number, currentOperation: number) => {
  countdown.value = seconds

  countdownTimer = setInterval(() => {
    if (!isCurrentOperation(currentOperation)) {
      clearTimers()

      return
    }

    countdown.value--

    if (countdown.value <= 0) {
      clearCountdownTimer()
      statusMessage.value = '二维码已过期，请重新生成'
      statusType.value = 'warning'
    }
  }, 1000)
}

// 手动检查登录状态
const handleCheckLogin = () => {
  if (loading.value || checkLoading.value || successPending.value) {
    return
  }

  if (!qrcodeUuid.value) {
    message.error('二维码ID不存在，请重新生成二维码')
    return
  }

  checkLoading.value = true
  const currentOperation = operationVersion

  const checkData = {
    uuid: qrcodeUuid.value,
    ...(props.updateMode && props.tokenId ? { id: props.tokenId } : {}),
  }

  checkQrcode(checkData)
    .then((response) => {
      if (!isCurrentOperation(currentOperation)) {
        return
      }

      if (response.code === 200) {
        // 登录成功
        clearTimers()
        successPending.value = true
        statusMessage.value = '登录成功！'
        statusType.value = 'success'
        message.success('扫码登录成功')

        // 延迟关闭弹窗并触发成功回调
        successCloseTimer = setTimeout(() => {
          if (!isCurrentOperation(currentOperation)) {
            return
          }

          showModal.value = false
          emit('success')
        }, 1500)
      } else if (response.code === 40001) {
        // 二维码已过期
        clearTimers()
        countdown.value = 0
        statusMessage.value = '二维码已过期，请重新生成'
        statusType.value = 'warning'
        message.warning('二维码已过期')
      } else if (response.code === 40002) {
        // 用户取消登录
        statusMessage.value = '用户取消了登录'
        statusType.value = 'info'
        message.info('用户取消了登录')
      } else if (response.code === 40003) {
        // 等待用户扫码
        statusMessage.value = '请先使用天翼云盘APP扫码并确认登录'
        statusType.value = 'warning'
        message.warning('请先扫码确认登录')
      } else {
        // 其他错误
        message.error(response.msg || '检查登录状态失败')
      }
    })
    .catch((error) => {
      if (!isCurrentOperation(currentOperation)) {
        return
      }

      console.error('检查二维码状态失败:', error)
      message.error(getErrorMessage(error, '检查登录状态失败'))
    })
    .finally(() => {
      if (isCurrentOperation(currentOperation)) {
        checkLoading.value = false
      }
    })
}

const clearCountdownTimer = () => {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
}

// 清理定时器
const clearTimers = () => {
  clearCountdownTimer()

  if (successCloseTimer) {
    clearTimeout(successCloseTimer)
    successCloseTimer = null
  }
}

// 重置状态
const resetState = () => {
  qrcodeUuid.value = ''
  qrcodeUrl.value = ''
  qrcodeFormatInvalid.value = false
  countdown.value = 0
  statusMessage.value = ''
  loading.value = false
  checkLoading.value = false
  successPending.value = false
}

// 取消操作
const handleCancel = () => {
  if (isBusy.value) {
    return
  }

  showModal.value = false
}

// 组件卸载时清理定时器
onUnmounted(() => {
  invalidatePendingWork()
})
</script>

<style scoped>
.qrcode-login-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20px;
  min-height: 400px;
}

.qrcode-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-bottom: 24px;
}

.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  padding: 40px;
}

.qrcode-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.qrcode-info {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #18a058;
  font-size: 14px;
}

.error-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  padding: 40px;
}

.format-warning-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  padding: 40px;
}

.instructions {
  width: 100%;
  max-width: 300px;
  text-align: left;
}

.instructions h4 {
  margin: 0 0 12px;
  color: #333;
  font-size: 16px;
}

.instructions ol {
  margin: 0 0 16px;
  padding-left: 20px;
}

.instructions li {
  margin-bottom: 4px;
  color: #666;
  font-size: 14px;
}

.countdown {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #f0a020;
  font-size: 14px;
  margin-top: 12px;
}

.expired {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #d03050;
  font-size: 14px;
  margin-top: 12px;
}

.status-message {
  width: 100%;
  margin-top: 16px;
}
</style>
