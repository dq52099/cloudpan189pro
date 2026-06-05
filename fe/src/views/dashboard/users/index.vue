<template>
  <div class="users-page">
    <!-- 头部区域 -->
    <div class="header">
      <div class="header-left">
        <n-input
          v-model:value="searchKeyword"
          placeholder="请输入用户名搜索"
          clearable
          class="header-search-input"
          @keyup.enter="handleSearch"
        />
        <n-button type="primary" @click="handleSearch"> 搜索 </n-button>
        <n-button @click="handleReset"> 重置 </n-button>
      </div>
      <div class="header-right">
        <n-button type="primary" @click="handleAddUser">
          <template #icon>
            <n-icon>
              <PersonAddOutline />
            </n-icon>
          </template>
          添加用户
        </n-button>
      </div>
    </div>

    <!-- 用户列表表格 -->
    <n-data-table
      :columns="columns"
      :data="tableData"
      :loading="loading"
      :pagination="paginationReactive"
      class="users-table"
      remote
    />

    <!-- 添加用户弹窗 -->
    <AddUserModal v-model:show="showAddModal" @success="handleAddSuccess" />

    <!-- 重置密码弹窗 -->
    <ResetPasswordModal
      v-model:show="showResetPasswordModal"
      :user-info="currentResetUser"
      @success="handleResetPasswordSuccess"
    />

    <!-- 绑定用户组弹窗 -->
    <BindGroupModal
      v-model:show="showBindGroupModal"
      :user-info="currentBindUser"
      @success="handleBindGroupSuccess"
    />
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
  useMessage,
  type DataTableColumns,
  type PaginationProps,
} from 'naive-ui'
import {
  PersonAddOutline,
  TrashOutline,
  KeyOutline,
  CheckmarkCircleOutline,
  BanOutline,
  PeopleOutline,
} from '@vicons/ionicons5'
import { getUserList, deleteUser, toggleUserStatus } from '@/api/user'
import { AddUserModal, ResetPasswordModal, BindGroupModal } from '@/components/user'
import { getListItems, getListTotal } from '@/utils/pagination'
import { normalizeDashboardUsers } from '@/utils/responseGuards'
import { formatDateTime } from '@/utils/time'
import { getErrorMessage } from '@/utils/api'

// 表格数据
const tableData = ref<Models.UserInfo[]>([])
const loading = ref(false)
const searchKeyword = ref('')
let userListRequestId = 0
let usersPageAlive = false
type UserRowAction = 'toggle' | 'delete'
const userActionPendingKeys = ref<Set<string>>(new Set())

// 添加用户相关
const showAddModal = ref(false)

// 重置密码相关
const showResetPasswordModal = ref(false)
const currentResetUser = ref<Models.UserInfo | null>(null)

// 绑定用户组相关
const showBindGroupModal = ref(false)
const currentBindUser = ref<Models.UserInfo | null>(null)

// 消息提示
const message = useMessage()

const getUserActionKey = (action: UserRowAction, userId: number) => `${action}:${userId}`

const setUserActionPending = (action: UserRowAction, userId: number, pending: boolean) => {
  const keys = new Set(userActionPendingKeys.value)
  const key = getUserActionKey(action, userId)
  if (pending) {
    keys.add(key)
  } else {
    keys.delete(key)
  }
  userActionPendingKeys.value = keys
}

const isUserActionPending = (action: UserRowAction, userId: number) =>
  userActionPendingKeys.value.has(getUserActionKey(action, userId))

const isAnyUserActionPending = (userId: number) =>
  isUserActionPending('toggle', userId) || isUserActionPending('delete', userId)

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
    fetchUserList()
  },
  onUpdatePageSize: (pageSize: number) => {
    paginationReactive.pageSize = pageSize
    paginationReactive.page = 1
    fetchUserList()
  },
})

