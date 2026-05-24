<template>
  <div class="cloud-tokens-page">
    <!-- 头部区域 -->
    <div class="header">
      <div class="header-left">
        <n-input
          v-model:value="searchKeyword"
          placeholder="请输入令牌名称搜索"
          clearable
          style="width: 200px; margin-right: 12px"
          @keyup.enter="handleSearch"
        />
        <n-button type="primary" @click="handleSearch" style="margin-right: 8px"> 搜索 </n-button>
        <n-button @click="handleReset"> 重置 </n-button>
      </div>
      <div class="header-right">
        <n-space>
          <n-button type="primary" @click="handleSelectQrcodeLogin">
            <template #icon>
              <n-icon>
                <QrCodeOutline />
              </n-icon>
            </template>
            扫码添加
          </n-button>
          <n-button @click="handleSelectPasswordLogin">
            <template #icon>
              <n-icon>
                <KeyOutline />
              </n-icon>
            </template>
            密码添加
          </n-button>
        </n-space>
      </div>
    </div>

    <!-- 令牌列表表格 -->
    <n-data-table
      :columns="columns"
      :data="tableData"
      :loading="loading"
      :pagination="paginationReactive"
      class="tokens-table"
      remote
    />

    <!-- 扫码登录弹窗 -->
    <QrcodeLoginModal v-model:show="showQrcodeModal" @success="handleAddSuccess" />

    <!-- 密码登录弹窗 -->
    <PasswordLoginModal v-model:show="showPasswordModal" @success="handleAddSuccess" />

    <!-- 更新扫码登录弹窗 -->
    <QrcodeLoginModal
      v-model:show="showUpdateQrcodeModal"
      :update-mode="true"
      :token-id="currentUpdateToken?.id"
      @success="handleUpdateSuccess"
    />

    <!-- 更新密码登录弹窗 -->
    <PasswordLoginModal
      v-model:show="showUpdatePasswordModal"
      :update-mode="true"
      :token-data="currentUpdateToken"
      @success="handleUpdateSuccess"
    />

    <!-- 编辑令牌名称弹窗 -->
    <n-modal
      :show="showEditModal"
      preset="dialog"
      title="编辑令牌名称"
      :closable="!editLoading"
      :mask-closable="!editLoading"
      :close-on-esc="!editLoading"
      @update:show="handleUpdateEditModalShow"
    >
      <n-form ref="editFormRef" :model="editForm" :rules="editRules">
        <n-form-item label="令牌名称" path="name">
          <n-input
            v-model:value="editForm.name"
            placeholder="请输入令牌名称"
            clearable
            :disabled="editLoading"
            @keyup.enter="handleConfirmEdit"
          />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space>
          <n-button :disabled="editLoading" @click="handleCloseEditModal">取消</n-button>
          <n-button
            type="primary"
            :loading="editLoading"
            :disabled="editLoading"
            @click="handleConfirmEdit"
            >确认</n-button
          >
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, h } from 'vue'
import {
  NDataTable,
  NInput,
  NButton,
  NIcon,
  NPopconfirm,
  NSpace,
  NModal,
  NForm,
  NFormItem,
  useMessage,
  type DataTableColumns,
  type PaginationProps,
  type FormInst,
} from 'naive-ui'
import {
  CreateOutline,
  RefreshOutline,
  TrashOutline,
  QrCodeOutline,
  KeyOutline,
} from '@vicons/ionicons5'
import { getCloudTokenList, deleteCloudToken, modifyCloudTokenName } from '@/api/cloudtoken'
import { QrcodeLoginModal, PasswordLoginModal } from '@/components/cloudtoken'
import { getListItems, getListTotal } from '@/utils/pagination'
import { normalizeDashboardCloudTokens } from '@/utils/responseGuards'
import { formatRemainingTime, formatDateTime } from '@/utils/time'

// 表格数据
const tableData = ref<Models.CloudToken[]>([])
const loading = ref(false)
const searchKeyword = ref('')
let tokenListRequestId = 0
let isComponentUnmounted = false

// 添加令牌相关
const showQrcodeModal = ref(false)
const showPasswordModal = ref(false)

// 更新令牌相关
const showUpdateQrcodeModal = ref(false)
const showUpdatePasswordModal = ref(false)
const currentUpdateToken = ref<Models.CloudToken | null>(null)

// 编辑令牌相关
const showEditModal = ref(false)
const currentEditToken = ref<Models.CloudToken | null>(null)
const editLoading = ref(false)
const editFormRef = ref<FormInst>()
const editForm = reactive({
  name: '',
})
let editSessionVersion = 0
const editRules = {
  name: [
    { required: true, message: '请输入令牌名称', trigger: 'blur' },
    { min: 1, max: 50, message: '令牌名称长度应在1-50个字符之间', trigger: 'blur' },
  ],
}

// 删除令牌行级 pending
const deletePendingTokenIds = ref<Set<number>>(new Set())
const deleteTokenTasks = new Map<number, Promise<void>>()

// 消息提示
const message = useMessage()

