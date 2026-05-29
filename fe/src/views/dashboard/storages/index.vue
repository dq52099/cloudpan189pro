<template>
  <div class="storages-page">
    <!-- 头部区域 -->
    <div class="header">
      <!-- 第一行 -->
      <div class="header-row">
        <div class="header-search">
          <n-input
            v-model:value="searchKeyword"
            placeholder="请输入路径搜索"
            clearable
            style="width: 200px; margin-right: 12px"
            @keyup.enter="handleSearch"
          >
            <template #prefix>
              <n-icon :size="16" :depth="3">
                <SearchOutline />
              </n-icon>
            </template>
          </n-input>
          <n-select
            v-model:value="selectedTaskLogStatus"
            placeholder="扫描状态"
            clearable
            style="width: 120px; margin-right: 12px"
            :options="taskLogStatusOptions"
            @update:value="handleSearch"
          />
          <n-button type="primary" @click="handleSearch" style="margin-right: 8px">
            <template #icon>
              <n-icon>
                <SearchOutline />
              </n-icon>
            </template>
            搜索
          </n-button>
          <n-button @click="handleReset">
            <template #icon>
              <n-icon>
                <RefreshOutline />
              </n-icon>
            </template>
            重置
          </n-button>
          <n-text v-if="pageAutoRefreshStore.autoRefreshEnabled" depth="3" style="margin-left: 8px">
            上次刷新：{{ refreshTime.format('YYYY-MM-DD HH:mm:ss') }}
          </n-text>
        </div>
        <div class="header-actions-left">
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-button text @click="handlePageSettings" style="font-size: 16px">
                <template #icon>
                  <n-icon>
                    <SettingsOutline />
                  </n-icon>
                </template>
              </n-button>
            </template>
            页面设置
          </n-tooltip>
          <n-dropdown trigger="click" :options="addMountOptions" @select="handleSelectMountType">
            <n-button type="primary">
              <template #icon>
                <n-icon>
                  <AddOutline />
                </n-icon>
              </template>
              新增挂载
            </n-button>
          </n-dropdown>
          <n-button
            type="error"
            ghost
            :loading="clearingAll"
            :disabled="
              clearingAll || batchSubmitting || storageDangerDialogOpen || hasRefreshingStorage
            "
            @click="handleClearAll"
          >
            <template #icon>
              <n-icon>
                <TrashOutline />
              </n-icon>
            </template>
            清空所有
          </n-button>
          <n-button v-if="!isBatchMode" @click="enterBatchMode">批量管理</n-button>
        </div>
      </div>
      <!-- 第二行 -->
      <div v-if="isBatchMode" class="header-row" style="justify-content: flex-end; gap: 24px">
        <n-button
          :type="isAllSelected ? 'warning' : 'default'"
          @click="toggleSelectAll"
          class="batch-btn"
          :disabled="isSelectAllPagesBlocked"
        >
          {{ isAllSelected ? '取消当页' : '全选当页' }}
        </n-button>
        <n-button
          :type="isAllSelectedAllPages ? 'warning' : 'info'"
          @click="selectAllPages"
          :loading="isSelectingAllPages"
          class="batch-btn"
          :disabled="isSelectAllPagesBlocked"
        >
          {{ isAllSelectedAllPages ? '取消全选' : '全选所有' }}
        </n-button>
        <n-button
          type="info"
          @click="handleBatchRefresh(false)"
          class="batch-btn"
          :disabled="
            batchSubmitting ||
            storageDangerDialogOpen ||
            hasRefreshingStorage ||
            selectedIds.length === 0 ||
            selectedIds.length > maxBatchActionIds
          "
        >
          <template #icon
            ><n-icon><RefreshOutline /></n-icon
          ></template>
          刷新选中 ({{ selectedIds.length }})
        </n-button>
        <n-button
          type="warning"
          @click="handleBatchRefresh(true)"
          class="batch-btn"
          :disabled="
            batchSubmitting ||
            storageDangerDialogOpen ||
            hasRefreshingStorage ||
            selectedIds.length === 0 ||
            selectedIds.length > maxBatchActionIds
          "
        >
          <template #icon
            ><n-icon><RefreshOutline /></n-icon
          ></template>
          深度刷新 ({{ selectedIds.length }})
        </n-button>
        <n-button
          type="primary"
          @click="handleBatchModifyToken"
          class="batch-btn"
          :disabled="
            batchSubmitting ||
            storageDangerDialogOpen ||
            hasRefreshingStorage ||
            selectedIds.length === 0 ||
            selectedIds.length > maxBatchModifyTokenIds
          "
        >
          <template #icon
            ><n-icon><KeyOutline /></n-icon
          ></template>
          修改令牌 ({{ selectedIds.length }})
        </n-button>
        <n-button
          type="error"
          @click="handleBatchDelete"
          class="batch-btn"
          :disabled="
            batchSubmitting ||
            storageDangerDialogOpen ||
            hasRefreshingStorage ||
            selectedIds.length === 0 ||
            selectedIds.length > maxBatchActionIds
          "
        >
          <template #icon
            ><n-icon><TrashOutline /></n-icon
          ></template>
          删除选中 ({{ selectedIds.length }})
        </n-button>
        <n-button @click="exitBatchMode" class="batch-btn" :disabled="batchSubmitting">
          取消
        </n-button>
      </div>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <n-spin size="large">
        <template #description>
          <n-text depth="2">正在加载存储数据...</n-text>
        </template>
      </n-spin>
    </div>

    <!-- 存储卡片列表 -->
    <div class="storage-cards">
      <n-card
        v-for="storage in tableData"
        :key="storage.mountPointId"
        class="storage-card"
        :class="{ 'is-selected': selectedIds.includes(storage.mountPointId) }"
        hoverable
        :bordered="false"
        @click="handleCardClick(storage.mountPointId)"
      >
        <!-- 选择遮罩 -->
        <div v-if="isBatchMode" class="selection-overlay">
          <n-checkbox
            :checked="selectedIds.includes(storage.mountPointId)"
            class="selection-checkbox"
            size="large"
            :disabled="batchSubmitting"
            @click.stop="toggleSelection(storage.mountPointId)"
          />
        </div>
        <!-- 存储卡片内容 -->
        <template #header>
          <div class="card-header">
            <div class="storage-info">
              <div class="storage-title">
                <n-text strong class="storage-name">
                  {{ storage.name || '未命名存储' }}
                </n-text>
              </div>
              <n-ellipsis class="storage-path" :tooltip="{ placement: 'top' }">
                {{ storage.fullPath || '-' }}
              </n-ellipsis>
            </div>

            <!-- 非批量模式才显示操作按钮 -->
            <div class="storage-actions" v-if="!isBatchMode">
              <n-button
                size="small"
                quaternary
                circle
                :disabled="isStorageActionBlocked(storage.mountPointId)"
                @click="handleModifyToken(storage)"
              >
                <template #icon>
                  <n-icon :size="16">
                    <KeyOutline />
                  </n-icon>
                </template>
              </n-button>

              <n-button
                size="small"
                quaternary
                circle
                :loading="isStorageDeleting(storage.mountPointId)"
                :disabled="isStorageActionBlocked(storage.mountPointId)"
                @click="handleDelete(storage)"
              >
                <template #icon>
                  <n-icon :size="16">
                    <TrashOutline />
                  </n-icon>
                </template>
              </n-button>

              <n-dropdown :options="getRefreshOptions(storage.mountPointId)" trigger="click">
                <n-button
                  size="small"
                  quaternary
                  circle
                  :loading="isStorageRefreshing(storage.mountPointId)"
                  :disabled="isStorageActionBlocked(storage.mountPointId)"
                >
                  <template #icon>
                    <n-icon :size="16">
                      <RefreshOutline />
                    </n-icon>
                  </template>
                </n-button>
              </n-dropdown>
            </div>
          </div>
        </template>

        <div class="card-content">
          <!-- 基础信息区域 - 使用flex容器包裹前三个字段 -->
          <div class="basic-info-container">
            <div class="info-item">
              <div class="info-label">
                <n-icon :size="14" class="info-icon">
                  <FolderOutline />
                </n-icon>
                <span>存储类型</span>
              </div>
              <n-tag :color="getOsTypeColor(storage.osType)" size="small" class="info-tag">
                {{ getOsTypeDisplayName(storage.osType) }}
              </n-tag>
            </div>

            <div class="info-item">
              <div class="info-label">
                <n-icon :size="14" class="info-icon">
                  <KeyOutline />
                </n-icon>
                <span>绑定令牌</span>
              </div>
              <n-text class="info-value">{{ storage.tokenName || '未绑定' }}</n-text>
            </div>

            <div class="info-item">
              <div class="info-label">
                <n-icon :size="14" class="info-icon">
                  <DocumentsOutline />
                </n-icon>
                <span>文件数量</span>
              </div>
              <n-text class="info-value">{{ storage.fileCount || 0 }}</n-text>
            </div>
          </div>

          <div class="additional-info">
            <div class="info-item">
              <div class="refresh-header">
                <div class="refresh-title-section">
                  <div class="info-label">
                    <n-icon :size="14" class="info-icon">
                      <RefreshOutline />
                    </n-icon>
                    <span>自动刷新</span>
                  </div>
                  <n-tag
                    v-if="storage.enableAutoRefresh"
                    :type="storage.isInAutoRefreshPeriod ? 'success' : 'warning'"
                    size="small"
                  >
                    {{ computedRefreshStatusText(storage) }}
                  </n-tag>
                  <n-tag v-else type="default" size="small">未启用</n-tag>
                </div>
                <n-button
                  size="tiny"
                  type="primary"
                  :disabled="isStorageActionBlocked(storage.mountPointId)"
                  @click.stop="handleEditAutoRefresh(storage)"
                >
                  编辑
                </n-button>
              </div>
              <div v-if="storage.enableAutoRefresh" class="refresh-details">
                <div class="refresh-status">
                  <n-text depth="3" class="refresh-detail"> {{ storage.refreshInterval }}m </n-text>
                  <n-text depth="3" class="refresh-detail">
                    {{ storage.enableDeepRefresh ? '深度刷新' : '普通刷新' }}
                  </n-text>
                </div>
                <n-text depth="3" class="refresh-period">
                  {{ formatRefreshPeriod(storage) }}
                </n-text>
              </div>
            </div>

            <!-- 最近一次运行日志 -->
            <div v-if="storage.taskLogs && storage.taskLogs.length > 0" class="info-item">
              <div class="task-log-header">
                <div class="task-log-first-row">
                  <n-popover trigger="hover">
                    <template #trigger>
                      <div class="info-label">
                        <n-icon :size="14" class="info-icon">
                          <component :is="getTaskStatusInfo(storage.taskLogs[0].status).icon" />
                        </n-icon>
                        <span>最近运行</span>
                        <n-text depth="3" class="task-log-time">
                          {{ formatTaskLogTime(storage.taskLogs[0]) }}
                        </n-text>
                      </div>
                    </template>
                    <n-text depth="1"> {{ storage.taskLogs[0].title }}; </n-text>
                    <n-text depth="2">
                      {{ storage.taskLogs[0].desc }}
                    </n-text>
                  </n-popover>
                  <n-popover trigger="hover" :disabled="storage.taskLogs[0].result ? false : true">
                    <template #trigger>
                      <n-tag
                        :type="getTaskStatusInfo(storage.taskLogs[0].status).type"
                        size="small"
                      >
                        {{ getTaskStatusInfo(storage.taskLogs[0].status).text }}
                      </n-tag>
                    </template>
                    {{ storage.taskLogs[0].result }}
                  </n-popover>
                </div>
                <div v-if="storage.enableAutoRefresh" class="next-run-time">
                  <n-text depth="3">下次 {{ formatNextRunTime(storage) }}</n-text>
                </div>
              </div>
              <div class="task-log-content"></div>
            </div>
          </div>
        </div>

        <template #footer>
          <div class="card-footer">
            <div class="footer-time">
              <n-icon :size="12" class="footer-icon">
                <TimeOutline />
              </n-icon>
              <n-text depth="3" class="footer-text">
                创建时间：{{ formatDateTime(storage.createdAt) }}
              </n-text>
            </div>
            <div class="footer-time">
              <n-icon :size="12" class="footer-icon">
                <TimeOutline />
              </n-icon>
              <n-text depth="3" class="footer-text">
                更新时间：{{ formatDateTime(storage.updatedAt) }}
              </n-text>
            </div>
          </div>
        </template>
      </n-card>

      <!-- 空状态 -->
      <div v-if="tableData.length === 0" class="empty-state">
        <n-empty description="暂无存储数据" size="large">
          <template #icon>
            <n-icon size="64" :depth="3">
              <ServerOutline />
            </n-icon>
          </template>
          <template #extra>
            <n-text depth="3">您还没有配置任何存储挂载点</n-text>
          </template>
        </n-empty>
      </div>
    </div>

    <!-- 分页 -->
    <div v-if="!loading && tableData.length > 0" class="pagination-container">
      <n-pagination
        v-model:page="paginationReactive.page"
        v-model:page-size="paginationReactive.pageSize"
        :item-count="paginationReactive.itemCount"
        :page-sizes="paginationReactive.pageSizes"
        show-size-picker
        @update:page="handlePageChange"
        @update:page-size="handlePageSizeChange"
      >
        <template #prefix="{ itemCount }"> 共 {{ itemCount }} 项 </template>
      </n-pagination>
    </div>

    <!-- 自动刷新配置弹窗 -->
    <n-modal
      :show="showAutoRefreshModal"
      preset="dialog"
      title="自动刷新配置"
      :closable="!autoRefreshSubmitting"
      :mask-closable="!autoRefreshSubmitting"
      :close-on-esc="!autoRefreshSubmitting"
      @update:show="handleAutoRefreshModalShowUpdate"
    >
      <div class="auto-refresh-config">
        <n-form
          ref="autoRefreshFormRef"
          :model="autoRefreshForm"
          :rules="autoRefreshRules"
          label-placement="left"
          label-width="120px"
        >
          <n-form-item label="启用自动刷新" path="enableAutoRefresh">
            <n-switch
              v-model:value="autoRefreshForm.enableAutoRefresh"
              :disabled="autoRefreshSubmitting"
            />
          </n-form-item>

          <template v-if="autoRefreshForm.enableAutoRefresh">
            <n-form-item label="刷新间隔(分钟)" path="refreshInterval">
              <n-input-number
                v-model:value="autoRefreshForm.refreshInterval"
                :min="30"
                :max="1440"
                placeholder="30-1440分钟"
                style="width: 100%"
                :disabled="autoRefreshSubmitting"
              />
            </n-form-item>

            <n-form-item label="持续天数" path="autoRefreshDays">
              <n-input-number
                v-model:value="autoRefreshForm.autoRefreshDays"
                :min="1"
                :max="365"
                placeholder="1-365天"
                style="width: 100%"
                :disabled="autoRefreshSubmitting"
              />
            </n-form-item>

            <n-form-item label="开始日期" path="refreshBeginAt">
              <n-date-picker
                v-model:value="autoRefreshForm.refreshBeginAt"
                type="date"
                placeholder="选择开始日期"
                style="width: 100%"
                :disabled="autoRefreshSubmitting"
              />
            </n-form-item>

            <n-form-item label="深度刷新" path="enableDeepRefresh">
              <n-switch
                v-model:value="autoRefreshForm.enableDeepRefresh"
                :disabled="autoRefreshSubmitting"
              />
            </n-form-item>
          </template>
        </n-form>
      </div>

      <template #action>
        <n-button :disabled="autoRefreshSubmitting" @click="closeAutoRefreshModal()">取消</n-button>
        <n-button
          type="primary"
          @click="handleAutoRefreshConfirm"
          :loading="autoRefreshSubmitting"
          :disabled="autoRefreshSubmitting"
        >
          确认
        </n-button>
      </template>
    </n-modal>

    <!-- 页面设置弹窗 -->
    <n-modal
      v-model:show="showPageSettingsModal"
      preset="dialog"
      title="页面设置"
      style="width: 420px"
    >
      <div class="page-settings-config">
        <div class="settings-section">
          <div class="setting-item">
            <div class="setting-label">启用自动刷新</div>
            <n-switch
              v-model:value="pageSettingsForm.autoRefreshEnabled"
              @update:value="handlePageAutoRefreshToggle"
              size="medium"
            >
              <template #checked>已开启</template>
              <template #unchecked>已关闭</template>
            </n-switch>
          </div>

          <div v-if="pageSettingsForm.autoRefreshEnabled" class="setting-item">
            <div class="setting-label">刷新间隔</div>
            <n-select
              v-model:value="pageSettingsForm.refreshInterval"
              :options="refreshIntervalOptions"
              style="width: 160px"
              @update:value="handlePageRefreshIntervalChange"
              size="small"
            />
          </div>

          <div v-if="pageSettingsForm.autoRefreshEnabled" class="setting-item">
            <div class="setting-label">下次刷新</div>
            <n-text depth="3">{{ nextRefreshTime.format('HH:mm:ss') }}</n-text>
          </div>
        </div>
      </div>

      <template #action>
        <n-button @click="showPageSettingsModal = false">关闭</n-button>
      </template>
    </n-modal>

    <!-- 修改令牌弹窗 -->
    <n-modal
      :show="showModifyTokenModal"
      preset="dialog"
      title="修改绑定令牌"
      :closable="!modifyTokenSubmitting"
      :mask-closable="!modifyTokenSubmitting"
      :close-on-esc="!modifyTokenSubmitting"
      @update:show="handleModifyTokenModalShowUpdate"
    >
      <div class="modify-token-config">
        <n-form label-placement="left" label-width="100px">
          <n-form-item label="存储名称">
            <n-text>{{ currentModifyStorage?.name || '未命名存储' }}</n-text>
          </n-form-item>

          <n-form-item label="当前令牌">
            <n-text depth="3">{{ currentModifyStorage?.tokenName || '未绑定' }}</n-text>
          </n-form-item>

          <n-form-item label="选择令牌">
            <n-select
              v-model:value="selectedTokenId"
              :options="cloudTokenOptions"
              placeholder="请选择令牌或解绑令牌"
              clearable
              style="width: 100%"
              :disabled="modifyTokenSubmitting"
            />
          </n-form-item>
        </n-form>
      </div>

      <template #action>
        <n-button :disabled="modifyTokenSubmitting" @click="closeModifyTokenModal()">取消</n-button>
        <n-button
          type="primary"
          @click="handleModifyTokenConfirm"
          :loading="modifyTokenSubmitting"
          :disabled="modifyTokenSubmitting"
        >
          确认
        </n-button>
      </template>
    </n-modal>

    <!-- 批量修改令牌弹窗 -->
    <n-modal
      :show="showBatchModifyTokenModal"
      preset="dialog"
      title="批量修改令牌"
      :closable="!batchSubmitting"
      :mask-closable="!batchSubmitting"
      :close-on-esc="!batchSubmitting"
      @update:show="handleBatchModifyTokenModalShowUpdate"
    >
      <div class="modify-token-config">
        <n-form label-placement="left" label-width="100px">
          <n-form-item label="已选中">
            <n-text>{{ batchModifyTokenIds.length }} 个挂载点</n-text>
          </n-form-item>

          <n-form-item label="选择令牌">
            <n-select
              v-model:value="batchModifyTokenId"
              :options="cloudTokenOptions"
              placeholder="请选择令牌或解绑令牌"
              clearable
              style="width: 100%"
              :disabled="batchSubmitting"
            />
          </n-form-item>
        </n-form>
      </div>

      <template #action>
        <n-button @click="closeBatchModifyTokenModal()" :disabled="batchSubmitting">
          取消
        </n-button>
        <n-button
          type="primary"
          @click="handleBatchModifyTokenConfirm"
          :loading="batchSubmitting"
          :disabled="batchSubmitting"
        >
          确认
        </n-button>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, computed, h } from 'vue' // 引入 h
