import { h, ref } from 'vue'
import { useModal, NButton } from 'naive-ui'
import { MountPointBindModal, type MountItem } from '@/components/storage'
import type { AddStorageResponse } from '@/api/storage'

export function useMountPointBind() {
  const modal = useModal()

  const show = (
    items: MountItem[],
    options?: { defaultCloudToken?: number }
  ): Promise<AddStorageResponse[]> => {
    return new Promise((resolve) => {
      const contentRef = ref<InstanceType<typeof MountPointBindModal> | null>(null)
      let settled = false
      let modalInstance: ReturnType<typeof modal.create>

      const settle = (payload: AddStorageResponse[]) => {
        if (settled) return
        settled = true
        resolve(payload)
      }

      const closeWithCancel = () => {
        if (settled) return
        if (contentRef.value?.state.submitLoading) return

        const confirmedItems = contentRef.value?.getConfirmedItems() ?? []
        settle(confirmedItems)
        modalInstance.destroy()
      }

      modalInstance = modal.create({
        title: '挂载点绑定',
        preset: 'dialog',
        style: {
          width: 'min(1000px, calc(100vw - 32px))',
        },
        content: () =>
          h(MountPointBindModal, {
            ref: contentRef,
            items,
            defaultCloudToken: options?.defaultCloudToken,
            onConfirm: (payload) => {
              settle(payload)
              modalInstance.destroy()
            },
            onCancel: () => {
              closeWithCancel()
            },
          }),
        action: () =>
          h('div', { style: 'display: flex; gap: 8px; justify-content: flex-end' }, [
            h(
              NButton,
              {
                onClick: () => {
                  closeWithCancel()
                },
                disabled: contentRef.value?.state.submitLoading,
              },
              { default: () => '取消' }
            ),
            h(
              NButton,
              {
                type: 'primary',
                loading: contentRef.value?.state.submitLoading,
                disabled: contentRef.value?.state.submitLoading,
                onClick: () => {
                  contentRef.value?.handleConfirm()
                },
              },
              { default: () => '确认挂载' }
            ),
          ]),
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
