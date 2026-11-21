<template>
  <div>
    <div :style="LayoutStyles.tabBar">
      <ApSegmentedControl
        v-if="tabs?.length"
        :model-value="activeTab"
        :options="segmentedOptions"
        @update:model-value="(v) => emit('update:activeTab', String(v))"
      />
    </div>
    <Transition name="tab-switch" mode="out-in">
      <div :key="activeTab" :style="contentWrapperStyle">
        <div v-if="wrapWithCard" :style="cardStyle">
          <slot />
        </div>
        <slot v-else />
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { LayoutStyles } from '@/styles/layout/page-styles';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import type { SegmentedOption } from '@/components/apple/ApSegmentedControl.vue';

interface TabItem {
  key: string;
  label: string;
}

const props = withDefaults(defineProps<{
  activeTab: string;
  tabs?: TabItem[];
  wrapWithCard?: boolean;
}>(), {
  wrapWithCard: true,
});

const emit = defineEmits<{
  (e: 'update:activeTab', value: string): void;
}>();

const segmentedOptions = computed<SegmentedOption[]>(() =>
  (props.tabs || []).map(t => ({ label: t.label, value: t.key }))
);

const contentWrapperStyle = {
  minHeight: '400px',
};

const cardStyle = {
  ...LayoutStyles.card,
};
</script>

<style scoped>
.tab-switch-enter-active,
.tab-switch-leave-active {
  transition: opacity var(--apple-dur-fast) ease;
}
.tab-switch-enter-from,
.tab-switch-leave-to {
  opacity: 0;
}
</style>
