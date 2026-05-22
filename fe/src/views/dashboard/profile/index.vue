<template>
  <div class="profile-page">
    <!-- 个人信息卡片 -->
    <n-card title="个人信息" class="info-card" :bordered="false">
      <n-descriptions
        :column="2"
        label-placement="left"
        label-style="width: 120px; font-weight: 500;"
      >
        <n-descriptions-item label="用户名">
          <n-text strong>{{ userInfo.username || '-' }}</n-text>
        </n-descriptions-item>
        <n-descriptions-item label="用户ID">
          <n-text>{{ userInfo.id || '-' }}</n-text>
        </n-descriptions-item>
        <n-descriptions-item label="状态">
          <n-tag :type="getUserStatusType(userInfo.status)" size="small">
            {{ getUserStatusText(userInfo.status) }}
          </n-tag>
        </n-descriptions-item>
        <n-descriptions-item label="用户组">
          <n-text>{{ userInfo.groupName || '-' }}</n-text>
        </n-descriptions-item>
        <n-descriptions-item label="管理员权限">
          <n-tag :type="userInfo.isAdmin ? 'success' : 'default'" size="small">
            {{ userInfo.isAdmin ? '是' : '否' }}
          </n-tag>
        </n-descriptions-item>
        <n-descriptions-item label="创建时间">
          <n-text>{{ formatDate(userInfo.createdAt) }}</n-text>
        </n-descriptions-item>
      </n-descriptions>
    </n-card>

    <!-- 快捷操作卡片 -->
    <n-card title="快捷操作" class="action-card" :bordered="false">
      <div class="action-buttons">
        <n-button type="primary" size="large" @click="handleChangePassword">
          <template #icon>
            <n-icon>
              <KeyOutline />
            </n-icon>
          </template>
          修改密码
        </n-button>
      </div>
    </n-card>

    <!-- 修改密码弹窗 -->
    <ChangePasswordModal
      v-model:show="showChangePasswordModal"
      @success="handleChangePasswordSuccess"
    />
  </div>
</template>

<script setup lang="ts">
import { NCard, NDescriptions, NDescriptionsItem, NTag, NText, NButton, NIcon } from 'naive-ui'
import { KeyOutline } from '@vicons/ionicons5'
import { ref } from 'vue'
import { useUserStore } from '@/stores'
import ChangePasswordModal from '@/components/profile/ChangePasswordModal.vue'

const userStore = useUserStore()

// 用户信息
const userInfo = userStore.get()

// 修改密码弹窗
const showChangePasswordModal = ref(false)

// 获取用户状态类型
const getUserStatusType = (status: number) => {
  switch (status) {
    case 1:
      return 'success'
    case 2:
      return 'warning'
    case 0:
    default:
      return 'error'
  }
}

// 获取用户状态文本
const getUserStatusText = (status: number) => {
  switch (status) {
    case 1:
      return '正常'
    case 2:
      return '受限'
    case 0:
    default:
      return '禁用'
  }
}

// 格式化日期
const formatDate = (dateString: string) => {
  if (!dateString) return '-'
  return new Date(dateString).toLocaleString('zh-CN')
}

// 修改密码
const handleChangePassword = () => {
  showChangePasswordModal.value = true
}

// 修改密码成功回调
const handleChangePasswordSuccess = () => {
  // 密码修改成功后的处理，组件内部已经处理了成功提示和退出登录
}
</script>

<style scoped>
.profile-page {
  padding: 0;
  background: var(--n-color-target);
}

.info-card,
.action-card {
  background: var(--n-card-color);
  border-radius: 12px;
  border: 1px solid var(--n-border-color);
  margin-bottom: 20px;
  box-shadow: 0 2px 8px rgb(0 0 0 / 6%);
}

.info-card :deep(.n-card__content) {
  padding: 24px;
}

.action-card :deep(.n-card__content) {
  padding: 24px;
}

.info-card :deep(.n-descriptions) {
  margin: 0;
}

.info-card :deep(.n-descriptions-item) {
  margin-bottom: 16px;
}

.info-card :deep(.n-descriptions-item:last-child) {
  margin-bottom: 0;
}

.info-card :deep(.n-descriptions-item__label) {
  color: var(--n-text-color-2);
}

.info-card :deep(.n-descriptions-item__content) {
  color: var(--n-text-color);
}

.action-buttons {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

/* 响应式设计 */
@media (width <= 1200px) {
  .info-card :deep(.n-descriptions) {
    --n-column: 1;
  }
}

@media (width <= 768px) {
  .profile-page {
    padding: 0;
  }

  .info-card,
  .action-card {
    margin: 0 0 16px;
    border-radius: 8px;
  }

  .info-card :deep(.n-card__content),
  .action-card :deep(.n-card__content) {
    padding: 16px;
  }

  .info-card :deep(.n-descriptions-item) {
    margin-bottom: 12px;
  }

  .action-buttons {
    gap: 12px;
  }
}

@media (width <= 480px) {
  .info-card :deep(.n-card__content),
  .action-card :deep(.n-card__content) {
    padding: 12px;
  }

  .action-buttons {
    flex-direction: column;
  }
}
</style>