import {
  NCheckbox,
  NInput,
  NButton,
  NCard,
  NText,
  NTag,
  NSpin,
  NEmpty,
  NPagination,
  NEllipsis,
  NIcon,
  NModal,
  NDropdown,
  NForm,
  NFormItem,
  NSwitch,
  NInputNumber,
  NDatePicker,
  NSelect,
  NTooltip,
  useMessage,
  useDialog,
  type PaginationProps,
  type DropdownOption,
  type FormInst,
  type FormRules,
} from 'naive-ui'
import {
  SearchOutline,
  RefreshOutline,
  FolderOutline,
  KeyOutline,
  TimeOutline,
  ServerOutline,
  AddOutline,
  TrashOutline,
  DocumentsOutline,
  SettingsOutline,
  ClipboardOutline, // 引入剪贴板图标用于批量文本导入
} from '@vicons/ionicons5'
import {
  getStorageList,
  refreshStorage,
  deleteStorage,
  toggleAutoRefresh,
  modifyToken,
  batchDeleteStorage,
  batchRefreshStorage,
  batchModifyToken,
  clearAllStorage,
} from '@/api/storage'
import type { BatchDispatchResponse, StorageInfo } from '@/api/storage'
import { getCloudTokenList } from '@/api/cloudtoken'
import { formatDateTime } from '@/utils/time'
import { getOsTypeDisplayName, getOsTypeColor, mountTypeConfigs } from '@/utils/osType'
import { getTaskStatusInfo } from '@/utils/taskStatus'
import { getListItems, getListTotal } from '@/utils/pagination'
import { normalizeCloudTokens, normalizeStorageInfos } from '@/utils/responseGuards'
import { getErrorMessage } from '@/utils/api'
import { useSubscribeMount } from '@/composables/useSubscribeMount'
import { useShareMount } from '@/composables/useShareMount'
import { usePersonMount } from '@/composables/usePersonMount'
import { useFamilyMount } from '@/composables/useFamilyMount'
import { useBatchCreateFromTextMount } from '@/composables/useBatchCreateFromTextMount' // 引入新组件的hook
import { usePageAutoRefreshStore } from '@/stores/modules/pageAutoRefresh'
import dayjs from 'dayjs'

