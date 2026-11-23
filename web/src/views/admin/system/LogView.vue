<template>
  <ApPageContainer
    :title="$t('adminSys.k139')"
    :subtitle="$t('adminSys.k143')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >

    <div v-if="activeTab === 'activity'" :style="crudStatsRowStyle('compact')">
      <ApStatCard size="compact" :label="$t('adminSys.k133')" :value="activityTotal" :icon="'Activity'" :color="cv('blue')" />
      <ApStatCard size="compact" :label="$t('adminSys.create2')" :value="activityCounts.create" :icon="'FilePlus'" :color="cv('green')" />
      <ApStatCard size="compact" :label="$t('adminSys.k135')" :value="activityCounts.edit" :icon="'FileEdit'" :color="cv('orange')" />
      <ApStatCard size="compact" :label="$t('adminSys.delete')" :value="activityCounts.delete" :icon="'FileX'" :color="cv('red')" />
      <ApStatCard size="compact" :label="$t('adminSys.k36')" :value="activityCounts.restore" :icon="'RotateCcw'" :color="cv('blue')" />
    </div>

    <!-- 日志来源切换以前独占一行，两个 tab 各自的筛选行又在它下面另起一行 —— 三行 chrome。
         现在合成一行：左边「看哪些」（来源 + 该来源的筛选条件），行随 activeTab 换内容。 -->
    <div :style="compactFilterToolbarStyle()">
      <ApSegmentedControl
        v-model="activeTab"
        :options="tabs"
        size="small"
        :aria-label="$t('adminSys.k144')"
      />
      <template v-if="activeTab === 'activity'">
        <div :style="filterGroupStyle()">
          <span :style="labelStyle">{{ $t('adminSys.k120') }}</span>
          <div :style="dateRangeStyle()">
            <ApInput v-model="dateRange.start" type="date" :aria-label="$t('adminSys.k145')" :style="dateInputStyle()" />
            <span :style="labelStyle">{{ $t('adminSys.k121') }}</span>
            <ApInput v-model="dateRange.end" type="date" :aria-label="$t('adminSys.k146')" :style="dateInputStyle()" />
          </div>
        </div>
        <div :style="filterGroupStyle()">
          <span :style="labelStyle">{{ $t('adminSys.k122') }}</span>
          <div :style="filterButtonsStyle()">
            <ApSegmentedControl
              :model-value="activeActionFilter"
              :options="actionFilterOptions"
              size="small"
              :aria-label="$t('adminSys.k122')"
              @update:model-value="(v: string | number) => activeActionFilter = String(v)"
            />
          </div>
        </div>
        <div :style="filterGroupStyle()">
          <span :style="labelStyle">{{ $t('adminSys.k123') }}</span>
          <ApSelect v-model="userFilter" :options="userFilterOptions" :placeholder="$t('adminSys.k137')" :style="filterSelectStyle()" />
        </div>
        <div :style="filterGroupStyle()">
          <span :style="labelStyle">{{ $t('adminSys.k124') }}</span>
          <ApSelect v-model="spaceFilter" :options="spaceFilterOptions" :placeholder="$t('adminSys.k138')" :style="filterSelectStyle()" />
        </div>
      </template>
      <template v-else>
        <div :style="filterGroupStyle()">
          <span :style="labelStyle">{{ $t('adminSys.k125') }}</span>
          <ApSelect
            v-model="level"
            :options="[
              { label: $t('vAdminUi.all'), value: 0 },
              { label: 'DEBUG', value: 7 },
              { label: 'INFO', value: 6 },
              { label: 'WARN', value: 4 },
              { label: 'ERROR', value: 3 }
            ]"
            :style="filterSelectStyle()"
          />
        </div>
        <div :style="filterGroupStyle()">
          <span :style="labelStyle">{{ $t('adminSys.k73') }}</span>
          <ApInput v-model="systemUsernameFilter" :placeholder="$t('adminSys.k73')" :style="filterSelectStyle()" @change="loadSystemLogs" />
        </div>
        <div :style="filterGroupStyle()">
          <span :style="labelStyle">{{ $t('adminSys.k126') }}</span>
          <ApInput v-model="systemMessageFilter" :placeholder="$t('adminSys.k126')" :style="filterSelectStyle()" @change="loadSystemLogs" />
        </div>
      </template>
    </div>

    <div v-if="activeTab === 'activity'">
      <div v-if="loading && activities.length === 0" :style="loadingStyle"><ApSkeleton variant="list" :lines="3" /></div>
      <ApEmpty v-else-if="filteredActivities.length === 0" icon="Activity" :title="$t('adminSys.k140')" />
      <ApTimeline v-else density="compact" :aria-label="$t('adminSys.k147')">
        <ApTimelineItem
          v-for="item in filteredActivities"
          :key="item.id"
          :color="getActionColor(item.action)"
          :style="{ cursor: 'pointer' }"
          tabindex="0"
          @click="handleActivityClick(item)"
          @keydown.enter="handleActivityClick(item)"
        >
          <div :style="compactTimelineEntryHeaderStyle">
            <ApStatusBadge
              :status="getActionStatus(item.action)"
              :label="formatActionName(item.action)"
              size="small"
            />
            <span :style="timelineDocNameStyle" :title="item.document_name || item.change_log || $t('common.document')">{{ item.document_name || item.change_log || $t('common.document') }}</span>
            <span :style="timeStyle">{{ formatTime(item.create_time) }}</span>
          </div>
          <div :style="compactTimelineEntryMetaStyle">
            <span :style="timelineLinkStyle" @click.stop="handleUserClick(item)">{{ item.given_name || item.username || $t('fallback.k90') }}</span>
            <span :style="logCardDividerStyle()">·</span>
            <span :style="metaItemStyle">{{ item.space_name || $t('fallback.k109') }}</span>
            <ApStatusBadge
              v-if="isCriticalAction(item.action)"
              status="error"
              :label="$t('adminSys.k136')"
              size="small"
              :style="criticalBadgeStyle()"
            />
          </div>
        </ApTimelineItem>
      </ApTimeline>
      <div v-if="hasMoreActivities" :style="compactLoadMoreContainerStyle">
        <ApButton variant="secondary" size="small" :loading="loading" @click="loadMoreActivities">{{ $t('adminSys.loadMore') }}</ApButton>
      </div>
    </div>

    <div v-if="activeTab === 'system'">
      <div v-if="loading && systemLogs.length === 0" :style="loadingStyle"><ApSkeleton variant="list" :lines="3" /></div>
      <ApEmpty v-else-if="systemLogs.length === 0" icon="ScrollText" :title="$t('adminSys.k141')" />
      <ApTimeline v-else density="compact" :aria-label="$t('adminSys.k148')">
        <ApTimelineItem
          v-for="log in systemLogs"
          :key="log.log_id"
          :color="getSystemLogColor(log.level)"
          :style="{ cursor: 'pointer' }"
          tabindex="0"
          @click="handleSystemLogClick(log)"
          @keydown.enter="handleSystemLogClick(log)"
        >
          <div :style="compactTimelineEntryHeaderStyle">
            <span :style="getLevelLabelStyle(log.level)">{{ formatLevel(log.level) }}</span>
            <span :style="logPathStyle()">{{ log.method }} {{ log.path }}</span>
            <span :style="timeStyle">{{ formatTime(log.create_time) }}</span>
          </div>
          <div :style="compactTimelineEntryMetaStyle">
            <span :style="timelineLinkStyle" @click.stop="log.user_id ? router.push(`/user/${log.user_id}`) : undefined">{{ log.given_name || log.username }}</span>
            <span :style="logCardDividerStyle()">·</span>
            <span :style="metaItemStyle">{{ log.ip }}</span>
          </div>
        </ApTimelineItem>
      </ApTimeline>
      <div v-if="hasMoreSystemLogs" :style="compactLoadMoreContainerStyle">
        <ApButton variant="secondary" size="small" :loading="loading" @click="loadMoreSystemLogs">{{ $t('adminSys.loadMore') }}</ApButton>
      </div>
    </div>

    <ApModal v-model="showLogDetail" :title="$t('adminSys.k142')" size="medium" density="compact">
      <div v-if="logDetail" :style="logDetailStyle">
        <div :style="logDetailRowStyle">
          <span :style="logDetailLabelStyle">{{ $t('adminSys.k128') }}</span>
          <span :style="logDetailValueStyle">{{ logDetail.method }}</span>
        </div>
        <div :style="logDetailRowStyle">
          <span :style="logDetailLabelStyle">{{ $t('adminSys.k129') }}</span>
          <span :style="[logDetailValueStyle, { wordBreak: 'break-all' }]">{{ logDetail.path }}</span>
        </div>
        <div :style="logDetailRowStyle">
          <span :style="logDetailLabelStyle">{{ $t('adminSys.k125') }}</span>
          <span :style="getLevelLabelStyle(logDetail.level)">{{ formatLevel(logDetail.level) }}</span>
        </div>
        <div :style="logDetailRowStyle">
          <span :style="logDetailLabelStyle">User Agent</span>
          <span :style="[logDetailValueStyle, { wordBreak: 'break-all', fontSize: AppleType.microUI.size }]">{{ logDetail.user_agent || '-' }}</span>
        </div>
        <div :style="logDetailRowStyle">
          <span :style="logDetailLabelStyle">IP</span>
          <span :style="logDetailValueStyle">{{ logDetail.ip }}</span>
        </div>
        <div :style="logDetailRowStyle">
          <span :style="logDetailLabelStyle">{{ $t('adminSys.k130') }}</span>
          <span :style="timelineLinkStyle" @click="logDetail.user_id ? router.push(`/user/${logDetail.user_id}`) : undefined">{{ logDetail.given_name || logDetail.username }}</span>
        </div>
        <div :style="logDetailRowStyle">
          <span :style="logDetailLabelStyle">{{ $t('adminSys.k28') }}</span>
          <span :style="logDetailValueStyle">{{ formatTime(logDetail.create_time) }}</span>
        </div>
        <div v-if="maskedGetParams" :style="logDetailRowStyle">
          <span :style="logDetailLabelStyle">{{ $t('adminSys.k131') }}</span>
          <code :style="logDetailCodeStyle">{{ maskedGetParams }}</code>
        </div>
        <div v-if="maskedPostParams" :style="logDetailRowStyle">
          <span :style="logDetailLabelStyle">{{ $t('adminSys.k132') }}</span>
          <code :style="logDetailCodeStyle">{{ maskedPostParams }}</code>
        </div>
        <div v-if="logDetail.referer" :style="logDetailRowStyle">
          <span :style="logDetailLabelStyle">Referer</span>
          <span :style="[logDetailValueStyle, { wordBreak: 'break-all' }]">{{ logDetail.referer }}</span>
        </div>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showLogDetail = false">{{ $t('adminSys.close') }}</ApButton>
      </template>
    </ApModal>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted, watch } from 'vue';
