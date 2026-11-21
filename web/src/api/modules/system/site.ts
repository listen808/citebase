import { get } from '../../client';

export interface SiteInfo {
  name: string;
  /** 主页描述（没填是空串，前端据此退回语言包里那句一句话定位） */
  description: string;
}

/** 站点展示名（公开端点）。管理员在「系统设置 → 基础」改的就是它，判据在 internal/brand。 */
export function getSiteInfo() {
  return get<SiteInfo>('/site');
}
