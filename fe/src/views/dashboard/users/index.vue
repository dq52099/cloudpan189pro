<template>
  <div class="users-page">
    <!-- 头部区域 -->
    <div class="header">
      <div class="header-left">
        <n-input
          v-model:value="searchKeyword"
          placeholder="请输入用户名搜索"
          clearable
          style="width: 200px; margin-right: 12px"
          @keyup.enter="handleSearch"
        />
        <n-button type="primary" @click="handleSearch" style="margin-right: 8px"> 搜索 </n-button>
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
import { ref, reactive, onMounted, h } from 'vue'
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

// 表格数据
const tableData = ref<Models.UserInfo[]>([])
const loading = ref(false)
const searchKeyword = ref('')
let userListRequestId = 0

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
  const currentRequestId = ++userListRequestId
  loading.value = true

  const params = {
    currentPage: paginationReactive.page || 1,
    pageSize: paginationReactive.pageSize || 10,
    username: searchKeyword.value || undefined,
  }

  getUserList(params)
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
      }
    })
    .catch((error) => {
      if (currentRequestId !== userListRequestId) return

      console.error('获取用户列表失败:', error)
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
  deleteUser({ id: userId })
    .then((response) => {
      if (response.code === 200) {
        message.success('删除用户成功')
        // 刷新用户列表
        fetchUserList()
      } else {
        message.error(response.msg || '删除用户失败')
      }
    })
    .catch((error) => {
      console.error('删除用户失败:', error)
      message.error('删除用户失败')
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
  const newStatus = user.status === 1 ? 2 : 1
  const actionText = newStatus === 1 ? '启用' : '禁用'

  toggleUserStatus({ id: user.id, status: newStatus })
    .then((response) => {
      if (response.code === 200) {
        message.success(`${actionText}用户成功`)
        // 刷新用户列表
        fetchUserList()
      } else {
        message.error(response.msg || `${actionText}用户失败`)
      }
    })
    .catch((error) => {
      console.error(`${actionText}用户失败:`, error)
      message.error(`${actionText}用户失败`)
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
        2: '受限',
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
              },
              {
                trigger: () =>
                  h(
                    NButton,
                    {
                      size: 'tiny',
                      type: row.status === 1 ? 'warning' : 'success',
                      secondary: true,
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
              },
              {
                trigger: () =>
                  h(
                    NButton,
                    {
                      size: 'tiny',
                      type: 'error',
                      secondary: true,
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
  fetchUserList()
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
</style>
