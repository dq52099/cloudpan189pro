import { api, type ApiResponse } from '@/utils/api'

export interface HotMovieItem {
  id: number
  title: string
  originalTitle: string
  year: string
  rating: number
  cover: string
  posterPath: string
  type: string
  description: string
}

export interface HotMoviesResponse {
  movies: HotMovieItem[]
  source: string
}

export interface SubscriptionConfig {
  enableTMDB: boolean
  enableDouban: boolean
  panSearchURL: string
  defaultMountPath: string
  autoMount: boolean
  cronExpression: string
  tmdbAPIKey: string
}

export interface SearchResult {
  shareUrl: string
  shareCode: string
  name: string
  size: string
  uploadTime: string
  source: string
}

export const getTMDbMovies = (): Promise<ApiResponse<HotMoviesResponse>> => {
  return api.get('/subscription/tmdb/movies').then((res) => res.data)
}

export const getTMDbTVs = (): Promise<ApiResponse<HotMoviesResponse>> => {
  return api.get('/subscription/tmdb/tvs').then((res) => res.data)
}

export const getDoubanMovies = (): Promise<ApiResponse<HotMoviesResponse>> => {
  return api.get('/subscription/douban/movies').then((res) => res.data)
}

export const getSubscriptionConfig = (): Promise<ApiResponse<SubscriptionConfig>> => {
  return api.get('/subscription/config').then((res) => res.data)
}

export const updateSubscriptionConfig = (
  data: SubscriptionConfig
): Promise<ApiResponse<SubscriptionConfig>> => {
  return api.post('/subscription/config', data).then((res) => res.data)
}

export const searchPan = (keyword: string): Promise<ApiResponse<SearchResult[]>> => {
  return api.get('/subscription/search', { params: { keyword } }).then((res) => res.data)
}

export const searchPanWithAI = (
  keyword: string
): Promise<
  ApiResponse<{
    message: string
    keyword: string
    aiDescription: string
    result: SearchResult | null
    allResults: SearchResult[]
  }>
> => {
  return api.get('/subscription/search/ai', { params: { keyword } }).then((res) => res.data)
}

export const mountSubscription = (data: {
  title: string
  shareUrl: string
  shareCode?: string
  cover?: string
}): Promise<ApiResponse<{ message: string; mountPath: string }>> => {
  return api.post('/subscription/mount', data).then((res) => res.data)
}
