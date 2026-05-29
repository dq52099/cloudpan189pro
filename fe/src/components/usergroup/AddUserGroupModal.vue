<template>
  <n-modal
    v-model:show="showModal"
    preset="dialog"
    title="添加用户组"
    :closable="!loading"
    :mask-closable="!loading"
    :close-on-esc="!loading"
  >
    <n-form
      ref="formRef"
      :model="formData"
      :rules="rules"
      label-placement="left"
      label-width="auto"
      require-mark-placement="right-hanging"
    >
      <n-form-item label="用户组名称" path="name">
        <n-input
          v-model:value="formData.name"
          placeholder="请输入用户组名称"
          maxlength="255"
          show-count
          :disabled="loading"
        />
      </n-form-item>
    </n-form>

    <template #action>
      <n-space>
        <n-button :disabled="loading" @click="handleCancel">取消</n-button>
        <n-button type="primary" :loading="loading" :disabled="loading" @click="handleSubmit">
          确定
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, reactive, watch, onUnmounted } from 'vue'
import {
  NModal,
  NForm,
  NFormItem,
  NInput,
  NButton,
  NSpace,
  useMessage,
  type FormInst,
} from 'naive-ui'
import { addUserGroup } from '@/api/usergroup'
import { getErrorMessage } from '@/utils/api'

interface Props {
  show: boolean
}

interface Emits {
  (e: 'update:show', value: boolean): void
  (e: 'success'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

// 表单引用
const formRef = ref<FormInst | null>(null)

// 弹窗显示状态
const showModal = ref(false)

// 加载状态
const loading = ref(false)

// 消息提示
const message = useMessage()

// 表单数据
const formData = reactive({
  name: '',
})

let operationVersion = 0

// 表单验证规则
const rules = {
  name: [
    { required: true, message: '请输入用户组名称', trigger: 'blur' },
    { min: 1, max: 255, message: '用户组名称长度应在1-255位之间', trigger: 'blur' },
  ],
}

// 监听 props.show 变化
watch(
  () => props.show,
  (newVal) => {
    showModal.value = newVal
    if (newVal) {
      operationVersion++
      // 重置表单
      resetForm()

      return
    }

    invalidatePendingWork()
  }
)

// 监听 showModal 变化
watch(showModal, (newVal) => {
  if (!newVal && loading.value) {
    showModal.value = true

    return
  }

  emit('update:show', newVal)
})

// 重置表单
const resetForm = () => {
  formData.name = ''
  formRef.value?.restoreValidation()
}

const isCurrentOperation = (version: number) => showModal.value && operationVersion === version

const invalidatePendingWork = () => {
  operationVersion++
  loading.value = false
}

// 取消
const handleCancel = () => {
  if (loading.value) return

  showModal.value = false
}

// 提交
const handleSubmit = async () => {
  if (loading.value || !formRef.value) return

  const currentOperation = operationVersion
  loading.value = true

  try {
    await formRef.value.validate()

    if (!isCurrentOperation(currentOperation)) {
      return
    }

    const response = await addUserGroup({
      name: formData.name,
    })

    if (!isCurrentOperation(currentOperation)) {
      return
    }

    if (response.code === 200) {
      message.success('添加用户组成功')
      showModal.value = false
      emit('success')
    } else {
      message.error(response.msg || '添加用户组失败')
    }
  } catch (error) {
    if (!isCurrentOperation(currentOperation)) {
      return
    }

    if (Array.isArray(error)) {
      return
    }

    console.error('添加用户组失败:', error)
    message.error(getErrorMessage(error, '添加用户组失败'))
  } finally {
    if (isCurrentOperation(currentOperation)) {
      loading.value = false
    }
  }
}

onUnmounted(() => {
  invalidatePendingWork()
})
</script>
