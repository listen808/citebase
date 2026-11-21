<template>
  <ApPageContainer
    :title="$t('user.k55')"
    :max-width="AppleLayout.maxContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <div v-if="loading" :style="loadingStyle"><ApSkeleton variant="card" :lines="4" /></div>
    <ApEmpty v-else-if="!user" icon="User" :title="$t('user.k56')" :description="$t('vUserUi.k3')" />
    <template v-else>
      <ApCard :padding="AppleSpace.sm">
        <div :style="infoStyle">
          <div v-if="user.given_name && user.username !== user.given_name" :style="usernameStyle">@{{ user.username }}</div>
          <div :style="metaGridStyle">
            <div v-if="user.email" :style="metaItemStyle">
              <span :style="metaLabelStyle">{{ $t('user.k3') }}</span>
              <span :style="metaValueStyle">{{ user.email }}</span>
            </div>
            <div v-if="user.mobile" :style="metaItemStyle">
              <span :style="metaLabelStyle">{{ $t('user.k51') }}</span>
              <span :style="metaValueStyle">{{ user.mobile }}</span>
            </div>
            <div v-if="user.department" :style="metaItemStyle">
              <span :style="metaLabelStyle">{{ $t('user.k5') }}</span>
              <span :style="metaValueStyle">{{ user.department }}</span>
            </div>
            <div v-if="user.position" :style="metaItemStyle">
              <span :style="metaLabelStyle">{{ $t('user.k6') }}</span>
              <span :style="metaValueStyle">{{ user.position }}</span>
            </div>
            <div v-if="user.location" :style="metaItemStyle">
              <span :style="metaLabelStyle">{{ $t('user.k7') }}</span>
              <span :style="metaValueStyle">{{ user.location }}</span>
            </div>
          </div>
        </div>
      </ApCard>

      <div :style="crudStatsRowStyle('compact')">
        <ApStatCard size="compact" :label="$t('user.k52')" :value="user.following ?? 0" :icon="'UserCheck'" :color="cv('purple')" />
        <ApStatCard size="compact" :label="$t('user.k53')" :value="user.followers ?? 0" :icon="'Users'" :color="cv('blue')" />
      </div>

      <PageContentCard
        v-model:active-tab="activeTab"
        :tabs="tabItems"
        :wrap-with-card="false"
      >
        <div v-if="activeTab === 'info'">
          <div v-if="activityLoading" :style="loadingStyle"><ApSkeleton variant="list" :lines="3" /></div>
          <ApEmpty v-else-if="activities.length === 0" icon="Activity" :title="$t('user.k57')" />
          <div v-else :style="timelineStyle()">
            <div v-for="item in activities" :key="item.id" :style="timelineItemStyle()" @click="handleActivityClick(item)">
              <div :style="timelineNodeStyle(getActionColor(item.action))" />
              <div :style="timelineContentStyle">
                <div :style="timelineEntryHeaderStyle">
                  <ApStatusBadge
                    :status="getActionStatus(item.action)"
                    :label="formatActionName(item.action)"
                    size="small"
                  />
                  <span :style="timelineDocNameStyle" :title="item.document_name || $t('common.document')">{{ item.document_name || $t('common.document') }}</span>
                  <span :style="timeStyle">{{ formatTime(item.create_time) }}</span>
                </div>
                <div :style="timelineEntryMetaStyle">
                  <ApStatusBadge
                    v-if="isCriticalAction(item.action)"
                    status="error"
                    :label="$t('user.k54')"
                    size="small"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <div v-else-if="activeTab === 'following'">
          <div v-if="followingLoading" :style="loadingStyle"><ApSkeleton variant="grid" :count="3" /></div>
          <ApEmpty v-else-if="followingUsers.length === 0" icon="Users" :title="$t('user.k58')" />
          <div v-else :style="followCardListStyle">
            <ApCard
              v-for="fu in followingUsers"
              :key="fu.id"
              variant="default"
              :hoverable="true"
              size="small"
              :style="followUserCardStyle"
              @click="goToUser(fu.target_user?.user_id)"
            >
              <ApAvatar
                :src="fu.target_user?.avatar || ''"
                :text="getFollowUserName(fu)"
                size="small"
              />
              <div :style="followUserInfoStyle">
                <div :style="followUserNameStyle">{{ getFollowUserName(fu) }}</div>
                <div :style="followUserDescStyle">{{ fu.target_user?.email || $t('fallback.k82') }}</div>
              </div>
            </ApCard>
          </div>
        </div>
      </PageContentCard>
    </template>
    <template #actions>
      <div v-if="!loading && user" :style="heroRightInnerStyle">
        <ApStatusBadge
          :status="user.role_id === 1 ? 'online' : user.role_id === 2 ? 'enabled' : 'default'"
          size="small"
          :label="getRoleName(user.role_id)"
        />
        <ApStatusBadge
          :status="getUserStatusBadge(getUserStatus(user))"
          size="small"
          :label="getUserStatusLabel(user)"
        />
        <ApButton
          v-if="!isCurrentUser"
          :variant="user.is_following ? 'ghost' : 'primary'"
          @click="handleToggleFollow"
        >
          {{ user.is_following ? $t('vUserUi.k4') : $t('vUserUi.k5') }}
        </ApButton>
      </div>
    </template>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, watch, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { AppleFont, AppleFontWeight, AppleSpace, AppleType, cv, AppleLayout } from '@/utils/core/apple-tokens';
