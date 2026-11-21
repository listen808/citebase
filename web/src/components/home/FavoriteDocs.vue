<template>
  <div :style="sectionStyle">
    <div :style="headerStyle">
      <h3 :style="titleStyle">{{ $t('homeUi.k1') }}</h3>
      <router-link to="/collections" :style="linkStyle">
        {{ $t('homeUi.k2') }}
        <ApIcon name="ChevronRight" :size="14" />
      </router-link>
    </div>
    <div v-if="loading" :style="loadingStyle"><ApSkeleton variant="grid" :count="3" /></div>
    <div v-else-if="!docs.length" :style="emptyStyle">
      <ApIcon name="Star" :size="AppleIconSize.listLead" :style="{ color: cv('text-quaternary') }" />
      <span>{{ $t('homeUi.k3') }}</span>
    </div>
    <div v-else :style="gridStyle">
      <ApEntityCard
        v-for="doc in docs"
        :key="doc.id"
        :title="doc.name"
        :subtitle="doc.space_name"
        @click="$emit('navigate', doc)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { AppleIconSize, cv, AppleType, AppleSpace, appleCardGrid } from '@/utils/core/apple-tokens';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApEntityCard from '@/components/apple/ApEntityCard.vue';
import { loadingStyle, emptyStateStyle as emptyStyle, layoutSectionHeaderTitle as titleStyle } from '@/styles/layout/page-styles';

export interface FavDoc {
  id: number;
  name: string;
  space_id?: number;
  space_name?: string;
}

defineProps<{
  docs: FavDoc[];
  loading: boolean;
}>();

defineEmits<{
  (e: 'navigate', doc: FavDoc): void;
}>();

const sectionStyle = {
  marginTop: AppleSpace.lg,
  marginBottom: AppleSpace.lg,
};

const headerStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  marginBottom: AppleSpace.md,
};

const linkStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.microUI.size,
  color: cv('blue'),
  textDecoration: 'none',
  cursor: 'pointer',
};

// 与共享卡片网格同构：直接走工厂，别再抄一份字面量（克隆会被 check:style 记账）
const gridStyle = appleCardGrid('entity');
</script>
