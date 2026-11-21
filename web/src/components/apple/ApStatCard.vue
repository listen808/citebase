<template>
  <div :style="cardStyle">
    <div :style="layoutStyle">
      <div v-if="icon" :style="iconBoxStyle">
      <ApIcon v-if="isStringIcon" :name="icon as IconName" :size="sizeConfig.iconSize" :color="resolvedColor" />
      <component :is="icon" v-else :size="sizeConfig.iconSize" :color="resolvedColor" :stroke-width="iconStrokeFor(sizeConfig.iconSize)" />
    </div>
      <div :style="bodyStyle">
        <div :style="labelStyle" :title="label">{{ label }}</div>
        <div :style="rowStyle">
          <div :style="valueStyle" :title="String(value)">{{ value }}</div>
          <ApTrendIndicator
            v-if="showTrendIndicator"
            :direction="trendDirection!"
            :value="trendValue"
            size="small"
          />
          <ApTrendIndicator
            v-else-if="trend !== undefined"
            :direction="trend > 0 ? 'up' : trend < 0 ? 'down' : 'neutral'"
            :value="`${trend > 0 ? '+' : ''}${trend.toFixed(1)}%`"
            size="small"
          />
        </div>
      </div>
    </div>
    <div v-if="$slots.sparkline" :style="sparklineContainerStyle">
      <slot name="sparkline" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { Component } from 'vue';
import { subtleBg, AppleRadius, AppleSpace, AppleTransition, AppleType, cv, AppleLayout, AppleIconSize, APPLE_ICON_TINT_ALPHA, iconStrokeFor } from '@/utils/core/apple-tokens';
import type { AppleDensity } from '@/utils/core/apple-tokens';
import ApTrendIndicator from './ApTrendIndicator.vue';
import ApIcon from './ApIcon.vue';
import type { IconName } from '@/utils/icons';

const props = withDefaults(defineProps<{
  label: string;
  value: number | string;
  icon?: Component | IconName;
  color?: string;
  trend?: number;
  theme?: string;
  trendDirection?: 'up' | 'down' | 'neutral';
  trendValue?: string;
  iconBg?: boolean;
  size?: AppleDensity | 'mini';
}>(), {
  color: cv('blue'),
  // 浅染底默认开：以前默认关，于是全站统计卡都是「36px 空盒里浮着一个 16px 图标」——
  // 盒子只占地、不表态，而同一页的 ApEntityCard 却带 32px 浅染底，同一个想法两种画法。
  // 两种收口里选「底默认开」：图标与文字的坐标一格没动（只多一层 APPLE_ICON_TINT_ALPHA 的浅染），
  // 另一条「盒子贴住图标」会把整列文字左移 9~18px，改动反而更大。实测全站无一处显式传 iconBg，
  // 所以这条默认值就是唯一在屏上的形态；确需无底的调用点显式传 false。
  iconBg: true,
  size: 'default',
});

const isStringIcon = computed(() => typeof props.icon === 'string');

const resolvedColor = computed(() => {
  return props.color;
});

const showTrendIndicator = computed(() => props.trendDirection !== undefined);

const sizeConfig = computed(() => {
  // size 只决定密度，不决定字号：同一个指标在 mini/compact/default 下必须是同一字号
  // 图标尺寸同理只走 AppleIconSize 两档（原先 14/16/18 三个数：14 是 16 的复制品，已并档）
  switch (props.size) {
    case 'mini':
      return {
        gap: AppleSpace.xs,
        padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
        iconSize: AppleIconSize.control,
        iconBox: AppleLayout.iconBoxSmall,
        bodyGap: '0',
        rowGap: AppleSpace.xxs,
      };
    case 'compact':
      return {
        gap: AppleSpace.sm,
        padding: `${AppleSpace.sm} ${AppleSpace.sm}`,
        iconSize: AppleIconSize.control,
        iconBox: AppleLayout.iconBoxMedium,
        bodyGap: AppleSpace.xxs,
        rowGap: AppleSpace.xs,
      };
    default:
      return {
        gap: AppleSpace.sm,
        padding: `${AppleSpace.md} ${AppleSpace.md}`,
        iconSize: AppleIconSize.stat,
        iconBox: AppleLayout.iconBoxMedium,
        bodyGap: AppleSpace.xxs,
        rowGap: AppleSpace.xs,
      };
  }
});

const cardStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  alignItems: 'stretch',
  background: cv('surface-panel'),
  borderRadius: AppleRadius.card,
  // 静置只有一根发丝线：阴影留给浮层与 hover
  border: `1px solid ${cv('border-light')}`,
  // 上缘一像素高光：卡片在暗底上第一次有"板立着"的物理线索。
  // 这里不接 shadow-card —— 那一档带 32px 外投影，会顺带改掉浅外观的观感；
  // 这一档只有内高光，在四套外观下都只是把上缘收亮一点。
  boxShadow: cv('shadow-level1'),
  transition: AppleTransition.smooth,
  // 统计卡同为流水面（同屏按指标数放大），不付模糊 —— 同 ApCard，红线 ≤3。
  overflow: 'hidden' as const,
};

const layoutStyle = computed(() => ({
  display: 'flex',
  alignItems: 'center',
  gap: sizeConfig.value.gap,
  padding: sizeConfig.value.padding,
}));

const iconBoxStyle = computed(() => ({
  width: sizeConfig.value.iconBox,
  height: sizeConfig.value.iconBox,
  borderRadius: AppleRadius.panel,
  // alpha 走 APPLE_ICON_TINT_ALPHA（原先这里 10、page-styles 的图标底 12，同一个想法两种灰）
  background: props.iconBg ? subtleBg(props.color, APPLE_ICON_TINT_ALPHA, 'bg-tertiary') : 'transparent',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  flexShrink: 0,
}));

const bodyStyle = computed(() => ({
  display: 'flex',
  flexDirection: 'column' as const,
  gap: sizeConfig.value.bodyGap,
  flex: 1,
  minWidth: 0,
}));

const labelStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  whiteSpace: 'nowrap' as const,
  overflow: 'hidden' as const,
  textOverflow: 'ellipsis' as const,
};

const rowStyle = computed(() => ({
  display: 'flex',
  alignItems: 'baseline',
  gap: sizeConfig.value.rowGap,
}));

const valueStyle = {
  fontFamily: AppleType.statValue.font,
  fontSize: AppleType.statValue.size,
  fontWeight: String(AppleType.statValue.weight),
  lineHeight: AppleType.statValue.lineHeight,
  letterSpacing: AppleType.statValue.letterSpacing,
  color: cv('text-primary'),
  // 数字等宽，纵向排列表格化对齐
  fontVariantNumeric: 'tabular-nums',
};

const sparklineContainerStyle = {
  marginTop: AppleSpace.sm,
  opacity: '0.7',
  padding: `0 ${AppleSpace.md} ${AppleSpace.md}`,
};
</script>
