import { defineStore } from 'pinia'
import { ref, type VNode } from 'vue'

/**
 * 跨组件传递顶栏操作按钮。
 * 各页面 onMounted 时调用 setActions 设置 VNode，
 * AuthenticatedShell 顶栏渲染 actions vnode。
 */
export const useHeaderActionsStore = defineStore('headerActions', () => {
  const actions = ref<VNode | null>(null)

  function setActions(vnode: VNode | null) {
    actions.value = vnode
  }

  function clear() {
    actions.value = null
  }

  return { actions, setActions, clear }
})