// 表格数据
const tableData = reactive<StorageInfo[]>([])
const loading = ref(false)
const searchKeyword = ref('')
const selectedTaskLogStatus = ref<string>('')

// 弹窗控制
const showAutoRefreshModal = ref(false)
const showPageSettingsModal = ref(false)
const showModifyTokenModal = ref(false)
const showBatchModifyTokenModal = ref(false)
const batchModifyTokenId = ref<number | null>(null)
let cloudTokenRequestId = 0

const isBusinessSuccess = (response: { code: number }) => response.code === 200
const isNonNegativeSafeInteger = (value: unknown): value is number =>
  typeof value === 'number' && Number.isSafeInteger(value) && value >= 0

const isBatchDispatchResponse = (result: unknown): result is BatchDispatchResponse => {
  if (!result || typeof result !== 'object') {
    return false
  }

  const data = result as Partial<BatchDispatchResponse>

  return (
    isNonNegativeSafeInteger(data.total) &&
    isNonNegativeSafeInteger(data.success) &&
    isNonNegativeSafeInteger(data.failed)
  )
}

const showBatchDispatchResult = (label: string, result: BatchDispatchResponse | undefined) => {
  if (!isBatchDispatchResponse(result)) {
    message.error('响应数据格式异常')

    return false
  }

  const { total, success, failed } = result
  const detail = `${label}已提交：总计 ${total} 个，成功 ${success} 个，失败 ${failed} 个`

  if (failed > 0 && success > 0) {
    message.warning(detail)
  } else if (failed > 0) {
    message.error(detail)
  } else if (total === 0) {
    message.warning(`${label}没有提交：没有可处理的挂载点`)
  } else {
    message.success(`${label}已提交：成功 ${success} 个`)
  }

  return true
}

const maxStorageListPageSize = 500
const maxBatchActionIds = 1000
const maxBatchModifyTokenIds = 500

// 加载令牌列表（单个和批量修改令牌共用）
const loadCloudTokenOptions = (requestId: number) => {
  return getCloudTokenList({ noPaginate: true })
    .then((response) => {
      if (!isPageMounted || requestId !== cloudTokenRequestId) {
        return false
      }

      if (isBusinessSuccess(response) && response.data) {
        const rawItems = getListItems<Models.CloudToken>(response.data)
        const items = normalizeCloudTokens(rawItems)
        if (!items) {
          message.error('获取令牌列表失败：响应数据格式异常')

          return false
        }

        cloudTokenOptions.value = items.map((token) => ({
          label: token.name,
          value: token.id,
        }))
        cloudTokenOptions.value.unshift({
          label: '解绑令牌',
          value: 0,
        })
        return true
      }
      message.error(response.msg || '获取令牌列表失败')

      return false
    })
    .catch((error) => {
      if (!isPageMounted || requestId !== cloudTokenRequestId) {
        return false
      }

      console.error('获取云盘令牌列表失败:', error)
      message.error(getErrorMessage(error, '获取令牌列表失败'))
      return false
    })
}

const subscribeMount = useSubscribeMount()
const shareMount = useShareMount()
const personMount = usePersonMount()
const familyMount = useFamilyMount()
const batchTextMount = useBatchCreateFromTextMount() // 初始化批量文本导入

// 自动刷新配置表单
const autoRefreshFormRef = ref<FormInst>()
const autoRefreshSubmitting = ref(false)
const currentEditStorage = ref<StorageInfo | null>(null)
let autoRefreshModalSession = 0

const autoRefreshForm = ref({
  enableAutoRefresh: false,
  refreshInterval: 60,
  autoRefreshDays: 7,
  refreshBeginAt: null as number | null,
  enableDeepRefresh: false,
})

const autoRefreshRules: FormRules = {
  refreshInterval: [
    {
      type: 'number',
      min: 30,
      max: 1440,
      message: '刷新间隔必须在30-1440分钟之间',
      trigger: 'blur',
    },
  ],
  autoRefreshDays: [
    {
      type: 'number',
      min: 1,
      max: 365,
      message: '持续天数必须在1-365天之间',
      trigger: 'blur',
    },
  ],
  refreshBeginAt: [],
}

// 消息提示和对话框
const message = useMessage()
const dialog = useDialog()

// 分页配置
const paginationReactive = reactive<PaginationProps>({
  page: 1,
  pageSize: 12,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [12, 24, 48, 96],
})

// 分页处理函数
const handlePageChange = (page: number) => {
  paginationReactive.page = page
  fetchStorageList()
}

const handlePageSizeChange = (pageSize: number) => {
  paginationReactive.pageSize = pageSize
  paginationReactive.page = 1
  fetchStorageList()
}

const refreshTime = ref(dayjs())

