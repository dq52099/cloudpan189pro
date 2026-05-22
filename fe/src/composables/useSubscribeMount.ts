import { h } from 'vue'
import { useModal } from 'naive-ui'
import { SubscribeMountModal } from '@/components/storage'

export function useSubscribeMount() {
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
        title: '订阅号资源挂载',
        preset: 'dialog',
        style: {
          width: 'auto',
          maxWidth: '1200px',
        },
        content: () =>
          h(SubscribeMountModal, {
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
