<template>
  <div :style="avatarStyle" :aria-label="ariaLabel || $t('fallback.k91')">
    <img v-if="src" :src="src" :alt="ariaLabel" :style="imgStyle" @error="fallback = true" />
    <span v-else-if="text" :style="textStyle">{{ text }}</span>
    <span v-else :style="iconStyle">
      <User :size="iconSize" />
    </span>
    <span v-if="online !== undefined" :style="dotStyle" />
  </div>
</template>

<script setup lang="ts">
import {shallowRef, computed} from 'vue';
import { User } from 'lucide-vue-next';
import { AppleLayout, AppleFontWeight, AppleIconSize, AppleRadius, AppleType, cv } from '@/utils/core/apple-tokens';
import type { AppleControlSize } from '@/utils/core/apple-tokens';

const props = withDefaults(
  defineProps<{
    src?: string;
    text?: string;
    size?: AppleControlSize;
    online?: boolean;
    ariaLabel?: string;
  }>(),
  { size: 'medium' },
);

const fallback = shallowRef(false);

const sizeMap = {
  // 头像里的图标位同样走 AppleIconSize（原先 16/20/28：28 是尺外的孤档，收到 24）
  small: { size: AppleLayout.avatarSizeSmall, fontSize: AppleType.microUI.size, letterSpacing: AppleType.microUI.letterSpacing, iconSize: AppleIconSize.control, dotSize: '8px', dotOffset: '1px' },
  medium: { size: AppleLayout.avatarSizeMedium, fontSize: AppleType.bodyPrimary.size, letterSpacing: AppleType.bodyPrimary.letterSpacing, iconSize: AppleIconSize.listLead, dotSize: '10px', dotOffset: '2px' },
  large: { size: AppleLayout.avatarSizeLarge, fontSize: AppleType.pageTitle.size, letterSpacing: AppleType.pageTitle.letterSpacing, iconSize: AppleIconSize.section, dotSize: '12px', dotOffset: '2px' },
};

const config = computed(() => sizeMap[props.size]);
const iconSize = computed(() => config.value.iconSize);

const showImage = computed(() => props.src && !fallback.value);

const avatarStyle = computed(() => ({
  width: config.value.size,
  height: config.value.size,
  borderRadius: AppleRadius.medium,
  background: showImage.value ? cv('bg-tertiary') : cv('blue'),
  color: cv('on-accent'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontSize: config.value.fontSize,
  letterSpacing: config.value.letterSpacing,
  fontWeight: String(AppleFontWeight.semibold),
  flexShrink: '0',
  position: 'relative' as const,
  overflow: 'hidden' as const,
}));

const imgStyle = {
  width: '100%',
  height: '100%',
  objectFit: 'cover' as const,
  borderRadius: AppleRadius.medium,
};

const textStyle = computed(() => ({
  lineHeight: '1',
  textTransform: 'uppercase' as const,
}));

const iconStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

const dotStyle = computed(() => ({
  position: 'absolute' as const,
  bottom: config.value.dotOffset,
  right: config.value.dotOffset,
  width: config.value.dotSize,
  height: config.value.dotSize,
  borderRadius: AppleRadius.circle,
  background: props.online ? cv('green') : cv('text-quaternary'),
  border: `2px solid ${cv('pure-white-canvas')}`,
}));
</script>