// 计算下一次运行时间
const getNextRunTime = (storage: StorageInfo) => {
  if (!storage.enableAutoRefresh || !storage.refreshInterval) return null
  if ('nextRefreshTime' in storage && !storage.nextRefreshTime) return null
  if (storage.nextRefreshTime) {
    const serverNextRun = dayjs(storage.nextRefreshTime)
    if (serverNextRun.isValid()) return serverNextRun
  }

  const lastRun = storage.updatedAt ? dayjs(storage.updatedAt) : null
  if (!lastRun) return null

  const interval = storage.refreshInterval
  const now = dayjs()
  let nextRun = lastRun.add(interval, 'minute')

  // 如果计算出的下次运行时间已经过了，计算下一个未来的运行时间
  while (nextRun.isBefore(now)) {
    nextRun = nextRun.add(interval, 'minute')
  }

  return nextRun
}

// 格式化下一次运行时间
const formatNextRunTime = (storage: StorageInfo) => {
  const nextRun = getNextRunTime(storage)
  if (!nextRun) return ''
  return nextRun.format('MM-DD HH:mm')
}

// 使用页面自动刷新store
const pageAutoRefreshStore = usePageAutoRefreshStore()

// 自动刷新状态管理
const nextRefreshTime = ref(dayjs().add(pageAutoRefreshStore.refreshInterval, 'second'))
const intervalTimer = ref<ReturnType<typeof setInterval> | null>(null)
let isPageMounted = false
let storageListRequestId = 0
let storageListRequestInFlight = false
let selectAllPagesRequestId = 0
let storageActionSession = 0

// 页面设置表单
const pageSettingsForm = ref({
  autoRefreshEnabled: pageAutoRefreshStore.autoRefreshEnabled,
  refreshInterval: pageAutoRefreshStore.refreshInterval,
})

// 刷新间隔选项（秒）
const refreshIntervalOptions = [
  { label: '30秒', value: 30 },
  { label: '1分钟', value: 60 },
  { label: '3分钟', value: 180 },
  { label: '5分钟', value: 300 },
  { label: '10分钟', value: 600 },
]

// 启动自动刷新定时器
const startAutoRefresh = () => {
  if (!isPageMounted) {
    return
  }

  if (intervalTimer.value) {
    clearInterval(intervalTimer.value)
  }

  if (pageAutoRefreshStore.autoRefreshEnabled) {
    updateNextRefreshTime()
    intervalTimer.value = setInterval(() => {
      if (!isPageMounted) {
        return
      }

      fetchStorageList(true)
      updateNextRefreshTime()
    }, pageAutoRefreshStore.refreshInterval * 1000)
  }
}

// 停止自动刷新定时器
const stopAutoRefresh = () => {
  if (intervalTimer.value) {
    clearInterval(intervalTimer.value)
    intervalTimer.value = null
  }
}

const areSameIds = (left: number[], right: number[]) => {
  return left.length === right.length && left.every((id, index) => id === right[index])
}

// 更新下次刷新时间
const updateNextRefreshTime = () => {
  nextRefreshTime.value = dayjs().add(pageAutoRefreshStore.refreshInterval, 'second')
}

// 打开页面设置
const handlePageSettings = () => {
  pageSettingsForm.value = {
    autoRefreshEnabled: pageAutoRefreshStore.autoRefreshEnabled,
    refreshInterval: pageAutoRefreshStore.refreshInterval,
  }
  showPageSettingsModal.value = true
}

// 处理页面设置中的自动刷新开关切换
const handlePageAutoRefreshToggle = (enabled: boolean) => {
  pageSettingsForm.value.autoRefreshEnabled = enabled
  pageAutoRefreshStore.updateSettings({ autoRefreshEnabled: enabled })

  if (enabled) {
    startAutoRefresh()
    message.success('自动刷新已开启')
  } else {
    stopAutoRefresh()
    message.info('自动刷新已关闭')
  }
}

// 处理页面设置中的刷新间隔变更
const handlePageRefreshIntervalChange = (interval: number) => {
  pageSettingsForm.value.refreshInterval = interval
  pageAutoRefreshStore.updateSettings({ refreshInterval: interval })

  if (pageAutoRefreshStore.autoRefreshEnabled) {
    startAutoRefresh()
    message.success(`刷新间隔已更改为 ${Math.floor(interval / 60)} 分钟`)
  }
}

// 获取存储列表
const getStorageListFilterParams = () => ({
  path: searchKeyword.value || undefined,
  taskLogStatus: selectedTaskLogStatus.value || undefined,
})

const fetchStorageList = (silent = false) => {
  if (!isPageMounted) {
    return
  }

  if (silent && storageListRequestInFlight) {
    return
  }

  const requestId = ++storageListRequestId
  storageListRequestInFlight = true

  if (!silent) {
    loading.value = true
  }

  const params = {
    currentPage: paginationReactive.page || 1,
    pageSize: paginationReactive.pageSize || 10,
    ...getStorageListFilterParams(),
  }

  getStorageList(params)
    .then((response) => {
      if (!isPageMounted || requestId !== storageListRequestId) {
        return
      }

      if (isBusinessSuccess(response) && response.data) {
        const rawItems = getListItems<StorageInfo>(response.data)
        const items = normalizeStorageInfos(rawItems)
        const total = getListTotal(response.data)

        if (!items) {
          message.error('获取存储列表失败：响应数据格式异常')

          return
        }

        tableData.splice(0, tableData.length, ...items)
        if (total === null) {
          message.warning('存储列表响应缺少有效总数，已保留原分页统计')
        } else {
          paginationReactive.itemCount = total
        }

        return
      }
      message.error(response.msg || '获取存储列表失败')
    })
    .catch((error) => {
      if (!isPageMounted || requestId !== storageListRequestId) {
        return
      }

      console.error('获取存储列表失败:', error)
      message.error(getErrorMessage(error, '获取存储列表失败'))
    })
    .finally(() => {
      if (isPageMounted && requestId === storageListRequestId) {
        loading.value = false
        storageListRequestInFlight = false
        refreshTime.value = dayjs()
      }
    })
}

// 任务日志状态筛选选项
const taskLogStatusOptions = [
  { label: '全部', value: '' },
  { label: '失败', value: 'failed' },
  { label: '成功', value: 'completed' },
  // { label: '进行中', value: 'running' },
  // { label: '等待中', value: 'pending' },
]

// 搜索
const handleSearch = () => {
  paginationReactive.page = 1
  selectedIds.value = []
  selectAllPagesRequestId++
  fetchStorageList()
}

// 重置
const handleReset = () => {
  searchKeyword.value = ''
  selectedTaskLogStatus.value = ''
  paginationReactive.page = 1
  selectedIds.value = []
  selectAllPagesRequestId++
  fetchStorageList()
}

// 挂载类型配置
const mountTypes = mountTypeConfigs

// 批量文本导入
const addMountOptions = computed(() => {
  const options: DropdownOption[] = mountTypes.map((type) => ({
    label: type.label,
    key: type.value,
  }))

  // 添加分割线
  options.push({
    type: 'divider',
    key: 'divider-batch',
  })

  // 添加批量文本导入选项
  options.push({
    label: '批量文本导入',
    key: 'batch_text_import',
    icon: () => h(NIcon, null, { default: () => h(ClipboardOutline) }),
  })

  return options
})

// 选择挂载类型
const handleSelectMountType = (mountType: string) => {
  if (mountType === 'subscribe') {
    subscribeMount.show().then(addNewStorageCallback)
  } else if (mountType === 'share_folder') {
    shareMount.show().then(addNewStorageCallback)
  } else if (mountType === 'person_folder') {
    personMount.show().then(addNewStorageCallback)
  } else if (mountType === 'family_folder') {
    familyMount.show().then(addNewStorageCallback)
  } else if (mountType === 'batch_text_import') {
    // 处理批量文本导入
    batchTextMount.show().then(addNewStorageCallback)
  } else {
    message.info(
      `您选择了：${mountTypes.find((t) => t.value === mountType)?.label}，该功能正在开发中`
    )
  }
}

const addNewStorageCallback = (data: { success: boolean }) => {
  if (data.success) {
    message.success('操作成功')
  }
  // 无论成功与否，如果是成功的话通常需要刷新列表
  // 这里可以根据 data.success 判断，或者简单地都刷新
  fetchStorageList()
}

// 获取刷新选项
const getRefreshOptions = (mountPointId: number): DropdownOption[] => {
  const disabled = isStorageActionBlocked(mountPointId)

  return [
    {
      label: '普通刷新',
      key: `normal-${mountPointId}`,
      disabled,
      props: {
        onClick: () => handleRefresh(mountPointId, false),
      },
    },
    {
      label: '深度刷新',
      key: `deep-${mountPointId}`,
      disabled,
      props: {
        onClick: () => handleRefresh(mountPointId, true),
      },
    },
  ]
}

