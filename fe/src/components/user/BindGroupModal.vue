<template>
  <n-modal
    v-model:show="visible"
    preset="dialog"
    title="绑定用户组"
    :closable="!loading"
    :mask-closable="!loading"
    :close-on-esc="!loading"
  >
    <div style="margin: 20px 0">
      <n-alert type="info" style="margin-bottom: 20px">
        正在为用户 <strong>{{ userInfo?.username }}</strong> 绑定用户组
      </n-alert>
    </div>

    <n-form
      ref="formRef"
      :model="form"
      :rules="formRules"
      label-placement="left"
      label-width="80px"
      style="margin-top: 20px"
    >
      <n-form-item label="用户组" path="groupId">
        <n-select
          v-model:value="form.groupId"
          placeholder="请选择用户组"
          :options="groupOptions"
          :loading="groupLoading"
          :disabled="loading"
          clearable
        />
      </n-form-item>
    </n-form>

    <template #action>
      <n-space>
        <n-button :disabled="loading" @click="handleCancel">取消</n-button>
        <n-button type="primary" :loading="loading" :disabled="loading" @click="handleConfirm">
          确认绑定
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue'
import {
  NModal,
  NForm,
  NFormItem,
  NSelect,
  NButton,
  NSpace,
  NAlert,
  type FormInst,
  type FormRules,
  type SelectOption,
  useMessage,
} from 'naive-ui'
import { bindUserGroup, type BindGroupRequest } from '@/api/user'
import { getUserGroupList, type UserGroupInfo } from '@/api/usergroup'
import { getListItems } from '@/utils/pagination'

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
const groupLoading = ref(false)
const formRef = ref<FormInst | null>(null)
const form = reactive({
  groupId: undefined as number | undefined,
})

// 用户组选项
const groupOptions = ref<SelectOption[]>([])
let operationVersion = 0
let isComponentMounted = false

// 表单验证规则
const formRules: FormRules = {
  groupId: [{ required: true, message: '请选择用户组', trigger: 'change', type: 'number' }],
}

// 获取用户组列表
const isCurrentOperation = (version: number) => {
  return isComponentMounted && operationVersion === version
}

const invalidatePendingWork = () => {
  operationVersion++
}

const getErrorMessage = (error: unknown) => {
  if (typeof error !== 'object' || error === null) {
    return undefined
  }

  const response = (error as { response?: { data?: { msg?: unknown } } }).response
  return typeof response?.data?.msg === 'string' ? response.data.msg : undefined
}

const fetchUserGroups = (currentOperation = operationVersion) => {
  if (!isComponentMounted) {
    return
  }

  groupLoading.value = true

  getUserGroupList({ noPaginate: true })
    .then((response) => {
      if (!isCurrentOperation(currentOperation)) {
        return
      }

      if (response.code === 200 && response.data) {
        const groups = getListItems<UserGroupInfo>(response.data)
        if (!groups) {
          message.error('获取用户组列表失败：响应数据格式异常')

          return
        }

        // 添加默认用户组选项
        const options: SelectOption[] = [
          {
            label: '默认用户组',
            value: 0,
          },
        ]

        // 添加其他用户组选项
        groups.forEach((group) => {
          options.push({
            label: group.name,
            value: group.id,
          })
        })

        groupOptions.value = options

        return
      }

      message.error(response.msg || '获取用户组列表失败')
    })
    .catch((error) => {
      if (!isCurrentOperation(currentOperation)) {
        return
      }

      console.error('获取用户组列表失败:', error)
      message.error('获取用户组列表失败')
    })
    .finally(() => {
      if (isCurrentOperation(currentOperation)) {
        groupLoading.value = false
      }
    })
}

// 重置表单
const resetForm = () => {
  form.groupId = props.userInfo?.groupId ?? undefined
  formRef.value?.restoreValidation()
}

// 监听弹窗显示状态，显示时重置表单并获取用户组列表
watch(
  () => props.show,
  (newVal) => {
    if (newVal) {
      operationVersion++
      const currentOperation = operationVersion

      resetForm()

      fetchUserGroups(currentOperation)

      return
    }

    invalidatePendingWork()
    loading.value = false
    groupLoading.value = false
  }
)

// 取消操作
const handleCancel = () => {
  if (loading.value) return

  visible.value = false
}

// 确认绑定用户组
const handleConfirm = async () => {
  if (loading.value || !formRef.value || !props.userInfo) return

  const currentOperation = operationVersion
  loading.value = true

  try {
    // 验证表单
    await formRef.value.validate()

    if (!isCurrentOperation(currentOperation) || !visible.value || !props.userInfo) {
      return
    }

    const requestData: BindGroupRequest = {
      userId: props.userInfo.id,
      groupId: form.groupId,
    }

    // 调用绑定用户组API
    const response = await bindUserGroup(requestData)

    if (!isCurrentOperation(currentOperation) || !visible.value) {
      return
    }

    if (response.code === 200) {
      message.success('用户组绑定成功')
      visible.value = false
      emit('success')
    } else {
      message.error(response.msg || '用户组绑定失败')
    }
  } catch (error) {
    if (!isCurrentOperation(currentOperation) || !visible.value) {
      return
    }

    if (Array.isArray(error)) {
      return
    }

    console.error('绑定用户组失败:', error)
    message.error(getErrorMessage(error) || '绑定用户组失败，请稍后重试')
  } finally {
    if (isCurrentOperation(currentOperation)) {
      loading.value = false
    }
  }
}

// 组件挂载时获取用户组列表
onMounted(() => {
  isComponentMounted = true
  fetchUserGroups()
})

onUnmounted(() => {
  isComponentMounted = false
  invalidatePendingWork()
})
</script>
