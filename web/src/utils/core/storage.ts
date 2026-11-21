/**
 * LocalStorage 工具函数
 * 提供统一的本地存储API，支持JSON序列化和类型安全
 */

const PREFIX = 'citebase:';

/**
 * 2026-10-07 改名（AIWiki → Citebase）把这一层的前缀一起换了，而当时**没有做回落**：
 * 老用户浏览器里 `aiwiki:` 下的每一个键 —— 主题、材质、透明增强、侧栏折叠、卡片/列表档位、
 * 最近文档、搜索历史、展开状态、登录令牌 —— 全部变成孤儿，升级当天表现为"设置被清空"。
 * 后端同一个改名对做了双读（`envname.Getenv("CITEBASE_X","AIWIKI_X")`，见
 * docs/decision-log/2026-10-07-rename-scope.md），所以这里补的不是新功能，是那条既定口径的另外半边。
 *
 * 判据：只在**新键不存在**时采纳旧值（升级后又手动设过一次的偏好不能被旧值盖掉），
 * 采纳后立刻删旧键，因此这条回落对干净安装是零成本、对老用户是一次性。
 */
const LEGACY_PREFIX = 'aiwiki:';

function newKey(key: string): string {
  return PREFIX + key;
}

/**
 * 本模块**外面**直接读写 localStorage 的键（改名那次的机械替换按 `aiwiki`→`citebase`
 * 字符串形状扫过它们，所以它们同样换了名）。草稿是这里面最贵的一档：那是用户没保存的正文。
 *
 * 这几条是**等长前缀替换**（`aiwiki_` → `citebase_`），数组里的先后不影响结果 ——
 * 实测把短的那条排到前面，16 条测试全绿，时间键照样搬对。真正 load-bearing 的是
 * "两类草稿键都得列进来"：只列正文那条，时间戳会留在旧名下，读时按新名取不到时间，
 * 表现是草稿明明还在、却被判定成已过期。
 */
const LEGACY_RAW_KEYS: ReadonlyArray<readonly [string, string]> = [
  ['aiwiki_doc_draft_time_', 'citebase_doc_draft_time_'],
  ['aiwiki_doc_server_time_', 'citebase_doc_server_time_'],
  ['aiwiki_doc_draft_', 'citebase_doc_draft_'],
  ['aiwiki-sidebar-width', 'citebase-sidebar-width'],
];

/**
 * Safari 隐私模式与配额写满时 localStorage 会直接抛错，而 api/client.ts 的每个请求都要读 token。
 * 因此所有读写都过下面三个原语：异常时退到内存 Map，保证会话内功能不中断。
 */
const memory = new Map<string, string>();

function rawGet(key: string): string | null {
  try {
    const value = localStorage.getItem(key);
    if (value !== null) return value;
  } catch {
    /* 落到内存兜底 */
  }
  const cached = memory.get(key);
  return cached === undefined ? null : cached;
}

function rawSet(key: string, value: string): void {
  memory.set(key, value);
  try {
    localStorage.setItem(key, value);
  } catch {
    /* 隐私模式 / 配额已满：仅内存生效 */
  }
}

function rawRemove(key: string): void {
  memory.delete(key);
  try {
    localStorage.removeItem(key);
  } catch {
    /* 忽略 */
  }
}

/**
 * 获取存储值（字符串）
 * @param key 存储键
 * @returns 存储值或null
 */
export function storageGet(key: string): string | null {
  const value = rawGet(newKey(key));
  if (value !== null) return value;
  return adoptLegacy(key);
}

/**
 * 新键缺席时采纳改名前的值，采纳即删旧键。
 * 只读 localStorage、不读内存兜底：内存里不会有 `aiwiki:` 的键，读了等于凭空造值。
 */
