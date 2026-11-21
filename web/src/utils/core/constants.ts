import i18n from '@/i18n';

const { t } = i18n.global;

export const ROLE = { SUPER_ADMIN: 1, ADMIN: 2, USER: 3 } as const;
export const DOC_TYPE = { DOCUMENT: 1, DIRECTORY: 2 } as const;
// 与后端接口保持一致：visit_level 是字符串（"0" 所有人 / "1" 登录 / "2" 成员）
export const VISIT_LEVEL = { PUBLIC: '0', LOGGED_IN: '1', MEMBERS_ONLY: '2' } as const;
export const CACHE_TTL = 10 * 60 * 1000;
export const DOCUMENT_SIZE_LIMIT = 100000;
export const DEBOUNCE_DELAY = 300;

/** macOS 的修饰键是 ⌘，其余平台显示 Ctrl —— 快捷键监听本身 metaKey/ctrlKey 两者都收，
    错的只是徽标上的符号。平台不是语言事实，不进语言包。 */
export const IS_MAC = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.userAgent);
export const SEARCH_SHORTCUT_HINT = IS_MAC ? '⌘K' : 'Ctrl K';

export function getRoleName(roleId?: number): string {
  // 角色**编号**是数据，名字是显示文字 —— 之前直接返回中文字面量，
  // 于是英文界面的侧边栏底部写着"超级管理员"。
  if (roleId === ROLE.SUPER_ADMIN) return t('common.roleSuperAdmin');
  if (roleId === ROLE.ADMIN) return t('common.roleAdmin');
  return t('common.roleUser');
}
