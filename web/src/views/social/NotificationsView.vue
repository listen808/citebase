<template>
  <ApPageContainer :title="$t('social.k45')" :subtitle="$t('social.k48')" :max-width="AppleLayout.maxWideContentWidth" :show-breadcrumb="true" compact>
    <template #actions>
      <ApButton
        v-if="notificationStore.unreadCount > 0"
        size="small"
        variant="ghost"
        :loading="markingAllRead"
        :disabled="isBusy()"
        @click="handleMarkAllRead"
      >
        {{ $t('social.k40') }}
      </ApButton>
    </template>

    <!-- 统计概览 -->
    <div :style="crudStatsRowStyle('compact')">
      <ApStatCard size="compact" :label="$t('social.k42')" :value="notificationStore.total" :icon="'Bell'" :color="cv('blue')" />
      <ApStatCard size="compact" :label="$t('social.k43')" :value="notificationStore.unreadCount" :icon="'BellRing'" :color="cv('red')" />
      <ApStatCard size="compact" :label="$t('social.k44')" :value="Math.max(0, notificationStore.total - notificationStore.unreadCount)" :icon="'CheckCheck'" :color="cv('green')" />
    </div>

    <!-- 筛选标签：以前是 tabBarStyle 自建的独占行，并回共享工具条（FilterBar #leading），
         与收藏页/关注页同一形态，将来加搜索框不必再另起一行 -->
    <FilterBar>
      <template #leading>
        <!-- 不用 v-model + @change：ApSegmentedControl 只 emit update:modelValue，
             `@change` 会落到根 div 上变成永不触发的原生监听 —— 点「未读」只改状态不重拉列表 -->
        <ApSegmentedControl
          :model-value="activeTab"
          :options="tabs"
          size="small"
          :aria-label="$t('social.k49')"
          @update:model-value="onTabChange"
        />
      </template>
    </FilterBar>

    <!-- 通知列表 -->
    <div v-if="notificationStore.loading" :style="compactLoadingStyle">
      <ApSkeleton variant="card" :count="3" />
    </div>
    <!-- 加载失败必须与「确实没有通知」长成两样（同 DocumentViewsView 的 loadError 形态） -->
    <ApEmpty
      v-else-if="notificationStore.error"
      type="error"
      :title="$t('social.k46')"
      :description="notificationStore.error"
      :action-text="$t('lastAttrsUi.retry')"
      @action="reload"
    />
    <ApEmpty
      v-else-if="notificationStore.items.length === 0"
      icon="Bell"
      :title="$t('social.k47')"
      :description="activeTab === 'unread' ? $t('social.k91') : $t('social.k92')"
    />
    <div v-else :style="listStyle(AppleSpace.sm)">
      <div
        v-for="notif in notificationStore.items"
        :key="notif.id"
        :style="compactItemStyle(notif.is_read === 0)"
        @click="handleClick(notif)"
      >
        <div :style="compactItemHeaderStyle">
          <span :style="typeBadgeStyle(notif.type)">{{ typeLabel(notif.type) }}</span>
          <span :style="timeStyle">{{ formatTimeAgo(notif.create_time) }}</span>
        </div>
        <div :style="itemTitleStyle(notif.is_read === 0)">{{ notif.title }}</div>
        <div v-if="notif.content" :style="itemContentStyle">{{ notif.content }}</div>
        <div :style="compactItemActionsStyle">
          <ApButton
            v-if="notif.is_read === 0"
            variant="ghost"
            size="mini"
            :loading="isBusy(notif.id, 'read')"
            :disabled="isBusy(notif.id) || markingAllRead"
            @click.stop="handleMarkRead(notif.id)"
          >
            {{ $t('social.k41') }}
          </ApButton>
          <ApButton
            variant="ghost"
            size="mini"
            :loading="isBusy(notif.id, 'remove')"
            :disabled="isBusy(notif.id) || markingAllRead"
            @click.stop="handleRemove(notif.id)"
          >
            <ApIcon name="Trash2" :size="16" /> {{ $t('social.delete') }}
          </ApButton>
        </div>
      </div>
    </div>

    <!-- 分页 -->
    <ApPagination
      v-if="notificationStore.total > pageSize"
      :page="page"
      :page-size="pageSize"
      :total="notificationStore.total"
      @change="changePage"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, onMounted, onUnmounted } from 'vue';
import { useRouter } from 'vue-router';
import { cv, AppleSpace, AppleRadius, AppleType, AppleLayout, AppleTransition, AppleFontWeight, cssType } from '@/utils/core/apple-tokens';
import { crudStatsRowStyle, listStyle, compactLoadingStyle } from '@/styles/layout/page-styles';
import FilterBar from '@/components/layout/FilterBar.vue';
import { formatTimeAgo } from '@/utils/format/format-time';
import { useNotificationStore } from '@/stores/social/notification';
import { useUIStore } from '@/stores/app/ui';
import { onNotificationUpdate } from '@/composables/websocket/useNotificationSocket';
import type { NotificationItem } from '@/api/modules/social/notification';

