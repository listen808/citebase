<template>
  <span :style="containerStyle">
    <component :is="arrowIcon" :size="iconSize" :style="arrowStyle" aria-hidden="true" />
    <span v-if="value" :style="valueStyle">{{ value }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { ArrowUp, ArrowDown, ArrowRight } from 'lucide-vue-next';
import { AppleFont, AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';
import type { AppleControlSize } from '@/utils/core/apple-tokens';

const props = withDefaults(defineProps<{
  direction: 'up' | 'down' | 'neutral';
  value?: string;
  size?: Extract<AppleControlSize, 'small' | 'medium'>;
}>(), {
  size: 'medium',
})

const arrowIcon = computed(() =>
  props.direction === 'up' ? ArrowUp : props.direction === 'down' ? ArrowDown : ArrowRight,
)

const iconSize = computed(() => (props.size === 'small' ? 10 : 12))

const color = computed(() => {
  switch (props.direction) {
    case 'up':
      return cv('green')
    case 'down':
      return cv('red')
    default:
      return cv('text-tertiary')
  }
})

const containerStyle = computed(() => ({
  display: 'inline-flex',
  alignItems: 'center',
  gap: AppleSpace.xxs,
  color: color.value,
  fontFamily: AppleFont.text,
  fontSize: props.size === 'small' ? AppleType.legalMeta.size : AppleType.microUI.size,
  letterSpacing: props.size === 'small' ? AppleType.legalMeta.letterSpacing : AppleType.microUI.letterSpacing,
  fontWeight: String(AppleType.microUI.weight),
  lineHeight: '1',
}))

const arrowStyle = {
  display: 'inline-flex',
  flexShrink: '0' as const,
}

const valueStyle = {
  lineHeight: '1',
}
</script>
