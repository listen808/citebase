import { ref, onBeforeUnmount } from 'vue'
import { cv, type AppleCssVarName } from '@/utils/core/apple-tokens'
import * as Y from 'yjs'
import { WebsocketProvider } from 'y-websocket'
import { useAuthStore } from '@/stores/auth/auth'
import { storageGet, StorageKeys } from '@/utils/core/storage'
import i18n from '@/i18n';

const { t } = i18n.global;

export interface CollabUser {
  id: number
  name: string
  color: string
}

// 身份色取主题纯色族：同一 userId 恒定同色，且换主题后自动跟着该主题的调色板走
const CURSOR_VARS: readonly AppleCssVarName[] = ['blue', 'green', 'orange', 'purple', 'teal', 'pink', 'indigo', 'yellow']

function presenceColor(id: string | number): string {
  let h = 0
  const s = String(id)
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0
  return cv(CURSOR_VARS[h % CURSOR_VARS.length] ?? 'blue')
}

// 重连配置
const RECONNECT_BASE_DELAY = 1000 // 1s
const RECONNECT_MAX_DELAY = 30000 // 30s
const RECONNECT_MAX_RETRIES = 5

/**
 * Composable for real-time collaborative editing using Yjs CRDT.
 *
 * @param documentId The document being edited
 * @returns Collaboration state and control functions
 */
export function useCollaboration(documentId: number) {
  const authStore = useAuthStore()

  const doc = new Y.Doc()
  const provider = ref<WebsocketProvider | null>(null)
  const isConnected = ref(false)
  const isSynced = ref(false)
  const connectedUsers = ref<CollabUser[]>([])
  const error = ref<string | null>(null)

  // 重连状态
  let reconnectAttempts = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null

  function getWsUrl(): string {
    const token = storageGet(StorageKeys.AUTH_TOKEN) ?? ''
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const baseUrl = `${protocol}//${window.location.host}/api/v1/collab/ws/${documentId}`
    return `${baseUrl}?token=${encodeURIComponent(token)}`
  }

  function clearReconnectTimer() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
  }

  function scheduleReconnect() {
    if (reconnectAttempts >= RECONNECT_MAX_RETRIES) {
      error.value = t('tsUi.k6')
      return
    }
    clearReconnectTimer()
    const delay = Math.min(RECONNECT_BASE_DELAY * Math.pow(2, reconnectAttempts), RECONNECT_MAX_DELAY)
    reconnectAttempts++
    reconnectTimer = setTimeout(() => {
      if (!provider.value || !isConnected.value) {
        connect()
      }
    }, delay)
  }

  function connect() {
    if (provider.value) return

    try {
      const wsUrl = getWsUrl()
      const roomName = `doc-${documentId}`

      // 使用默认 awareness，不传入 undefined
      provider.value = new WebsocketProvider(wsUrl, roomName, doc, {
        connect: true,
      })

      // 设置用户信息
      const user = authStore.user
      provider.value.awareness.setLocalStateField('user', {
        id: user?.id ?? 0,
        name: user?.given_name || user?.username || 'Anonymous',
        color: presenceColor(user?.id ?? 'anon'),
      })

      // 监听用户变化
      provider.value.awareness.on('change', () => {
        if (!provider.value?.awareness) return
        interface AwarenessState {
          user?: { id: number; name: string; color: string }
        }
        const states = Array.from(provider.value.awareness.getStates().values()) as AwarenessState[]
        connectedUsers.value = states
          .filter((s) => s.user != null)
          .map((s) => ({
            id: s.user!.id,
            name: s.user!.name,
            color: s.user!.color,
          }))
      })

      provider.value.on('status', ({ status }: { status: string }) => {
        isConnected.value = status === 'connected'
        if (status === 'connected') {
          // 连接成功，重置重连计数
          reconnectAttempts = 0
          error.value = null
        } else if (status === 'disconnected') {
          // 断线后尝试重连
          scheduleReconnect()
        }
      })

      provider.value.on('sync', (synced: unknown) => {
        isSynced.value = synced === true
      })

      provider.value.on('connection-error', () => {
        error.value = t('tsUi.k7')
        scheduleReconnect()
      })
    } catch (err) {
      error.value = `连接失败: ${err}`
      scheduleReconnect()
    }
  }

  function disconnect() {
    clearReconnectTimer()
    if (provider.value) {
      provider.value.awareness?.destroy()
      provider.value.destroy()
      provider.value = null
    }
    isConnected.value = false
    isSynced.value = false
    connectedUsers.value = []
    error.value = null
    reconnectAttempts = 0
  }

  /**
   * Get the Yjs Text type for the document content.
   * Use this with Tiptap's Collaboration extension.
   */
  function getYText(): Y.Text {
    return doc.getText('content')
  }

  /**
   * Get the Yjs XmlFragment for rich content (if using ProseMirror/Tiptap).
   */
  function getXmlFragment(): Y.XmlFragment {
    return doc.get('prosemirror', Y.XmlFragment) as Y.XmlFragment
  }

  onBeforeUnmount(() => {
    disconnect()
    doc.destroy()
  })

  return {
    doc,
    provider,
    isConnected,
    isSynced,
    connectedUsers,
    error,
    connect,
    disconnect,
    getYText,
    getXmlFragment,
  }
}
