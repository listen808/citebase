<template>
  <ApCard>
    <div :style="cardHeaderStyle">
      <h2 :style="cardTitleStyle">{{ $t('admin.k119') }}</h2>
    </div>
    <div :style="statGridStyle">
      <div :style="statItemStyle">
        <div :style="statLabelStyle">{{ $t('admin.k120') }}</div>
        <div :style="statValueStyle" :title="String(activeUsers)">{{ activeUsers }}</div>
        <div :style="statCountStyle">{{ $t('admin.k121') }}</div>
      </div>
      <div v-if="topCreator" :style="statItemStyle">
        <div :style="statLabelStyle">{{ $t('admin.k122') }}</div>
        <div :style="statValueStyle" :title="String(topCreator.given_name || topCreator.username)">{{ topCreator.given_name || topCreator.username }}</div>
        <div :style="statCountStyle">{{ topCreator.count }} {{ $t('vAdminUi.k14') }}</div>
      </div>
      <div v-if="topEditor" :style="statItemStyle">
        <div :style="statLabelStyle">{{ $t('admin.k123') }}</div>
        <div :style="statValueStyle" :title="String(topEditor.given_name || topEditor.username)">{{ topEditor.given_name || topEditor.username }}</div>
        <div :style="statCountStyle">{{ topEditor.count }} {{ $t('vAdminUi.k12') }}</div>
      </div>
      <div v-if="topCollector" :style="statItemStyle">
        <div :style="statLabelStyle">{{ $t('admin.k124') }}</div>
        <div :style="statValueStyle" :title="String(topCollector.given_name || topCollector.username)">{{ topCollector.given_name || topCollector.username }}</div>
        <div :style="statCountStyle">{{ topCollector.count }} {{ $t('vAdminUi.k15') }}</div>
      </div>
      <div v-if="topFollower" :style="statItemStyle">
        <div :style="statLabelStyle">{{ $t('admin.k125') }}</div>
        <div :style="statValueStyle" :title="String(topFollower.given_name || topFollower.username)">{{ topFollower.given_name || topFollower.username }}</div>
        <div :style="statCountStyle">{{ topFollower.count }} {{ $t('vAdminUi.k16') }}</div>
      </div>
    </div>
  </ApCard>
</template>

<script setup lang="ts">
import { AppleSpace, AppleFont, AppleType, AppleFontWeight, cv, AppleRadius, AppleGridFloor } from '@/utils/core/apple-tokens';
import { monitorChartTitleStyle as cardTitleStyle } from '@/styles/layout/page-styles';
import ApCard from '@/components/apple/ApCard.vue';

export interface RankUser {
  username: string;
  given_name?: string;
  count: number;
}

defineProps<{
  activeUsers: number;
  topCreator?: RankUser | null;
  topEditor?: RankUser | null;
  topCollector?: RankUser | null;
  topFollower?: RankUser | null;
}>();

const cardHeaderStyle = {
  marginBottom: AppleSpace.md,
};

const statGridStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor.tile})`,
  gap: AppleSpace.sm,
};

const statItemStyle = {
  padding: AppleSpace.sm,
  borderRadius: AppleRadius.control,
  background: cv('bg-secondary'),
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
};

const statLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

const statValueStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const statCountStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
};
</script>
