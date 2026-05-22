import { getUserInfo } from '@/api/user'
import { localStg } from '@/utils/storage'
import { defineStore } from 'pinia'
import { computed, reactive } from 'vue'

type UserInfo = Models.UserInfo

const initUser: () => UserInfo = () => ({
  id: 0,
  username: '',
  status: 0,
  isAdmin: false,
  groupId: 0,
  version: 0,
  createdAt: '2023-01-01T00:00:00.000Z',
  updatedAt: '2023-01-01T00:00:00.000Z',
})

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return typeof value === 'object' && value !== null
}

const isString = (value: unknown): value is string => typeof value === 'string'

const isBoolean = (value: unknown): value is boolean => typeof value === 'boolean'

const isSafeNonNegativeInteger = (value: unknown): value is number => {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

const normalizeUserInfo = (value: unknown): UserInfo | null => {
  if (!isRecord(value)) {
    return null
  }

  if (
    !isSafeNonNegativeInteger(value.id) ||
    !isString(value.username) ||
    !isSafeNonNegativeInteger(value.status) ||
    !isBoolean(value.isAdmin) ||
    !isSafeNonNegativeInteger(value.groupId) ||
    !isSafeNonNegativeInteger(value.version) ||
    !isString(value.createdAt) ||
    !isString(value.updatedAt) ||
    (value.groupName !== undefined && !isString(value.groupName))
  ) {
    return null
  }

  return {
    id: value.id,
    username: value.username,
    status: value.status,
    isAdmin: value.isAdmin,
    groupId: value.groupId,
    version: value.version,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
    groupName: value.groupName,
  }
}

export const useUserStore = defineStore('user', () => {
  const user: UserInfo = reactive(initUser())

  const load = () => {
    const userLocal = localStg.get('user')
    if (!userLocal) {
      return
    }

    const normalizedUser = normalizeUserInfo(userLocal)
    if (!normalizedUser) {
      localStg.remove('user')

      return
    }

    Object.assign(user, normalizedUser)
  }

  const store = (_user: unknown) => {
    const normalizedUser = normalizeUserInfo(_user)
    if (!normalizedUser) {
      return false
    }

    localStg.set('user', normalizedUser)
    Object.assign(user, normalizedUser)

    return true
  }

  const refresh = () => {
    return getUserInfo().then((res) => {
      if (res.code === 200 && res.data) {
        if (!store(res.data)) {
          return undefined
        }
      }

      return normalizeUserInfo(res.data)
    })
  }

  const clear = () => {
    localStg.remove('user')
    Object.assign(user, initUser())
  }

  const get = () => user

  const isAdmin = computed(() => user.isAdmin)

  return {
    load,
    store,
    refresh,
    clear,
    get,
    isAdmin,
  }
})