// 处理刷新
const handleRefresh = (mountPointId: number, deep: boolean) => {
  if (!isPageMounted || isStorageActionBlocked(mountPointId)) {
    return
  }

  const storage = tableData.find((s) => s.mountPointId === mountPointId)
  const refreshType = deep ? '深度刷新' : '普通刷新'
  const actionSession = storageActionSession

  setRefreshingStorage(mountPointId, true)

  message.loading(`正在执行${refreshType}...`)

  refreshStorage({ id: mountPointId, deep })
    .then((res) => {
      if (!isCurrentStorageAction(actionSession) || !isStorageRefreshing(mountPointId)) {
        return
      }

      if (!isBusinessSuccess(res)) {
        message.error(res.msg || '刷新失败')

        return
      }
      message.success(`${storage?.name || '存储'} ${refreshType}成功`)
      fetchStorageList()
    })
    .catch((error) => {
      if (!isCurrentStorageAction(actionSession) || !isStorageRefreshing(mountPointId)) {
        return
      }

      console.error('刷新存储失败:', error)
      message.error(getErrorMessage(error, '刷新失败'))
    })
    .finally(() => {
      if (isCurrentStorageAction(actionSession)) {
        setRefreshingStorage(mountPointId, false)
      }
    })
}

// 处理删除
const handleDelete = (storage: StorageInfo) => {
  const id = storage.mountPointId
  if (!isPageMounted || isStorageActionBlocked(id)) return

  setDeleteDialogOpen(id, true)

  dialog.warning({
    title: '确认删除',
    content: `确定要删除存储挂载点 "${storage.name || '未命名存储'}" 吗？此操作不可撤销。`,
    positiveText: '确认删除',
    negativeText: '取消',
    onAfterLeave: () => {
      if (isPageMounted && !isStorageDeleting(id)) {
        setDeleteDialogOpen(id, false)
      }
    },
    onPositiveClick: () => {
      if (!isPageMounted || isStorageDeleting(id)) return

      setDeletingStorage(id, true)
      const actionSession = storageActionSession
      message.loading(`正在删除 ${storage.name || '存储'}...`)

      return deleteStorage({ id })
        .then((res) => {
          if (!isCurrentStorageAction(actionSession)) {
            return
          }

          if (!isBusinessSuccess(res)) {
            message.error(res.msg || '删除失败')

            return
          }
          message.success(`${storage.name || '存储'} 删除成功`)
          fetchStorageList()
        })
        .catch((error) => {
          if (!isCurrentStorageAction(actionSession)) {
            return
          }

          console.error('删除存储失败:', error)
          message.error(getErrorMessage(error, '删除失败'))
        })
        .finally(() => {
          if (isCurrentStorageAction(actionSession)) {
            setDeletingStorage(id, false)
            setDeleteDialogOpen(id, false)
          }
        })
    },
  })
}

// 计算属性：当前页面展示的挂载点 ID
const currentViewIds = computed(() => tableData.map((item) => item.mountPointId))

// 计算属性：是否已全选当前页
const isAllSelected = computed(() => {
  if (tableData.length === 0) return false
  return currentViewIds.value.every((id) => selectedIds.value.includes(id))
})

// 处理全选/取消全选
const toggleSelectAll = () => {
  if (isSelectAllPagesBlocked.value) {
    return
  }

  if (isAllSelected.value) {
    selectedIds.value = selectedIds.value.filter((id) => !currentViewIds.value.includes(id))
  } else {
    const newIds = currentViewIds.value.filter((id) => !selectedIds.value.includes(id))
    selectedIds.value.push(...newIds)
  }
}

// 全选所有页
const isSelectingAllPages = ref(false)
const isSelectAllPagesBlocked = computed(
  () =>
    batchSubmitting.value ||
    clearingAll.value ||
    storageDangerDialogOpen.value ||
    hasRefreshingStorage.value
)
const isAllSelectedAllPages = computed(() => {
  const total = paginationReactive.itemCount || 0
  return total > 0 && selectedIds.value.length === total
})
const selectAllPages = async () => {
  const total = paginationReactive.itemCount || 0
  if (total <= 0 || isSelectingAllPages.value || isSelectAllPagesBlocked.value) {
    return
  }

  if (selectedIds.value.length === total) {
    selectedIds.value = []
    selectAllPagesRequestId++

    return
  }

  const requestId = ++selectAllPagesRequestId
  const filterParams = getStorageListFilterParams()
  isSelectingAllPages.value = true

  try {
    const pageSize = maxStorageListPageSize
    const pageCount = Math.ceil(total / pageSize)
    const allIds: number[] = []

    for (let page = 1; page <= pageCount; page++) {
      const res = await getStorageList({
        currentPage: page,
        pageSize,
        ...filterParams,
      })

      if (!isPageMounted || requestId !== selectAllPagesRequestId) {
        return
      }

      const rawItems = isBusinessSuccess(res) ? getListItems<StorageInfo>(res.data) : null
      const items = normalizeStorageInfos(rawItems)
      if (!items) {
        message.error(res.msg || '获取全量数据失败')
        return
      }

      allIds.push(...items.map((item: StorageInfo) => item.mountPointId))

      if (items.length < pageSize) {
        break
      }
    }

    selectedIds.value = Array.from(new Set(allIds))
    if (selectedIds.value.length < total) {
      message.warning(`已选中 ${selectedIds.value.length} 个，少于当前筛选总数 ${total} 个`)
    }
  } catch (error) {
    if (!isPageMounted || requestId !== selectAllPagesRequestId) {
      return
    }

    console.error('获取全量数据失败:', error)
    message.error(getErrorMessage(error, '获取全量数据失败'))
  } finally {
    if (isPageMounted && requestId === selectAllPagesRequestId) {
      isSelectingAllPages.value = false
    }
  }
}

const validateBatchSelectionLimit = (limit: number, actionText: string) => {
  if (selectedIds.value.length <= limit) {
    return true
  }

  message.warning(`${actionText}最多支持 ${limit} 个挂载点，请减少选择数量`)

  return false
}

// 批量操作状态
const isBatchMode = ref(false)
// 批量操作统一使用挂载点表主键，不能使用 StorageInfo.id（该字段兼容历史接口，值为 fileId）。
const selectedIds = ref<number[]>([])
const batchSubmitting = ref(false)
const clearingAll = ref(false)
const clearAllDialogOpen = ref(false)
const batchRefreshDialogOpen = ref(false)
const batchDeleteDialogOpen = ref(false)
const deletingStorageIds = ref<Set<number>>(new Set())
const deleteDialogOpenIds = ref<Set<number>>(new Set())
const refreshingStorageIds = ref<Set<number>>(new Set())
const batchModifyTokenIds = ref<number[]>([])
let batchModifyTokenModalSession = 0

const storageDangerDialogOpen = computed(
  () =>
    clearAllDialogOpen.value ||
    batchRefreshDialogOpen.value ||
    batchDeleteDialogOpen.value ||
    deleteDialogOpenIds.value.size > 0
)

const setDeletingStorage = (id: number, deleting: boolean) => {
  const ids = new Set(deletingStorageIds.value)
  if (deleting) {
    ids.add(id)
  } else {
    ids.delete(id)
  }
  deletingStorageIds.value = ids
}

const setDeleteDialogOpen = (id: number, open: boolean) => {
  const ids = new Set(deleteDialogOpenIds.value)
  if (open) {
    ids.add(id)
  } else {
    ids.delete(id)
  }
  deleteDialogOpenIds.value = ids
}

const setRefreshingStorage = (id: number, refreshing: boolean) => {
  const ids = new Set(refreshingStorageIds.value)
  if (refreshing) {
    ids.add(id)
  } else {
    ids.delete(id)
  }
  refreshingStorageIds.value = ids
}

const isStorageDeleting = (id: number) => deletingStorageIds.value.has(id)
const isStorageRefreshing = (id: number) => refreshingStorageIds.value.has(id)
const hasRefreshingStorage = computed(() => refreshingStorageIds.value.size > 0)

const isStorageDeleteBlocked = (id: number) =>
  batchSubmitting.value ||
  clearingAll.value ||
  storageDangerDialogOpen.value ||
  isStorageDeleting(id) ||
  deleteDialogOpenIds.value.has(id)

const isStorageActionBlocked = (id: number) =>
  isBatchMode.value || isStorageDeleteBlocked(id) || isStorageRefreshing(id)

const isCurrentStorageAction = (session: number) =>
  isPageMounted && session === storageActionSession

// 进入批量模式
const enterBatchMode = () => {
  closeModifyTokenModal()
  isBatchMode.value = true
  selectedIds.value = []
}

// 退出批量模式
const exitBatchMode = () => {
  isBatchMode.value = false
  selectedIds.value = []
  resetBatchModifyTokenModal()
  batchModifyTokenIds.value = []
}

// 切换选中状态
const toggleSelection = (id: number) => {
  if (isSelectAllPagesBlocked.value) return

  if (selectedIds.value.includes(id)) {
    selectedIds.value = selectedIds.value.filter((item) => item !== id)
  } else {
    selectedIds.value.push(id)
  }
}

// 处理卡片点击
const handleCardClick = (id: number) => {
  if (isBatchMode.value && !isSelectAllPagesBlocked.value) {
    toggleSelection(id)
  }
}

