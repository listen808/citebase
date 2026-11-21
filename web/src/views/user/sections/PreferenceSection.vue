<template>
  <div :style="preferencesFormStyle">
      <!-- 同步状态没跟上时唯一可见的地方：写库失败按设计不弹 toast（换个主题不该被网络状态打断），
          但"设置页永远看着像已保存"会让"偏好跟账号走"这件事在用户眼里变成一句谎。
          缺列、离线、401、限流四类原因都汇到这一行，具体原因看服务端日志的 [migrate] 与 [auth] 行。 -->
      <div v-if="ui.prefSyncPending" :style="pendingRowStyle" role="status">
        {{ $t('chrome.prefSyncPending') }}
      </div>
      <div :style="preferenceItemStyle">
        <div :style="preferenceInfoStyle">
          <div :style="preferenceLabelStyle">{{ $t('user.k36') }}</div>
          <div :style="preferenceDescStyle">{{ $t('user.k37') }}</div>
        </div>
        <ApSwitch
          :model-value="!ui.sidebarCollapsed"
          @update:model-value="handleSidebarToggle"
        />
      </div>
      <div :style="preferenceItemStyle">
        <div :style="preferenceInfoStyle">
          <div :style="preferenceLabelStyle">{{ $t('user.k38') }}</div>
          <div :style="preferenceDescStyle">{{ $t('user.k39') }}</div>
        </div>
        <ApSelect
          :model-value="ui.themePreference"
          :options="themeOptions"
          @update:model-value="handleThemeChange"
        />
      </div>
      <div v-if="supportsGlass(ui.theme)" :style="preferenceItemStyle">
        <div :style="preferenceInfoStyle">
          <div :style="preferenceLabelStyle">{{ $t('user.k40') }}</div>
          <div :style="preferenceDescStyle">{{ $t('user.k41') }}</div>
        </div>
        <ApSelect
          :model-value="ui.material"
          :options="materialOptions"
          @update:model-value="handleMaterialChange"
        />
      </div>
      <!-- 纸感/杂志不支持玻璃：这一行不藏起来，改成说明 —— 与上面行为三档同一条规矩，
           藏掉的设置项会被读成"功能坏了/没了"。 -->
      <div v-else :style="preferenceItemStyle">
        <div :style="preferenceInfoStyle">
          <div :style="preferenceLabelStyle">{{ $t('user.k40') }}</div>
          <div :style="preferenceDescStyle">{{ $t('chrome.materialUnavailable') }}</div>
        </div>
      </div>
      <!-- 透明增强（clear）：以前只有导航栏那个菜单能开这一档，偏好页没有 ——
           于是"设置里能配的主题"和"顶栏上能配的主题"是两套能力，
           而从偏好页永远看不到、也关不掉这一档。 -->
      <div v-if="supportsGlassClear(ui.theme, ui.material)" :style="preferenceItemStyle">
        <div :style="preferenceInfoStyle">
          <div :style="preferenceLabelStyle">{{ $t('chrome.glassClear') }}</div>
          <div :style="preferenceDescStyle">{{ $t('chrome.glassClearDesc') }}</div>
        </div>
        <ApSwitch
          :model-value="ui.glassClear"
          @update:model-value="ui.toggleGlassClear()"
        />
      </div>
      <!-- 行为三档（个性化批1/批2）：与外观 × 材质正交，所以三行都无条件渲染 ——
           纸感外观下透明度那一档不会改变画面（它本来就是实心），这一点由文案交代，
           而不是把行藏起来：藏起来的用户会以为"透明度设置在纸感下坏了"。 -->
      <div :style="preferenceItemStyle">
        <div :style="preferenceInfoStyle">
          <div :style="preferenceLabelStyle">{{ $t('chrome.motionLabel') }}</div>
          <div :style="preferenceDescStyle">{{ $t('chrome.motionDesc') }}{{ motionHint }}</div>
        </div>
        <ApSelect
          :model-value="ui.motion"
          :options="motionOptions"
          @update:model-value="handleMotionChange"
        />
      </div>
      <div :style="preferenceItemStyle">
        <div :style="preferenceInfoStyle">
          <div :style="preferenceLabelStyle">{{ $t('chrome.contrastLabel') }}</div>
          <div :style="preferenceDescStyle">{{ $t('chrome.contrastDesc') }}</div>
        </div>
        <ApSelect
          :model-value="ui.contrast"
          :options="contrastOptions"
          @update:model-value="handleContrastChange"
        />
      </div>
      <div :style="preferenceItemStyle">
        <div :style="preferenceInfoStyle">
          <div :style="preferenceLabelStyle">{{ $t('chrome.transparencyLabel') }}</div>
          <div :style="preferenceDescStyle">{{ $t('chrome.transparencyDesc') }}</div>
        </div>
        <ApSelect
          :model-value="ui.transparency"
          :options="transparencyOptions"
          @update:model-value="handleTransparencyChange"
        />
      </div>
      <div :style="preferenceItemStyle">
        <div :style="preferenceInfoStyle">
          <div :style="preferenceLabelStyle">{{ $t('chrome.personaLabel') }}</div>
          <div :style="preferenceDescStyle">{{ $t('chrome.personaDesc') }}</div>
        </div>
        <ApSelect
          :model-value="ui.persona"
          :options="personaOptions"
          @update:model-value="handlePersonaChange"
        />
      </div>
      <div :style="preferenceItemStyle">
        <div :style="preferenceInfoStyle">
          <div :style="preferenceLabelStyle">{{ $t('chrome.accentLabel') }}</div>
          <div :style="preferenceDescStyle">{{ $t('chrome.accentDesc') }}</div>
        </div>
        <!-- 与顶栏那个菜单同一份实现：色板只有一处定义，两处都只是消费 -->
        <ApAccentPicker
          :model-value="ui.accent"
          :theme="ui.theme"
          :material="ui.material"
          :is-dark="isDarkThemeId(ui.theme)"
          @update:model-value="ui.setAccent($event)"
        />
      </div>
      <div :style="preferenceItemStyle">
        <div :style="preferenceInfoStyle">
          <div :style="preferenceLabelStyle">{{ $t('user.k42') }}</div>
          <div :style="preferenceDescStyle">{{ $t('user.k43') }}</div>
        </div>
        <ApSelect
          :model-value="locale"
          :options="languageOptions"
          @update:model-value="handleLanguageChange"
        />
      </div>
    </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { computed } from 'vue';
