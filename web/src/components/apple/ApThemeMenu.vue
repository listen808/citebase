<template>
  <div ref="wrapRef" :style="wrapStyle">
    <ApButton
      variant="ghost"
      :size="size"
      icon-only
      :style="triggerGlassStyle"
      :aria-label="$t('chrome.switchTheme')"
      aria-haspopup="menu"
      :aria-expanded="open"
      @click="open = !open"
    >
      <!-- 触发器用单枚 lucide 图标（与顶栏设置/铃铛同一语言：裸图标 + 按钮自带的控件档底），
           不付色块缩略图——那枚 20px 圆角方块压在按钮自带的 28px 底上就是「框中框」，
           与隔壁图标钮不一致。四套外观各自认得的符号：太阳/月亮/报纸/翻开的书。 -->
      <component :is="triggerIcon" :size="16" :style="{ color: cv('text-secondary') }" />
    </ApButton>

    <transition name="apple-dropdown">
      <div v-if="open" class="apple-glass-layer" :style="menuStyle" role="menu" :aria-label="$t('chrome.themeMenu')">
        <!-- 跟随系统：只有这个入口能进 auto 档。偏好页有它而菜单没有 = 从导航栏切换过一次，
             就再也回不到"跟着系统走"，而 ui.ts 里那条 matchMedia 监听还在等这个偏好。 -->
        <span
          :style="[itemStyle, ui.themePreference === 'auto' ? itemActiveStyle : null]"
          role="menuitemradio"
          :aria-checked="ui.themePreference === 'auto'"
          @click="pickAuto"
        >
          <span :style="itemLabelStyle">
            <span>{{ $t('chrome.themeAuto') }}</span>
          </span>
          <ApIcon v-if="ui.themePreference === 'auto'" name="Check" :size="12" />
        </span>
        <span
          v-for="t in THEME_REGISTRY"
          :key="t.id"
          :style="[itemStyle, ui.theme === t.id ? itemActiveStyle : null]"
          role="menuitemradio"
          :aria-checked="ui.theme === t.id"
          @click="pick(t.id)"
        >
          <span :style="itemLabelStyle">
            <span :style="swatchStyle(t.id)" />
            <span>{{ $t(THEME_LABEL_KEYS[t.id]) }}</span>
          </span>
          <ApIcon v-if="ui.theme === t.id" name="Check" :size="12" />
        </span>

        <template v-if="supportsGlass(ui.theme)">
          <div :style="dividerStyle" />
          <div :style="sectionTitleStyle">{{ $t('chrome.materialSection') }}</div>
          <span
            v-for="m in MATERIAL_PICKS"
            :key="m.id"
            :style="[itemStyle, ui.material === m.id ? itemActiveStyle : null]"
            role="menuitemradio"
            :aria-checked="ui.material === m.id"
            @click="pickMaterial(m.id)"
          >
            <span :style="itemLabelStyle">
              <span :style="materialSwatchStyle(m.id)" />
              <span>{{ $t(m.labelKey) }}</span>
            </span>
            <ApIcon v-if="ui.material === m.id" name="Check" :size="12" />
          </span>
        </template>
        <!-- 纸感/杂志没有材质可选：不藏整段，留一句说明 —— 否则这里的菜单比浅/深色少一截，
             用户只会以为材质开关丢了（做减法要交代去处）。 -->
        <template v-else>
          <div :style="dividerStyle" />
          <div :style="materialNoteStyle">{{ $t('chrome.materialUnavailable') }}</div>
        </template>

        <template v-if="supportsGlassClear(ui.theme, ui.material)">
          <div :style="dividerStyle" />
          <div :style="itemStyle" role="menuitemcheckbox" :aria-checked="ui.glassClear" @click="ui.toggleGlassClear()">
            <ApCheckbox :model-value="ui.glassClear" :label="$t('chrome.glassClear')" />
          </div>
        </template>

        <!-- 版面人格不在这里：它是「登录/安装/分享页的入场编排」这一个开关，
             应用内切换看不到任何效果，摆在快切菜单里只会换来"这是什么、为什么没反应"
             （2026-10-25 用户在登录页连着三次问同一句，且这一行把窄菜单撑出横向滚动条）。
             唯一入口回到 设置 → 偏好（PreferenceSection），那里有配套说明。 -->
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { computed, shallowRef } from 'vue';
import ApButton from './ApButton.vue';
import ApIcon from './ApIcon.vue';
import ApCheckbox from './ApCheckbox.vue';
import { useUIStore } from '@/stores/app/ui';
import { supportsGlass, supportsGlassClear, THEME_REGISTRY, themeSwatch, type Material, type ThemeId } from '@/styles/theme/themes';
import { useDismiss } from '@/composables/ui/useClickOutsideClose';
import {
  AppleFontWeight,
  AppleGlass,
  AppleRadius,
  AppleSpace,
  AppleType,
  cssVariables,
  cv,
  menuSurfaceStyle,
} from '@/utils/core/apple-tokens';
import { withAlpha } from '@/utils/core/shared';
import { Sun, Moon, Newspaper, BookOpen } from 'lucide-vue-next';
import type { AppleButtonSizeName } from '@/utils/core/apple-tokens';

