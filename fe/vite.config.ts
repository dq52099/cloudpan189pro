import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

const chunkIncludes = (id: string, packages: string[]) =>
  packages.some((pkg) => id.includes(`/node_modules/${pkg}/`))

const manualChunks = (id: string) => {
  if (!id.includes('node_modules')) {
    return
  }

  if (chunkIncludes(id, ['vue', '@vue', 'vue-router', 'pinia'])) {
    return 'vue-vendor'
  }

  if (chunkIncludes(id, ['@vicons'])) {
    return 'icons'
  }

  if (chunkIncludes(id, ['hls.js', 'plyr'])) {
    return 'media-vendor'
  }

  if (
    chunkIncludes(id, [
      '@css-render',
      'css-render',
      'date-fns',
      'evtd',
      'seemly',
      'treemate',
      'vdirs',
      'vooks',
      'vueuc',
    ])
  ) {
    return 'naive-core'
  }

  if (id.includes('/node_modules/naive-ui/')) {
    if (
      id.includes('/naive-ui/es/_internal/') ||
      id.includes('/naive-ui/es/_mixins/') ||
      id.includes('/naive-ui/es/_styles/') ||
      id.includes('/naive-ui/es/_utils/') ||
      id.includes('/naive-ui/es/themes/')
    ) {
      return 'naive-core'
    }

    if (
      id.includes('/naive-ui/es/data-table/') ||
      id.includes('/naive-ui/es/pagination/') ||
      id.includes('/naive-ui/es/tree/') ||
      id.includes('/naive-ui/es/cascader/') ||
      id.includes('/naive-ui/es/select/') ||
      id.includes('/naive-ui/es/dropdown/')
    ) {
      return 'naive-data'
    }

    if (
      id.includes('/naive-ui/es/form/') ||
      id.includes('/naive-ui/es/input/') ||
      id.includes('/naive-ui/es/input-number/') ||
      id.includes('/naive-ui/es/date-picker/') ||
      id.includes('/naive-ui/es/radio/') ||
      id.includes('/naive-ui/es/checkbox/') ||
      id.includes('/naive-ui/es/switch/') ||
      id.includes('/naive-ui/es/slider/') ||
      id.includes('/naive-ui/es/dynamic-tags/')
    ) {
      return 'naive-form'
    }

    if (
      id.includes('/naive-ui/es/modal/') ||
      id.includes('/naive-ui/es/dialog/') ||
      id.includes('/naive-ui/es/drawer/') ||
      id.includes('/naive-ui/es/popover/') ||
      id.includes('/naive-ui/es/tooltip/') ||
      id.includes('/naive-ui/es/message/') ||
      id.includes('/naive-ui/es/notification/')
    ) {
      return 'naive-overlay'
    }

    if (
      id.includes('/naive-ui/es/layout/') ||
      id.includes('/naive-ui/es/grid/') ||
      id.includes('/naive-ui/es/space/') ||
      id.includes('/naive-ui/es/list/') ||
      id.includes('/naive-ui/es/breadcrumb/') ||
      id.includes('/naive-ui/es/menu/') ||
      id.includes('/naive-ui/es/tabs/') ||
      id.includes('/naive-ui/es/card/') ||
      id.includes('/naive-ui/es/descriptions/')
    ) {
      return 'naive-layout'
    }

    return 'naive-basic'
  }

  if (chunkIncludes(id, ['axios', 'dayjs', 'localforage'])) {
    return 'app-vendor'
  }

  return 'vendor'
}

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const isProduction = mode === 'production'
  const apiProxy = {
    '/api': {
      target: 'http://localhost:12395',
      changeOrigin: true,
    },
  }

  return {
    plugins: [vue()],
    esbuild: {
      drop: isProduction ? ['console', 'debugger'] : [],
    },
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    build: {
      rollupOptions: {
        output: {
          manualChunks,
        },
      },
    },
    server: {
      host: '0.0.0.0',
      port: 5173,
      proxy: apiProxy,
    },
    preview: {
      proxy: apiProxy,
    },
  }
})
