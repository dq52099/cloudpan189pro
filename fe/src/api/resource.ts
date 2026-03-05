import { api, type ApiResponse } from '@/utils/api'

export interface ResourceSummary {
  users: {
    total: number
    active: number
    disabled: number
  }
  userGroups: number
  mountPoints: {
    total: number
    enabled: number
    autoRefresh: number
  }
  cloudTokens: {
    total: number
    active: number
  }
  media: {
    enabled: boolean
    strmFiles: number
    mediaFiles: number
  }
  autoIngest: {
    plans: number
    logs24h: number
  }
  tasks: {
    pending: number
    running: number
    failed: number
    completed: number
  }
}

export const getResourceSummary = (): Promise<ApiResponse<ResourceSummary>> => {
  return api.get('/resource/summary').then((res) => res.data)
}
