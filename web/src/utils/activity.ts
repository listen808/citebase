/**
 * 活动记录工具函数
 * 统一处理文档操作动作的名称、状态、颜色等映射
 */

import { cv } from '@/utils/core/apple-tokens';
import i18n from '@/i18n';

// 动作名以前是一张中文表，界面切成英文照样印"修改"。名字归 common.*，
// 数字与字符串两套编码（1/create）都映射到同一个键。
const { t } = i18n.global;

const actionKeyMap: Record<string, string> = {
  create: 'common.create',
  edit: 'common.modify',
  delete: 'common.delete',
  restore: 'common.restore',
  view: 'common.view',
  collect: 'common.favorite',
  '1': 'common.create',
  '2': 'common.modify',
  '3': 'common.delete',
  '4': 'common.restore',
};

/**
 * 获取动作的显示名称
 */
export function formatActionName(action: string | number): string {
  const key = actionKeyMap[String(action)];
  return key ? t(key) : String(action);
}

/**
 * 获取动作的 ApStatusBadge 状态
 */
export function getActionStatus(action: string): 'enabled' | 'warning' | 'error' | 'default' {
  switch (String(action)) {
    case '1':
    case 'create': return 'enabled';
    case '2':
    case 'edit': return 'warning';
    case '3':
    case 'delete': return 'error';
    case '4':
    case 'restore': return 'default';
    default: return 'default';
  }
}

/**
 * 获取动作对应的颜色
 */
export function getActionColor(action: string): string {
  const colorMap: Record<string, string> = {
    'create': cv('green'),
    '1': cv('green'),
    'edit': cv('orange'),
    '2': cv('orange'),
    'delete': cv('red'),
    '3': cv('red'),
    'restore': cv('blue'),
    '4': cv('blue'),
  };
  return colorMap[String(action)] || cv('text-quaternary');
}

/**
 * 判断是否为关键操作（删除或权限变更）
 */
export function isCriticalAction(action: string): boolean {
  const a = String(action);
  return a === 'delete' || a === '3' || a === 'permission_change';
}

/**
 * 判断是否为删除操作
 */
export function isDeleteAction(action: string): boolean {
  const a = String(action);
  return a === 'delete' || a === '3';
}
