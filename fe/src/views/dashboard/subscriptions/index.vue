<template>
  <div class="subscriptions-page">
    <n-tabs type="line" v-model:value="activeTab" animated>
      <n-tab-pane name="hot" tab="热门数据">
        <div class="hot-section">
          <div class="source-tabs">
            <n-radio-group v-model:value="selectedSource" name="source">
              <n-radio-button value="tmdb">TMDB</n-radio-button>
              <n-radio-button value="douban">豆瓣</n-radio-button>
            </n-radio-group>
          </div>
          <div class="category-tabs" v-if="selectedSource === 'tmdb'">
            <n-cascader
              v-model:value="selectedCategory"
              :options="tmdbCategories"
              @update:value="handleCategoryChange"
              placeholder="选择分类"
              style="width: 300px"
            />
          </div>
          <div class="category-tabs" v-else>
            <n-cascader
              v-model:value="selectedCategory"
              :options="doubanCategories"
              @update:value="handleCategoryChange"
              placeholder="选择分类"
              style="width: 300px"
            />
          </div>
          <n-spin :show="loading">
            <div class="movies-grid">
              <div
                v-for="movie in movies"
                :key="movie.id || movie.title"
                class="movie-card"
                @click="showDetail(movie)"
              >
                <div class="movie-cover">
                  <img :src="movie.cover" :alt="movie.title" />
                  <div class="movie-rating" v-if="movie.rating">
                    <span>{{ movie.rating.toFixed(1) }}</span>
                  </div>
                </div>
                <div class="movie-info">
                  <div class="movie-title" :title="movie.title">{{ movie.title }}</div>
                  <div class="movie-year">{{ movie.year }}</div>
                </div>
              </div>
              <n-empty v-if="!loading && movies.length === 0" description="暂无数据" />
            </div>
          </n-spin>
        </div>
      </n-tab-pane>

      <n-tab-pane name="search" tab="盘搜资源">
        <div class="search-section">
          <div class="search-bar">
            <n-input
              v-model:value="searchKeyword"
              placeholder="输入关键词搜索天翼云盘资源"
              clearable
              @keyup.enter="handleSearch"
              style="width: 400px"
            >
              <template #prefix>
                <n-icon><SearchOutline /></n-icon>
              </template>
            </n-input>
            <n-button type="primary" @click="handleSearch" :loading="searching"> 搜索 </n-button>
            <n-button
              type="success"
              @click="handleAISearch"
              :loading="searching"
              style="margin-left: 8px"
            >
              AI 智能推荐
            </n-button>
          </div>
          <n-alert v-if="!tmdbApiKey" type="warning" style="margin-bottom: 16px">
            TMDB API Key 未配置，无法获取部分热门数据。请在环境变量中设置 TMDB_API_KEY
          </n-alert>
          <div class="search-results" v-if="searchResults.length > 0">
            <n-list hoverable clickable>
              <n-list-item
                v-for="(item, index) in searchResults"
                :key="index"
                @click="selectSearchResult(item)"
              >
                <n-thing>
                  <template #header>{{ item.name }}</template>
                  <template #description>
                    <div class="search-result-meta">
                      <n-tag v-if="item.size" size="small" type="info">{{ item.size }}</n-tag>
                      <span class="upload-time">{{ item.uploadTime }}</span>
                    </div>
                  </template>
                </n-thing>
              </n-list-item>
            </n-list>
          </div>
          <n-empty
            v-else-if="searched && searchResults.length === 0"
            description="未找到相关资源"
          />
        </div>
      </n-tab-pane>

      <n-tab-pane name="settings" tab="定时任务">
        <div class="settings-section">
          <n-form :model="configForm" label-placement="left" label-width="140px">
            <n-form-item label="启用 TMDB 数据">
              <n-switch v-model:value="configForm.enableTMDB" />
            </n-form-item>
            <n-form-item label="TMDB API Key">
              <n-input v-model:value="configForm.tmdbAPIKey" placeholder="请输入 TMDB API Key" />
              <template #feedback>
                <span style="font-size: 12px; color: #999"
                  >从 https://www.themoviedb.org/settings/api 获取</span
                >
              </template>
            </n-form-item>
            <n-form-item label="启用豆瓣数据">
              <n-switch v-model:value="configForm.enableDouban" />
            </n-form-item>
            <n-form-item label="盘搜 API 地址">
              <n-input
                v-model:value="configForm.panSearchURL"
                placeholder="https://tg.252035.xyz"
              />
            </n-form-item>
            <n-form-item label="默认挂载路径">
              <n-input v-model:value="configForm.defaultMountPath" placeholder="/热门订阅" />
            </n-form-item>
            <n-form-item label="自动挂载">
              <n-switch v-model:value="configForm.autoMount" />
              <template #feedback>
                <span style="font-size: 12px; color: #999"
                  >开启后定时任务会自动搜索并挂载新资源</span
                >
              </template>
            </n-form-item>
            <n-form-item label="定时表达式">
              <n-input v-model:value="configForm.cronExpression" placeholder="0 2 * * *" />
              <template #feedback>
                <span style="font-size: 12px; color: #999">Cron 表达式，默认每天凌晨 2 点执行</span>
              </template>
            </n-form-item>
            <n-divider>AI 配置</n-divider>
            <n-form-item label="OpenAI API Key">
              <n-input
                v-model:value="configForm.openaiAPIKey"
                type="password"
                placeholder="sk-..."
                show-password-on="click"
              />
              <template #feedback>
                <span style="font-size: 12px; color: #999"
                  >用于 AI 智能推荐和资源质量分析，不配置则使用规则匹配</span
                >
              </template>
            </n-form-item>
            <n-form-item label="API 地址">
              <n-input
                v-model:value="configForm.openaiBaseURL"
                placeholder="https://api.openai.com"
              />
              <template #feedback>
                <span style="font-size: 12px; color: #999"
                  >可使用代理或兼容 OpenAI 的 API 地址</span
                >
              </template>
            </n-form-item>
            <n-form-item label="模型">
              <n-input v-model:value="configForm.openaiModel" placeholder="gpt-4o-mini" />
            </n-form-item>
            <n-form-item>
              <n-button type="primary" @click="handleSaveConfig" :loading="savingConfig">
                保存配置
              </n-button>
            </n-form-item>
          </n-form>
        </div>
      </n-tab-pane>
    </n-tabs>

    <!-- 资源详情弹窗 -->
    <n-modal v-model:show="showDetailModal" preset="card" title="资源详情" style="width: 600px">
      <n-descriptions v-if="selectedMovie" :column="2" label-placement="left">
        <n-descriptions-item label="标题">{{ selectedMovie.title }}</n-descriptions-item>
        <n-descriptions-item label="原名">{{ selectedMovie.originalTitle }}</n-descriptions-item>
        <n-descriptions-item label="年份">{{ selectedMovie.year }}</n-descriptions-item>
        <n-descriptions-item label="评分">{{
          selectedMovie.rating?.toFixed(1)
        }}</n-descriptions-item>
        <n-descriptions-item label="类型">{{
          selectedMovie.type === 'movie' ? '电影' : '电视剧'
        }}</n-descriptions-item>
      </n-descriptions>
      <n-divider />
      <n-text depth="3">{{ selectedMovie?.description }}</n-text>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showDetailModal = false">关闭</n-button>
          <n-button type="primary" @click="handleSearchResource">搜索资源</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 挂载确认弹窗 -->
    <n-modal
      :show="showMountModal"
      preset="card"
      title="挂载资源"
      style="width: 500px"
      :closable="!mounting"
      :mask-closable="!mounting"
      :close-on-esc="!mounting"
      @update:show="handleMountModalShowUpdate"
    >
      <n-form :model="mountForm" label-placement="left">
        <n-form-item label="资源名称">
          <n-input v-model:value="mountForm.title" :disabled="mounting" />
        </n-form-item>
        <n-form-item label="分享链接">
          <n-input v-model:value="mountForm.shareUrl" :disabled="mounting" />
        </n-form-item>
        <n-form-item label="分享码">
          <n-input v-model:value="mountForm.shareCode" :disabled="mounting" />
        </n-form-item>
        <n-form-item label="挂载路径">
          <n-input v-model:value="mountForm.mountPath" :disabled="mounting" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button :disabled="mounting" @click="closeMountModal">取消</n-button>
          <n-button type="primary" @click="handleMount" :loading="mounting">确认挂载</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useMessage } from 'naive-ui'
