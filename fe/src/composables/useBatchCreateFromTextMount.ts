import { h } from 'vue'
import { useModal } from 'naive-ui'
import { BatchCreateFromTextModal, type MountItem } from '@/components/storage'
import { useMountPointBind } from '@/composables/useMountPointBind'
import type { BatchParseItem } from '@/api/storage'

export function useBatchCreateFromTextMount() {
  const modal = useModal()
  const mountPointBind = useMountPointBind()

  const show = (): Promise<{ success: boolean }> => {
    return new Promise((resolve) => {
      let settled = false
      let modalInstance: ReturnType<typeof modal.create>

      const settle = (payload: { success: boolean }) => {
        if (settled) return
        settled = true
        resolve(payload)
      }

      const closeWithCancel = () => {
        if (settled) return
        settle({ success: false })
        modalInstance.destroy()
      }

      modalInstance = modal.create({
        title: '批量文本导入',
        preset: 'dialog',
        style: { width: '700px' },
        content: () =>
          h(BatchCreateFromTextModal, {
            onParsed: async (payload: { items: BatchParseItem[]; token: number }) => {
              modalInstance.destroy()

              const mountItems: MountItem[] = payload.items.map((item) => ({
                name: item.name,
                osType: item.osType,
                subscribeUser: item.subscribeUser,
                shareCode: item.shareCode,
                shareAccessCode: item.shareAccessCode,
                fileId: item.fileId,
                cloudToken: payload.token,
                disableSwitchCloudToken: false,
              }))

              const result = await mountPointBind.show(mountItems, {
                defaultCloudToken: payload.token,
              })

              if (result && result.length > 0) {
                settle({ success: true })
              } else {
                settle({ success: false })
              }
            },
            onCancel: () => {
              closeWithCancel()
            },
          }),
        closable: false,
        maskClosable: false,
        closeOnEsc: false,
        onClose: closeWithCancel,
      })
    })
  }

  return { show }
}