// 获取用户列表
const fetchUserList = () => {
  if (!usersPageAlive) {
    return Promise.resolve()
  }

  const currentRequestId = ++userListRequestId
  loading.value = true

  const params = {
    currentPage: paginationReactive.page || 1,
    pageSize: paginationReactive.pageSize || 10,
    username: searchKeyword.value || undefined,
  }

  return getUserList(params)
    .then((response) => {
      if (currentRequestId !== userListRequestId) return

      if (response.code === 200 && response.data) {
        const rawItems = getListItems<Models.UserInfo>(response.data)
        const items = normalizeDashboardUsers(rawItems)
        const total = getListTotal(response.data)

        if (!items) {
          message.error('获取用户列表失败：响应数据格式异常')

          return
        }

        tableData.value = items
        if (total === null) {
          message.warning('用户列表响应缺少有效总数，已保留原分页统计')
        } else {
          paginationReactive.itemCount = total
        }
      } else {
        message.error(response.msg || '获取用户列表失败')
      }
    })
    .catch((error) => {
      if (currentRequestId !== userListRequestId) return

      const errorMessage = getErrorMessage(error, '获取用户列表失败')

      console.error('获取用户列表失败:', errorMessage)
      message.error(errorMessage)
    })
    .finally(() => {
      if (currentRequestId !== userListRequestId) return

      loading.value = false
    })
}

// 搜索
const handleSearch = () => {
  paginationReactive.page = 1 // 搜索时重置到第一页
  fetchUserList()
}

// 重置
const handleReset = () => {
  searchKeyword.value = ''
  paginationReactive.page = 1 // 重置时回到第一页
  fetchUserList()
}

// 添加用户
const handleAddUser = () => {
  // 显示弹窗
  showAddModal.value = true
}

// 添加用户成功回调
const handleAddSuccess = () => {
  // 刷新用户列表
  fetchUserList()
}

// 删除用户
const handleDeleteUser = (userId: number) => {
  if (isAnyUserActionPending(userId)) return

  setUserActionPending('delete', userId, true)

  return deleteUser({ id: userId })
    .then((response) => {
      if (!usersPageAlive) return

      if (response.code === 200) {
        message.success('删除用户成功')
        // 刷新用户列表
        return fetchUserList()
      } else {
        message.error(response.msg || '删除用户失败')
      }
    })
    .catch((error) => {
      if (!usersPageAlive) return

      const errorMessage = getErrorMessage(error, '删除用户失败')

      console.error('删除用户失败:', errorMessage)
      message.error(errorMessage)
    })
    .finally(() => {
      if (usersPageAlive) {
        setUserActionPending('delete', userId, false)
      }
    })
}

// 重置密码
const handleResetPassword = (user: Models.UserInfo) => {
  currentResetUser.value = user
  showResetPasswordModal.value = true
}

// 重置密码成功回调
const handleResetPasswordSuccess = () => {
  // 重置密码不需要刷新列表，只需要显示成功消息
  currentResetUser.value = null
}

// 绑定用户组
const handleBindGroup = (user: Models.UserInfo) => {
  currentBindUser.value = user
  showBindGroupModal.value = true
}

// 绑定用户组成功回调
const handleBindGroupSuccess = () => {
  // 刷新用户列表以显示最新的用户组信息
  fetchUserList()
  currentBindUser.value = null
}

// 切换用户状态
const handleToggleStatus = (user: Models.UserInfo) => {
  if (isAnyUserActionPending(user.id)) return

  const newStatus = user.status === 1 ? 2 : 1
  const actionText = newStatus === 1 ? '启用' : '禁用'
  setUserActionPending('toggle', user.id, true)

  return toggleUserStatus({ id: user.id, status: newStatus })
    .then((response) => {
      if (!usersPageAlive) return

      if (response.code === 200) {
        message.success(`${actionText}用户成功`)
        // 刷新用户列表
        return fetchUserList()
      } else {
        message.error(response.msg || `${actionText}用户失败`)
      }
    })
    .catch((error) => {
      if (!usersPageAlive) return

      const errorMessage = getErrorMessage(error, `${actionText}用户失败`)

      console.error(`${actionText}用户失败:`, errorMessage)
      message.error(errorMessage)
    })
    .finally(() => {
      if (usersPageAlive) {
        setUserActionPending('toggle', user.id, false)
      }
    })
}