import { SearchOutline } from '@vicons/ionicons5'
import {
  getCategories,
  getTMDbMovies,
  getTMDbTVs,
  getDoubanMovies,
  getSubscriptionConfig,
  updateSubscriptionConfig,
  searchPan,
  searchPanWithAI,
  mountSubscription,
  type HotMovieItem,
  type MountSubscriptionResponse,
  type SubscriptionConfig,
  type SearchResult,
  type CategoryOption,
} from '@/api/subscription'
import {
  normalizeCategoriesResponse,
  normalizeHotMoviesResponse,
  normalizeSearchResults,
} from '@/utils/responseGuards'

const message = useMessage()

const activeTab = ref('hot')
const selectedSource = ref('douban')
const selectedCategory = ref<string>('热门')
const loading = ref(false)
const movies = ref<HotMovieItem[]>([])

const tmdbCategories = ref<CategoryOption[]>([])
const doubanCategories = ref<CategoryOption[]>([])

const searchKeyword = ref('')
const searching = ref(false)
const searched = ref(false)
const searchResults = ref<SearchResult[]>([])
const tmdbApiKey = ref(true)

const configForm = ref<SubscriptionConfig>({
  enableTMDB: true,
  enableDouban: true,
  panSearchURL: 'https://so.252035.xyz/api/search',
  defaultMountPath: '/热门订阅',
  autoMount: false,
  cronExpression: '0 2 * * *',
  tmdbAPIKey: '',
  openaiAPIKey: '',
  openaiBaseURL: 'https://api.openai.com',
  openaiModel: 'gpt-4o-mini',
})
const savingConfig = ref(false)