// 处理批量删除
const handleBatchDelete = () => {
  if (
    batchSubmitting.value ||
    storageDangerDialogOpen.value ||
    hasRefreshingStorage.value ||
    selectedIds.value.length === 0
  )
    return
  if (!validateBatchSelectionLimit(maxBatchActionIds, '批量删除')) return

  const ids = [...selectedIds.value]
  batchDeleteDialogOpen.value = true

  dialog.warning({
    title: '批量删除',
    content: `确定要删除选中的 ${ids.length} 个挂载点吗？此操作不可撤销。`,
    positiveText: '确认删除',
    negativeText: '取消',
    onAfterLeave: () => {
      if (isPageMounted && !batchSubmitting.value) {
        batchDeleteDialogOpen.value = false
      }
    },
    onPositiveClick: () => {
      if (!isPageMounted || batchSubmitting.value) return

      const actionSession = ++storageActionSession
      batchSubmitting.value = true
      message.loading('正在批量删除...')

      return batchDeleteStorage({ ids })
        .then((res) => {
          if (!isCurrentStorageAction(actionSession)) {
            return
          }

          if (!isBusinessSuccess(res)) {
            message.error(res.msg || '批量删除失败')

            return
          }
          if (!showBatchDispatchResult('批量删除', res.data)) {
            return
          }

          exitBatchMode()
          fetchStorageList()
        })
        .catch((error) => {
          if (!isCurrentStorageAction(actionSession)) {
            return
          }

          message.error(getErrorMessage(error, '批量删除失败'))
        })
        .finally(() => {
          if (isCurrentStorageAction(actionSession)) {
            batchSubmitting.value = false
            batchDeleteDialogOpen.value = false
          }
        })
    },
  })
}

// 处理清空所有
const handleClearAll = () => {
  if (
    clearingAll.value ||
    batchSubmitting.value ||
    storageDangerDialogOpen.value ||
    hasRefreshingStorage.value
  )
    return

  clearAllDialogOpen.value = true

  dialog.warning({
    title: '清空所有挂载点',
    content: '确定要清空所有存储挂载点吗？此操作会同时删除挂载点、媒体文件和虚拟文件，不可恢复！',
    positiveText: '确认清空',
    negativeText: '取消',
    onAfterLeave: () => {
      if (isPageMounted && !clearingAll.value) {
        clearAllDialogOpen.value = false
      }
    },
    onPositiveClick: () => {
      if (!isPageMounted || clearingAll.value || batchSubmitting.value) return

      const actionSession = ++storageActionSession
      exitBatchMode()
      clearingAll.value = true
      message.loading('正在清空所有数据...')

      return clearAllStorage({ deleteFiles: true })
        .then((res) => {
          if (!isCurrentStorageAction(actionSession)) {
            return
          }

          if (!isBusinessSuccess(res)) {
            message.error(res.msg || '清空失败')

            return
          }
          if (typeof res.data !== 'number') {
            message.error('清空完成，但响应统计缺失')

            return
          }

          message.success(`已清空 ${res.data} 个挂载点`)
          exitBatchMode()
          fetchStorageList()
        })
        .catch((error) => {
          if (!isCurrentStorageAction(actionSession)) {
            return
          }

          message.error(getErrorMessage(error, '清空失败'))
        })
        .finally(() => {
          if (isCurrentStorageAction(actionSession)) {
            clearingAll.value = false
            clearAllDialogOpen.value = false
          }
        })
    },
  })
}

// 处理批量刷新
const handleBatchRefresh = (deep: boolean) => {
  if (
    batchSubmitting.value ||
    storageDangerDialogOpen.value ||
    hasRefreshingStorage.value ||
    selectedIds.value.length === 0
  )
    return
  if (!validateBatchSelectionLimit(maxBatchActionIds, '批量刷新')) return

  const ids = [...selectedIds.value]
  const refreshType = deep ? '深度刷新' : '普通刷新'
  batchRefreshDialogOpen.value = true

  dialog.warning({
    title: `批量${refreshType}`,
    content: `确定要${refreshType}选中的 ${ids.length} 个挂载点吗？`,
    positiveText: '确认刷新',
    negativeText: '取消',
    onAfterLeave: () => {
      if (isPageMounted && !batchSubmitting.value) {
        batchRefreshDialogOpen.value = false
      }
    },
    onPositiveClick: () => {
      if (!isPageMounted || batchSubmitting.value) return

      const actionSession = ++storageActionSession
      batchSubmitting.value = true
      message.loading(`正在批量${refreshType}...`)

      return batchRefreshStorage({ ids, deep })
        .then((res) => {
          if (!isCurrentStorageAction(actionSession)) {
            return
          }

          if (!isBusinessSuccess(res)) {
            message.error(res.msg || `批量${refreshType}失败`)

            return
          }
          if (!showBatchDispatchResult(`批量${refreshType}任务`, res.data)) {
            return
          }

          exitBatchMode()
        })
        .catch((error) => {
          if (!isCurrentStorageAction(actionSession)) {
            return
          }

          message.error(getErrorMessage(error, `批量${refreshType}失败`))
        })
        .finally(() => {
          if (isCurrentStorageAction(actionSession)) {
            batchSubmitting.value = false
            batchRefreshDialogOpen.value = false
          }
        })
    },
  })
}

// 确认批量修改令牌
const handleBatchModifyTokenConfirm = () => {
  if (
    batchSubmitting.value ||
    batchModifyTokenIds.value.length === 0 ||
    batchModifyTokenId.value === null ||
    batchModifyTokenId.value === undefined
  )
    return

  const ids = [...batchModifyTokenIds.value]
  const session = batchModifyTokenModalSession
  const tokenId = batchModifyTokenId.value
  batchSubmitting.value = true

  batchModifyToken({
    ids,
    tokenId,
  })
    .then((res) => {
      if (
        !isPageMounted ||
        session !== batchModifyTokenModalSession ||
        !showBatchModifyTokenModal.value ||
        !areSameIds(ids, batchModifyTokenIds.value)
      ) {
        return
      }

      if (!isBusinessSuccess(res)) {
        message.error(res.msg || '批量修改令牌失败')

        return
      }

      if (typeof res.data !== 'string' || res.data.trim() === '') {
        message.error('批量修改令牌任务已提交，但响应结果缺失')

        return
      }

      message.success(res.data)
      closeBatchModifyTokenModal(true)
      exitBatchMode()
      fetchStorageList()
    })
    .catch((error) => {
      if (
        !isPageMounted ||
        session !== batchModifyTokenModalSession ||
        !showBatchModifyTokenModal.value ||
        !areSameIds(ids, batchModifyTokenIds.value)
      ) {
        return
      }

      console.error('批量修改令牌失败:', error)
      message.error(getErrorMessage(error, '批量修改令牌失败'))
    })
    .finally(() => {
      if (
        isPageMounted &&
        session === batchModifyTokenModalSession &&
        showBatchModifyTokenModal.value &&
        areSameIds(ids, batchModifyTokenIds.value)
      ) {
        batchSubmitting.value = false
      }
    })
}

const handleAutoRefreshModalShowUpdate = (show: boolean) => {
  if (show) {
    showAutoRefreshModal.value = true

    return
  }

  if (autoRefreshSubmitting.value) {
    return
  }

  closeAutoRefreshModal()
}

const closeAutoRefreshModal = (force = false) => {
  if (autoRefreshSubmitting.value && !force) {
    return
  }

  autoRefreshModalSession++
  showAutoRefreshModal.value = false
  autoRefreshSubmitting.value = false
  currentEditStorage.value = null
}

// 处理编辑自动刷新
const handleEditAutoRefresh = (storage: StorageInfo) => {
  if (!isPageMounted || isStorageActionBlocked(storage.mountPointId)) {
    return
  }

  autoRefreshModalSession++
  currentEditStorage.value = storage
  autoRefreshForm.value = {
    enableAutoRefresh: storage.enableAutoRefresh || false,
    refreshInterval: storage.refreshInterval || 60,
    autoRefreshDays: storage.autoRefreshDays || 7,
    refreshBeginAt: storage.autoRefreshBeginAt
      ? new Date(storage.autoRefreshBeginAt).getTime()
      : Date.now(),
    enableDeepRefresh: storage.enableDeepRefresh || false,
  }
  showAutoRefreshModal.value = true
}

