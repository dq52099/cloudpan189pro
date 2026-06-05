import { defineStore } from 'pinia'
import { ref } from 'vue'
import { localStg } from '@/utils/storage'

type ThemeMode = 'light' | 'dark'

const storagePrefix = import.meta.env.VITE_STORAGE_PREFIX || ''
const legacyThemeKeys = Array.from(new Set([`${storagePrefix}theme`, 'theme']))

const isThemeMode = (value: unknown): value is ThemeMode => {
  return value === 'light' || value === 'dark'
}

const readLegacyTheme = (): ThemeMode | null => {
  try {
    for (const key of legacyThemeKeys) {
      const value = window.localStorage.getItem(key)
      if (isThemeMode(value)) {
        return value
      }
    }
  } catch (error) {
    console.error('读取主题偏好失败:', error)
  }

  return null
}

const readStoredTheme = (): ThemeMode | null => {
  const legacyTheme = readLegacyTheme()
  if (legacyTheme) {
    localStg.set('theme', legacyTheme)

    return legacyTheme
  }

  const storedTheme = localStg.get('theme')
  if (isThemeMode(storedTheme)) {
    return storedTheme
  }

  if (storedTheme !== null) {
    localStg.remove('theme')
  }

  return null
}

const getSystemPrefersDark = () => {
  try {
    return window.matchMedia('(prefers-color-scheme: dark)').matches
  } catch (error) {
    console.error('读取系统主题失败:', error)

    return false
  }
}

export const useThemeStore = defineStore('theme', () => {
  const isDark = ref(false)

  const toggleTheme = () => {
    isDark.value = !isDark.value
    localStg.set('theme', isDark.value ? 'dark' : 'light')
  }

  const initTheme = () => {
    const savedTheme = readStoredTheme()
    if (savedTheme) {
      isDark.value = savedTheme === 'dark'
    } else {
      // 检测系统主题
      isDark.value = getSystemPrefersDark()
    }
  }

  return {
    isDark,
    toggleTheme,
    initTheme,
  }
})