const showDetailModal = ref(false)
const selectedMovie = ref<HotMovieItem | null>(null)
const showMountModal = ref(false)
const mounting = ref(false)
const mountForm = ref({
  title: '',
  shareUrl: '',
  shareCode: '',
  cover: '',
  mountPath: '',
})

let isPageMounted = false
let hotDataRequestId = 0
let searchRequestId = 0
let saveConfigRequestId = 0
let mountRequestId = 0
let mountModalSession = 0

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return value !== null && typeof value === 'object'
}

const isString = (value: unknown): value is string => {
  return typeof value === 'string'
}

const isBoolean = (value: unknown): value is boolean => {
  return typeof value === 'boolean'
}

const isFiniteNumber = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isFinite(value)
}

const isSubscriptionConfig = (value: unknown): value is SubscriptionConfig => {
  if (!isRecord(value)) {
    return false
  }

  return (
    isBoolean(value.enableTMDB) &&
    isBoolean(value.enableDouban) &&
    isString(value.panSearchURL) &&
    isString(value.defaultMountPath) &&
    isBoolean(value.autoMount) &&
    isString(value.cronExpression) &&
    isString(value.tmdbAPIKey) &&
    isString(value.openaiAPIKey) &&
    isString(value.openaiBaseURL) &&
    isString(value.openaiModel)
  )
}

const isMountSubscriptionResponse = (value: unknown): value is MountSubscriptionResponse => {
  if (!isRecord(value)) {
    return false
  }

  return (
    isString(value.message) &&
    isString(value.mountPath) &&
    isString(value.shareURL) &&
    isString(value.shareCode) &&
    isFiniteNumber(value.fileId) &&
    isString(value.name)
  )
}

const loadCategories = async () => {
  try {
    const res = await getCategories()
    if (res.code === 200) {
      const categories = normalizeCategoriesResponse(res.data)
      if (!categories) {
        message.error('加载分类失败：响应数据格式异常')

        return
      }

      tmdbCategories.value = categories.tmdb
      doubanCategories.value = categories.douban

      return
    }

    message.error(res.msg || '加载分类失败')
  } catch (err) {
    console.error('加载分类失败', err)
    message.error('加载分类失败')
  }
}

const loadHotData = async () => {
  const requestId = ++hotDataRequestId

  if (selectedSource.value === 'tmdb' && !configForm.value.tmdbAPIKey) {
    message.warning('TMDB API Key 未配置，已自动切换到豆瓣数据源')
    selectedSource.value = 'douban'
    selectedCategory.value = '热门'
    loading.value = false
    return
  }
  loading.value = true
  try {
    let res
    const category = selectedCategory.value
    if (selectedSource.value === 'tmdb') {
      if (
        category.startsWith('movie') ||
        category.startsWith('anime') ||
        category.startsWith('doc')
      ) {
        res = await getTMDbMovies(category)
      } else {
        res = await getTMDbTVs(category)
      }
    } else {
      res = await getDoubanMovies(category)
    }
    if (!isPageMounted || requestId !== hotDataRequestId) {
      return
    }

    if (res.code === 200) {
      const hotData = normalizeHotMoviesResponse(res.data)
      if (!hotData) {
        message.error('加载数据失败：响应数据格式异常')
        movies.value = []

        return
      }

      movies.value = hotData.movies
    } else {
      message.error(res.msg || '加载数据失败')
      movies.value = []
    }
  } catch {
    if (!isPageMounted || requestId !== hotDataRequestId) {
      return
    }

    message.error('加载数据失败')
    movies.value = []
  } finally {
    if (isPageMounted && requestId === hotDataRequestId) {
      loading.value = false
    }
  }
}

