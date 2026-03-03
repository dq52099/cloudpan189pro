declare namespace Models {
  // 用户模型
  interface User {
    id: number
    username: string
    status: number
    isAdmin: boolean
    groupId: number
    version: number
    createdAt: string
    updatedAt: string
  }

  // 用户信息（包含用户组名称）
  interface UserInfo extends User {
    groupName?: string
  }

  // 云盘令牌模型
  interface CloudToken {
    id: number
    name: string
    username: string
    accessToken: string
    expiresIn: number
    loginType: number // 1: 扫码登录 2: 密码登录
    status: number // 状态 1:正常 2: 登录失败
    addition: Record<string, unknown> // 附属参数
    createdAt: string
    updatedAt: string
  }

  // 用户组模型
  interface UserGroup {
    id: number
    name: string
    createdAt: string
    updatedAt: string
    userCount?: number // 该用户组下的用户数量
  }

  // 虚拟文件模型
  interface VirtualFile {
    id: number
    cloudId: string
    parentId: number
    topId: number
    isTop: boolean
    isDir: boolean
    name: string
    size: number
    hash: string
    osType: string
    addition: Record<string, unknown>
    rev: string
    createDate: string
    modifyDate: string
    createdAt: string
    updatedAt: string
  }

  // 文件任务日志模型（对应后端 FileTaskLog）
  interface FileTaskLog {
    id: number
    title: string
    type: string
    desc: string
    beginAt: string
    endAt: string | null
    status: string
    result: string
    errorMsg: string
    addition: Record<string, unknown>
    duration: number
    fileId: number
    userId: number
    completed: number
    total: number
    createdAt: string
    updatedAt: string
  }

  // 挂载点模型（对应后端 MountPoint）
  interface MountPoint {
    id: number
    fileId: number
    osType: string
    tokenId: number
    name: string
    fullPath: string
    enableAutoRefresh: boolean
    refreshInterval: number
    enableDeepRefresh: boolean
    autoRefreshBeginAt: string
    autoRefreshDays: number
    lastState: string
    createdAt: string
    updatedAt: string
  }

  // 分页响应基础结构
  interface PaginationResponse<T> {
    currentPage: number
    pageSize: number
    total: number
    data: T[]
  }

  interface SystemInfo {
    baseURL: string
    enableAuth: boolean
    initialized: boolean
    runTime: number // 运行时间 单位 s
    runTimeHuman: string // 运行时间 格式 例如：1年2月3天4小时5分6秒
    title: string
  }

  // 系统附加设置（对应后端 models.SettingAddition）
  interface SettingAddition {
    localProxy: boolean
    multipleStream: boolean
    multipleStreamThreadCount: number
    multipleStreamChunkSize: number
    taskThreadCount: number
  }

  // 任务引擎统计信息（对应后端 TaskStats）
  interface TaskStats {
    totalTasks: number
    pendingTasks: number
    runningTasks: number
    completedTasks: number
    failedTasks: number
  }

  // 处理器结果（对应后端 ProcessorResult）
  interface ProcessorResult {
    processorId: string
    status: string
    error: string
    startTime: string
    endTime: string
    duration: number // time.Duration
  }

  // 任务信息（对应后端 TaskInfo）
  interface TaskInfo {
    id: string // 任务唯一ID
    topic: string // 消息主题
    payload: number[] // 载荷数据
    status: string // 状态
    workerId: string // 处理的Worker ID
    receiveAt: string // 接收时间
    startAt: string // 开始时间
    endAt: string // 结束时间
    results: ProcessorResult[] // 处理器结果
  }

  // ===== 自动挂载（Auto Ingest）模型 =====

  // 刷新策略（对应后端 github_com_xxcheng123_cloudpan189-share_internal_repository_models.RefreshStrategy）
  interface RefreshStrategy {
    enableAutoRefresh: boolean
    enableDeepRefresh: boolean
    autoRefreshDays: number
    refreshInterval: number
  }

  // 自动挂载计划（对应后端 github_com_xxcheng123_cloudpan189-share_internal_repository_models.AutoIngestPlan）
  interface AutoIngestPlan {
    id: number
    name: string
    tokenId: number
    parentPath: string // 父目录路径
    sourceType: 'subscribe' // 来源类型，目前仅 subscribe
    autoIngestInterval: number // 单位分钟
    onConflict: 'rename' | 'abandon' // 冲突处理策略
    refreshStrategy: RefreshStrategy
    addition: Record<string, unknown>
    offset: number // 偏移量
    enabled: boolean
    addCount: number // 新增挂载数
    failedCount: number // 失败挂载数
    createdAt: string
    updatedAt: string
  }

  // 自动挂载日志（对应后端 github_com_xxcheng123_cloudpan189-share_internal_repository_models.AutoIngestLog）
  interface AutoIngestLog {
    id: number
    planId: number
    content: string
    level: 'info' | 'warn' | 'error'
    createdAt: string
    updatedAt: string
  }

  // 登录日志（对应后端 models.LoginLog）
  interface LoginLog {
    id: number
    userId: number
    username: string
    addr: string
    location: string
    userAgent: string
    traceId: string
    reason: string
    method: Enums.LoginMethod
    event: Enums.LoginEvent
    status: Enums.LoginStatus
    createdAt: string
    updatedAt: string
  }

  // 媒体配置（对应后端 models.MediaConfig）
  interface MediaConfig {
    id: number
    enable: boolean
    storagePath: string // 落盘根路径
    autoClean: boolean // 自动清理空文件夹
    conflictPolicy: Enums.MediaFileConflictPolicy // 冲突策略：skip/replace
    baseURL: string
    includedSuffixes: string[] // 包括的后缀格式 不包括的将过滤 如果为空则表示不过滤
    autoRebuildEnable?: boolean // 定时重建strm开关
    autoRebuildInterval?: number // 定时重建间隔（小时）
    createdAt: string
    updatedAt: string
  }
}