import { loadingStyle, gridStandard, crudStatsRowStyle, timelineStyle, timelineItemStyle, timelineNodeStyle, timelineContentStyle, timelineEntryHeaderStyle, timelineDocNameStyle, timelineEntryMetaStyle, timeStyle } from '@/styles/layout/page-styles';
import { getUserByID } from '@/api/modules/user/user';
import { getUserActivityById, type UserActivity } from '@/api/modules/system/log';
import type { FollowUser } from '@/api/modules/space/follow';
import type { User as UserType } from '@/types/models';
import { useAuthStore } from '@/stores/auth/auth';
import { useFollowStore } from '@/stores/workspace/follow';
import { useUIStore } from '@/stores/app/ui';
import { getRoleName } from '@/utils/core/constants';
import { getUserStatus, getUserStatusLabel, getUserStatusBadge } from '@/types/user-helpers';
import { formatTime } from '@/utils/format/format-time';
import { formatActionName, getActionStatus, getActionColor, isCriticalAction, isDeleteAction } from '@/utils/activity';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import ApAvatar from '@/components/apple/ApAvatar.vue';
import PageContentCard from '@/components/layout/PageContentCard.vue';

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const followStore = useFollowStore();
const ui = useUIStore();

const userId = computed(() => Number(route.params.userId));
const user = ref<UserType | null>(null);
const loading = shallowRef(true);
const activeTab = ref<'info' | 'following'>('info');
const activities = ref<UserActivity[]>([]);
const followingUsers = computed<FollowUser[]>(() =>
  followStore.followingList.filter((f) => f.type === 2 && f.target_user),
);
const activityLoading = shallowRef(false);
const followingLoading = shallowRef(false);

const isCurrentUser = computed(() => auth.user?.id === userId.value);

const tabItems = [
  { key: 'info', label: t('user.k59') },
  { key: 'following', label: t('user.k60') },
];

async function loadUser() {
  loading.value = true;
  try {
    if (isCurrentUser.value && auth.user) {
      user.value = auth.user;
    } else {
      user.value = await getUserByID(userId.value);
    }
  } catch {
    user.value = null;
  } finally {
    loading.value = false;
  }
}

async function loadActivities() {
  activityLoading.value = true;
  try {
    const res = await getUserActivityById(userId.value, 1, 20);
    activities.value = res.data || [];
  } catch {
    activities.value = [];
  } finally {
    activityLoading.value = false;
  }
}

async function loadFollowing() {
  followingLoading.value = true;
  try {
    await followStore.loadAll(userId.value);
  } finally {
    followingLoading.value = false;
  }
}

watch(activeTab, (tab) => {
  if (tab === 'info') loadActivities();
  if (tab === 'following' && followingUsers.value.length === 0) loadFollowing();
});

watch(() => auth.user, (newUser) => {
  if (isCurrentUser.value && newUser) {
    user.value = newUser;
  }
});

async function handleToggleFollow() {
  if (!user.value) return;
  try {
    const followed = await followStore.toggle('user', userId.value);
    if (followed !== null) {
      const currentFollowers = user.value.followers ?? 0;
      user.value = {
        ...user.value!,
        is_following: followed,
        followers: followed ? currentFollowers + 1 : Math.max(0, currentFollowers - 1),
      };
      ui.showToast(followed ? '已关注' : '已取消关注', 'success');
    }
  } catch {
    ui.showToast(t('user.k61'), 'error');
  }
}

function handleActivityClick(item: UserActivity) {
  if (item.document_id && !isDeleteAction(item.action)) {
    router.push(`/space/${item.space_id || 0}/doc/${item.document_id}`);
  }
}

function goToUser(uid?: number) {
  if (uid) router.push(`/user/${uid}`);
}

function getFollowUserName(fu: FollowUser): string {
  return fu.target_user?.given_name || fu.target_user?.username || t('fallback.k90');
}

onMounted(async () => {
  await Promise.allSettled([loadUser(), loadActivities()]);
});

const heroRightInnerStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.xs, flexWrap: 'wrap' as const };
const infoStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xs };
const usernameStyle = { fontFamily: AppleFont.text, fontSize: AppleType.controlLabel.size, color: cv('text-secondary') };
const metaGridStyle = { ...gridStandard, marginTop: AppleSpace.xxs };
const metaItemStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xxs };
const metaLabelStyle = { fontFamily: AppleFont.text, fontSize: AppleType.microUI.size, color: cv('text-secondary') };
const metaValueStyle = { fontFamily: AppleFont.text, fontSize: AppleType.controlLabel.size, color: cv('text-primary') };

const followCardListStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xs };
const followUserCardStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.sm, cursor: 'pointer' as const };
const followUserInfoStyle = { flex: '1', minWidth: '0' };
const followUserNameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
};
const followUserDescStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  marginTop: AppleSpace.xxs,
};
</script>