const handleSourceChange = () => {
  if (selectedSource.value === 'tmdb') {
    selectedCategory.value = 'movie_popular'
  } else {
    selectedCategory.value = '热门'
  }
  loadHotData()
}

const handleCategoryChange = (value: string | number | null) => {
  if (typeof value === 'string') {
    selectedCategory.value = value
  }

  loadHotData()
}

const loadConfig = async () => {
  try {
    const res = await getSubscriptionConfig()
    if (res.code === 200 && isSubscriptionConfig(res.data)) {
      configForm.value = res.data
      tmdbApiKey.value = !!res.data.tmdbAPIKey
    }
  } catch (err) {
    console.error('加载配置失败', err)
  }
}

watch(selectedSource, () => {
  if (selectedSource.value === 'tmdb' && !configForm.value.tmdbAPIKey) {
    message.warning('TMDB API Key 未配置，已自动切换到豆瓣数据源')
    selectedSource.value = 'douban'
    selectedCategory.value = '热门'
    return
  }
  handleSourceChange()
})

const showDetail = (movie: HotMovieItem) => {
  selectedMovie.value = movie
  showDetailModal.value = true
}

const handleSearch = async () => {
  if (!searchKeyword.value.trim()) {
    message.warning('请输入搜索关键词')
    return
  }

  const requestId = ++searchRequestId

  searching.value = true
  searched.value = true
  try {
    const res = await searchPan(searchKeyword.value)
    if (!isPageMounted || requestId !== searchRequestId) {
      return
    }

    if (res.code === 200) {
      const results = normalizeSearchResults(res.data)
      if (!results) {
        message.error('搜索失败：响应数据格式异常')

        return
      }

      searchResults.value = results
    } else {
      message.error(res.msg || '搜索失败')
      searchResults.value = []
    }
  } catch {
    if (!isPageMounted || requestId !== searchRequestId) {
      return
    }

    message.error('搜索失败')
    searchResults.value = []
  } finally {
    if (isPageMounted && requestId === searchRequestId) {
      searching.value = false
    }
  }
}

const handleAISearch = async () => {
  if (!searchKeyword.value.trim()) {
    message.warning('请输入搜索关键词')
    return
  }

  const requestId = ++searchRequestId

  searching.value = true
  searched.value = true
  try {
    const res = await searchPanWithAI(searchKeyword.value)
    if (!isPageMounted || requestId !== searchRequestId) {
      return
    }

    if (res.code === 200 && res.data) {
      if (res.data.result) {
        const results = normalizeSearchResults([res.data.result])
        if (!results) {
          message.error('AI搜索失败：响应数据格式异常')

          return
        }

        message.success('AI推荐: ' + res.data.keyword)
        searchResults.value = results
      } else if (Array.isArray(res.data.allResults) && res.data.allResults.length > 0) {
        const results = normalizeSearchResults(res.data.allResults)
        if (!results) {
          message.error('AI搜索失败：响应数据格式异常')

          return
        }

        message.info('AI服务未配置，返回全部搜索结果')
        searchResults.value = results
      } else {
        message.warning(res.data.message || '未找到相关资源')
        searchResults.value = []
      }
    } else {
      message.error(res.msg || 'AI搜索失败')
      searchResults.value = []
    }
  } catch {
    if (!isPageMounted || requestId !== searchRequestId) {
      return
    }

    message.error('AI搜索失败')
    searchResults.value = []
  } finally {
    if (isPageMounted && requestId === searchRequestId) {
      searching.value = false
    }
  }
}

const selectSearchResult = (item: SearchResult) => {
  mountModalSession++
  mountForm.value = {
    title: item.name,
    shareUrl: item.shareUrl,
    shareCode: item.shareCode,
    cover: item.cover || '',
    mountPath: buildDefaultMountPath(item.name),
  }
  showMountModal.value = true
}

const buildDefaultMountPath = (title: string) => {
  const basePath = (configForm.value.defaultMountPath || '/热门订阅').replace(/\/+$/, '')
  const safeTitle = title.trim().replace(/^\/+/, '')

  return `${basePath || ''}/${safeTitle}`.replace(/\/+/g, '/')
}

const handleMountModalShowUpdate = (show: boolean) => {
  if (show) {
    showMountModal.value = true

    return
  }
  if (mounting.value) {
    return
  }

  closeMountModal()
}

