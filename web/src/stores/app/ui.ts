import { defineStore } from 'pinia';
import { ref, shallowRef } from 'vue';
import { storageGet, storageSet, storageRemove, StorageKeys } from '@/utils/core/storage';
import {
  decideSync,
  parseServerPreferences,
  readLocalSyncStamp,
  toPayload,
  writeLocalSyncStamp,
} from '@/utils/core/preference-sync';
import {
  isThemeId,
  isThemePreference,
  isMaterial,
  isDarkThemeId,
  supportsGlass,
  supportsGlassClear,
  LEGACY_THEME_ALIAS,
  type ThemeId,
  type ThemePreference,
  type Material,
} from '@/styles/theme/themes';
import {
  CONTRAST_MODES,
  DEFAULT_CONTRAST_MODE,
  DEFAULT_MOTION_MODE,
  DEFAULT_TRANSPARENCY_MODE,
  MOTION_MODES,
  TRANSPARENCY_MODES,
  applyPersonalizationAttrs,
  isContrastMode,
  isMotionMode,
  isTransparencyMode,
  resolveContrastMode,
  resolveMotionMode,
  resolveTransparencyMode,
  DEFAULT_PERSONA_MODE,
  isPersonaMode,
  PERSONA_MODES,
  type ContrastMode,
  type MotionMode,
  type PersonaMode,
  type TransparencyMode,
} from '@/styles/theme/personalization';
import {
  ACCENT_IDS,
  DEFAULT_ACCENT,
  applyAccent,
  isAccentId,
  type AccentId,
} from '@/styles/theme/accents';

export type { ThemeId, Material };
export type { AccentId, ContrastMode, MotionMode, TransparencyMode };

/**
 * 全新访客（localStorage 一个键都没写过）的默认档：浅色 + 液态玻璃 + 透明增强。
 * 判据由 web/tests/theme-default.test.ts 钉住。
 *
 * 这三行是默认值的**唯一落点**：ref 初始化与 initTheme 的兜底分支都引用它们。
 * 默认值原先在两个地方各写一遍（ref('auto') 和 initTheme 里那句 else 'auto'），
 * 只改一处界面照样好看，但换台机器或系统深色时第一眼就不是那个东西了。
 */
export const DEFAULT_THEME_PREFERENCE: ThemePreference = 'light';
export const DEFAULT_MATERIAL: Material = 'glass';
export const DEFAULT_GLASS_CLEAR = true;
/**
 * 行为档（动效强度 / 对比度 / 透明度）的默认值：**三条全是 `auto`**。
 * 与外观/材质刻意不同 —— 那两条是"审美选择"，可以给定一个招牌档；
 * 这三条默认必须是"这台机器没表过态"，也就是继续由系统信号决定，
 * 否则一次升级就把系统开了「减弱动态效果」的用户推到 full 上。
 */
export const DEFAULT_MOTION: MotionMode = DEFAULT_MOTION_MODE;
export const DEFAULT_CONTRAST: ContrastMode = DEFAULT_CONTRAST_MODE;
export const DEFAULT_TRANSPARENCY: TransparencyMode = DEFAULT_TRANSPARENCY_MODE;
/** 版面人格默认 plain：新访客第一眼不许替所有人决定要不要编排（理由见 PERSONA_MODES 注释） */
export const DEFAULT_PERSONA: PersonaMode = DEFAULT_PERSONA_MODE;

/** 偏好 → 实际主题：'auto' 由系统深浅色决定 */
function resolveTheme(pref: ThemePreference): ThemeId {
  if (pref !== 'auto') return pref;
  const prefersDark = typeof window !== 'undefined' && window.matchMedia
    ? window.matchMedia('(prefers-color-scheme: dark)').matches
    : false;
  return prefersDark ? 'dark' : 'light';
}

/**
 * 玻璃 Clear 档的读法：**只有「这个键从没写过」才给默认值**。
 * 写过的一律按存的值来，包括 'false' —— 默认值一旦开始盖掉用户关掉它的那次选择就是回归。
 */
function readGlassClearPref(): boolean {
  const stored = storageGet(StorageKeys.UI_GLASS_CLEAR);
  return stored === null ? DEFAULT_GLASS_CLEAR : stored === 'true';
}