const tabs = [
  { value: 'all', label: t('social.all') },
  { value: 'unread', label: t('social.k43') },
];

const router = useRouter();
const notificationStore = useNotificationStore();
const ui = useUIStore();

const activeTab = ref<'all' | 'unread'>('all');
const page = ref(1);
const pageSize = 20;

// 三个写操作原先既没有请求期锁也没有失败反馈（store 把错误吞掉，页面照样显示成功）。
// 锁按「哪一行的哪个动作」记，形态照 CitebaseCompileView 的 busy/isBusy。
const busy = ref<{ id: number; action: 'read' | 'remove' } | null>(null);
const markingAllRead = ref(false);

/** 不传 action = 这一行有在途写操作 */
function isBusy(id?: number, action?: 'read' | 'remove'): boolean {
  const b = busy.value;
  if (!b) return false;
  if (id === undefined) return true;
  return action ? b.id === id && b.action === action : b.id === id;
}

function reload() {
  notificationStore.loadList({ type: activeTab.value, page: page.value, page_size: pageSize });
  notificationStore.loadUnreadCount();
}

function compactItemStyle(unread: boolean) {
  return {
    padding: AppleSpace.sm,
    borderRadius: AppleRadius.medium,
    border: `1px solid ${unread ? cv('blue') : cv('border-light')}`,
    backgroundColor: unread ? cv('blue-light') : cv('surface-field'),
    cursor: 'pointer',
    transition: AppleTransition.fast,
  };
}

const compactItemHeaderStyle = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginBottom: AppleSpace.xxs,
};

function typeBadgeStyle(type: string) {
  const colors: Record<string, string> = {
    system: cv('text-tertiary'),
    document: cv('blue'),
    comment: cv('green'),
    follow: cv('purple'),
    like: cv('red'),
    workspace: cv('orange'),
  };
  return {
    fontSize: AppleType.microUI.size,
    color: colors[type] || cv('text-secondary'),
    fontWeight: String(AppleFontWeight.semibold),
  };
}

const timeStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

function itemTitleStyle(unread: boolean) {
  return {
    ...cssType(AppleType.bodyPrimary),
    fontWeight: String(unread ? AppleFontWeight.semibold : AppleFontWeight.regular),
    color: cv('text-primary'),
    marginBottom: AppleSpace.xxs,
  };
}

const itemContentStyle = {
  ...cssType(AppleType.microUI),
  color: cv('text-secondary'),
  marginBottom: AppleSpace.xs,
};

const compactItemActionsStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
  marginTop: AppleSpace.xs,
};

function typeLabel(type: string): string {
  const labels: Record<string, string> = {
    system: '系统',
    document: '文档',
    comment: '评论',
    follow: '关注',
    like: '点赞',
    workspace: '空间',
  };
  return labels[type] || type;
}

function onTabChange(value: string | number) {
  activeTab.value = value === 'unread' ? 'unread' : 'all';
  page.value = 1;
  notificationStore.loadList({ type: activeTab.value, page: page.value, page_size: pageSize });
}

function changePage(newPage: number) {
  page.value = newPage;
  notificationStore.loadList({ type: activeTab.value, page: page.value, page_size: pageSize });
}

async function handleMarkRead(id: number) {
  if (isBusy(id)) return;
  busy.value = { id, action: 'read' };
  try {
    await notificationStore.markRead(id);
  } catch {
    ui.showToast(t('social.k50'), 'error');
  } finally {
    busy.value = null;
  }
}

async function handleRemove(id: number) {
  if (isBusy(id)) return;
  busy.value = { id, action: 'remove' };
  try {
    await notificationStore.remove(id);
    ui.showToast(t('social.k51'), 'success');
  } catch {
    ui.showToast(t('social.k52'), 'error');
  } finally {
    busy.value = null;
  }
}

async function handleMarkAllRead() {
  if (markingAllRead.value || isBusy()) return;
  markingAllRead.value = true;
  try {
    await notificationStore.markAllRead();
    ui.showToast(t('social.k53'), 'success');
  } catch {
    ui.showToast(t('social.k54'), 'error');
  } finally {
    markingAllRead.value = false;
  }
}

function handleClick(notif: NotificationItem) {
  if (notif.is_read === 0) {
    // 不 await：标记已读的成败由 handleMarkRead 自己弹提示，跳转不该等这一次往返
    void handleMarkRead(notif.id);
  }
  const rid = notif.related_id;
  if (!rid) return;
  switch (notif.type) {
    case 'document':
    case 'comment':
      router.push(`/space/0/doc/${rid}`);
      break;
    case 'workspace':
      router.push(`/space/${rid}`);
      break;
    case 'follow':
    case 'like':
      router.push(`/user/${rid}`);
      break;
    case 'system':
    default:
      break;
  }
}

onMounted(() => {
  reload();
});

// 实时接收后端 NotifyUser / NotifyUnreadCount 推送，刷新列表与未读数
const offWs = onNotificationUpdate(() => {
  reload();
});

onUnmounted(() => {
  offWs();
});
</script>