import { useRouter } from 'vue-router';
import { cv, AppleLayout, AppleType, AppleRadius, AppleFont, AppleFontWeight, AppleSpace } from '@/utils/core/apple-tokens';
import { loadingStyle, crudStatsRowStyle, timelineLinkStyle, timeStyle, metaItemStyle, labelStyle, filterGroupStyle, dateRangeStyle, dateInputStyle, filterButtonsStyle, filterSelectStyle, logCardDividerStyle, criticalBadgeStyle, logPathStyle, timelineDocNameStyle } from '@/styles/layout/page-styles';
import { formatActionName, getActionStatus, getActionColor, isCriticalAction } from '@/utils/activity';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApTimeline from '@/components/apple/ApTimeline.vue';
import ApTimelineItem from '@/components/apple/ApTimelineItem.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import { getUserActivity, getSystemLogs, getLogDetail, type UserActivity, type SystemLog } from '@/api/modules/system/log';
import { getUsers } from '@/api/modules/user/user';
import type { User } from '@/types/models';
import { formatTime } from '@/utils/format/format-time';
import { maskSensitiveData } from '@/utils/core/mask-sensitive';
import { useUIStore } from '@/stores/app/ui';
import { useRequest } from '@/composables/shared/useRequest';

const router = useRouter();
const ui = useUIStore();

