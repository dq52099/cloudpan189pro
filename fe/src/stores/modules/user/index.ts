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

export const useUserStore = defineStore('user', () => {
  const user: UserInfo = reactive(initUser())

  const load = () => {
    const userLocal = localStg.get('user')
    if (userLocal) {
      Object.assign(user, userLocal)
    }
  }

  const store = (_user: UserInfo) => {
    localStg.set('user', _user)
    Object.assign(user, _user)
  }

  const refresh = () => {
    return getUserInfo().then((res) => {
      if (res.code === 200 && res.data) {
        store(res.data)
      }

      return res.data
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
