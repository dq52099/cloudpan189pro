<template>
  <div class="usergroups-page">
    <!-- 头部区域 -->
    <div class="header">
      <div class="header-left">
        <n-input
          v-model:value="searchKeyword"
          placeholder="请输入用户组名称搜索"
          clearable
          style="width: 200px; margin-right: 12px"
          @keyup.enter="handleSearch"
        />
        <n-button type="primary" @click="handleSearch" style="margin-right: 8px"> 搜索 </n-button>
        <n-button @click="handleReset"> 重置 </n-button>
      </div>
      <div class="header-right">
        <n-button type="primary" @click="handleAddUserGroup">
          <template #icon>
            <n-icon>
              <PeopleOutline />
            </n-icon>
          </template>
          添加用户组
        </n-button>
      </div>
    </div>

    <!-- 用户组列表表格 -->
    <n-data-table
      :columns="columns"
      :data="tableData"
      :loading="loading"
      :pagination="paginationReactive"
      class="usergroups-table"
      remote
    />

    <!-- 添加用户组弹窗 -->
    <AddUserGroupModal v-model:show="showAddModal" @success="handleAddSuccess" />

    <!-- 修改用户组名称弹窗 -->
    <ModifyUserGroupNameModal
      v-model:show="showModifyNameModal"
      :user-group-info="currentModifyUserGroup"
      @success="handleModifyNameSuccess"
    />

    <!-- 绑定存储弹窗 -->
    <BindFilesModal
      v-model:show="showBindFilesModal"
      :user-group-info="currentBindUserGroup"
      @success="handleBindFilesSuccess"
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
import { PeopleOutline, TrashOutline, CreateOutline, LinkOutline } from '@vicons/ionicons5'
import { getUserGroupList, deleteUserGroup } from '@/api/usergroup'
import { AddUserGroupModal, ModifyUserGroupNameModal, BindFilesModal } from '@/components/usergroup'
import { formatDateTime } from '@/utils/time'

// 表格数据
const tableData = ref<Models.UserGroup[]>([])
const loading = ref(false)
const searchKeyword = ref('')
let userGroupListRequestId = 0

// 添加用户组相关
const showAddModal = ref(false)

// 修改用户组名称相关
const showModifyNameModal = ref(false)
const currentModifyUserGroup = ref<Models.UserGroup | null>(null)

// 绑定文件相关
const showBindFilesModal = ref(false)
const currentBindUserGroup = ref<Models.UserGroup | null>(null)

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
    fetchUserGroupList()
  },
  onUpdatePageSize: (pageSize: number) => {
    paginationReactive.pageSize = pageSize
    paginationReactive.page = 1
    fetchUserGroupList()
  },
})

// 获取用户组列表
const fetchUserGroupList = () => {
  const currentRequestId = ++userGroupListRequestId
  loading.value = true

  const params = {
    currentPage: paginationReactive.page || 1,
    pageSize: paginationReactive.pageSize || 10,
    name: searchKeyword.value || undefined,
  }

  getUserGroupList(params)
    .then((response) => {
      if (currentRequestId !== userGroupListRequestId) return

      if (response.code === 200 && response.data) {
        tableData.value = response.data.data || []
        paginationReactive.itemCount = response.data.total || 0
      }
    })
    .catch((error) => {
      if (currentRequestId !== userGroupListRequestId) return

      console.error('获取用户组列表失败:', error)
    })
    .finally(() => {
      if (currentRequestId !== userGroupListRequestId) return

      loading.value = false
    })
}

// 搜索
const handleSearch = () => {
  paginationReactive.page = 1 // 搜索时重置到第一页
  fetchUserGroupList()
}

// 重置
const handleReset = () => {
  searchKeyword.value = ''
  paginationReactive.page = 1 // 重置时回到第一页
  fetchUserGroupList()
}

// 添加用户组
const handleAddUserGroup = () => {
  // 显示弹窗
  showAddModal.value = true
}

// 添加用户组成功回调
const handleAddSuccess = () => {
  // 刷新用户组列表
  fetchUserGroupList()
}

// 删除用户组
const handleDeleteUserGroup = (userGroupId: number) => {
  deleteUserGroup({ id: userGroupId })
    .then((response) => {
      if (response.code === 200) {
        message.success('删除用户组成功')
        // 刷新用户组列表
        fetchUserGroupList()
      } else {
        message.error(response.msg || '删除用户组失败')
      }
    })
    .catch((error) => {
      console.error('删除用户组失败:', error)
      message.error('删除用户组失败')
    })
}

// 修改用户组名称
const handleModifyName = (userGroup: Models.UserGroup) => {
  currentModifyUserGroup.value = userGroup
  showModifyNameModal.value = true
}

// 修改用户组名称成功回调
const handleModifyNameSuccess = () => {
  // 刷新用户组列表
  fetchUserGroupList()
  currentModifyUserGroup.value = null
}

// 绑定文件
const handleBindFiles = (userGroup: Models.UserGroup) => {
  currentBindUserGroup.value = userGroup
  showBindFilesModal.value = true
}

// 绑定文件成功回调
const handleBindFilesSuccess = () => {
  // 绑定文件不需要刷新列表
  currentBindUserGroup.value = null
}

// 表格列定义
const columns: DataTableColumns<Models.UserGroup> = [
  {
    title: '用户组ID',
    key: 'id',
    width: 100,
    align: 'center',
  },
  {
    title: '用户组名称',
    key: 'name',
    width: 200,
    align: 'center',
  },
  {
    title: '用户数量',
    key: 'userCount',
    width: 120,
    align: 'center',
    render(row) {
      return `${row.userCount || 0}`
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
    title: '更新时间',
    key: 'updatedAt',
    width: 180,
    align: 'center',
    render(row) {
      return formatDateTime(row.updatedAt)
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    align: 'center',
    render(row) {
      return h(
        NSpace,
        { size: 'small' },
        {
          default: () => [
            // 修改名称按钮
            h(
              NButton,
              {
                size: 'tiny',
                type: 'primary',
                secondary: true,
                onClick: () => handleModifyName(row),
              },
              {
                icon: () => h(NIcon, { size: 12 }, { default: () => h(CreateOutline) }),
                default: () => '修改',
              }
            ),
            // 绑定存储按钮
            h(
              NButton,
              {
                size: 'tiny',
                type: 'info',
                secondary: true,
                onClick: () => handleBindFiles(row),
              },
              {
                icon: () => h(NIcon, { size: 12 }, { default: () => h(LinkOutline) }),
                default: () => '绑定存储',
              }
            ),
            // 删除按钮
            h(
              NPopconfirm,
              {
                onPositiveClick: () => handleDeleteUserGroup(row.id),
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
                default: () => `确定要删除用户组 "${row.name}" 吗？此操作不可撤销。`,
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
  fetchUserGroupList()
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

.usergroups-table {
  background: var(--n-card-color);
  border-radius: 6px;
}

.usergroups-table :deep(.n-data-table-th) {
  text-align: center;
  font-weight: 600;
}

.usergroups-table :deep(.n-data-table-td) {
  text-align: center;
}
</style>