import { useUIStore } from '@/stores/app/ui';
import { useLocale } from '@/composables/ui/useLocale';
import { isThemePreference, isMaterial, isDarkThemeId, supportsGlass, supportsGlassClear, THEME_REGISTRY, type ThemePreference, type Material } from '@/styles/theme/themes';
import {
  CONTRAST_MODES,
  MOTION_MODES,
  PERSONA_MODES,
  TRANSPARENCY_MODES,
  isContrastMode,
  isMotionMode,
  isPersonaMode,
  isTransparencyMode,
  type ContrastMode,
  type MotionMode,
  type PersonaMode,
  type TransparencyMode,
} from '@/styles/theme/personalization';
import { AppleFontWeight, AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';
import ApSwitch from '@/components/apple/ApSwitch.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApAccentPicker from '@/components/apple/ApAccentPicker.vue';

const ui = useUIStore();
const { locale, setLocale } = useLocale();

/** 主题选项 = THEME_REGISTRY 派生，不在这里维护第二份主题清单 */
const themeOptions = computed(() => [
  { label: t('user.k44'), value: 'auto' },
  ...THEME_REGISTRY.map((t) => ({ label: `${t.label} · ${t.desc}`, value: t.id })),
]);

/** 材质选项：与外观正交，玻璃仅浅/深色可用（该行仅在 supportsGlass 时渲染） */
const materialOptions = [
  { label: t('user.k45'), value: 'solid' },
  { label: t('user.k46'), value: 'glass' },
];

const languageOptions = [
  { label: t('user.k47'), value: 'zh-CN' },
  { label: 'English', value: 'en-US' },
];

/**
 * 行为三档（个性化批1/批2）。清单一律从 `styles/theme/personalization.ts` 派生 ——
 * 在这里再写一遍 `'auto','full','soft','reduced'` 的话，加一档时这一行会变成一个静默的漏登记点
 * （`check:theme` 的 T15 管的是注册表自洽，管不住这里的第二份名单）。
 */
const behaviorLabels: Record<string, () => string> = {
  auto: () => t('chrome.motionAuto'),
  full: () => t('chrome.motionFull'),
  soft: () => t('chrome.motionSoft'),
  reduced: () => t('chrome.motionReduced'),
  standard: () => t('chrome.contrastStandard'),
  more: () => t('chrome.contrastMore'),
  reduce: () => t('chrome.transparencyReduce'),
};
const behaviorOptions = (modes: readonly string[]) =>
  modes.map((value) => ({ label: behaviorLabels[value]?.() ?? value, value }));
const motionOptions = computed(() => behaviorOptions(MOTION_MODES));
const contrastOptions = computed(() => behaviorOptions(CONTRAST_MODES));
const transparencyOptions = computed(() => behaviorOptions(TRANSPARENCY_MODES));
// persona 没有 auto（系统里没有“要不要编排”这一档），默认 plain
const behaviorLabels2: Record<string, () => string> = {
  plain: () => t('chrome.personaPlain'),
  signature: () => t('chrome.personaSignature'),
};
const personaOptions = computed(() => PERSONA_MODES.map((value) => ({ label: behaviorLabels2[value]?.() ?? value, value })));

/** auto 档必须看得见"现在实际是哪一档"：否则这一行显示的是意图而不是状态 */
const motionHint = computed(() => {
  if (ui.motion === 'auto') {
    const eff = ui.effectivePersonalization().motion;
    return ` · ${t('chrome.motionEffective')}: ${behaviorLabels[eff]?.() ?? eff}`;
  }
  return '';
});

function handleMotionChange(v: string | number | null) {
  const mode = String(v) as MotionMode;
  if (!isMotionMode(mode)) return;
  ui.setMotionMode(mode);
}
function handleContrastChange(v: string | number | null) {
  const mode = String(v) as ContrastMode;
  if (!isContrastMode(mode)) return;
  ui.setContrastMode(mode);
}
function handleTransparencyChange(v: string | number | null) {
  const mode = String(v) as TransparencyMode;
  if (!isTransparencyMode(mode)) return;
  ui.setTransparencyMode(mode);
}
function handlePersonaChange(v: string | number | null) {
  const mode = String(v) as PersonaMode;
  if (!isPersonaMode(mode)) return;
  ui.setPersona(mode);
}

function handleSidebarToggle(checked: boolean) {
  ui.sidebarCollapsed = !checked;
}

function handleThemeChange(pref: string | number | null) {
  const p = String(pref) as ThemePreference;
  if (!isThemePreference(p)) return;
  ui.setThemePreference(p);
  ui.showToast(t('user.k48'), 'success');
}

function handleMaterialChange(m: string | number | null) {
  const v = String(m) as Material;
  if (!isMaterial(v)) return;
  ui.setMaterial(v);
  ui.showToast(t('user.k49'), 'success');
}

function handleLanguageChange(lang: string | number | null) {
  if (lang !== 'zh-CN' && lang !== 'en-US') return;
  setLocale(lang);
  ui.showToast(t('user.k50'), 'success');
}

const pendingRowStyle = {
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  borderRadius: 'var(--apple-radius-panel)' as const,
  background: cv('orange-bg-subtle'),
  color: cv('orange-text'),
  fontSize: AppleType.controlLabel.size,
};
const preferencesFormStyle = {
  display: 'flex' as const,
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
};
const preferenceItemStyle = {
  display: 'flex' as const,
  justifyContent: 'space-between' as const,
  alignItems: 'center',
  paddingBottom: AppleSpace.md,
  borderBottom: `1px solid ${cv('border-light')}`,
};
const preferenceInfoStyle = {
  display: 'flex' as const,
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
};
const preferenceLabelStyle = {
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
};
const preferenceDescStyle = {
  fontSize: AppleType.controlLabel.size,
  color: cv('text-secondary'),
};
</script>
