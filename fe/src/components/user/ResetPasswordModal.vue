<template>
  <n-modal
    v-model:show="visible"
    preset="dialog"
    title="重置用户密码"
    :closable="!loading"
    :mask-closable="!loading"
    :close-on-esc="!loading"
  >
    <!-- 隐藏的假输入框，用于欺骗浏览器自动填充 -->
    <div style="position: absolute; left: -9999px; opacity: 0; pointer-events: none">
      <input type="text" name="fake-username" autocomplete="username" />
      <input type="password" name="fake-password" autocomplete="current-password" />
    </div>

    <div style="margin: 20px 0">
      <n-alert type="info" style="margin-bottom: 20px">
        正在为用户 <strong>{{ userInfo?.username }}</strong> 重置密码
      </n-alert>
    </div>

    <n-form
      ref="formRef"
      :model="form"
      :rules="formRules"
      label-placement="left"
      label-width="80px"
      style="margin-top: 20px"
      autocomplete="off"
    >
      <n-form-item label="新密码" path="password">
        <n-input
          v-model:value="form.password"
          type="password"
          placeholder="请输入新密码（6-20位）"
          clearable
          show-password-on="mousedown"
          autocomplete="off"
          :name="`new-password-${Date.now()}`"
          readonly
          :disabled="loading"
          @focus="handlePasswordFocus"
        />
      </n-form-item>
      <n-form-item label="确认密码" path="confirmPassword">
        <n-input
          v-model:value="form.confirmPassword"
          type="password"
          placeholder="请再次输入新密码"
          clearable
          show-password-on="mousedown"
          autocomplete="off"
          :name="`confirm-password-${Date.now()}`"
          readonly
          :disabled="loading"
          @focus="handleConfirmPasswordFocus"
        />
      </n-form-item>
    </n-form>
    <template #action>
      <n-space>
        <n-button :disabled="loading" @click="handleCancel">取消</n-button>
        <n-button type="primary" :loading="loading" :disabled="loading" @click="handleConfirm">
          确认重置
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
  NInput,
  NButton,
  NSpace,
  NAlert,
  type FormInst,
  type FormRules,
  useMessage,
} from 'naive-ui'
import { modifyUserPassword, type ModifyPasswordRequest } from '@/api/user'

interface Props {
  show: boolean
  userInfo?: Models.UserInfo | null
}

interface Emits {
  (e: 'update:show', value: boolean): void
  (e: 'success'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

// 消息提示
const message = useMessage()

// 控制弹窗显示
const visible = computed({
  get: () => props.show,
  set: (value) => {
    if (!value && loading.value) {
      return
    }

    emit('update:show', value)
  },
})

// 表单相关
const loading = ref(false)
const formRef = ref<FormInst | null>(null)
const form = reactive({
  password: '',
  confirmPassword: '',
})

let operationVersion = 0

// 表单验证规则
const formRules: FormRules = {
  password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, max: 20, message: '密码长度应为6-20位', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (_rule, value) => {
        if (value !== form.password) {
          return new Error('两次输入的密码不一致')
        }
        return true
      },
      trigger: 'blur',
    },
  ],
}

// 重置表单
const resetForm = () => {
  form.password = ''
  form.confirmPassword = ''
  formRef.value?.restoreValidation()
}

// 监听弹窗显示状态，显示时重置表单
watch(
  () => props.show,
  (newVal) => {
    if (newVal) {
      operationVersion++
      resetForm()

      return
    }

    invalidatePendingWork()
  }
)

const isCurrentOperation = (version: number) => visible.value && operationVersion === version

const invalidatePendingWork = () => {
  operationVersion++
  loading.value = false
}

// 取消操作
const handleCancel = () => {
  if (loading.value) return

  visible.value = false
}

// 处理密码输入框焦点事件
const handlePasswordFocus = (event: FocusEvent) => {
  const target = event.target as HTMLInputElement
  if (target) {
    target.removeAttribute('readonly')
  }
}

// 处理确认密码输入框焦点事件
const handleConfirmPasswordFocus = (event: FocusEvent) => {
  const target = event.target as HTMLInputElement
  if (target) {
    target.removeAttribute('readonly')
  }
}

// 确认重置密码
const handleConfirm = () => {
  if (loading.value) return

  if (!formRef.value || !props.userInfo) return

  const currentOperation = operationVersion

  // 验证表单
  formRef.value
    .validate()
    .then(() => {
      if (!isCurrentOperation(currentOperation) || !props.userInfo) {
        return null
      }

      loading.value = true

      const requestData: ModifyPasswordRequest = {
        id: props.userInfo!.id,
        password: form.password,
      }

      // 调用重置密码API
      return modifyUserPassword(requestData)
    })
    .then((response) => {
      if (!response || !isCurrentOperation(currentOperation)) {
        return
      }

      if (response.code === 200) {
        message.success('密码重置成功')
        visible.value = false
        emit('success')
      } else {
        message.error(response.msg || '密码重置失败')
      }
    })
    .catch((error) => {
      if (!isCurrentOperation(currentOperation)) {
        return
      }

      console.error('重置密码失败:', error)
      if (error?.response?.data?.msg) {
        message.error(error.response.data.msg)
      } else {
        message.error('重置密码失败，请稍后重试')
      }
    })
    .finally(() => {
      if (isCurrentOperation(currentOperation)) {
        loading.value = false
      }
    })
}

onUnmounted(() => {
  invalidatePendingWork()
})
</script>