const activeTab = ref<'activity' | 'system'>('activity');
const activities = ref<UserActivity[]>([]);
const systemLogs = ref<SystemLog[]>([]);
const level = shallowRef(0);
const systemUsernameFilter = shallowRef('');
const systemMessageFilter = shallowRef('');
const page = shallowRef(1);
const pageSize = shallowRef(20);
const activityTotal = shallowRef(0);
const systemLogTotal = shallowRef(0);

const actionFilterOptions = [
  { label: t('adminSys.all'), value: 'all', color: cv('blue') },
  { label: t('adminSys.create2'), value: 'create', color: cv('green') },
  { label: t('adminSys.k135'), value: 'edit', color: cv('orange') },
  { label: t('adminSys.delete'), value: 'delete', color: cv('red') },
  { label: t('adminSys.k36'), value: 'restore', color: cv('blue') },
];

const activeActionFilter = ref('all');
const userFilter = ref('');
const spaceFilter = ref('');
const dateRange = ref({ start: '', end: '' });

const userFilterOptions = ref([{ label: t('adminSys.k137'), value: '' }]);

async function loadUserFilterOptions() {
  try {
    const res = await getUsers({ page: 1, page_size: 100 });
    const users = res.data || [];
    userFilterOptions.value = [
      { label: t('adminSys.k137'), value: '' },
      ...users.map((u: User) => ({ label: u.given_name || u.username, value: String(u.id) })),
    ];
  } catch {
    const users = new Map<string, string>();
    activities.value.forEach(a => {
      if (a.username) {
        users.set(a.username, a.given_name || a.username);
      }
    });
    userFilterOptions.value = [{ label: t('adminSys.k137'), value: '' }, ...Array.from(users.entries()).map(([username, name]) => ({ label: name, value: username }))];
  }
}