const closeMountModal = () => {
  mountModalSession++
  showMountModal.value = false
  mounting.value = false
}

const handleSearchResource = () => {
  if (selectedMovie.value) {
    searchKeyword.value = selectedMovie.value.title
    activeTab.value = 'search'
    showDetailModal.value = false
    handleSearch()
  }
}

const handleSaveConfig = async () => {
  if (savingConfig.value) {
    return
  }

  const requestId = ++saveConfigRequestId

  savingConfig.value = true
  try {
    const res = await updateSubscriptionConfig(configForm.value)
    if (!isPageMounted || requestId !== saveConfigRequestId) {
      return
    }

    if (res.code === 200) {
      if (!isSubscriptionConfig(res.data)) {
        message.error('保存完成但响应配置缺失/异常')

        return
      }

      configForm.value = res.data
      tmdbApiKey.value = !!res.data.tmdbAPIKey
      message.success('保存成功')
    } else {
      message.error(res.msg || '保存失败')
    }
  } catch {
    if (!isPageMounted || requestId !== saveConfigRequestId) {
      return
    }

    message.error('保存失败')
  } finally {
    if (isPageMounted && requestId === saveConfigRequestId) {
      savingConfig.value = false
    }
  }
}

const handleMount = async () => {
  if (mounting.value) {
    return
  }

  mountForm.value.mountPath = mountForm.value.mountPath.trim()

  if (!mountForm.value.title || !mountForm.value.shareUrl || !mountForm.value.mountPath) {
    message.warning('请填写完整信息')
    return
  }

  if (!mountForm.value.mountPath.startsWith('/')) {
    message.warning('挂载路径必须以 / 开头')
    return
  }

  const requestId = ++mountRequestId
  const session = mountModalSession
  const payload = { ...mountForm.value }

  mounting.value = true
  try {
    const res = await mountSubscription(payload)
    if (
      !isPageMounted ||
      requestId !== mountRequestId ||
      session !== mountModalSession ||
      !showMountModal.value
    ) {
      return
    }

    if (res.code === 200) {
      if (!isMountSubscriptionResponse(res.data)) {
        message.error('挂载完成但响应结果缺失/异常')

        return
      }

      message.success(res.data.message || '挂载成功')
      closeMountModal()
    } else {
      message.error(res.msg || '挂载失败')
    }
  } catch {
    if (
      !isPageMounted ||
      requestId !== mountRequestId ||
      session !== mountModalSession ||
      !showMountModal.value
    ) {
      return
    }

    message.error('挂载失败')
  } finally {
    if (
      isPageMounted &&
      requestId === mountRequestId &&
      session === mountModalSession &&
      showMountModal.value
    ) {
      mounting.value = false
    }
  }
}

onMounted(async () => {
  isPageMounted = true

  await loadConfig()
  if (!isPageMounted) return

  await loadCategories()
  if (!isPageMounted) return

  // 设置默认分类
  if (configForm.value.tmdbAPIKey) {
    selectedSource.value = 'tmdb'
    selectedCategory.value = 'movie_popular'
  } else {
    message.warning('TMDB API Key 未配置，已自动切换到豆瓣数据源')
    selectedSource.value = 'douban'
    selectedCategory.value = '热门'
  }

  // 加载热门数据
  loadHotData()
})

onUnmounted(() => {
  isPageMounted = false
  hotDataRequestId++
  searchRequestId++
  saveConfigRequestId++
  mountRequestId++
  mountModalSession++
})
</script>

<style scoped>
.subscriptions-page {
  padding: 0;
}

.hot-section,
.search-section,
.settings-section {
  padding-top: 16px;
}

.source-tabs {
  margin-bottom: 16px;
}

.category-tabs {
  margin-bottom: 20px;
}

.movies-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 16px;
  min-height: 400px;
}

.movie-card {
  cursor: pointer;
  transition: transform 0.2s;
}

.movie-card:hover {
  transform: scale(1.05);
}

.movie-cover {
  position: relative;
  border-radius: 8px;
  overflow: hidden;
  aspect-ratio: 2/3;
}

.movie-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.movie-rating {
  position: absolute;
  top: 8px;
  right: 8px;
  background: rgb(0 0 0 / 70%);
  color: #ffd700;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: bold;
}

.movie-info {
  padding: 8px 4px;
}

.movie-title {
  font-size: 14px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.movie-year {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

.search-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
}

.search-result-meta {
  display: flex;
  align-items: center;
  gap: 12px;
}

.upload-time {
  color: #999;
  font-size: 12px;
}
</style>
