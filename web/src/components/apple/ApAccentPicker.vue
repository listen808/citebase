<template>
  <div role="radiogroup" :aria-label="$t('chrome.accentLabel')" :style="rowStyle">
    <button
      v-for="opt in options"
      :key="opt.id"
      type="button"
      role="radio"
      :aria-checked="modelValue === opt.id"
      :aria-label="opt.label"
      :title="opt.label"
      class="apple-focus-ring"
      :style="[swatchStyle(opt), activeStyle(opt.id)]"
      @click="emit('update:modelValue', opt.id)"
    >
      <ApIcon v-if="modelValue === opt.id" name="Check" :size="16" />
    </button>
  </div>
</template>

<script setup lang="ts">
/**
 * 强调色选块（个性化批3）。
 *
 * 为什么单独成组件而不是在两处各写一排色块：`ApThemeMenu` 与偏好设置页都要这一条能力，
 * 而"两处不齐平"在这个仓库里翻过车（`clear` 档曾只有导航菜单能开，偏好页看不到也关不掉）。
 * 一份实现、两处消费，是这个仓库对"同一个概念"的固定解法。
 *
 * 色值一律从 `styles/theme/accents.ts` 取，**不在这里画第二套色板** ——
 * 门禁 `check:contrast` 判的是那份注册表里的 hex，这里抄一份就会"看到的不等于验过的"。
 * `auto` 那一枚用当前外观 × 材质实际解析出的强调色（themeSwatch），所以它不是灰扑扑的占位色，
 * 而是"选了它你会得到什么"的真实预览。
 */
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import ApIcon from './ApIcon.vue';
import { ACCENT_REGISTRY, ACCENT_AUTO, type AccentId } from '@/styles/theme/accents';
import { themeSwatch, type Material, type ThemeId } from '@/styles/theme/themes';
import { AppleRadius, AppleSpace, cssVariables, cv } from '@/utils/core/apple-tokens';

const props = defineProps<{
  modelValue: AccentId;
  /** 预览 auto 那一枚用哪个外观与材质（不传就退成浅色实心） */
  theme: ThemeId;
  material: Material;
  isDark: boolean;
}>();

const emit = defineEmits<{ (e: 'update:modelValue', value: AccentId): void }>();

const { t } = useI18n();

const options = computed(() => [
  { id: ACCENT_AUTO as AccentId, label: t('chrome.accentAuto'), hex: '', auto: true },
  ...ACCENT_REGISTRY.map((a) => ({
    id: a.id as AccentId,
    label: t(a.labelKey),
    hex: props.isDark ? a.dark : a.light,
    auto: false,
  })),
]);

/**
 * 尺寸取 28px：里面的勾必须是 16px（I5 统一按钮内图标尺寸），24px 的盘装不下它——
 * 宁可让这一排宽一点，也不为了塞进去把图标缩到判据之外的尺寸。
 * 但触控目标靠 `padding` + `AppleLayout.touchTargetMin` 撑到 44 ——
 * 色块这类"必须一眼看完一排"的控件最容易做成一按就误触旁边那颗。
 */
const rowStyle = computed(() => ({
  display: 'flex' as const,
  alignItems: 'center',
  gap: AppleSpace.xs,
  flexWrap: 'wrap' as const,
}));

const swatchStyle = (opt: { hex: string; auto: boolean }) => {
  const auto = opt.auto
    ? themeSwatch(props.theme, cssVariables, props.material).accent
    : opt.hex;
  return {
    display: 'inline-flex' as const,
    alignItems: 'center',
    justifyContent: 'center',
    width: '1.75rem',
    height: '1.75rem',
    minWidth: '1.75rem',
    padding: 0,
    borderRadius: AppleRadius.circle,
    background: auto,
    border: `0.0625rem solid ${cv('border-light')}`,
    cursor: 'pointer',
    color: cv('on-accent'),
    // 时长走令牌（G32）：这一排在「柔和/减弱」档下也必须跟着变慢/停下
    transition: `transform var(--apple-dur-fast) var(--apple-easing-standard), box-shadow var(--apple-dur-fast) var(--apple-easing-standard)`,
  };
};

/** 选中态：**同一套语言** —— 加一圈 2px 的强调色外环（不是实心色块反白，也不是勾选改变大小） */
const activeStyle = (id: AccentId) =>
  props.modelValue === id
    ? { boxShadow: `0 0 0 0.125rem ${cv('surface-panel')}, 0 0 0 0.25rem ${cv('blue')}` }
    : {};
</script>