function adoptLegacy(key: string): string | null {
  const legacy = LEGACY_PREFIX + key;
  let value: string | null;
  try {
    value = localStorage.getItem(legacy);
  } catch {
    return null; // 隐私模式：枚举不了就算了，不报错
  }
  if (value === null) return null;
  rawSet(newKey(key), value);
  rawRemove(legacy);
  return value;
}

/**
 * 把 LEGACY_RAW_KEYS 里登记的裸键搬到新名下。由 main.ts 在挂载前调用一次。
 * 返回搬动的条数 —— 调用方用它决定要不要打一条日志（0 条时不该刷屏）。
 */
export function migrateLegacyRawKeys(): number {
  let moved = 0;
  const names: string[] = [];
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i);
      if (k) names.push(k);
    }
  } catch {
    return 0;
  }
  for (const oldName of names) {
    const pair = LEGACY_RAW_KEYS.find(([from]) => oldName.startsWith(from));
    if (!pair) continue;
    const newName = pair[1] + oldName.slice(pair[0].length);
    let value: string | null;
    try {
      if (localStorage.getItem(newName) !== null) continue; // 新名已有值：不覆盖
      value = localStorage.getItem(oldName);
    } catch {
      continue;
    }
    if (value === null) continue;
    try {
      localStorage.setItem(newName, value);
      localStorage.removeItem(oldName);
      moved++;
    } catch {
      /* 配额满 / 隐私模式：这条留在旧名下，下次再搬 */
    }
  }
  return moved;
}

/**
 * 设置存储值（字符串）
 * @param key 存储键
 * @param value 存储值
 */
export function storageSet(key: string, value: string): void {
  rawSet(newKey(key), value);
}

/**
 * 移除存储值
 * @param key 存储键
 */
export function storageRemove(key: string): void {
  rawRemove(newKey(key));
}

/**
 * 获取存储值（JSON解析）
 * @param key 存储键
 * @param defaultValue 解析失败时的默认值
 * @returns 解析后的对象或默认值
 */
export function storageGetJSON<T>(key: string, defaultValue: T): T {
  const val = storageGet(key);
  if (val === null) return defaultValue;
  try {
    return JSON.parse(val) as T;
  } catch {
    return defaultValue;
  }
}

/**
 * 设置存储值（JSON序列化）
 * @param key 存储键
 * @param value 要存储的对象
 */
export function storageSetJSON(key: string, value: unknown): void {
  storageSet(key, JSON.stringify(value));
}

/**
 * 获取存储值（带默认值）
 * @param key 存储键
 * @param defaultValue 默认值
 * @returns 存储值或默认值
 */
export function storageGetWithDefault(key: string, defaultValue: string): string {
  return storageGet(key) || defaultValue;
}

/**
 * 获取存储值（数字）
 * @param key 存储键
 * @param defaultValue 默认值
 * @returns 解析后的数字或默认值
 */
export function storageGetNumber(key: string, defaultValue: number): number {
  const val = storageGet(key);
  if (val === null) return defaultValue;
  const num = Number(val);
  return isNaN(num) ? defaultValue : num;
}

/**
 * 设置存储值（数字）
 * @param key 存储键
 * @param value 数字值
 */
export function storageSetNumber(key: string, value: number): void {
  storageSet(key, String(value));
}

/**
 * 获取存储值（布尔）
 * @param key 存储键
 * @param defaultValue 默认值
 * @returns 解析后的布尔值或默认值
 */
export function storageGetBoolean(key: string, defaultValue: boolean): boolean {
  const val = storageGet(key);
  if (val === null) return defaultValue;
  return val === 'true';
}

/**
 * 设置存储值（布尔）
 * @param key 存储键
 * @param value 布尔值
 */
export function storageSetBoolean(key: string, value: boolean): void {
  storageSet(key, value ? 'true' : 'false');
}

/**
 * 检查键是否存在
 * @param key 存储键
 * @returns 是否存在
 */
export function storageHas(key: string): boolean {
  return storageGet(key) !== null;
}

/**
 * 清除所有应用存储（带前缀的键）
 */
