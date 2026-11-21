import { useAuthStore } from '@/stores/auth/auth';
import { storageGet, StorageKeys } from '@/utils/core/storage';

// 通知 WebSocket 客户端：登录后自动连接后端 /api/v1/ws，接收实时通知/未读数更新。
// 后端 NotifyUser / NotifyUnreadCount 已实现（internal/api/websocket/websocket.go），
// 此前前端零接入，导致通知无法实时推送。

export interface WsNotificationPayload {
  title: string;
  content: string;
  notif_type: string;
  related_id: number;
}

export interface WsUnreadPayload {
  count: number;
}

let socket: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let heartbeatTimer: ReturnType<typeof setInterval> | null = null;
let started = false;
// P2 修复：指数退避重连（1s 起，每次翻倍，上限 30s），连接成功后重置
let reconnectAttempts = 0;
const BASE_RECONNECT_DELAY = 1000;
const MAX_RECONNECT_DELAY = 30000;

// 全局事件订阅（供通知中心和侧边栏徽标共用）
type Listener = () => void;
const unreadListeners = new Set<Listener>();

function getWsBaseUrl(): string {
  const base = import.meta.env.VITE_WS_BASE_URL as string | undefined;
  if (base) return base.replace(/\/$/, '');
  // 与 api base 对齐：/api/v1 -> /api/v1/ws
  return '/api/v1/ws';
}

function clearTimers() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
  if (heartbeatTimer) {
    clearInterval(heartbeatTimer);
    heartbeatTimer = null;
  }
}

function scheduleReconnect() {
  if (reconnectTimer) return;
  const delay = Math.min(
    BASE_RECONNECT_DELAY * Math.pow(2, reconnectAttempts),
    MAX_RECONNECT_DELAY,
  );
  reconnectAttempts++;
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    connectNotificationSocket();
  }, delay);
}

function connectNotificationSocket() {
  const auth = useAuthStore();
  const token = storageGet(StorageKeys.AUTH_TOKEN);
  if (!token || !auth.isLoggedIn) return;

  clearTimers();

  const url = `${getWsBaseUrl()}?token=${encodeURIComponent(token)}`;
  try {
    socket = new WebSocket(url);
  } catch {
    scheduleReconnect();
    return;
  }

  socket.onopen = () => {
    // P2 修复：连接成功后重置退避计数，下次断开从 1s 重新开始
    reconnectAttempts = 0;
    // 心跳保活，避免代理断开
    heartbeatTimer = setInterval(() => {
      if (socket && socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: 'ping' }));
      }
    }, 25000);
  };

  socket.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data as string) as {
        type: string;
        data: WsNotificationPayload | WsUnreadPayload;
      };
      if (msg.type === 'unread_count') {
        unreadListeners.forEach((fn) => fn());
      } else if (msg.type === 'notification') {
        unreadListeners.forEach((fn) => fn());
      }
    } catch {
      /* 忽略无法解析的消息 */
    }
  };

  socket.onclose = () => {
    socket = null;
    if (heartbeatTimer) {
      clearInterval(heartbeatTimer);
      heartbeatTimer = null;
    }
    scheduleReconnect();
  };

  socket.onerror = () => {
    socket?.close();
  };
}

function disconnectNotificationSocket() {
  clearTimers();
  if (socket) {
    socket.close();
    socket = null;
  }
  started = false;
  // P2 修复：断开时重置退避计数，下次连接从初始延迟重新开始
  reconnectAttempts = 0;
}

/** 启动全局通知 WebSocket（在 App 根组件挂载一次） */
export function startNotificationSocket() {
  const auth = useAuthStore();
  if (started || !auth.isLoggedIn) return;
  started = true;
  connectNotificationSocket();

  // token 失效自动断开
  window.addEventListener('auth:logout', disconnectNotificationSocket, { once: true });
}

/** 注册未读数/通知变更监听（通知中心、侧边栏徽标调用） */
export function onNotificationUpdate(listener: Listener): () => void {
  unreadListeners.add(listener);
  return () => unreadListeners.delete(listener);
}

export function stopNotificationSocket() {
  disconnectNotificationSocket();
}
