import { h } from 'vue'
import { useModal } from 'naive-ui'
import { ShareMountModal } from '@/components/storage'

export function useShareMount() {
  const modal = useModal()

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
        title: '文件分享挂载',
        preset: 'dialog',
        style: {
          width: 'auto',
          maxWidth: '1200px',
        },
        content: () =>
          h(ShareMountModal, {
            onConfirm: (payload) => {
              settle(payload)
              modalInstance.destroy()
            },
            onCancel: () => {
              closeWithCancel()
            },
          }),
        action: () => null, // 动作按钮已在内容组件中处理
        closable: false,
        maskClosable: false,
        closeOnEsc: false,
        onClose: closeWithCancel,
      })
    })
  }

  return {
    show,
  }
}
