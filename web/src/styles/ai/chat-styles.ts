/**
 * AI 对话页面共享样式 — AIChatView 共用
 */
import { cv, AppleRadius, AppleSpace, AppleZIndex } from '@/utils/core/apple-tokens'

// ====== Shared layout ======

export const chatContainerStyle = {
  display: 'flex',
  flex: 1,
  minHeight: 0,
  overflow: 'hidden',
  border: `1px solid ${cv('border-light')}`,
  borderRadius: AppleRadius.card,
  backgroundColor: cv('surface-panel'),
}

export const chatMainAreaStyle = {
  flex: 1,
  display: 'flex',
  flexDirection: 'column' as const,
  position: 'relative' as const,
  minWidth: 0,
  overflow: 'hidden',
}

// 「跳至最新」浮层：悬在消息区右下角，不挤压对话流
export const chatJumpLatestStyle = {
  position: 'absolute' as const,
  right: AppleSpace.lg,
  bottom: AppleSpace.md,
  zIndex: AppleZIndex.raised,
  boxShadow: cv('shadow-level3'),
}

export const chatMessagesContainerStyle = {
  flex: 1,
  overflowY: 'auto' as const,
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  minHeight: 0,
}
