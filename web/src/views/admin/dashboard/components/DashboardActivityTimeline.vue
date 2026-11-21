<template>
  <ApCard>
    <div :style="cardHeaderStyle">
      <h2 :style="cardTitleStyle">{{ $t('admin.k106') }}</h2>
    </div>
    <ApTimeline v-if="items.length > 0" :aria-label="$t('admin.k106')">
      <ApTimelineItem
        v-for="(act, idx) in items"
        :key="idx"
        :color="dotColor(act.action)"
        :title="act.title"
        :time="act.time"
      />
    </ApTimeline>
    <ApEmpty v-else icon="FileText" size="small" :title="$t('admin.k107')" />
  </ApCard>
</template>

<script setup lang="ts">
import { cv, AppleSpace, AppleFont, AppleType, AppleFontWeight } from '@/utils/core/apple-tokens';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApTimeline from '@/components/apple/ApTimeline.vue';
import ApTimelineItem from '@/components/apple/ApTimelineItem.vue';

export interface ActivityItem {
  title: string;
  time: string;
  action?: 'create' | 'edit' | 'delete' | 'collect' | 'info';
}

defineProps<{
  items: ActivityItem[];
}>();

const cardHeaderStyle = {
  marginBottom: AppleSpace.md,
};

const cardTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  margin: '0',
};

const dotColor = (action?: string) =>
  cv(({ create: 'green', edit: 'orange', delete: 'red', collect: 'blue' }[action || ''] || 'blue') as Parameters<typeof cv>[0]);
</script>