/**
 * 行为档的读法与 glass-clear 同一条口径：**只有"这个键从没写过"才给默认值**。
 * 认不得的存量值（清单改过名、手工改过 localStorage）一律回默认档，
 * 但**不许回成"生效档"** —— 默认档是 `auto`，也就是把判断权交回系统信号。
 */
function readModePref<T extends string>(key: string, valid: readonly T[], fallback: T): T {
  const stored = storageGet(key);
  return stored !== null && (valid as readonly string[]).includes(stored) ? (stored as T) : fallback;
}

/**
 * 把历史存储里的主题字符串解成 { 外观 base, 材质 material }。
 * 老的独立主题 liquid-glass 会解成 base=light + material=glass，避免升级后丢主题。
 */
function normalizeStoredTheme(raw: string | null): { base: ThemeId | null; material: Material | null } {
  if (!raw) return { base: null, material: null };
  const alias = LEGACY_THEME_ALIAS[raw];
  if (alias) return { base: alias.base, material: alias.material };
  if (isThemeId(raw)) return { base: raw, material: null };
  return { base: null, material: null };
}

export const useUIStore = defineStore('ui', () => {
  const sidebarCollapsed = shallowRef(false);
  const theme = ref<ThemeId>(resolveTheme(DEFAULT_THEME_PREFERENCE));
  /** 用户偏好：'auto' 跟随系统，或锁定某一主题（默认档见 DEFAULT_THEME_PREFERENCE） */
  const themePreference = ref<ThemePreference>(DEFAULT_THEME_PREFERENCE);
  /** 材质轴：与外观正交。glass 仅在浅/深色下真正生效（见 effectiveMaterial） */
  const material = ref<Material>(DEFAULT_MATERIAL);
  /** 玻璃 Clear 变体：更透一档，仅 material=glass 且外观支持玻璃时生效 */
  const glassClear = ref(readGlassClearPref());
  /** 行为三档（与外观/材质正交）：动效强度 / 对比度 / 透明度。默认全 auto = 由系统信号决定 */
  const motion = ref<MotionMode>(DEFAULT_MOTION);
  const contrast = ref<ContrastMode>(DEFAULT_CONTRAST);
  const transparency = ref<TransparencyMode>(DEFAULT_TRANSPARENCY);
  /** 强调色档（批3）：auto = 用每套外观自己设计的那枚；选定的那一档会覆写 --apple-blue */
  const accent = ref<AccentId>(DEFAULT_ACCENT);
  /** 版面人格（批4）：默认 plain = 今天的静置版 */
  const persona = ref<PersonaMode>(DEFAULT_PERSONA);

  // 支持多个 toast 堆叠
  interface ToastItem {
    id: number;
    message: string;
    type: 'success' | 'error' | 'warning' | 'info';
  }
  const toasts = ref<ToastItem[]>([]);
  let toastIdCounter = 0;

  const globalLoading = shallowRef(false);
  let _requestCount = 0;
  let _loadingTimeout: ReturnType<typeof setTimeout> | null = null;
  const LOADING_TIMEOUT_MS = 30000; // 30秒超时

  function startLoading() {
    _requestCount++;
    globalLoading.value = true;
    if (_loadingTimeout) clearTimeout(_loadingTimeout);
    _loadingTimeout = setTimeout(() => {
      _requestCount = 0;
      globalLoading.value = false;
    }, LOADING_TIMEOUT_MS);
  }

  function stopLoading() {
    _requestCount = Math.max(0, _requestCount - 1);
    if (_requestCount === 0) {
      globalLoading.value = false;
      if (_loadingTimeout) {
        clearTimeout(_loadingTimeout);
        _loadingTimeout = null;
      }
    }
  }

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value;
    storageSet(StorageKeys.UI_SIDEBAR_COLLAPSED, String(sidebarCollapsed.value));
  }

  function initSidebarState() {
    const saved = storageGet(StorageKeys.UI_SIDEBAR_COLLAPSED);
    if (saved !== null) {
      sidebarCollapsed.value = saved === 'true';
    }
  }

  /** 只应用主题（DOM + 派生 storage），不改用户偏好：启动同步与跟随系统走这里 */
  function applyTheme(t: ThemeId) {
    theme.value = t;
    document.documentElement.setAttribute('data-theme', t);
    storageSet(StorageKeys.UI_THEME, t);
    applyMaterialAttr();
    // 强调色的取值分浅/深两列，所以**换外观必须重算覆写**：
    // 只在 setAccent 里应用的话，从浅色切到深色还带着浅色那一枚 hex（对照度门禁是绿的，对屏幕是错的）。
    applyAccentAttr();
  }

  /** 用户显式选定某一主题 = 锁定偏好，不再跟随系统 */
  function setTheme(t: ThemeId) {
    if (!isThemeId(t)) return;
    setThemePreference(t);
  }

  function setThemePreference(pref: ThemePreference) {
    if (!isThemePreference(pref)) return;
    themePreference.value = pref;
    storageSet(StorageKeys.UI_THEME_PREF, pref);
    applyTheme(resolveTheme(pref));
    schedulePreferencePush();
    // 不在这里调用 clearRenderCache()，避免闪屏。
    // ApMarkdownPreview 的 MutationObserver 会监听 data-theme 变化并处理缓存。
  }

  /** 启动时调用（挂载前，避免 FOUC）：读偏好 → 应用 → 挂上系统深浅色监听 */
  function initTheme() {
    const savedPref = storageGet(StorageKeys.UI_THEME_PREF);
    const savedTheme = storageGet(StorageKeys.UI_THEME);
    const savedMaterial = storageGet(StorageKeys.UI_MATERIAL);

    // --- 材质轴：优先新键；否则从老的 liquid-glass 主题里迁移出来 ---
    // 三个分支都不命中 = 谁都没写过，此时**故意不改 material.value**，让它保留上面 ref 的默认档。
    // 别在这里补一个 solid 兜底：那会把默认值藏到第二个地方，改默认的人只会看到 ref()。
    const prefNorm = normalizeStoredTheme(savedPref);
    const themeNorm = normalizeStoredTheme(savedTheme);
    if (isMaterial(savedMaterial)) {
      material.value = savedMaterial;
    } else if (prefNorm.material) {
      material.value = prefNorm.material;
    } else if (themeNorm.material) {
      material.value = themeNorm.material;
    }

    // --- 外观偏好：显式 'auto' > 新 base > 迁移出的 base > 回退默认浅色 ---
    if (savedPref === 'auto') {
      themePreference.value = 'auto';
    } else if (prefNorm.base) {
      themePreference.value = prefNorm.base;
    } else if (themeNorm.base) {
      themePreference.value = themeNorm.base;
    } else {
      themePreference.value = DEFAULT_THEME_PREFERENCE;
    }

    applyTheme(resolveTheme(themePreference.value));
    // 把迁移后的规范值写回，清掉历史里的 liquid-glass 遗留键
    storageSet(StorageKeys.UI_THEME_PREF, themePreference.value);
    storageSet(StorageKeys.UI_MATERIAL, material.value);

    if (typeof window === 'undefined' || !window.matchMedia) return;
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (themePreference.value === 'auto') applyTheme(resolveTheme('auto'));
    });
  }

  // ---- 材质轴：solid / glass（glass 仅浅/深色可用），及玻璃 Clear 变体 ----
  /** 材质在当前外观下真正生效的值：纸感主题没有玻璃，回落 solid */
  function effectiveMaterial(base: ThemeId): Material {
    return material.value === 'glass' && supportsGlass(base) ? 'glass' : 'solid';
  }
  function applyMaterialAttr() {
    const eff = effectiveMaterial(theme.value);
    document.documentElement.setAttribute('data-material', eff);
    if (supportsGlassClear(theme.value, eff) && glassClear.value) {
      document.documentElement.setAttribute('data-glass', 'clear');
    } else {
      document.documentElement.removeAttribute('data-glass');
    }
  }
  /** 用户切换材质：锁定偏好并立即生效（外观不动） */
  function setMaterial(m: Material) {
    if (!isMaterial(m)) return;
    material.value = m;
    storageSet(StorageKeys.UI_MATERIAL, m);
    applyMaterialAttr();
    schedulePreferencePush();
  }
  function toggleGlassClear() {
    glassClear.value = !glassClear.value;
    storageSet(StorageKeys.UI_GLASS_CLEAR, String(glassClear.value));
    applyMaterialAttr();
    schedulePreferencePush();
  }

  // ---- 行为三档：动效强度 / 对比度 / 透明度（与外观 × 材质正交）------------------------
  /**
   * 当前**生效**的动效档（偏好 + 系统信号的解算结果）。
   * 单独留一份响应式值，而不是让组件各自去读 DOM 属性：`data-motion` 是非响应式的，
   * `ApMorphIcon` 这类要在档位变化后立刻重算 props 的组件读它不会更新
   * —— 症状是"改了动效档，图标形变还按旧档跑，刷新页面才变"。
   */
  const motionEffective = shallowRef<Exclude<MotionMode, 'auto'>>(resolveMotionMode(DEFAULT_MOTION));
  /** 把三档偏好解成生效值并写 DOM。任何一处改了档位都要过这里，别在组件里自己 setAttribute */
  function applyPersonalizationAttr() {
    const eff = {
      motion: resolveMotionMode(motion.value),
      contrast: resolveContrastMode(contrast.value),
      transparency: resolveTransparencyMode(transparency.value),
      persona: persona.value,
    };
    motionEffective.value = eff.motion;
    applyPersonalizationAttrs(eff);
  }
  /** 当前真正生效的三档（界面要显示"现在是什么"，而不只是"选了 auto"） */
  function effectivePersonalization() {
    return {
      motion: resolveMotionMode(motion.value),
      contrast: resolveContrastMode(contrast.value),
      transparency: resolveTransparencyMode(transparency.value),
      persona: persona.value,
    };
  }
  /**
   * 三档 setter 长得一样，但**不许合并成一个函数**：每档的合法清单与存储键不同，
   * 合并的代价是把"选错清单"变成"写错键"——后者是可逆的，前者会把非法值推上账号。
   */
  function setMotionMode(m: MotionMode) {
    if (!isMotionMode(m)) return;
    motion.value = m;
    storageSet(StorageKeys.UI_MOTION, m);
    applyPersonalizationAttr();
    schedulePreferencePush();
  }
  function setContrastMode(c: ContrastMode) {
    if (!isContrastMode(c)) return;
    contrast.value = c;
    storageSet(StorageKeys.UI_CONTRAST, c);
    applyPersonalizationAttr();
    schedulePreferencePush();
  }
  function setTransparencyMode(t: TransparencyMode) {
    if (!isTransparencyMode(t)) return;
    transparency.value = t;
    storageSet(StorageKeys.UI_TRANSPARENCY, t);
    applyPersonalizationAttr();
    // 系统那边要求减弱透明度时，用户如果显式选了 full，材质层那一档要看得见地跟着改
    schedulePreferencePush();
  }

  // ---- 强调色（批3）：覆写 --apple-blue 一枚令牌，整条染色族自动跟随 --------------------
  /** 按当前外观的明暗档应用强调色覆写（auto = 撤掉覆写，把颜色交回外观自己的调色板） */
  function applyAccentAttr() {
    applyAccent(accent.value, isDarkThemeId(theme.value) ? 'dark' : 'light');
  }
  /** 版面人格：只有公开版面读它（射程由设计教条 G34 钉住），产品内页拿到这个档也不会多出任何动效 */
  function setPersona(p: PersonaMode) {
    if (!isPersonaMode(p)) return;
    persona.value = p;
    storageSet(StorageKeys.UI_PERSONA, p);
    applyPersonalizationAttr();
    schedulePreferencePush();
  }
  function setAccent(id: AccentId) {
    if (!isAccentId(id)) return;
    accent.value = id;
    storageSet(StorageKeys.UI_ACCENT, id);
    applyAccentAttr();
    schedulePreferencePush();
  }

  /**
   * 启动时调用（与 initTheme 同一批，挂载前）：读三档 → 应用 → 挂系统信号监听。
   * 监听只在**该档是 auto** 时才动作 —— 用户显式选过的那一次表态不能被系统开关盖掉，
   * 这条与外观的 `auto` 监听是同一个口径。
   */
  function initPersonalization() {
    motion.value = readModePref(StorageKeys.UI_MOTION, MOTION_MODES, DEFAULT_MOTION);
    contrast.value = readModePref(StorageKeys.UI_CONTRAST, CONTRAST_MODES, DEFAULT_CONTRAST);
    transparency.value = readModePref(StorageKeys.UI_TRANSPARENCY, TRANSPARENCY_MODES, DEFAULT_TRANSPARENCY);
    accent.value = readModePref(StorageKeys.UI_ACCENT, ACCENT_IDS, DEFAULT_ACCENT);
    persona.value = readModePref(StorageKeys.UI_PERSONA, PERSONA_MODES, DEFAULT_PERSONA);
    applyPersonalizationAttr();
    applyAccentAttr();
    storageSet(StorageKeys.UI_MOTION, motion.value);
    storageSet(StorageKeys.UI_CONTRAST, contrast.value);
    storageSet(StorageKeys.UI_TRANSPARENCY, transparency.value);
    storageSet(StorageKeys.UI_ACCENT, accent.value);
    storageSet(StorageKeys.UI_PERSONA, persona.value);

    if (typeof window === 'undefined' || !window.matchMedia) return;
    const watch = (query: string, onSignal: (matches: boolean) => void) => {
      try {
        window.matchMedia(query).addEventListener('change', (e) => onSignal(e.matches));
      } catch {
        // Safari < 14 只有 addListener；这里没有备用方案是故意的 ——
        // 挂不上监听的代价是"这一台要到下次进应用才跟上系统开关"，而不是界面坏掉。
      }
    };
    watch('(prefers-reduced-motion: reduce)', () => {
      if (motion.value === 'auto') applyPersonalizationAttr();
    });
    watch('(prefers-contrast: more)', () => {
      if (contrast.value === 'auto') applyPersonalizationAttr();
    });
    watch('(prefers-reduced-transparency: reduce)', () => {
      if (transparency.value === 'auto') applyPersonalizationAttr();
    });
  }

  // ---- 偏好跟账号走（PUT /auth/me/preferences） ------------------------------------
  //
  // 三条规则，都是踩出来的：
  //  ① **本地改了就推**，不比较时间戳 —— 用户刚在这台设备上做的选择，这台就是权威；
  //     拿客户端时钟和服务器的 updated_at 比，钟慢的那台会把自己的新改动判给旧的服务器值。
  //  ② **推失败就登记 pending**，下次读取时优先把本地推上去（见 applyServerPreferences）。
  //  ③ 采纳服务器值期间**不再回推**（applyingFromServer），否则"采纳"这个动作本身
  //     会触发一次推送，把刚拿到的值原样发回去 —— 每次登录一趟无意义的写。
  let applyingFromServer = false;
  let pushTimer: ReturnType<typeof setTimeout> | null = null;
  /**
   * 「这次的改动还没同步到账号」是不是成立（响应式，供界面显示）。
   * 存的那个 `ui:pref-pending` 键是事实源（跨刷新要活下来），这一枚是它的镜像 ——
   * 之所以两边都要：读 localStorage 不是响应式的，界面不会在补推成功后自己变回"已同步"。
   *
   * 这条存在的理由是**症状不可见**：写库失败（列还没补上、离线、401、限流）按设计不该弹 toast，
   * 但"不弹提示"与"连设置页都永远显示得像已经同步了"是两件事。后者让
   * "换设备主题丢了"这件事除了日志以外没有任何地方能被发现。
   */
  const prefSyncPending = shallowRef(storageGet(StorageKeys.UI_PREF_PENDING) === '1');

  async function pushPreferences(): Promise<void> {
    // 没登录就不推：登录页也能换主题，那是本机选择，不是账号属性
    if (!storageGet(StorageKeys.AUTH_TOKEN)) return;
    try {
      const { updateOwnPreferences } = await import('@/api/modules/auth/auth');
      const info = await updateOwnPreferences(
        toPayload({
          theme: themePreference.value,
          material: material.value,
          glassClear: glassClear.value,
          motion: motion.value,
          contrast: contrast.value,
          transparency: transparency.value,
          accent: accent.value,
          persona: persona.value,
        }),
      );
      const stamp = info?.preferences?.updated_at;
      writeLocalSyncStamp(typeof stamp === 'number' && stamp > 0 ? stamp : Math.floor(Date.now() / 1000));
      storageRemove(StorageKeys.UI_PREF_PENDING);
      prefSyncPending.value = false;
    } catch {
      // 不 toast：换个主题不该被网络状态打断。留个 pending，下次读 /auth/me 时补推。
      storageSet(StorageKeys.UI_PREF_PENDING, '1');
      prefSyncPending.value = true;
    }
  }

  /** 连点主题菜单不该发五次 PUT —— 合并成一次 */
  function schedulePreferencePush(): void {
    if (applyingFromServer || typeof window === 'undefined') return;
    if (pushTimer !== null) window.clearTimeout(pushTimer);
    pushTimer = window.setTimeout(() => {
      pushTimer = null;
      void pushPreferences();
    }, 700);
  }

  /**
   * `/auth/me` 回来之后调用：按时间戳决定谁赢。
   * 判据是纯函数（`decideSync`），单测在 utils/core/preference-sync.test.ts ——
   * 这一层一旦写成"服务器永远赢"，用户离线时的选择就会在下次登录被静默吞掉。
   */
  function applyServerPreferences(raw: unknown): void {
    const server = parseServerPreferences(raw, {
      motion: motion.value,
      contrast: contrast.value,
      transparency: transparency.value,
      accent: accent.value,
      persona: persona.value,
    });
    if (storageGet(StorageKeys.UI_PREF_PENDING) === '1') {
      void pushPreferences();
      return;
    }
    const hasLocal = storageGet(StorageKeys.UI_THEME_PREF) !== null;
    const decision = decideSync(server, readLocalSyncStamp(), hasLocal);
    if (decision === 'push' || decision === 'seed') {
      void pushPreferences();
      return;
    }
    if (decision !== 'adopt' || !server) return;
    applyingFromServer = true;
    try {
      material.value = server.material;
      glassClear.value = server.glass_clear;
      storageSet(StorageKeys.UI_MATERIAL, server.material);
      storageSet(StorageKeys.UI_GLASS_CLEAR, String(server.glass_clear));
      // 行为三档跟过来：这三条在老账号那一行里可能压根没有，parse 会用本机的值补，
      // 所以这里"照单写回"不会把 auto 之外的值凭空造出来。
      motion.value = server.motion;
      contrast.value = server.contrast;
      transparency.value = server.transparency;
      accent.value = server.accent;
      persona.value = server.persona;
      storageSet(StorageKeys.UI_MOTION, server.motion);
      storageSet(StorageKeys.UI_CONTRAST, server.contrast);
      storageSet(StorageKeys.UI_TRANSPARENCY, server.transparency);
      storageSet(StorageKeys.UI_ACCENT, server.accent);
      storageSet(StorageKeys.UI_PERSONA, server.persona);
      applyPersonalizationAttr();
      // 强调色不在这里应用：下面的 setThemePreference 会走 applyTheme → applyAccentAttr。
      // **顺序不能反**：先定外观再算强调色的浅/深列，否则深色外观配的是浅色那枚 hex。
      writeLocalSyncStamp(server.updated_at);
      setThemePreference(server.theme); // 内部会 applyTheme → 写 DOM → 触发各渲染器重绘
    } finally {
      applyingFromServer = false;
    }
  }

  function showToast(message: string, type: 'success' | 'error' | 'warning' | 'info' = 'info', duration?: number) {
    const MAX_TOASTS = 3;
    if (toasts.value.length >= MAX_TOASTS) {
      toasts.value.shift();
    }
    
    const id = ++toastIdCounter;
    const toast: ToastItem = { id, message, type };
    toasts.value.push(toast);
    
    const defaultDuration = type === 'error' ? 4000 : type === 'warning' ? 3000 : 2000;
    const finalDuration = duration || defaultDuration;
    
    setTimeout(() => {
      removeToast(id);
    }, finalDuration);
  }
  
  function removeToast(id: number) {
    const index = toasts.value.findIndex(t => t.id === id);
    if (index > -1) {
      toasts.value.splice(index, 1);
    }
  }

  return {
    sidebarCollapsed,
    theme,
    themePreference,
    material,
    glassClear,
    motion,
    contrast,
    transparency,
    accent,
    persona,
    motionEffective,
    prefSyncPending,
    toasts,
    globalLoading,
    toggleSidebar,
    initSidebarState,
    setTheme,
    setThemePreference,
    setMaterial,
    applyTheme,
    initTheme,
    toggleGlassClear,
    initPersonalization,
    setMotionMode,
    setContrastMode,
    setTransparencyMode,
    applyPersonalizationAttr,
    effectivePersonalization,
    setAccent,
    setPersona,
    applyAccentAttr,
    applyServerPreferences,
    pushPreferences,
    showToast,
    removeToast,
    startLoading,
    stopLoading,
  };
});