export function storageClearAll(): void {
  const keysToRemove: string[] = [];
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      // 旧前缀一起清：只清新前缀的话，下一次 storageGet 会把 aiwiki: 的值又采纳回来，
      // "清除所有应用存储"于是变成一次刷新就撤销的操作。
      if (key && (key.startsWith(PREFIX) || key.startsWith(LEGACY_PREFIX))) {
        keysToRemove.push(key);
      }
    }
  } catch {
    /* 隐私模式下无法枚举，只清内存副本 */
  }
  keysToRemove.forEach((key) => rawRemove(key));
  for (const key of [...memory.keys()]) {
    if (key.startsWith(PREFIX) || key.startsWith(LEGACY_PREFIX)) memory.delete(key);
  }
}

export const StorageKeys = {
  AUTH_TOKEN: 'auth:token',
  AUTH_REFRESH: 'auth:refresh',
  AUTH_EXPIRES_AT: 'auth:expires_at',
  AUTH_REMEMBER_ME: 'auth:remember_me',
  UI_SIDEBAR_COLLAPSED: 'ui:sidebar-collapsed',
  UI_THEME: 'ui:theme',
  UI_THEME_PREF: 'ui:theme-pref',
  /** 材质轴：solid | glass（与外观主题正交，玻璃仅浅/深色可用） */
  UI_MATERIAL: 'ui:material',
  UI_GLASS_CLEAR: 'ui:glass-clear',
  /**
   * 动效强度档：auto | full | soft | reduced（真相源 `styles/theme/personalization.ts`）。
   * `auto` 才是默认值 —— 没表过态的人继续尊重系统的「减弱动态效果」。
   */
  UI_MOTION: 'ui:motion',
  /** 对比度档：auto | standard | more —— 与系统 prefers-contrast 同源但可显式选择 */
  UI_CONTRAST: 'ui:contrast',
  /** 透明度档：auto | full | reduce —— 与系统 prefers-reduced-transparency 同源 */
  UI_TRANSPARENCY: 'ui:transparency',
  /** 强调色档：auto | blue | purple | …（清单与 hex 的真相源 `styles/theme/accents.ts`） */
  UI_ACCENT: 'ui:accent',
  /** 版面人格：plain（默认，静置）| signature（公开版面入场编排）。射程由设计教条 G34 钉住 */
  UI_PERSONA: 'ui:persona',
  /** 集合页「卡片 / 列表」档位：全站一个键，切页与刷新都记住 */
  UI_COLLECTION_VIEW_MODE: 'ui:collection-view-mode',
  /**
   * 这台设备最后一次"知道自己与账号上的偏好一致"的时刻（Unix 秒）。
   * 跨设备同步靠它和服务器那一份的 updated_at 比大小 —— 没有它，
   * 采纳服务器值之后会立刻判定"本地更新"再 PUT 回去，每次登录一趟回声。
   */
  UI_PREF_SYNCED_AT: 'ui:pref-synced-at',
  /**
   * 上一次推送账号失败（离线、401、限流）。它的作用是让"本地优先"这件事**不依赖时钟**：
   * 客户端与服务器的钟不一致时，纯比时间戳会把用户刚改的那一次判给服务器盖掉，
   * 而"我在这台设备上刚改过"是个事实，不是一个可以推算的时刻。
   */
  UI_PREF_PENDING: 'ui:pref-pending',
  HISTORY_STACK: 'history:stack',
  SEARCH_HISTORY: 'search:history',
  SEARCH_SPOTLIGHT_HISTORY: 'search:spotlight_history',
  RECENT_DOCS: 'recent-docs:list',
  EXPANDED_NODES: 'expanded-nodes:map',
  CLIPBOARD: 'clipboard:data',
  EDITOR_SKIP_CHANGELOG: 'editor:skip_changelog',
  READER_WIDE_MODE: 'reader:wide-mode',
} as const;
