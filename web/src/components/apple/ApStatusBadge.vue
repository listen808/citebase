<template>
  <div :style="badgeStyle">
    <span :style="dotStyle" />
    <span :style="labelStyle">{{ displayLabel }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { subtleBg, textOnTint, AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleType, cv, AppleStatusBadgeSize } from '@/utils/core/apple-tokens';

const props = withDefaults(defineProps<{
  status: 'online' | 'offline' | 'enabled' | 'disabled' | 'default' | 'warning' | 'error';
  size?: keyof typeof AppleStatusBadgeSize;
  label?: string;
}>(), {
  size: 'medium',
});

const statusColorMap: Record<string, string> = {
  online: cv('success'),
  offline: cv('text-tertiary'),
  enabled: cv('success'),
  disabled: cv('error'),
  default: cv('blue'),
  warning: cv('warning'),
  error: cv('error'),
};

const statusLabelMap: Record<string, string> = {
  online: '在线',
  offline: '离线',
  enabled: '启用',
  disabled: '禁用',
  default: '默认',
  warning: '警告',
  error: '错误',
};

const sizeConfigMap: Record<string, { padding: string; fontSize: string; dotSize: string; height: string }> = {
  small: {
    padding: `${AppleSpace.xxs} ${AppleSpace.sm}`,
    fontSize: AppleType.legalMeta.size,
    dotSize: AppleStatusBadgeSize.small.dot,
    height: AppleStatusBadgeSize.small.height,
  },
  medium: {
    padding: `${AppleSpace.xs} ${AppleSpace['12px']}`,
    fontSize: AppleType.microUI.size,
    dotSize: AppleStatusBadgeSize.medium.dot,
    height: AppleStatusBadgeSize.medium.height,
  },
  large: {
    padding: `${AppleSpace.xs} ${AppleSpace.md}`,
    fontSize: AppleType.controlLabel.size,
    dotSize: AppleStatusBadgeSize.large.dot,
    height: AppleStatusBadgeSize.large.height,
  },
};

const displayLabel = computed(() => props.label || statusLabelMap[props.status]);
const statusColor = computed(() => statusColorMap[props.status]);
const sizeConfig = computed(() => sizeConfigMap[props.size] || sizeConfigMap.medium);

const badgeStyle = computed(() => {
  const cfg = sizeConfig.value!;
  return {
    display: 'inline-flex',
    alignItems: 'center',
    gap: AppleSpace.xs,
    padding: cfg.padding,
    borderRadius: AppleRadius.control,
    background: subtleBg(statusColor.value ?? cv('blue'), 20, 'blue-bg-subtle'),
    border: `1px solid ${subtleBg(statusColor.value ?? cv('blue'), 40, 'blue-selection-strong')}`,
    height: cfg.height,
    fontFamily: AppleFont.text,
    fontSize: cfg.fontSize,
    fontWeight: String(AppleFontWeight.medium),
    color: textOnTint(statusColor.value ?? cv('blue')),
    lineHeight: 1,
  };
});

const dotStyle = computed(() => {
  const cfg = sizeConfig.value!;
  return {
    width: cfg.dotSize,
    height: cfg.dotSize,
    borderRadius: AppleRadius.circle,
    background: statusColor.value,
    flexShrink: 0,
  };
});

const labelStyle = computed(() => {
  const cfg = sizeConfig.value!;
  return {
    fontSize: cfg.fontSize,
  };
});
</script>