// 表格列定义
const columns: DataTableColumns<Models.UserInfo> = [
  {
    title: '用户ID',
    key: 'id',
    width: 100,
    align: 'center',
  },
  {
    title: '用户名',
    key: 'username',
    width: 150,
    align: 'center',
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    align: 'center',
    render(row) {
      const statusMap: Record<number, string> = {
        1: '正常',
        2: '禁用',
        0: '禁用',
      }
      return statusMap[row.status] || '未知'
    },
  },
  {
    title: '是否管理员',
    key: 'isAdmin',
    width: 120,
    align: 'center',
    render(row) {
      return row.isAdmin ? '是' : '否'
    },
  },
  {
    title: '用户组',
    key: 'groupName',
    width: 120,
    align: 'center',
    render(row) {
      return row.groupName || '默认组'
    },
  },
  {
    title: '创建时间',
    key: 'createdAt',
    width: 180,
    align: 'center',
    render(row) {
      return formatDateTime(row.createdAt)
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 280,
    align: 'center',
    render(row) {
      const rowPending = isAnyUserActionPending(row.id)
      const togglePending = isUserActionPending('toggle', row.id)
      const deletePending = isUserActionPending('delete', row.id)

      return h(
        NSpace,
        { size: 'small' },
        {
          default: () => [
            // 启用/禁用按钮
            h(
              NPopconfirm,
              {
                onPositiveClick: () => handleToggleStatus(row),
                negativeText: '取消',
                positiveText: '确认',
                positiveButtonProps: {
                  loading: togglePending,
                  disabled: rowPending,
                },
              },
              {
                trigger: () =>
                  h(
                    NButton,
                    {
                      size: 'tiny',
                      type: row.status === 1 ? 'warning' : 'success',
                      secondary: true,
                      loading: togglePending,
                      disabled: rowPending,
                    },
                    {
                      icon: () =>
                        h(
                          NIcon,
                          { size: 12 },
                          {
                            default: () =>
                              row.status === 1 ? h(BanOutline) : h(CheckmarkCircleOutline),
                          }
                        ),
                      default: () => (row.status === 1 ? '禁用' : '启用'),
                    }
                  ),
                default: () =>
                  `确定要${row.status === 1 ? '禁用' : '启用'}用户 "${row.username}" 吗？`,
              }
            ),
            // 重置密码按钮
            h(
              NButton,
              {
                size: 'tiny',
                type: 'info',
                secondary: true,
                disabled: rowPending,
                onClick: () => handleResetPassword(row),
              },
              {
                icon: () => h(NIcon, { size: 12 }, { default: () => h(KeyOutline) }),
                default: () => '重置',
              }
            ),
            // 绑定用户组按钮
            h(
              NButton,
              {
                size: 'tiny',
                type: 'primary',
                secondary: true,
                disabled: rowPending,
                onClick: () => handleBindGroup(row),
              },
              {
                icon: () => h(NIcon, { size: 12 }, { default: () => h(PeopleOutline) }),
                default: () => '绑定',
              }
            ),
            // 删除按钮
            h(
              NPopconfirm,
              {
                onPositiveClick: () => handleDeleteUser(row.id),
                negativeText: '取消',
                positiveText: '确认删除',
                positiveButtonProps: {
                  loading: deletePending,
                  disabled: rowPending,
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
                      disabled: rowPending,
                    },
                    {
                      icon: () => h(NIcon, { size: 12 }, { default: () => h(TrashOutline) }),
                      default: () => '删除',
                    }
                  ),
                default: () => `确定要删除用户 "${row.username}" 吗？此操作不可撤销。`,
              }
            ),
          ],
        }
      )
    },
  },
]

// 初始化
onMounted(() => {
  usersPageAlive = true
  fetchUserList()
})

onUnmounted(() => {
  usersPageAlive = false
  userListRequestId++
  loading.value = false
  userActionPendingKeys.value = new Set()
})
</script>

<style scoped>
.header {
  margin-bottom: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1 1 320px;
  flex-wrap: wrap;
  min-width: 0;
}

.header-right {
  display: flex;
  justify-content: flex-end;
  flex: 0 1 auto;
}

.header-search-input {
  width: min(220px, 100%);
}

.users-table {
  background: var(--n-card-color);
  border-radius: 6px;
}

.users-table :deep(.n-data-table-th) {
  text-align: center;
  font-weight: 600;
}

.users-table :deep(.n-data-table-td) {
  text-align: center;
}

@media (width <= 640px) {
  .header,
  .header-left,
  .header-right {
    align-items: stretch;
    flex-direction: column;
    width: 100%;
  }

  .header-search-input,
  .header-left :deep(.n-button),
  .header-right :deep(.n-button) {
    width: 100%;
  }
}
</style>
