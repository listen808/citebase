<template>
  <div :style="containerStyle">
    <!-- 入场编排（批4）：错误页允许（低频、且动效在这里是在交代"发生了什么"） -->
    <div :style="contentStyle" class="apple-persona-stack">
      <div :style="iconBgStyle">
        <!-- 整页错误的图标由调用方传组件（不是注册表名字），所以拿不到 ApIcon 的强制点，
             这里显式解一次描边：40px 不写细就是 3.3px 的粗线 -->
        <component :is="icon" :size="40" :stroke-width="iconStrokeFor(40)" :style="{ color: cv(iconColor) }" />
      </div>
      <p :style="statusCodeStyle">{{ statusCode }}</p>
      <h1 :style="titleStyle" data-ap-page-title>{{ title }}</h1>
      <p :style="descriptionStyle">{{ description }}</p>
      <div :style="actionsStyle">
        <ApButton variant="secondary" size="small" @click="onSecondaryAction">{{ secondaryLabel }}</ApButton>
        <ApButton variant="primary" size="small" @click="onPrimaryAction">{{ primaryLabel }}</ApButton>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Component } from 'vue';
import { computed } from 'vue';
import { AppleRadius, AppleSpace, AppleType, cv, AppleLineHeight, iconStrokeFor, type AppleCssVarName } from '@/utils/core/apple-tokens';
import ApButton from '@/components/apple/ApButton.vue';

const props = defineProps<{
  icon: Component;
  iconColor: 'blue' | 'red';
  statusCode: string;
  title: string;
  description: string;
  primaryLabel: string;
  secondaryLabel: string;
  onPrimaryAction: () => void;
  onSecondaryAction: () => void;
}>();

const iconBgStyle = computed(() => ({
  width: '72px',
  height: '72px',
  borderRadius: AppleRadius['24px'],
  background: cv(`${props.iconColor}-light` as AppleCssVarName),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  margin: `0 auto ${AppleSpace.lg}`,
}));

const containerStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  flex: '1',
  minHeight: '0',
  background: cv('bg-secondary'),
  padding: `${AppleSpace.xl} ${AppleSpace.md}`,
};

const contentStyle = {
  textAlign: 'center' as const,
  maxWidth: '480px',
  width: '100%',
};

const statusCodeStyle = {
  fontFamily: AppleType.heroDisplayXL.font,
  fontSize: AppleType.heroDisplayXL.size,
  fontWeight: String(AppleType.heroDisplayXL.weight),
  color: cv(props.iconColor),
  margin: '0',
  lineHeight: String(AppleType.heroDisplayXL.lineHeight),
};

const titleStyle = {
  fontFamily: AppleType.sectionDisplay.font,
  fontSize: AppleType.sectionDisplay.size,
  fontWeight: String(AppleType.sectionDisplay.weight),
  color: cv('text-primary'),
  margin: `${AppleSpace.md} 0 ${AppleSpace.md}`,
  lineHeight: String(AppleType.sectionDisplay.lineHeight),
};

const descriptionStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-secondary'),
  margin: `0 0 ${AppleSpace.xl}`,
  lineHeight: AppleLineHeight.spacious,
};

const actionsStyle = {
  display: 'flex',
  gap: AppleSpace.md,
  justifyContent: 'center',
  flexWrap: 'wrap' as const,
};
</script>