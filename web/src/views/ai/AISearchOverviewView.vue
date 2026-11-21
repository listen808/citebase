<template>
  <div :style="aiSubPageStyle">
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Search" :size="16" :color="cv('blue')" />
          <span :style="sectionTitleStyle">{{ $t('aiSetNav.k1') }}</span>
        </div>
      </template>

      <div :style="aiSearchRowStyle">
        <div :style="searchInputWrapStyle">
          <ApInput
            v-model="query"
            :aria-label="$t('aiSetNav.k7')"
            :placeholder="$t('aiSetNav.k4')"
            :disabled="loading"
            @keydown.enter="handleSearch"
          />
        </div>
        <!-- 定宽：ApSelect 根是 width:100%，不兜住会把整行撑成三行（B9 实测） -->
        <div :style="filterSelectGroupStyle()">
          <ApSelect
            v-model="spaceId"
            :options="spaceOptions"
            :placeholder="$t('aiSetNav.k5')"
            size="medium"
            :searchable="true"
          />
        </div>
        <ApButton variant="primary" size="small" :loading="loading" @click="handleSearch">
          <ApIcon name="Search" :size="16" /> {{ $t('aiSetNav.search') }}
        </ApButton>
      </div>
    </ApCard>

    <ApSkeleton v-if="loading" :lines="3" />

    <ApCard v-else-if="overview" size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Zap" :size="16" :color="cv('purple')" />
          <span :style="sectionTitleStyle">{{ $t('aiSetNav.k3') }}</span>
        </div>
      </template>
      <div :style="overviewBodyStyle">
        <ApMarkdownPreview :content="overview" />
      </div>
    </ApCard>

    <ApCard v-if="!loading && !searched" size="small">
      <ApEmpty icon="Search" :title="$t('aiSetNav.k16')" :description="$t('aiSetNav.k126')" />
    </ApCard>

    <ApCard v-if="!loading && searched && !overview" size="small">
      <ApEmpty icon="Search" :title="$t('aiSetNav.k6')" :description="$t('vAiUi.k70')" />
    </ApCard>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { logger } from '@/utils/logger';
import { ref, shallowRef, onMounted } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import { useSpaceStore } from '@/stores/workspace/space';

import { cv, AppleRadius, AppleSpace } from '@/utils/core/apple-tokens';
import { aiSubPageStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle } from '@/styles/ai/ai-page-styles';
import { filterSelectGroupStyle } from '@/styles/layout/page-styles';
import { searchOverview } from '@/api/modules/ai/ai-search';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();
const spaceStore = useSpaceStore();

const query = ref('');
const spaceId = ref<string | number | null>(null);
const loading = shallowRef(false);
const searched = ref(false);
const overview = ref('');

const spaceOptions = ref<{ label: string; value: string | number }[]>([
  { label: t('aiSetNav.k5'), value: '' },
]);

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    const spaces = spaceStore.mySpaces;
    spaceOptions.value = [
      { label: t('aiSetNav.k5'), value: '' },
      ...spaces.map(s => ({ label: s.name, value: s.id })),
    ];
  } catch (e) {
    logger.error('Failed to load spaces:', e);
    ui.showToast(t('aiSetNav.k8'), 'error');
  }
}

onMounted(loadSpaces);

async function handleSearch() {
  const q = query.value.trim();
  if (!q) return;
  loading.value = true;
  searched.value = true;
  overview.value = '';
  try {
    const res = await searchOverview({
      query: q,
      space_id: spaceId.value ? Number(spaceId.value) : undefined,
    });
    overview.value = res.overview?.overview || '';
  } catch (e: unknown) {
    ui.showToast((e as { message?: string }).message || t('fallback.k63'), 'error');
  } finally {
    loading.value = false;
  }
}

const sectionTitleStyle = aiSubSectionTitleStyle;

// 命名避开共享层 page-styles.searchRowStyle（本视图未用它，别同名遮蔽）
const aiSearchRowStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.md,
  flexWrap: 'wrap' as const,
};

const searchInputWrapStyle = {
  flex: '1',
  minWidth: '200px',
};

const overviewBodyStyle = {
  borderRadius: AppleRadius.medium,
  border: `1px solid ${cv('border-light')}`,
  background: cv('surface-panel'),
  overflow: 'hidden' as const,
};
</script>
