<template>
  <ApPageContainer
    :title="$t('social.k8')"
    :subtitle="$t('social.k10')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <div :style="crudStatsRowStyle('compact')">
      <ApStatCard size="compact" :label="$t('social.create2')" :value="actionCounts.create" :icon="'FilePlus'" :color="cv('green')" />
      <ApStatCard size="compact" :label="$t('social.k4')" :value="actionCounts.edit" :icon="'FileEdit'" :color="cv('orange')" />
      <ApStatCard size="compact" :label="$t('social.delete')" :value="actionCounts.delete" :icon="'FileX'" :color="cv('red')" />
      <ApStatCard size="compact" :label="$t('social.k6')" :value="actionCounts.restore" :icon="'RotateCcw'" :color="cv('blue')" />
    </div>
    <div :style="statsHintStyle">{{ $t('social.k1') }}</div>

    <div :style="filterBarStyle">
      <ApSegmentedControl
        :model-value="activeFilter"
        :options="filterOptions"
        size="small"
        :aria-label="$t('social.k11')"
        @update:model-value="(v: string | number) => activeFilter = String(v)"
      />
    </div>

    <div v-if="loading && items.length === 0" :style="compactLoadingStyle"><ApSkeleton variant="list" :lines="3" /></div>
    <ApEmpty v-else-if="items.length === 0" icon="Activity" :title="$t('social.k9')" />
    <div v-else :style="compactTimelineStyle()">
      <div v-for="(group, date) in groupedItems" :key="date">
        <div :style="dateHeaderStyle">{{ date }}</div>
        <div v-for="item in group" :key="item.id" v-bind="keyboardClick(() => handleDocClick(item))" :style="compactTimelineItemStyle()">
          <div :style="timelineNodeStyle(getActionColor(item.action))" />
          <div :style="timelineContentStyle">
            <div :style="timelineEntryHeaderStyle">
              <ApStatusBadge
                :status="getActionStatus(item.action)"
                :label="formatActionName(item.action)"
                size="small"
              />
              <span :style="timelineDocNameStyle" :title="item.document_name || item.detail || $t('common.document')">{{ item.document_name || item.detail || $t('common.document') }}</span>
              <span :style="timeStyle">{{ formatTime(item.create_time) }}</span>
            </div>
            <div :style="timelineEntryMetaStyle">
              <span :style="timelineLinkStyle" @click.stop="handleUserClick(item)">{{ item.given_name || item.username || $t('common.user') }}</span>
              <span :style="metaDividerStyle">·</span>
              <span :style="metaItemStyle">{{ item.space_name || $t('common.defaultSpace') }}</span>
              <ApStatusBadge
                v-if="isCriticalAction(item.action)"
                status="error"
                :label="$t('social.k7')"
                size="small"
              />
            </div>
          </div>
        </div>
      </div>
      <div v-if="total > items.length" :style="compactLoadMoreStyle">
        <ApButton variant="secondary" size="small" :loading="loading" @click="loadMore">{{ $t('social.loadMore') }}</ApButton>
      </div>
    </div>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted, watch } from 'vue';
import { useRouter } from 'vue-router';
import { keyboardClick } from '@/composables/ui/useKeyboardClick';
import { getUserActivity, type UserActivity } from '@/api/modules/system/log';
import { cv, AppleLayout, AppleSpace, AppleFont, AppleType, AppleFontWeight } from '@/utils/core/apple-tokens';
import { crudStatsRowStyle, timeStyle, timelineNodeStyle, timelineContentStyle, timelineEntryHeaderStyle, timelineDocNameStyle, timelineEntryMetaStyle, timelineLinkStyle, metaItemStyle, compactLoadingStyle } from '@/styles/layout/page-styles';
import { formatActionName, getActionStatus, getActionColor, isCriticalAction } from '@/utils/activity';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import { formatTime, formatDate } from '@/utils/format/format-time';
import { useUIStore } from '@/stores/app/ui';
import { useRequest } from '@/composables/shared/useRequest';

const router = useRouter();
const ui = useUIStore();

const activeFilter = ref('all');
const items = ref<UserActivity[]>([]);
const page = shallowRef(1);
const pageSize = 20;
const total = shallowRef(0);

const { loading, execute: execLoad } = useRequest(
  async (append: boolean = false) => {
    const params: Record<string, unknown> = { page: page.value, page_size: pageSize };
    if (activeFilter.value !== 'all') {
      params.action = activeFilter.value;
    }
    const res = await getUserActivity(params);
    return { res, append };
  },
  {
    onSuccess: ({ res, append }) => {
      if (append) {
        items.value.push(...res.data);
      } else {
        items.value = res.data;
        total.value = res.total;
      }
    },
    onError: () => {
      ui.showToast(t('social.k13'), 'error');
      if (page.value > 1) page.value--;
    },

  },
);

function loadMore() {
  page.value++;
  execLoad(true);
}

watch(activeFilter, () => {
  page.value = 1;
  execLoad(false);
});

const groupedItems = computed(() => {
  const groups: Record<string, UserActivity[]> = {};
  items.value.forEach((item) => {
    const date = formatDate(item.create_time);
    if (!groups[date]) groups[date] = [];
    groups[date].push(item);
  });
  return Object.fromEntries(
    Object.entries(groups).sort(([a], [b]) => b.localeCompare(a))
  );
});

const actionCounts = computed(() => {
  const counts = { create: 0, edit: 0, delete: 0, restore: 0 };
  items.value.forEach((item) => {
    const action = String(item.action);
    if (action === '1' || action === 'create') counts.create++;
    else if (action === '2' || action === 'edit') counts.edit++;
    else if (action === '3' || action === 'delete') counts.delete++;
    else if (action === '4' || action === 'restore') counts.restore++;
  });
  return counts;
});

const filterOptions = computed(() => [
  { value: 'all', label: t('social.all') },
  { value: 'create', label: t('social.create2') },
  { value: 'edit', label: t('social.k4') },
  { value: 'delete', label: t('social.delete') },
  { value: 'restore', label: t('social.k6') },
]);

function handleUserClick(item: UserActivity) {
  if (item.user_id) {
    router.push(`/user/${item.user_id}`);
  }
}

function handleDocClick(item: UserActivity) {
  const spaceId = item.space_id || 0;
  const docId = item.document_id;
  if (docId) {
    router.push(`/space/${spaceId}/doc/${docId}`);
  }
}

onMounted(() => { execLoad(false); });

const filterBarStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
  marginBottom: AppleSpace.md,
  flexWrap: 'wrap' as const,
};

const dateHeaderStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-secondary'),
  padding: `${AppleSpace.xs} 0 ${AppleSpace.sm} 0`,
};

const metaDividerStyle = {
  fontFamily: AppleFont.text,
  color: cv('text-quaternary'),
  fontSize: AppleType.controlLabel.size,
};

const statsHintStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  marginTop: 0,
  marginBottom: AppleSpace.md,
  textAlign: 'right' as const,
};

function compactTimelineStyle() {
  return {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '0',
    position: 'relative' as const,
    paddingLeft: AppleSpace.md,
  };
}

function compactTimelineItemStyle() {
  return {
    position: 'relative' as const,
    paddingBottom: AppleSpace.md,
    borderLeft: `2px solid ${cv('border-light')}`,
    paddingLeft: AppleSpace.sm,
  };
}

const compactLoadMoreStyle = {
  display: 'flex',
  justifyContent: 'center',
  padding: `${AppleSpace.md} 0`,
};
</script>
