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
  category?: string
}

export interface HotMoviesResponse {
  movies: HotMovieItem[]
  source: string
  category: string
}

export interface CategoryOption {
  value: string
  label: string
  children?: CategoryOption[]
}

export interface CategoriesResponse {
  tmdb: CategoryOption[]
  douban: CategoryOption[]
}

export interface SubscriptionConfig {
  enableTMDB: boolean
  enableDouban: boolean
  panSearchURL: string
  defaultMountPath: string
  autoMount: boolean
  cronExpression: string
  tmdbAPIKey: string
  openaiAPIKey: string
  openaiBaseURL: string
  openaiModel: string
}

export interface SearchResult {
  shareUrl: string
  shareCode: string
  name: string
  size?: string
  uploadTime: string
  source: string
  cover?: string
  note?: string
}

export interface MountSubscriptionResponse {
  message: string
  mountPath: string
  shareURL: string
  shareCode: string
  fileId: number
  name: string
}

export const getCategories = (): Promise<ApiResponse<CategoriesResponse>> => {
  return api.get('/subscription/categories').then((res) => res.data)
}

export const getTMDbMovies = (category?: string): Promise<ApiResponse<HotMoviesResponse>> => {
  return api.get('/subscription/tmdb/movies', { params: { category } }).then((res) => res.data)
}

export const getTMDbTVs = (category?: string): Promise<ApiResponse<HotMoviesResponse>> => {
  return api.get('/subscription/tmdb/tvs', { params: { category } }).then((res) => res.data)
}

export const getDoubanMovies = (category?: string): Promise<ApiResponse<HotMoviesResponse>> => {
  return api.get('/subscription/douban/movies', { params: { category } }).then((res) => res.data)
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
  mountPath?: string
}): Promise<ApiResponse<MountSubscriptionResponse>> => {
  return api.post('/subscription/mount', data).then((res) => res.data)
}
