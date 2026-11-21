<template>
  <div :style="containerStyle" role="status" aria-busy="true" :aria-label="$t('apUi1.k32')">
    <div v-if="variant === 'text'" :style="skeletonStyle" />
    <div v-else-if="variant === 'circular'" :style="{ ...skeletonStyle, borderRadius: AppleRadius.circle }" />
    <div v-else-if="variant === 'rectangular'" :style="skeletonStyle" />
    <div v-else-if="variant === 'rounded'" :style="{ ...skeletonStyle, borderRadius: AppleRadius.small }" />
    <div v-else-if="variant === 'card'" :style="cardStyle">
      <div :style="cardHeaderStyle" />
      <div :style="cardContentStyle">
        <div v-for="i in lines" :key="i" :style="{ ...skeletonStyle, width: i === lines ? '70%' : '100%', height: '12px', marginBottom: AppleSpace.sm }" />
      </div>
    </div>
    <div v-else-if="variant === 'list'" :style="listStyle">
      <div v-for="i in lines" :key="i" :style="listItemStyle">
        <div :style="{ ...skeletonStyle, width: '40px', height: '40px', borderRadius: AppleRadius.circle, flexShrink: '0' }" />
        <div :style="listContentStyle">
          <div :style="{ ...skeletonStyle, width: '60%', height: '14px', marginBottom: AppleSpace.sm }" />
          <div :style="{ ...skeletonStyle, width: '40%', height: '12px' }" />
        </div>
      </div>
    </div>
    <div v-else-if="variant === 'form'" :style="formStyle">
      <div v-for="i in lines" :key="i" :style="formRowStyle">
        <div :style="{ ...skeletonStyle, width: '80px', height: '14px', marginBottom: AppleSpace.sm }" />
        <div :style="{ ...skeletonStyle, width: '100%', height: '36px', borderRadius: AppleRadius.small }" />
      </div>
    </div>
    <div v-else-if="variant === 'grid'" :style="gridStyle">
      <div v-for="i in count" :key="i" :style="gridItemStyle">
        <div :style="{ ...skeletonStyle, width: '100%', height: '120px', borderRadius: AppleRadius.card }" />
        <div :style="{ ...skeletonStyle, width: '80%', height: '14px', marginTop: AppleSpace.sm }" />
        <div :style="{ ...skeletonStyle, width: '50%', height: '12px', marginTop: AppleSpace.sm }" />
      </div>
    </div>
    <div v-else-if="variant === 'custom'" :style="customStyle">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { AppleTiming, AppleAnimation, AppleRadius, AppleSpace, cv, appleCardGrid } from '@/utils/core/apple-tokens';
import { listStyle as sharedListStyle } from '@/styles/layout/page-styles';

interface Props {
  width?: string;
  height?: string;
  variant?: 'text' | 'circular' | 'rectangular' | 'rounded' | 'card' | 'list' | 'form' | 'grid' | 'custom';
  lines?: number;
  count?: number;
}

const props = withDefaults(defineProps<Props>(), {
  width: '100%',
  height: '16px',
  variant: 'text',
  lines: 3,
  count: 3,
});

const animation = `${AppleAnimation.shimmer} ${AppleTiming.skeletonSweep} infinite`;

const skeletonStyle = computed(() => ({
  width: props.width,
  height: props.height,
  background: `linear-gradient(90deg, ${cv('bg-tertiary')} 25%, ${cv('bg-secondary')} 50%, ${cv('bg-tertiary')} 75%)`,
  backgroundSize: '200% 100%',
  borderRadius: AppleRadius.micro,
  animation,
}));

const containerStyle = computed(() => ({
  display: props.variant === 'list' || props.variant === 'grid' || props.variant === 'form' ? 'block' : 'inline-block',
  width: props.variant === 'list' || props.variant === 'grid' || props.variant === 'form' ? '100%' : 'auto',
}));

const cardStyle = computed(() => ({
  background: cv('surface-panel'),
  borderRadius: AppleRadius.card,
  border: `1px solid ${cv('border-light')}`,
  padding: AppleSpace.md,
  marginBottom: AppleSpace.md,
}));

const cardHeaderStyle = computed(() => ({
  ...skeletonStyle.value,
  width: '40%',
  height: '18px',
  marginBottom: AppleSpace.md,
}));

const cardContentStyle = computed(() => ({
  display: 'flex',
  flexDirection: 'column' as const,
}));

const listStyle = computed(() => sharedListStyle(AppleSpace.md));

const listItemStyle = computed(() => ({
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.md,
  padding: `${AppleSpace.md} 0`,
  borderBottom: `1px solid ${cv('border-light')}`,
}));

const listContentStyle = computed(() => ({
  flex: '1',
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
}));

const formStyle = computed(() => ({
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
  width: '100%',
}));

const formRowStyle = computed(() => sharedListStyle(AppleSpace.xs));

const gridStyle = computed(() => appleCardGrid('entity'));

const gridItemStyle = computed(() => ({
  display: 'flex',
  flexDirection: 'column' as const,
}));

const customStyle = computed(() => ({
  width: '100%',
}));
</script>