// 处理自动刷新配置确认
const handleAutoRefreshConfirm = () => {
  if (
    !currentEditStorage.value ||
    autoRefreshSubmitting.value ||
    isStorageActionBlocked(currentEditStorage.value.mountPointId)
  )
    return

  const session = autoRefreshModalSession
  const storageId = currentEditStorage.value.mountPointId

  autoRefreshFormRef.value?.validate((errors) => {
    if (
      !isPageMounted ||
      session !== autoRefreshModalSession ||
      !showAutoRefreshModal.value ||
      currentEditStorage.value?.mountPointId !== storageId
    ) {
      return
    }

    if (errors) {
      message.error('请检查表单输入')
      return
    }

    autoRefreshSubmitting.value = true

    const refreshBeginAt = autoRefreshForm.value.refreshBeginAt
      ? dayjs(autoRefreshForm.value.refreshBeginAt).format('YYYY-MM-DD')
      : dayjs().format('YYYY-MM-DD')

    toggleAutoRefresh({
      id: currentEditStorage.value!.mountPointId,
      enableAutoRefresh: autoRefreshForm.value.enableAutoRefresh,
      refreshInterval: autoRefreshForm.value.enableAutoRefresh
        ? autoRefreshForm.value.refreshInterval
        : undefined,
      autoRefreshDays: autoRefreshForm.value.enableAutoRefresh
        ? autoRefreshForm.value.autoRefreshDays
        : undefined,
      refreshBeginAt: autoRefreshForm.value.enableAutoRefresh ? refreshBeginAt : undefined,
      enableDeepRefresh: autoRefreshForm.value.enableAutoRefresh
        ? autoRefreshForm.value.enableDeepRefresh
        : undefined,
    })
      .then((res) => {
        if (
          !isPageMounted ||
          session !== autoRefreshModalSession ||
          !showAutoRefreshModal.value ||
          currentEditStorage.value?.mountPointId !== storageId
        ) {
          return
        }

        if (!isBusinessSuccess(res)) {
          message.error(res.msg || '配置更新失败')

          return
        }
        message.success('自动刷新配置更新成功')
        closeAutoRefreshModal(true)
        fetchStorageList()
      })
      .catch((error) => {
        if (
          !isPageMounted ||
          session !== autoRefreshModalSession ||
          !showAutoRefreshModal.value ||
          currentEditStorage.value?.mountPointId !== storageId
        ) {
          return
        }

        console.error('更新自动刷新配置失败:', error)
        message.error(getErrorMessage(error, '配置更新失败'))
      })
      .finally(() => {
        if (
          isPageMounted &&
          session === autoRefreshModalSession &&
          showAutoRefreshModal.value &&
          currentEditStorage.value?.mountPointId === storageId
        ) {
          autoRefreshSubmitting.value = false
        }
      })
  })
}

// 格式化刷新周期显示
const formatRefreshPeriod = (storage: StorageInfo) => {
  if (!storage.autoRefreshBeginAt || !storage.autoRefreshDays) {
    return '未设置刷新周期'
  }
  const beginDate = new Date(storage.autoRefreshBeginAt)
  const endDate = new Date(beginDate)
  endDate.setDate(beginDate.getDate() + storage.autoRefreshDays)

  const formatDate = (date: Date) => {
    return date.toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    })
  }
  return `${formatDate(beginDate)} ~ ${formatDate(endDate)} (${storage.autoRefreshDays}天)`
}

const computedRefreshStatusText = (storage: StorageInfo): string => {
  if (!storage.autoRefreshBeginAt || !storage.autoRefreshDays) {
    return '未设置刷新周期'
  }

  if (storage.isInAutoRefreshPeriod) {
    return '待执行'
  }
  let refreshBeginAt = dayjs(storage.autoRefreshBeginAt)
  if (refreshBeginAt.isAfter(dayjs())) {
    return '未到开始时间'
  } else {
    return '已失效'
  }
}

// 修改令牌相关变量
const modifyTokenSubmitting = ref(false)
const currentModifyStorage = ref<StorageInfo | null>(null)
const cloudTokenOptions = ref<{ label: string; value: number }[]>([])
const selectedTokenId = ref<number | null>(null)
let modifyTokenModalSession = 0

const handleModifyTokenModalShowUpdate = (show: boolean) => {
  if (show) {
    showModifyTokenModal.value = true

    return
  }

  if (modifyTokenSubmitting.value) {
    return
  }

  closeModifyTokenModal()
}

const closeModifyTokenModal = (force = false) => {
  if (modifyTokenSubmitting.value && !force) {
    return
  }

  modifyTokenModalSession++
  showModifyTokenModal.value = false
  modifyTokenSubmitting.value = false
  currentModifyStorage.value = null
  selectedTokenId.value = null
}

const handleBatchModifyTokenModalShowUpdate = (show: boolean) => {
  if (show) {
    showBatchModifyTokenModal.value = true

    return
  }

  if (batchSubmitting.value) {
    return
  }

  closeBatchModifyTokenModal()
}

const closeBatchModifyTokenModal = (force = false) => {
  if (batchSubmitting.value && !force) {
    return
  }

  resetBatchModifyTokenModal()
  batchSubmitting.value = false
}

const resetBatchModifyTokenModal = () => {
  batchModifyTokenModalSession++
  showBatchModifyTokenModal.value = false
  batchModifyTokenId.value = null
  batchModifyTokenIds.value = []
}

// 处理批量修改令牌（先加载令牌列表再打开弹窗）
const handleBatchModifyToken = () => {
  if (batchSubmitting.value || selectedIds.value.length === 0) return
  if (!validateBatchSelectionLimit(maxBatchModifyTokenIds, '批量修改令牌')) return

  const requestId = ++cloudTokenRequestId
  const session = ++batchModifyTokenModalSession
  const ids = [...selectedIds.value]

  batchModifyTokenId.value = null
  batchModifyTokenIds.value = ids
  loadCloudTokenOptions(requestId).then((success) => {
    if (
      success &&
      isPageMounted &&
      requestId === cloudTokenRequestId &&
      session === batchModifyTokenModalSession &&
      isBatchMode.value &&
      areSameIds(ids, selectedIds.value) &&
      areSameIds(ids, batchModifyTokenIds.value)
    ) {
      showBatchModifyTokenModal.value = true
    }
  })
}

// 处理修改令牌
const handleModifyToken = (storage: StorageInfo) => {
  if (!isPageMounted || isStorageActionBlocked(storage.mountPointId)) {
    return
  }

  const requestId = ++cloudTokenRequestId
  const session = ++modifyTokenModalSession

  currentModifyStorage.value = storage
  selectedTokenId.value = storage.tokenId ?? 0

  loadCloudTokenOptions(requestId).then((success) => {
    if (
      success &&
      isPageMounted &&
      requestId === cloudTokenRequestId &&
      session === modifyTokenModalSession &&
      currentModifyStorage.value?.mountPointId === storage.mountPointId
    ) {
      showModifyTokenModal.value = true
    }
  })
}

// 确认修改令牌
const handleModifyTokenConfirm = () => {
  if (
    !currentModifyStorage.value ||
    modifyTokenSubmitting.value ||
    selectedTokenId.value === null ||
    selectedTokenId.value === undefined ||
    isStorageActionBlocked(currentModifyStorage.value.mountPointId)
  )
    return

  const session = modifyTokenModalSession
  const storageId = currentModifyStorage.value.mountPointId
  modifyTokenSubmitting.value = true
  const tokenId = selectedTokenId.value

  modifyToken({
    id: storageId,
    tokenId,
  })
    .then((res) => {
      if (
        !isPageMounted ||
        session !== modifyTokenModalSession ||
        !showModifyTokenModal.value ||
        currentModifyStorage.value?.mountPointId !== storageId
      ) {
        return
      }

      if (!isBusinessSuccess(res)) {
        message.error(res.msg || '令牌修改失败')

        return
      }
      const actionText = tokenId === 0 ? '解绑' : '修改绑定'
      message.success(`令牌${actionText}成功`)
      closeModifyTokenModal(true)
      fetchStorageList()
    })
    .catch((error) => {
      if (
        !isPageMounted ||
        session !== modifyTokenModalSession ||
        !showModifyTokenModal.value ||
        currentModifyStorage.value?.mountPointId !== storageId
      ) {
        return
      }

      console.error('修改令牌失败:', error)
      message.error(getErrorMessage(error, '令牌修改失败'))
    })
    .finally(() => {
      if (
        isPageMounted &&
        session === modifyTokenModalSession &&
        showModifyTokenModal.value &&
        currentModifyStorage.value?.mountPointId === storageId
      ) {
        modifyTokenSubmitting.value = false
      }
    })
}

// 格式化任务日志时间
const formatTaskLogTime = (taskLog: Models.FileTaskLog) => {
  if (!taskLog.beginAt) return '未知时间'
  const beginTime = dayjs(taskLog.beginAt)
  const startTime = beginTime.format('MM-DD HH:mm')

  if ((taskLog.status === 'completed' || taskLog.status === 'failed') && taskLog.duration) {
    const duration = taskLog.duration
    let durationText = ''
    if (duration < 1000) {
      durationText = `${duration}ms`
    } else if (duration < 60000) {
      durationText = `${Math.round(duration / 1000)}s`
    } else {
      const minutes = Math.floor(duration / 60000)
      const seconds = Math.round((duration % 60000) / 1000)
      durationText = `${minutes}m${seconds}s`
    }
    return `${startTime} (用时 ${durationText})`
  }
  return startTime
}

// 初始化
onMounted(() => {
  isPageMounted = true
  fetchStorageList()
  if (pageAutoRefreshStore.autoRefreshEnabled) {
    startAutoRefresh()
  }
})

