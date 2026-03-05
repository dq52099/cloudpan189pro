<template>
  <div class="telegram-page">
    <n-tabs type="line" v-model:value="activeTab">
      <n-tab name="settings">Bot 设置</n-tab>
      <n-tab name="users">用户管理</n-tab>
      <n-tab name="send">发送消息</n-tab>
    </n-tabs>

    <!-- Bot 设置 -->
    <div v-if="activeTab === 'settings'" class="tab-content">
      <div class="setting-section">
        <div class="setting-item">
          <div class="item-left">
            <div class="item-title">启用 Telegram Bot</div>
            <div class="item-desc">开启后可以接收通知和管理文件</div>
          </div>
          <div class="item-right">
            <n-switch v-model:value="form.enable" />
          </div>
        </div>

        <div class="setting-item">
          <div class="item-left">
            <div class="item-title">Bot Token</div>
            <div class="item-desc">从 @BotFather 获取的 Token</div>
          </div>
          <div class="item-right">
            <n-input
              v-model:value="form.botToken"
              placeholder="请输入 Bot Token"
              clearable
              style="width: 320px"
              type="password"
              show-password-on="click"
            />
          </div>
        </div>

        <div class="setting-item">
          <div class="item-left">
            <div class="item-title">Chat ID</div>
            <div class="item-desc">接收通知的聊天 ID</div>
          </div>
          <div class="item-right">
            <n-input
              v-model:value="form.chatID"
              placeholder="请输入 Chat ID"
              clearable
              style="width: 320px"
            />
          </div>
        </div>

        <div class="setting-item">
          <div class="item-left">
            <div class="item-title">默认转存路径</div>
            <div class="item-desc">用户通过 Bot 转存文件时的默认保存路径</div>
          </div>
          <div class="item-right">
            <n-input
              v-model:value="form.defaultMountPath"
              placeholder="/转存"
              clearable
              style="width: 320px"
            />
          </div>
        </div>

        <div class="setting-item">
          <div class="item-left">
            <div class="item-title">启用通知</div>
            <div class="item-desc">入库任务完成时发送通知</div>
          </div>
          <div class="item-right">
            <n-switch v-model:value="form.enableNotify" />
          </div>
        </div>

        <div class="setting-item">
          <div class="item-left">
            <div class="item-title">API 地址</div>
            <div class="item-desc">Telegram API 地址（国内需要代理）</div>
          </div>
          <div class="item-right">
            <n-input
              v-model:value="form.apiURL"
              placeholder="https://api.telegram.org"
              clearable
              style="width: 320px"
            />
          </div>
        </div>

        <div class="setting-item">
          <div class="item-left">
            <div class="item-title">代理地址</div>
            <div class="item-desc">代理服务器地址（可选）</div>
          </div>
          <div class="item-right">
            <n-input
              v-model:value="form.proxyURL"
              placeholder="socks5://127.0.0.1:1080"
              clearable
              style="width: 320px"
            />
          </div>
        </div>

        <div class="setting-item">
          <div class="item-left">
            <div class="item-title">代理类型</div>
            <div class="item-desc">代理协议类型</div>
          </div>
          <div class="item-right">
            <n-select
              v-model:value="form.proxyType"
              :options="proxyTypeOptions"
              style="width: 320px"
            />
          </div>
        </div>

        <div class="setting-actions">
          <n-button type="primary" :loading="saving" @click="handleSave"> 保存设置 </n-button>
          <n-button :loading="testing" @click="handleTest" :disabled="!form.enable">
            测试连接
          </n-button>
        </div>
      </div>
    </div>

    <!-- 用户管理 -->
    <div v-else-if="activeTab === 'users'" class="tab-content">
      <div class="header">
        <n-input
          v-model:value="userSearch"
          placeholder="搜索用户..."
          clearable
          style="width: 240px; margin-right: 12px"
        >
          <template #prefix>
            <n-icon :size="16" :depth="3">
              <SearchOutline />
            </n-icon>
          </template>
        </n-input>
        <n-button type="primary" @click="loadUsers">
          <template #icon>
            <n-icon>
              <RefreshOutline />
            </n-icon>
          </template>
          刷新
        </n-button>
      </div>

      <n-data-table
        :columns="userColumns"
        :data="filteredUsers"
        :loading="loadingUsers"
        :row-key="(row: TelegramUser) => row.userID"
        :bordered="false"
        style="margin-top: 16px"
      />
    </div>

    <!-- 发送消息 -->
    <div v-else-if="activeTab === 'send'" class="tab-content">
      <div class="send-section">
        <n-form-item label="消息内容" path="message">
          <n-input
            v-model:value="sendMessage"
            type="textarea"
            placeholder="请输入要发送的消息..."
            :rows="6"
            show-count
          />
        </n-form-item>
        <n-button
          type="primary"
          :loading="sending"
          :disabled="!sendMessage.trim()"
          @click="handleSend"
        >
          发送消息
        </n-button>
      </div>
    </div>

    <!-- 编辑用户 Modal -->
    <n-modal v-model:show="showUserModal" preset="card" title="编辑用户" style="width: 480px">
      <n-form :model="editUserForm" label-placement="left">
        <n-form-item label="挂载路径">
          <n-input v-model:value="editUserForm.mountPath" placeholder="/转存/用户名" />
        </n-form-item>
        <n-form-item label="管理员权限">
          <n-switch v-model:value="editUserForm.isAdmin" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-button @click="showUserModal = false">取消</n-button>
        <n-button type="primary" :loading="savingUser" @click="handleSaveUser">保存</n-button>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useMessage } from 'naive-ui'