const spaceFilterOptions = computed(() => {
  const spaces = new Set(activities.value.map(a => a.space_name).filter(Boolean));
  return [{ label: t('adminSys.k138'), value: '' }, ...Array.from(spaces).map(s => ({ label: s!, value: s! }))];
});

const activityCounts = computed(() => ({
  create: activities.value.filter(a => a.action === 'create' || a.action === '1').length,
  edit: activities.value.filter(a => a.action === 'edit' || a.action === '2').length,
  delete: activities.value.filter(a => a.action === 'delete' || a.action === '3').length,
  restore: activities.value.filter(a => a.action === 'restore' || a.action === '4').length,
}));

const actionNumMap: Record<string, string> = { create: '1', edit: '2', delete: '3', restore: '4' };

const filteredActivities = computed(() => {
  let result = activities.value;
  if (activeActionFilter.value !== 'all') {
    const target = activeActionFilter.value;
    const numTarget = actionNumMap[target];
    result = result.filter(a => a.action === target || a.action === numTarget);
  }
  if (userFilter.value) {
    result = result.filter(a => String(a.user_id) === userFilter.value);
  }
  if (spaceFilter.value) {
    result = result.filter(a => a.space_name === spaceFilter.value);
  }
  if (dateRange.value.start) {
    result = result.filter(a => timestampToDateStr(a.create_time) >= dateRange.value.start);
  }
  if (dateRange.value.end) {
    result = result.filter(a => timestampToDateStr(a.create_time) <= dateRange.value.end);
  }
  return result;
});