/** 主题缩略图：主流做法是直接放主题的真实截图（GitHub、VS Code 外观菜单同款），
 *  而不是自己画的符号。图片资源住在 assets 根（见 assets/README.md 的分层约定）。 */
import iconLight from '@/assets/theme-icons/light.png';
import iconDark from '@/assets/theme-icons/dark.png';
import iconEditorial from '@/assets/theme-icons/editorial.png';
import iconWarmPaper from '@/assets/theme-icons/warm-paper.png';

const props = withDefaults(defineProps<{
  size?: Extract<AppleButtonSizeName, 'mini' | 'small'>;
  /** 浮在环境渐变上（登录页/作者登录页）时为 true：触发器付半透明浮层底 + 一层磨砂。
   *  顶栏那处**不许**传 —— 它压在 .apple-glass-nav 的玻璃上，再付模糊就是「玻璃压玻璃」。 */
  glass?: boolean;
}>(), { size: 'mini', glass: false });

const THEME_ICONS: Record<ThemeId, string> = {
  light: iconLight,
  dark: iconDark,
  editorial: iconEditorial,
  'warm-paper': iconWarmPaper,
};

// 玻璃触发器：macOS 顶栏图标按钮的语言 —— 半透明白 + 磨砂，身后是整页渐变环境时才看得见效果。
// 2026-10-26：流水面按钮的模糊被收敛收掉了（同屏层数红线 ≤3），但浮在环境上、无壳面承载的
// 触发器属于真浮层语言，模糊可见且同屏只此一层。
const triggerGlassStyle = computed(() => props.glass ? {
  // 控件玻璃档（与顶栏图标钮、secondary 按钮同一套：高光 + 发丝边 + 内棱线），
  // 底色走次级面而不是浮层档 —— 它是一颗按钮，不是一块浮层。
  background: cv('bg-secondary'),
  ...AppleGlass.control,
  backdropFilter: cv('blur-panel'),
  WebkitBackdropFilter: cv('blur-panel'),
} : undefined);

const ui = useUIStore();
const open = shallowRef(false);
const wrapRef = shallowRef<HTMLElement | null>(null);

useDismiss(open, () => [wrapRef.value]);

/** 材质选项：玻璃只在浅/深色可用，故整段仅在 supportsGlass 时渲染 */
const MATERIAL_PICKS: { id: Material; labelKey: string }[] = [
  { id: 'solid', labelKey: 'chrome.materialSolid' },
  { id: 'glass', labelKey: 'chrome.materialGlass' },
];

/**
 * 外观名走语言包。这里按 id 建表而不是改 THEME_REGISTRY 的 label：
 * 注册表那份 label + desc 还被偏好设置页与主题预览拿去用，那是另一批的射程，
 * 一批只推一个域，否则一次改动会把三套设计门禁同时拖进未知状态。
 */
const THEME_LABEL_KEYS: Record<ThemeId, string> = {
  light: 'chrome.themeLight',
  dark: 'chrome.themeDark',
  editorial: 'chrome.themeEditorial',
  'warm-paper': 'chrome.themeWarmPaper',
};

