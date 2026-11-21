import type { User } from './models';
import i18n from '@/i18n';

// 这里不是组件，用不了 useI18n()，只能取全局 composer 的 t。
const { t } = i18n.global;

export function getUserId(user: User | null | undefined): number | undefined {
  if (!user) return undefined;
  return user.id;
}

export function getUserDisplayName(
  user: { username: string; given_name?: string } | null | undefined
): string {
  if (!user) return t('common.notLoggedIn');
  return user.given_name || user.username;
}

export function getUserEmail(user: User | null | undefined): string | undefined {
  if (!user) return undefined;
  return user.email;
}

export function getUserAvatar(user: User | null | undefined): string | undefined {
  if (!user) return undefined;
  return user.avatar;
}

export function isUserForbidden(user: User | null | undefined): boolean {
  if (!user) return false;
  return user.is_forbidden === 1;
}

export function isUserDeleted(user: User | null | undefined): boolean {
  if (!user) return false;
  return user.is_delete === 1;
}

export type UserStatus = 'deleted' | 'disabled' | 'online';

export function getUserStatus(user: User | null | undefined): UserStatus {
  if (!user) return 'online';
  if (user.is_delete === 1) return 'deleted';
  if (user.is_forbidden === 1) return 'disabled';
  return 'online';
}

export function getUserStatusLabel(user: User | null | undefined): string {
  const status = getUserStatus(user);
  switch (status) {
    case 'deleted': return t('common.deleted');
    case 'disabled': return t('common.disabled');
    case 'online': return t('common.normal');
  }
}

export function getUserStatusBadge(status: UserStatus): 'error' | 'disabled' | 'online' {
  switch (status) {
    case 'deleted': return 'error';
    case 'disabled': return 'disabled';
    case 'online': return 'online';
  }
}

export function getUserKey(user: User | null | undefined): string {
  const id = getUserId(user);
  return id ? String(id) : 'unknown';
}

export function isSameUser(
  user1: User | null | undefined,
  user2: User | null | undefined
): boolean {
  const id1 = getUserId(user1);
  const id2 = getUserId(user2);
  return id1 !== undefined && id1 === id2;
}

export function formatUserInfo(user: User | null | undefined): {
  id: number | undefined;
  displayName: string;
  email: string | undefined;
  avatar: string | undefined;
} {
  return {
    id: getUserId(user),
    displayName: getUserDisplayName(user),
    email: getUserEmail(user),
    avatar: getUserAvatar(user),
  };
}