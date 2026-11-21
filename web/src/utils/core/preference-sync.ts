/**
 * 个性化偏好的跨设备同步规则（外观 × 材质 × 透明变体）。
 *
 * 为什么单独成模块：这条链上最容易写错的不是"怎么发请求"，而是**谁赢**。
 * 直觉的两种做法都是错的：
 *  - "服务器永远赢" —— 用户在没网的那台机器上换了主题，下次登录被三年前的选择盖回去；
 *  - "本地永远赢" —— 换设备/清缓存的人每次都要重新调一遍，等于没同步。
 * 所以判据是**时间戳比大小**，而且那个时间戳由服务端盖写（客户端的时钟不归它管）。
 *
 * 本地时间戳住 `ui:pref-synced-at`：它记的是"这台设备最后一次知道自己与账号一致的时刻"，
 * 采纳服务器值时写它、推送成功时也写它 —— 少了这一步，采纳后立刻会判定"本地更新"再推回去，
 * 每次登录一趟无意义的 PUT（回声）。
 */
import { storageGet, storageSet, StorageKeys } from './storage';
import type { Material, ThemePreference } from '@/styles/theme/themes';
import type { ContrastMode, MotionMode, PersonaMode, TransparencyMode } from '@/styles/theme/personalization';
import { isContrastMode, isMotionMode, isPersonaMode, isTransparencyMode } from '@/styles/theme/personalization';
import type { AccentId } from '@/styles/theme/accents';
import { isAccentId } from '@/styles/theme/accents';

/** 与后端 `model/user.Preferences` 逐字段对齐（snake_case 是接口形状，不是前端命名习惯） */
export interface ServerPreferences {
  theme: ThemePreference;
  material: Material;
  glass_clear: boolean;
  /** 2026-10-16 批2/批3 加的档：动效强度 / 对比度 / 透明度 / 强调色 */
  motion: MotionMode;
  contrast: ContrastMode;
  transparency: TransparencyMode;
  accent: AccentId;
  /** 批4：版面人格（公开版面的入场编排），没有 auto —— 系统里没有对应的开关 */
  persona: PersonaMode;
  updated_at: number;
}

/** 账号上这一行没写的新维度，用这台设备当前的值补 —— 见 parseServerPreferences 的理由 */
export type LocalAxes = Pick<ServerPreferences, 'motion' | 'contrast' | 'transparency' | 'accent' | 'persona'>;

export type SyncDecision =
  | 'none'   // 账号上没配过，本地也没写过：什么都不做（新用户不该被写入一份"偏好"）
  | 'seed'   // 账号上没配过，本地有：把本地这份推上去，让这台设备的选择变成账号的选择
  | 'adopt'  // 服务器更新：照它切
  | 'push'   // 本地更新：推上去
  | 'keep';  // 一致（或服务器值不可用）：什么都不做

const THEME_VALUES: readonly string[] = ['auto', 'light', 'dark', 'editorial', 'warm-paper'];
const MATERIAL_VALUES: readonly string[] = ['solid', 'glass'];

/**
 * 校验来自接口的形状。**不认识的取值一律当"账号上没配过"**而不是回退默认值 ——
 * 后端清单与前端的 THEME_REGISTRY 万一分叉（有一档外观只在其中一边登记），
 * 回退默认值会静默改掉用户的界面，而当没配过只是不同步。后者是可逆的。
 *
 * **新维度是例外，而且要留在这里**：2026-10-16 之前落库的那些行里根本没有
 * `motion / contrast / transparency` 三格。把它们当"非法"整份判 null 的后果是
 * **升级之后连主题都不同步了**（老数据行看起来像坏数据）。所以：
 * 外观/材质/透明变体认不得 → 整份判 null（这三条是同一年代一起写的）；
 * 新维度缺失或认不得 → 用 `local` 那台设备的值补上，其余三条照常采纳。
 */
export function parseServerPreferences(raw: unknown, local?: Partial<LocalAxes>): ServerPreferences | null {
  if (!raw || typeof raw !== 'object') return null;
  const o = raw as Record<string, unknown>;
  if (typeof o.theme !== 'string' || !THEME_VALUES.includes(o.theme)) return null;
  if (typeof o.material !== 'string' || !MATERIAL_VALUES.includes(o.material)) return null;
  if (typeof o.glass_clear !== 'boolean') return null;
  const stamp = typeof o.updated_at === 'number' && Number.isFinite(o.updated_at) ? o.updated_at : 0;
  return {
    theme: o.theme as ThemePreference,
    material: o.material as Material,
    glass_clear: o.glass_clear,
    motion: isMotionMode(o.motion) ? o.motion : (local?.motion ?? 'auto'),
    contrast: isContrastMode(o.contrast) ? o.contrast : (local?.contrast ?? 'auto'),
    transparency: isTransparencyMode(o.transparency) ? o.transparency : (local?.transparency ?? 'auto'),
    // 强调色的合法清单直接取自 accents.ts（不再在这里抄第二份名单）：
    // 色板加一档而这里没改，后果不是"用不了"，是这个账号的选择被判成非法值丢掉。
    accent: isAccentId(o.accent) ? o.accent : (local?.accent ?? 'auto'),
    persona: isPersonaMode(o.persona) ? o.persona : (local?.persona ?? 'plain'),
    updated_at: stamp,
  };
}

export function readLocalSyncStamp(): number {
  const raw = storageGet(StorageKeys.UI_PREF_SYNCED_AT);
  const n = raw === null ? Number.NaN : Number(raw);
  return Number.isFinite(n) ? n : 0;
}

export function writeLocalSyncStamp(stamp: number): void {
  storageSet(StorageKeys.UI_PREF_SYNCED_AT, String(stamp));
}

/**
 * 纯判据：给定服务器值与本地时间戳，决定这一趟怎么办。
 * 相等时选 `keep` 而不是 `push` —— 相等意味着这就是我们刚同步的那一份。
 */
export function decideSync(server: ServerPreferences | null, localStamp: number, localHasOwnChoice: boolean): SyncDecision {
  if (!server) return localHasOwnChoice ? 'seed' : 'none';
  if (server.updated_at > localStamp) return 'adopt';
  if (localStamp > server.updated_at) return 'push';
  return 'keep';
}

/**
 * 把当前 store 状态压成 PUT 请求体。`updated_at` 不给：那是服务端盖的。
 * 参数用对象而不是六个位置参数 —— 加到第五档时 `('auto','glass',true,'soft')` 这种调用
 * 已经分不清谁是谁了，而这类顺序错乱在类型上是合法的（都是 string）。
 */
export function toPayload(input: {
  theme: ThemePreference;
  material: Material;
  glassClear: boolean;
  motion: MotionMode;
  contrast: ContrastMode;
  transparency: TransparencyMode;
  accent: AccentId;
  persona: PersonaMode;
}): Omit<ServerPreferences, 'updated_at'> {
  // 透明变体依附玻璃：solid 下永远是 false，与后端 NormalizePreferences 同一条规则。
  // 两边各写一遍是因为这条规则一旦分叉，症状是"另一台设备上 clear 开关的状态莫名其妙"。
  return {
    theme: input.theme,
    material: input.material,
    glass_clear: input.material === 'glass' ? input.glassClear : false,
    motion: input.motion,
    contrast: input.contrast,
    transparency: input.transparency,
    accent: input.accent,
    persona: input.persona,
  };
}