function pick(id: string) {
  ui.setTheme(id as never);
  open.value = false;
}

/** 选"跟随系统"：把偏好交还给 matchMedia，而不是把当前解析结果钉死 */
function pickAuto() {
  ui.setThemePreference('auto');
  open.value = false;
}

function pickMaterial(m: Material) {
  ui.setMaterial(m);
}

/**
 * 四枚主题缩略图 = 各主题的真实截图（主流做法，GitHub / VS Code 外观菜单同款）。
 * 为什么不用自画符号：小尺寸下任何抽象画法（太阳、窗口骨架、色块）都会被否定——
 * 截图不存在歧义，四张图天然就是「这个主题长什么样」。
 * 图片由 scripts/gen-theme-icons.mjs 从同一份主题数据渲染（见文件头注释），
 * 颜色与 themes.ts 单一来源，不会分叉。
 */
function swatchStyle(id: string) {
  const img = THEME_ICONS[id as ThemeId];
  const s = themeSwatch(id as never, cssVariables, supportsGlass(id) ? ui.material : 'solid');
  const paper = s.surface === 'transparent' ? s.bg : s.surface;
  return {
    width: '1.25rem',
    height: '1.25rem',
    flexShrink: '0',
    display: 'inline-block',
    borderRadius: '0.25rem',
    overflow: 'hidden',
    background: paper,
    border: `0.0625rem solid ${withAlpha(s.text, 0.16)}`,
    backgroundImage: `url(${img})`,
    backgroundSize: 'cover',
    backgroundPosition: 'center',
  };
}

/** 触发器图标 = 当前主题的 lucide 符号（与顶栏设置/铃铛同一语言：裸图标 + 控件档底） */
const THEME_TRIGGER_ICONS: Record<ThemeId, typeof Sun> = {
  light: Sun,
  dark: Moon,
  editorial: Newspaper,
  'warm-paper': BookOpen,
};
const triggerIcon = computed(() => THEME_TRIGGER_ICONS[ui.theme]);

/** 材质缩略块：与主题名片同尺寸同圆角。实心=纯填充；玻璃=同一强调色的白色高光（不是第二种色相，也不是空心圈） */
function materialSwatchStyle(m: Material) {
  const accent = cv('blue');
  const base: Record<string, string> = {
    width: '1.125rem',
    height: '1.125rem',
    flexShrink: '0',
    display: 'inline-block',
    position: 'relative',
    borderRadius: AppleRadius['3px'],
    border: `0.0625rem solid ${accent}`,
  };
  return m === 'glass'
    ? { ...base, background: `linear-gradient(135deg, ${withAlpha(cv('white'), 0.32)} 0%, transparent 60%), ${accent}` }
    : { ...base, background: accent };
}

const wrapStyle = { position: 'relative' as const, display: 'inline-flex' };

const menuStyle = menuSurfaceStyle({
  position: 'absolute' as const,
  top: '100%',
  right: '0',
  minWidth: '9.25rem',
  padding: `${AppleSpace.xs} 0`,
  marginTop: AppleSpace.xs,
  overflow: 'hidden' as const,
});

const itemStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  gap: AppleSpace.sm,
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-primary'),
  cursor: 'pointer',
  borderRadius: AppleRadius.small,
};

const itemLabelStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  minWidth: '9.5rem',
};

const itemActiveStyle = {
  color: cv('blue-text'),
  fontWeight: String(AppleFontWeight.semibold),
  background: cv('blue-selection'),
};

const dividerStyle = {
  height: '0.0625rem',
  margin: `${AppleSpace.xxs} 0`,
  background: cv('border-light'),
};

const sectionTitleStyle = {
  padding: `${AppleSpace.xxs} ${AppleSpace.md}`,
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

/** 材质不可用时的说明行：与分区标题同字号，但多一行高度留给换行 */
const materialNoteStyle = {
  ...sectionTitleStyle,
  padding: `${AppleSpace.xxs} ${AppleSpace.md} ${AppleSpace.sm}`,
  lineHeight: '1.35',
  color: cv('text-quaternary'),
};
</script>