const setDeleteTokenPending = (tokenId: number, pending: boolean) => {
  const next = new Set(deletePendingTokenIds.value)

  if (pending) {
    next.add(tokenId)
  } else {
    next.delete(tokenId)
  }

  deletePendingTokenIds.value = next
}

const isDeleteTokenPending = (tokenId: number) => deletePendingTokenIds.value.has(tokenId)

const clearDeleteTokenPending = () => {
  deleteTokenTasks.clear()
  deletePendingTokenIds.value = new Set()
}

// 分页配置
const paginationReactive = reactive<PaginationProps>({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
  prefix: ({ itemCount }) => `共 ${itemCount} 条`,
  onChange: (page: number) => {
    paginationReactive.page = page
    fetchTokenList()
  },
  onUpdatePageSize: (pageSize: number) => {
    paginationReactive.pageSize = pageSize
    paginationReactive.page = 1
    fetchTokenList()
  },
})

// 表格列定义
const columns: DataTableColumns<Models.CloudToken> = [
  {
    title: '令牌ID',
    key: 'id',
    width: 100,
    align: 'center',
  },
  {
    title: '令牌名称',
    key: 'name',
    width: 150,
    align: 'center',
  },
  {
    title: '用户名',
    key: 'username',
    width: 150,
    align: 'center',
    render(row) {
      return row.username || '-'
    },
  },
  {
    title: '登录方式',
    key: 'loginType',
    width: 120,
    align: 'center',
    render(row) {
      const typeMap: Record<number, string> = {
        1: '扫码登录',
        2: '密码登录',
      }
      return typeMap[row.loginType] || '未知'
    },
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    align: 'center',
    render(row) {
      const statusMap: Record<number, string> = {
        1: '正常',
        2: '登录失败',
      }
      return statusMap[row.status] || '未知'
    },
  },
  {
    title: '剩余时间',
    key: 'expiresIn',
    width: 160,
    align: 'center',
    render(row) {
      return formatRemainingTime(row.expiresIn)
    },
  },
  {
    title: '更新时间',
    key: 'updatedAt',
    width: 160,
    align: 'center',
    render(row) {
      return formatDateTime(row.updatedAt)
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 220,
    align: 'center',
    render(row) {
      const deletePending = isDeleteTokenPending(row.id)

      return h(
        NSpace,
        { size: 'small' },
        {
          default: () => [
            // 编辑名称按钮
            h(
              NButton,
              {
                size: 'tiny',
                type: 'info',
                secondary: true,
                disabled: deletePending,
                onClick: () => handleEdit(row),
              },
              {
                icon: () => h(NIcon, { size: 12 }, { default: () => h(CreateOutline) }),
                default: () => '编辑',
              }
            ),
            // 更新按钮
            h(
              NButton,
              {
                size: 'tiny',
                type: 'warning',
                secondary: true,
                disabled: deletePending,
                onClick: () => handleUpdate(row),
              },
              {
                icon: () => h(NIcon, { size: 12 }, { default: () => h(RefreshOutline) }),
                default: () => '更新',
              }
            ),
            // 删除按钮
            h(
              NPopconfirm,
              {
                onPositiveClick: () => handleDelete(row.id),
                negativeText: '取消',
                positiveText: '确认删除',
                positiveButtonProps: {
                  loading: deletePending,
                  disabled: deletePending,
                },
              },
              {
                trigger: () =>
                  h(
                    NButton,
                    {
                      size: 'tiny',
                      type: 'error',
                      secondary: true,
                      loading: deletePending,
                      disabled: deletePending,
                    },
                    {
                      icon: () => h(NIcon, { size: 12 }, { default: () => h(TrashOutline) }),
                      default: () => '删除',
                    }
                  ),
                default: () => `确定要删除令牌 "${row.name}" 吗？此操作不可撤销。`,
              }
            ),
          ],
        }
      )
    },
  },
]

// 获取令牌列表
const fetchTokenList = () => {
  if (isComponentUnmounted) {
    return Promise.resolve()
  }

  const requestId = ++tokenListRequestId

  loading.value = true

  const params = {
    currentPage: paginationReactive.page || 1,
    pageSize: paginationReactive.pageSize || 10,
    name: searchKeyword.value || undefined,
  }

  return getCloudTokenList(params)
    .then((response) => {
      if (requestId !== tokenListRequestId) {
        return
      }

      if (response.code === 200 && response.data) {
        const rawItems = getListItems<Models.CloudToken>(response.data)
        const items = normalizeDashboardCloudTokens(rawItems)
        const total = getListTotal(response.data)

        if (!items) {
          message.error('获取令牌列表失败：响应数据格式异常')

          return
        }

        tableData.value = items
        if (total === null) {
          message.warning('令牌列表响应缺少有效总数，已保留原分页统计')
        } else {
          paginationReactive.itemCount = total
        }
      }
    })
    .catch((error) => {
      if (requestId !== tokenListRequestId) {
        return
      }

      console.error('获取令牌列表失败:', error)
    })
    .finally(() => {
      if (requestId !== tokenListRequestId) {
        return
      }

      loading.value = false
    })
}

// 搜索
const handleSearch = () => {
  paginationReactive.page = 1 // 搜索时重置到第一页
  fetchTokenList()
}

// 重置
const handleReset = () => {
  searchKeyword.value = ''
  paginationReactive.page = 1 // 重置时回到第一页
  fetchTokenList()
}

// 选择扫码登录
const handleSelectQrcodeLogin = () => {
  showQrcodeModal.value = true
}

// 选择密码登录
const handleSelectPasswordLogin = () => {
  showPasswordModal.value = true
}

// 添加令牌成功回调
const handleAddSuccess = () => {
  // 刷新令牌列表
  fetchTokenList()
}

// 更新令牌
const handleUpdate = (token: Models.CloudToken) => {
  if (isDeleteTokenPending(token.id)) {
    return
  }

  currentUpdateToken.value = token

  // 根据登录方式选择不同的更新流程
  if (token.loginType === 1) {
    // 扫码登录更新
    showUpdateQrcodeModal.value = true
  } else if (token.loginType === 2) {
    // 密码登录更新
    showUpdatePasswordModal.value = true
  }
}

// 更新令牌成功回调
const handleUpdateSuccess = () => {
  // 刷新令牌列表
  fetchTokenList()
}

// 编辑令牌
const handleEdit = (token: Models.CloudToken) => {
  if (isDeleteTokenPending(token.id)) {
    return
  }

  editSessionVersion += 1
  editLoading.value = false
  currentEditToken.value = token
  editForm.name = token.name
  showEditModal.value = true
}

const isCurrentEditSession = (sessionVersion: number) =>
  !isComponentUnmounted && sessionVersion === editSessionVersion

const handleCloseEditModal = () => {
  if (editLoading.value) {
    return
  }

  editSessionVersion += 1
  editLoading.value = false
  showEditModal.value = false
  currentEditToken.value = null
}

const handleUpdateEditModalShow = (show: boolean) => {
  if (show) {
    showEditModal.value = true

    return
  }

  if (editLoading.value) {
    return
  }

  handleCloseEditModal()
}

// 确认编辑令牌名称
const handleConfirmEdit = () => {
  if (editLoading.value) {
    return
  }

  const formRef = editFormRef.value
  const token = currentEditToken.value
  const sessionVersion = editSessionVersion

  if (!formRef || !token) {
    return
  }

  editLoading.value = true

  formRef.validate((errors: unknown) => {
    if (!isCurrentEditSession(sessionVersion)) {
      return
    }

    if (errors) {
      editLoading.value = false

      return
    }

    modifyCloudTokenName({
      id: token.id,
      name: editForm.name,
    })
      .then((response) => {
        if (!isCurrentEditSession(sessionVersion)) {
          return
        }

        if (response.code === 200) {
          message.success('修改令牌名称成功')
          handleCloseEditModal()
          fetchTokenList()
        } else {
          message.error(response.msg || '修改令牌名称失败')
        }
      })
      .catch((error) => {
        if (!isCurrentEditSession(sessionVersion)) {
          return
        }

        console.error('修改令牌名称失败:', error)
        message.error('修改令牌名称失败')
      })
      .finally(() => {
        if (!isCurrentEditSession(sessionVersion)) {
          return
        }

        editLoading.value = false
      })
  })
}

// 删除令牌
const handleDelete = (tokenId: number): Promise<void> => {
  const pendingTask = deleteTokenTasks.get(tokenId)

  if (pendingTask) {
    return pendingTask
  }

  setDeleteTokenPending(tokenId, true)

  const deleteTask = Promise.resolve()
    .then(() => deleteCloudToken({ id: tokenId }))
    .then((response) => {
      if (isComponentUnmounted) {
        return
      }

      if (response.code === 200) {
        message.success('删除令牌成功')
        // 刷新令牌列表
        return fetchTokenList()
      } else {
        message.error(response.msg || '删除令牌失败')
      }
    })
    .catch((error) => {
      if (isComponentUnmounted) {
        return
      }

      console.error('删除令牌失败:', error)
      message.error(error.message || '删除令牌失败')
    })
    .finally(() => {
      deleteTokenTasks.delete(tokenId)

      if (!isComponentUnmounted) {
        setDeleteTokenPending(tokenId, false)
      }
    })

  deleteTokenTasks.set(tokenId, deleteTask)

  return deleteTask
}

// 初始化
onMounted(() => {
  isComponentUnmounted = false
  fetchTokenList()
})

onUnmounted(() => {
  isComponentUnmounted = true
  tokenListRequestId += 1
  editSessionVersion += 1
  loading.value = false
  editLoading.value = false
  clearDeleteTokenPending()
})
</script>

<style scoped>
.header {
  margin-bottom: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-left {
  display: flex;
  align-items: center;
}

.tokens-table {
  background: var(--n-card-color);
  border-radius: 6px;
}

.tokens-table :deep(.n-data-table-th) {
  text-align: center;
  font-weight: 600;
}

.tokens-table :deep(.n-data-table-td) {
  text-align: center;
}

.login-method-selection {
  padding: 20px;
  text-align: center;
}

.login-method-selection p {
  margin: 0 0 20px;
  color: var(--n-text-color-2);
  font-size: 16px;
}
</style>
