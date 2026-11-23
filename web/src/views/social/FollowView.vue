<template>
  <ApPageContainer
    :title="$t('social.k33')"
    :subtitle="$t('social.k35')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <!-- 紫色主题统计 -->
    <div :style="crudStatsRowStyle('compact')">
      <ApStatCard size="compact" :label="$t('social.k31')" :value="stats.following" :color="cv('purple')" :icon="'UserCheck'" />
      <ApStatCard size="compact" :label="$t('social.k32')" :value="followingDocCount" :color="cv('blue')" :icon="'FileText'" />
    </div>

    <!-- 切换以前独占一行（一行里只有它自己），现在并进搜索那一行的开头：
         左边是"看哪些"，右边是"怎么看 / 搜什么" -->
    <FilterBar
      v-model:search-value="searchKeyword"
      :show-search="true"
      :search-placeholder="$t('social.k37')"
    >
      <template #leading>
        <ApSegmentedControl
          v-model="activeTab"
          :options="tabs.map(t => ({ label: t.label, value: t.key }))"
          size="small"
          :aria-label="$t('social.k36')"
        />
      </template>
    </FilterBar>

    <!-- 关注用户 / 关注的文档：与联系人、收藏等实体列表同一实现（ApEntityCard row），
         原来 <ApCard> 上硬叠 display:flex 的做法永远压不过组件内部的
         flexDirection:column（bodyStyle 也是列容器），行内容整列竖排、全不对齐 -->
    <div v-if="loading" :style="loadingStyle"><ApSkeleton variant="grid" :count="3" /></div>
    <ApEmpty
      v-else-if="currentList.length === 0"
      :type="searchKeyword ? 'search' : 'list'"
      :title="searchKeyword ? '未找到与「' + searchKeyword + '」相关的结果' : emptyText"
    />
    <div v-else :style="followCardListStyle()">
      <ApEntityCard
        v-for="item in currentList"
        :key="item.id"
        layout="row"
        :title="getItemName(item)"
        :subtitle="getItemDesc(item) || undefined"
        :letter="activeTab === 'following' ? getItemName(item).trim().slice(0, 1).toUpperCase() : undefined"
        :icon="activeTab === 'documents' ? 'FileText' : undefined"
        :icon-color="activeTab === 'documents' ? cv('blue') : undefined"
        @click="handleClick(item)"
      >
        <template #actions>
          <ApButton
            variant="ghost"
            size="mini"
            icon-only
            :tooltip="$t('social.k34')"
            :aria-label="$t('social.k34')"
            @click.stop="handleUnfollow(item)"
          >
            <ApIcon name="UserMinus" :size="16" :style="{ color: cv('red') }" />
          </ApButton>
        </template>
      </ApEntityCard>
    </div>

    <ApConfirmModal
      v-model="showUnfollowModal"
      :title="$t('social.k34')"
      :message="$t('vSocialUi.k2')"
      :confirm-text="$t('social.confirm')"
      type="danger"
      density="compact"
      @confirm="confirmUnfollow"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { cv, AppleLayout, AppleSpace } from '@/utils/core/apple-tokens';
import { loadingStyle, crudStatsRowStyle } from '@/styles/layout/page-styles';
import { type FollowUser } from '@/api/modules/space/follow';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import ApEntityCard from '@/components/apple/ApEntityCard.vue';
import ApButton from '@/components/apple/ApButton.vue';
import FilterBar from '@/components/layout/FilterBar.vue';
import { useFollowStore } from '@/stores/workspace/follow';
import { useUIStore } from '@/stores/app/ui';
import { logger } from '@/utils/logger';

const router = useRouter();
const ui = useUIStore();
const followStore = useFollowStore();

const showUnfollowModal = shallowRef(false);
const unfollowTarget = ref<FollowUser | null>(null);
const searchKeyword = ref('');
const activeTab = ref<'following' | 'documents'>('following');
const followingList = computed(() => followStore.followingList);
const stats = computed(() => followStore.stats);
const loading = computed(() => followStore.loading);

const followingDocCount = computed(() => followingList.value.filter((f) => f.type === 1 && f.target_doc).length);

const tabs = [
  { key: 'following' as const, label: t('social.k38'), value: 'following' },
  { key: 'documents' as const, label: t('social.k32'), value: 'documents' },
];

const currentList = computed(() => {
  let list: FollowUser[];
  if (activeTab.value === 'following') {
    list = followingList.value.filter((f) => f.type === 2 && f.target_user);
  } else {
    list = followingList.value.filter((f) => f.type === 1 && f.target_doc);
  }
  if (!searchKeyword.value.trim()) return list;
  const keyword = searchKeyword.value.trim().toLowerCase();
  return list.filter((item) => getItemName(item).toLowerCase().includes(keyword));
});

const emptyText = computed(() => {
  if (activeTab.value === 'following') return t('social.k89');
  return t('social.k90');
});

function getItemName(item: FollowUser) {
  if (item.type === 2 && item.target_user) return item.target_user.given_name || item.target_user.username;
  if (item.type === 1 && item.target_doc) return item.target_doc.name;
  return t('time.unknown');
}

function getItemDesc(item: FollowUser) {
  if (item.type === 1 && item.target_doc) {
    return item.target_doc.space_name || `空间#${item.target_doc.space_id}`;
  }
  if (item.type === 2 && item.target_user) {
    return item.target_user.email || '';
  }
  return '';
}

function handleClick(item: FollowUser) {
  if (item.type === 1 && item.target_doc) {
    router.push(`/space/${item.target_doc.space_id || 0}/doc/${item.target_doc.document_id}`);
  } else if (item.type === 2 && item.target_user) {
    router.push(`/user/${item.target_user.user_id}`);
  }
}

async function handleUnfollow(item: FollowUser) {
  unfollowTarget.value = item;
  showUnfollowModal.value = true;
}

async function confirmUnfollow() {
  const item = unfollowTarget.value;
  if (!item) return;
  try {
    if (item.type === 2 && item.target_user) {
      await followStore.toggle('user', item.target_user.user_id);
    } else if (item.type === 1 && item.target_doc) {
      await followStore.toggle('doc', item.target_doc.document_id);
    }
    showUnfollowModal.value = false;
    await loadData();
  } catch (e) {
    ui.showToast(t('social.k39'), 'error');
    logger.error('Failed to unfollow:', e);
  }
}

async function loadData() {
  await followStore.loadAll();
}

onMounted(loadData);

function followCardListStyle() {
  return {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: AppleSpace.sm,
  };
}

</script>