import { SearchOutline, RefreshOutline } from '@vicons/ionicons5'
import {
  getTelegramSetting,
  updateTelegramSetting,
  testTelegramConnection,
  getTelegramUsers,
  updateTelegramUser,
  sendTelegramMessage,
  type TelegramSetting,
  type TelegramUser,
} from '@/api/telegram'

const message = useMessage()

const activeTab = ref('settings')
const saving = ref(false)
const testing = ref(false)
const loadingUsers = ref(false)
const sending = ref(false)
const savingUser = ref(false)

const form = ref<TelegramSetting>({
  enable: false,
  botToken: '',
  proxyURL: '',
  proxyType: '',
  apiURL: 'https://api.telegram.org',
  chatID: '',
  defaultMountPath: '/转存',
  enableNotify: false,
})

const proxyTypeOptions = [
  { label: '无', value: '' },
  { label: 'SOCKS5', value: 'socks5' },
  { label: 'HTTP', value: 'http' },
  { label: 'HTTPS', value: 'https' },
]

const users = ref<TelegramUser[]>([])
const userSearch = ref('')
const showUserModal = ref(false)
const editUserForm = ref({
  userID: 0,
  mountPath: '',
  isAdmin: false,
})

const filteredUsers = computed(() => {
  if (!userSearch.value) return users.value
  const search = userSearch.value.toLowerCase()
  return users.value.filter(
    (u) =>
      u.username.toLowerCase().includes(search) ||
      u.firstName.toLowerCase().includes(search) ||
      u.lastName?.toLowerCase().includes(search)
  )
})

const userColumns = [
  {
    title: 'User ID',
    key: 'userID',
    width: 100,
  },
  {
    title: '用户名',
    key: 'username',
    width: 150,
  },
  {
    title: '姓名',
    key: 'firstName',
    width: 120,
    render: (row: TelegramUser) => `${row.firstName} ${row.lastName || ''}`.trim(),
  },
  {
    title: '挂载路径',
    key: 'mountPath',
    width: 180,
  },
  {
    title: '管理员',
    key: 'isAdmin',
    width: 80,
    render: (row: TelegramUser) => (row.isAdmin ? '是' : '否'),
  },
  {
    title: '最后活跃',
    key: 'lastSeenAt',
    width: 180,
    render: (row: TelegramUser) => {
      if (!row.lastSeenAt) return '-'
      return new Date(row.lastSeenAt).toLocaleString('zh-CN')
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 100,
    render: (row: TelegramUser) => {
      return h(
        NButton,
        {
          size: 'small',
          onClick: () => openEditUser(row),
        },
        { default: () => '编辑' }
      )
    },
  },
]

import { NButton } from 'naive-ui'

const loadSetting = async () => {
  try {
    const res = await getTelegramSetting()
    if (res.code === 200 && res.data) {
      form.value = res.data
    }
  } catch {
    message.error('加载设置失败')
  }
}

const handleSave = async () => {
  saving.value = true
  try {
    const res = await updateTelegramSetting(form.value)
    if (res.code === 200) {
      message.success('保存成功')
      loadSetting()
    } else {
      message.error(res.msg || '保存失败')
    }
  } catch {
    message.error('保存失败')
  } finally {
    saving.value = false
  }
}

const handleTest = async () => {
  testing.value = true
  try {
    const res = await testTelegramConnection()
    if (res.code === 200) {
      message.success('连接测试成功！')
    } else {
      message.error(res.msg || '连接测试失败')
    }
  } catch {
    message.error('连接测试失败')
  } finally {
    testing.value = false
  }
}

const loadUsers = async () => {
  loadingUsers.value = true
  try {
    const res = await getTelegramUsers()
    if (res.code === 200) {
      users.value = res.data || []
    }
  } catch {
    message.error('加载用户列表失败')
  } finally {
    loadingUsers.value = false
  }
}

const openEditUser = (user: TelegramUser) => {
  editUserForm.value = {
    userID: user.userID,
    mountPath: user.mountPath,
    isAdmin: user.isAdmin,
  }
  showUserModal.value = true
}

const handleSaveUser = async () => {
  savingUser.value = true
  try {
    const res = await updateTelegramUser(editUserForm.value)
    if (res.code === 200) {
      message.success('保存成功')
      showUserModal.value = false
      loadUsers()
    } else {
      message.error(res.msg || '保存失败')
    }
  } catch {
    message.error('保存失败')
  } finally {
    savingUser.value = false
  }
}

const sendMessage = ref('')

const handleSend = async () => {
  if (!sendMessage.value.trim()) {
    message.warning('请输入消息内容')
    return
  }
  sending.value = true
  try {
    const res = await sendTelegramMessage({ message: sendMessage.value })
    if (res.code === 200) {
      message.success('发送成功')
      sendMessage.value = ''
    } else {
      message.error(res.msg || '发送失败')
    }
  } catch {
    message.error('发送失败')
  } finally {
    sending.value = false
  }
}

onMounted(() => {
  loadSetting()
  loadUsers()
})
</script>

<style scoped>
.telegram-page {
  padding: 0;
}

.tab-content {
  padding-top: 20px;
}

.setting-section {
  max-width: 800px;
}

.setting-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 0;
  border-bottom: 1px solid #f0f0f0;
}

.item-left {
  flex: 1;
}

.item-title {
  font-size: 14px;
  font-weight: 500;
  color: #333;
}

.item-desc {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

.item-right {
  flex-shrink: 0;
}

.setting-actions {
  margin-top: 24px;
  display: flex;
  gap: 12px;
}

.header {
  display: flex;
  align-items: center;
  margin-top: 16px;
}

.send-section {
  max-width: 600px;
  margin-top: 16px;
}
</style>