onUnmounted(() => {
  isPageMounted = false
  storageListRequestId++
  storageListRequestInFlight = false
  selectAllPagesRequestId++
  storageActionSession++
  cloudTokenRequestId++
  autoRefreshModalSession++
  modifyTokenModalSession++
  batchModifyTokenModalSession++
  batchSubmitting.value = false
  clearingAll.value = false
  clearAllDialogOpen.value = false
  batchRefreshDialogOpen.value = false
  batchDeleteDialogOpen.value = false
  deletingStorageIds.value = new Set()
  deleteDialogOpenIds.value = new Set()
  refreshingStorageIds.value = new Set()
  stopAutoRefresh()
})
</script>

<style scoped>
/* 样式部分保持不变 */

/* 页面整体样式 */
.storages-page {
  padding: 24px;
  background: var(--n-color-target);
  flex: 1;
}

/* 头部搜索区域 */
.header {
  margin-bottom: 24px;
  padding: 20px;
  background: var(--n-card-color);
  border-radius: 12px;
  box-shadow: 0 2px 8px rgb(0 0 0 / 6%);
  border: 1px solid var(--n-border-color);
}

.header-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-row + .header-row {
  margin-top: 8px;
  padding-top: 8px;
}

.header-row.batch-actions {
  justify-content: flex-start;
}

.header-search {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.header-actions-left {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-shrink: 0;
}

.header-actions.batch-mode {
  flex-wrap: wrap;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed var(--n-border-color);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

/* 页面设置样式 */
.page-settings-config {
  padding: 8px 0;
}

.settings-section {
  padding: 16px 0;
}

.setting-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}

.setting-item:last-child {
  margin-bottom: 0;
}

.setting-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--n-text-color);
  min-width: 80px;
}

.header-right {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.header-right.batch-mode {
  flex-wrap: wrap;
}

.header-right .batch-btn {
  margin-bottom: 4px;
}

.header-right .n-button:hover {
  background-color: var(--n-color-hover);
  border-radius: 6px;
}

/* 加载状态 */
.loading-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 400px;
  background: var(--n-card-color);
  border-radius: 12px;
  border: 1px solid var(--n-border-color);
  box-shadow: 0 2px 8px rgb(0 0 0 / 6%);
}

/* 存储卡片网格布局 */
.storage-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
  margin-bottom: 24px;
}

/* 单个存储卡片样式 */
.storage-card {
  background: var(--n-card-color);
  border-radius: 12px;
  border: 1px solid var(--n-border-color);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  box-shadow: 0 2px 8px rgb(0 0 0 / 4%);
  position: relative;
}

.storage-card.is-selected {
  border: 1px solid var(--n-primary-color);
  background-color: var(--n-color-hover);
}

.selection-overlay {
  position: absolute;
  inset: 0;
  z-index: 10;
  cursor: pointer;

  /* 半透明背景，让用户知道处于选择模式 */
  background-color: rgb(0 0 0 / 2%);
}

.selection-checkbox {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 11;
}

.storage-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 12px 32px rgb(0 0 0 / 15%);
  border-color: var(--n-primary-color);
}

/* 卡片头部 */
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 0;
  margin-bottom: 0;
}

.storage-info {
  flex: 1;
  min-width: 0;

  /* 确保flex子项可以收缩 */
}

.storage-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.storage-name {
  font-size: 18px;
  font-weight: 600;
  color: var(--n-text-color);
  line-height: 1.4;

  /* 单行显示，超出省略号 */
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;

  /* 确保为操作按钮留出空间 */
  max-width: calc(100% - 80px);
}

.storage-path {
  font-size: 13px;
  color: var(--n-text-color-2);
  line-height: 1.4;

  /* 最多两行显示，超出省略号 */
  display: -webkit-box;
  -webkit-box-orient: vertical;
  overflow: hidden;
  text-overflow: ellipsis;
  word-break: break-all;
}

.storage-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
  align-items: flex-start;
}

.storage-actions .n-button {
  background-color: var(--n-color-target);
  border: 1px solid var(--n-border-color);
  transition: all 0.3s ease;
  width: 32px;
  height: 32px;
}

.storage-actions .n-button:hover {
  border-color: var(--n-primary-color);
  transform: scale(1.1);
}

.storage-actions .n-button:hover .n-icon {
  color: var(--n-primary-color);
}

/* 卡片内容 */
.card-content {
  padding: 0;
}

/* 基础信息容器 - 使用flex布局实现三个字段的对齐 */
.basic-info-container {
  display: flex;
  gap: 10px;
  margin-bottom: 10px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.basic-info-container .info-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  min-height: 60px;
  gap: 8px;
  text-align: center;
}

.additional-info {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.info-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 500;
  color: var(--n-text-color-2);
}

.info-icon {
  flex-shrink: 0;
}

.info-value {
  font-size: 14px;
  color: var(--n-text-color);
  font-weight: 500;
}

/* 时间行样式 - 一行显示，两边对齐 */
.time-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.time-row .info-label {
  flex-shrink: 0;
}

.time-row .info-value {
  text-align: right;
  flex-shrink: 0;
}

.info-tag {
  font-weight: 500;
}

/* 刷新信息样式 */
.refresh-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.refresh-detail {
  font-size: 12px;
  color: var(--n-text-color-2);
  background: var(--n-card-color);
  padding: 4px 8px;
  border-radius: 4px;
  font-weight: 500;
  border: 1px solid var(--n-border-color);
}

/* 自动刷新头部样式 */
.refresh-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.refresh-title-section {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
}

.refresh-title-section .info-label {
  flex-shrink: 0;
}

/* 刷新详情样式 - 复用 time-row 的 flex 布局 */
.refresh-details {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

/* 刷新状态样式 - 左侧内容 */
.refresh-status {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  flex-shrink: 0;
}

/* 刷新周期样式 - 右侧内容，使用灰色调避免与编辑按钮冲突 */
.refresh-period {
  font-size: 12px;
  color: var(--n-text-color);
  padding: 4px 8px;
  background: var(--n-color-hover);
  border-radius: 4px;
  border: 1px solid var(--n-border-color);
  font-weight: 500;
  flex-shrink: 0;
  text-align: right;
}

/* 任务日志样式 */
.task-log-header {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 6px;
}

.task-log-first-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.task-log-left {
  display: flex;
  flex-direction: row;
  align-items: center;
  flex: 1;
  gap: 5px;
}

.task-log-left .info-label {
  flex-shrink: 0;
}

.task-log-time {
  font-size: 12px;
  color: var(--n-text-color-3);
}

.next-run-time {
  font-size: 12px;
  color: var(--n-text-color-3);
  margin-top: 4px;
}

.task-log-content {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 0 12px;
  border-radius: 6px;
  margin-top: 2px;
}

.task-log-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--n-text-color);
  margin-bottom: 2px;
}

.task-log-desc {
  font-size: 13px;
  color: var(--n-text-color-2);
  line-height: 1.4;
  word-break: break-all;

  /* 确保省略号正确显示 */
  display: -webkit-box;
  -webkit-box-orient: vertical;
  overflow: hidden;
  text-overflow: ellipsis;
}

.task-log-error {
  margin-top: 6px;
  padding: 8px 10px;
  background: rgb(245 108 108 / 8%);
  border: 1px solid rgb(245 108 108 / 20%);
  border-radius: 4px;
  border-left: 3px solid #f56c6c;
}

.task-log-error .n-text {
  font-size: 12px;
  line-height: 1.4;
  font-weight: 500;
}

/* 卡片底部样式 */
.card-footer {
  padding: 0 0 16px;
}

/* 空状态 */
.empty-state {
  grid-column: 1 / -1;
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 400px;
  background: var(--n-card-color);
  border-radius: 12px;
  border: 2px dashed var(--n-border-color);
}

/* 分页容器 */
.pagination-container {
  display: flex;
  justify-content: center;
  padding: 20px;
  background: var(--n-card-color);
  border-radius: 12px;
  box-shadow: 0 2px 8px rgb(0 0 0 / 6%);
  border: 1px solid var(--n-border-color);
}

/* 响应式设计 */
@media (width <=1400px) {
  .storage-cards {
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 18px;
  }
}

@media (width <=1200px) {
  .storage-cards {
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 16px;
  }
}

@media (width <=768px) {
  .storages-page {
    padding: 16px;
  }

  .storage-cards {
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 14px;
  }

  .header {
    padding: 16px;
    flex-direction: column;
    gap: 16px;
    align-items: stretch;
  }

  .header-left {
    justify-content: center;
    flex-direction: column;
    align-items: stretch;
    gap: 16px;
  }

  .header-right {
    align-self: center;
    display: flex;
    justify-content: flex-end;
    width: 100%;
  }

  .storage-name {
    max-width: calc(100% - 70px);
  }
}

@media (width <=480px) {
  .storages-page {
    padding: 12px;
  }

  .storage-cards {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .header {
    padding: 12px;
  }

  .pagination-container {
    padding: 12px;
  }

  .storage-name {
    max-width: calc(100% - 60px);
  }

  .storage-actions {
    gap: 4px;
  }

  .storage-actions .n-button {
    width: 28px;
    height: 28px;
  }
}
</style>
