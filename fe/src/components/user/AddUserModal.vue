<template>
  <n-modal
    v-model:show="visible"
    preset="dialog"
    title="添加用户"
    :closable="!loading"
    :mask-closable="!loading"
    :close-on-esc="!loading"
  >
    <!-- 隐藏的假输入框，用于欺骗浏览器自动填充 -->
    <div style="position: absolute; left: -9999px; opacity: 0; pointer-events: none">
      <input type="text" name="fake-username" autocomplete="username" />
      <input type="password" name="fake-password" autocomplete="current-password" />
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
      <n-form-item label="用户名" path="username">
        <n-input
          v-model:value="form.username"
          placeholder="请输入用户名（3-20位）"
          clearable
          autocomplete="off"
          :name="`username-${Date.now()}`"
          readonly
          :disabled="loading"
          @focus="handleUsernameFocus"
        />
      </n-form-item>
      <n-form-item label="密码" path="password">
        <n-input
          v-model:value="form.password"
          type="password"
          placeholder="请输入密码（6-20位）"
          clearable
          show-password-on="mousedown"
          autocomplete="off"
          :name="`password-${Date.now()}`"
          readonly
          :disabled="loading"
          @focus="handlePasswordFocus"
        />
      </n-form-item>
    </n-form>
    <template #action>
      <n-space>
        <n-button :disabled="loading" @click="handleCancel">取消</n-button>
        <n-button type="primary" :loading="loading" :disabled="loading" @click="handleConfirm">
          确认添加
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
  type FormInst,
  type FormRules,
  useMessage,
} from 'naive-ui'
import { addUser, type AddUserRequest } from '@/api/user'

interface Props {
  show: boolean
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
const form = reactive<AddUserRequest>({
  username: '',
  password: '',
})

let operationVersion = 0

// 表单验证规则
const formRules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 20, message: '用户名长度应为3-20位', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, max: 20, message: '密码长度应为6-20位', trigger: 'blur' },
  ],
}

// 重置表单
const resetForm = () => {
  form.username = ''
  form.password = ''
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

// 处理用户名输入框焦点事件
const handleUsernameFocus = (event: FocusEvent) => {
  const target = event.target as HTMLInputElement
  if (target) {
    target.removeAttribute('readonly')
  }
}

// 处理密码输入框焦点事件
const handlePasswordFocus = (event: FocusEvent) => {
  const target = event.target as HTMLInputElement
  if (target) {
    target.removeAttribute('readonly')
  }
}

// 确认添加
const handleConfirm = () => {
  if (loading.value) return

  if (!formRef.value) return

  const currentOperation = operationVersion

  // 验证表单
  formRef.value
    .validate()
    .then(() => {
      if (!isCurrentOperation(currentOperation)) {
        return null
      }

      loading.value = true

      // 调用添加用户API
      return addUser(form)
    })
    .then((response) => {
      if (!response || !isCurrentOperation(currentOperation)) {
        return
      }

      if (response.code === 200) {
        message.success('用户添加成功')
        visible.value = false
        emit('success')
      } else {
        message.error(response.msg || '添加用户失败')
      }
    })
    .catch((error) => {
      if (!isCurrentOperation(currentOperation)) {
        return
      }

      console.error('添加用户失败:', error)
      if (error?.response?.data?.msg) {
        message.error(error.response.data.msg)
      } else {
        message.error('添加用户失败，请稍后重试')
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
