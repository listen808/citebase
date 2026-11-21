<script setup lang="ts">
import { computed } from 'vue';
import { siteBrandName } from '@/utils/core/site-brand';
import { AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';

defineOptions({ name: 'ApBrand' });

/**
 * 全站品牌唯一源：一枚镜像对称的「摊开的书 + 中心脊上升 + 顶部节点」标记 + 可选字标。
 * 语义：书 = 知识/Wiki（两页对称摊开），脊上的一点 = 从知识里生长出的智能/AI。
 * 颜色走 token（默认 cv('blue')，accent 不随主题变），不硬编码。
 */
const props = withDefaults(defineProps<{
  /** 标记边长（px） */
  size?: number;
  showWordmark?: boolean;
  /** 字标文字。不传就用当前站点展示名（site-brand），别在这里再抄一份产品名 */
  wordmark?: string;
  color?: string;
  /**
   * 入场编排（批4）。这个类只带来 CSS 动画，而动画整体挂在
   * `:root[data-persona='signature']` 下 —— 所以传 true 也不代表一定动：
   * 版面人格是 plain 时这里就是静置。默认 false，顶栏 / 分享页脚 / favicon 零影响。
   */
  enter?: boolean;
}>(), {
  size: 28,
  showWordmark: true,
  wordmark: '',
  enter: false,
});

const brandWordmark = computed(() => props.wordmark || siteBrandName.value);

const markColor = computed(() => props.color ?? cv('blue'));
const accessibleName = computed(() => (props.showWordmark ? undefined : brandWordmark.value));

const wordmarkStyle = {
  fontFamily: AppleType.bodyEmphasis.font,
  fontSize: AppleType.bodyEmphasis.size,
  fontWeight: String(AppleType.bodyEmphasis.weight),
  letterSpacing: AppleType.bodyEmphasis.letterSpacing,
  color: cv('text-primary'),
};
</script>

<template>
  <span
    :class="props.enter ? 'ap-brand-enter' : undefined"
    :style="{ display: 'inline-flex', alignItems: 'center', gap: AppleSpace.sm }"
  >
    <svg
      :width="size"
      :height="size"
      viewBox="0 0 24 24"
      fill="none"
      :role="accessibleName ? 'img' : undefined"
      :aria-label="accessibleName"
      :aria-hidden="accessibleName ? undefined : 'true'"
    >
      <!-- 左页 / 右页：绕 x=12 镜像对称，外高内低 = 摊开的书 -->
      <path d="M11.2 12.6 L11.2 19.6 L3.4 17.6 L3.4 9.4 Z" :fill="markColor" />
      <path d="M12.8 12.6 L12.8 19.6 L20.6 17.6 L20.6 9.4 Z" :fill="markColor" />
      <!-- 中心脊：从书谷升起，托住节点 -->
      <path d="M12 12.6 L12 7.3" :stroke="markColor" stroke-width="1.7" stroke-linecap="round" />
      <!-- 节点：知识之上的一点智能（AI），实心圆，非箭头非火花 -->
      <circle cx="12" cy="5" r="2.3" :fill="markColor" data-brand-part="node" />
    </svg>
    <span v-if="showWordmark" :style="wordmarkStyle">{{ brandWordmark }}</span>
  </span>
</template>