function timestampToDateStr(timestamp: number): string {
  if (!timestamp) return '';
  const d = new Date(timestamp * 1000);
  const year = d.getFullYear();
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

const hasMoreActivities = computed(() => activities.value.length < activityTotal.value);
const hasMoreSystemLogs = computed(() => systemLogs.value.length < systemLogTotal.value);

const { loading, execute: execLoadActivities } = useRequest(
  async (append: boolean = false) => {
    const params: Record<string, unknown> = { page: page.value, page_size: pageSize.value };
    if (activeActionFilter.value !== 'all') {
      params.action = activeActionFilter.value;
    }
    const res = await getUserActivity(params);
    return { res, append };
  },
  {
    onSuccess: ({ res, append }) => {
      if (append) {
        activities.value.push(...(res.data || []));
      } else {
        activities.value = res.data || [];
        activityTotal.value = res.total || 0;
      }
    },
    onError: () => {
      ui.showToast(t('adminSys.k151'), 'error');
    } },
);

const { execute: execLoadSystemLogs } = useRequest(
  async (append: boolean = false) => {
    const res = await getSystemLogs(level.value, page.value, pageSize.value, systemUsernameFilter.value || undefined, systemMessageFilter.value || undefined);
    return { res, append };
  },
  {
    onSuccess: ({ res, append }) => {
      if (append) {
        systemLogs.value.push(...(res.data || []));
      } else {
        systemLogs.value = res.data || [];
        systemLogTotal.value = res.total || 0;
      }
    },
    onError: () => {
      ui.showToast(t('adminSys.k152'), 'error');
    } },
);

const tabs = [
  { value: 'activity' as const, label: t('adminSys.k150') },
  { value: 'system' as const, label: t('adminSys.k148') },
];

function loadMoreActivities() {
  page.value++;
  execLoadActivities(true);
}

function loadMoreSystemLogs() {
  page.value++;
  execLoadSystemLogs(true);
}

function getSystemLogColor(level: string | number): string {
  const lvl = String(level);
  if (lvl === 'ERROR' || lvl === '3') return cv('red');
  if (lvl === 'WARN' || lvl === '4') return cv('orange');
  if (lvl === 'INFO' || lvl === '6') return cv('green');
  if (lvl === 'DEBUG' || lvl === '7') return cv('text-tertiary');
  return cv('blue');
}

function handleUserClick(item: UserActivity) {
  if (item.user_id) {
    router.push(`/user/${item.user_id}`);
  }
}

function formatLevel(level: string | number): string {
  const levelMap: Record<string, string> = {
    'ERROR': 'ERROR',
    'WARN': 'WARN',
    'INFO': 'INFO',
    'DEBUG': 'DEBUG',
    '3': 'ERROR',
    '4': 'WARN',
    '6': 'INFO',
    '7': 'DEBUG',
  };
  return levelMap[String(level)] || String(level);
}

function handleActivityClick(item: UserActivity) {
  const spaceId = item.space_id || 0;
  const docId = item.document_id;
  if (docId) {
    router.push(`/space/${spaceId}/doc/${docId}`);
  }
}

const showLogDetail = shallowRef(false);
const logDetail = ref<SystemLog | null>(null);

const maskedGetParams = computed(() => {
  if (!logDetail.value?.get_params) return '';
  return maskSensitiveData(logDetail.value.get_params);
});

const maskedPostParams = computed(() => {
  if (!logDetail.value?.post_params) return '';
  return maskSensitiveData(logDetail.value.post_params);
});

async function handleSystemLogClick(log: SystemLog) {
  try {
    logDetail.value = await getLogDetail(log.log_id);
  } catch {
    logDetail.value = log;
  }
  showLogDetail.value = true;
}

function getLevelLabelStyle(level: string | number) {
  const lvl = String(level);
  let bg = cv('text-quaternary');
  let color= cv('on-accent');
  if (lvl === 'ERROR' || lvl === '3') { bg = cv('red'); color= cv('on-accent'); }
  else if (lvl === 'WARN' || lvl === '4') { bg = cv('orange'); color= cv('on-accent'); }
  else if (lvl === 'INFO' || lvl === '6') { bg = cv('blue'); color= cv('on-accent'); }
  else if (lvl === 'DEBUG') { bg = cv('text-tertiary'); color= cv('on-accent'); }
  return {
    display: 'inline-block',
    padding: `${AppleSpace.xxs} ${AppleSpace.xs}`,
    borderRadius: AppleRadius.small,
    background: bg,
    color,
    fontSize: AppleType.microUI.size,
    fontWeight: String(AppleFontWeight.semibold),
    fontFamily: AppleFont.mono,
    flexShrink: 0,
  };
}

function compactFilterToolbarStyle(): Record<string, string> {
  return {
    display: 'flex',
    alignItems: 'flex-end',
    gap: AppleSpace.sm,
    marginBottom: AppleSpace.md,
    flexWrap: 'wrap' as const,
    padding: AppleSpace.sm,
    background: cv('bg-secondary'),
    borderRadius: AppleRadius.card,
  };
}

const compactLoadMoreContainerStyle = {
  display: 'flex',
  justifyContent: 'center',
  padding: `${AppleSpace.md} 0`,
};

const compactTimelineEntryHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  flexWrap: 'wrap' as const,
};

const compactTimelineEntryMetaStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  flexWrap: 'wrap' as const,
};

const logDetailStyle = { display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
};
const logDetailRowStyle = { display: 'flex', alignItems: 'flex-start', gap: AppleSpace.sm };
const logDetailLabelStyle = { fontSize: AppleType.microUI.size, color: cv('text-secondary'), minWidth: AppleSpace['4xl'], flexShrink: 0, paddingTop: AppleSpace.xxs };
const logDetailValueStyle = { fontSize: AppleType.controlLabel.size, color: cv('text-primary'), flex: 1 };
const logDetailCodeStyle = { fontFamily: AppleFont.mono, fontSize: AppleType.microUI.size, background: cv('bg-tertiary'), padding: `${AppleSpace.xs} ${AppleSpace.sm}`, borderRadius: AppleRadius.small, color: cv('text-primary'), whiteSpace: 'pre-wrap' as const, wordBreak: 'break-all' as const, flex: 1 };

async function loadActivities() {
  await execLoadActivities(false);
}

async function loadSystemLogs() {
  page.value = 1;
  await execLoadSystemLogs(false);
}

watch(activeTab, (newTab) => {
  page.value = 1;
  activityTotal.value = 0;
  systemLogTotal.value = 0;
  if (newTab === 'activity') {
    systemLogs.value = [];
    loadActivities();
  } else {
    activities.value = [];
    loadSystemLogs();
  }
});

// ApSelect 只 emit update:modelValue，不 emit change —— 靠 watch 驱动重拉，别用 @change（会静默失效）
watch(level, () => loadSystemLogs());

onMounted(() => {
  loadActivities();
  loadUserFilterOptions();
});
</script>
