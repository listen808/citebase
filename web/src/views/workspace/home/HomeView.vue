<template>
  <ApPageContainer :max-width="AppleLayout.maxWideContentWidth" compact>
    <!-- 问候区：放进容器的 #header 插槽，页头位置与其余页面同一节奏 -->
    <template #header>
      <PageHeader
        mode="greeting"
        :title="greetingResult.greeting"
        :subtitle="currentDate"
        :quote="greetingResult.quote"
        :actions="[{ label: $t('vWorkspaceUi.k32'), onClick: openCreateSpace, props: { variant: 'primary', size: 'small' } }]"
      />
    </template>

    <!-- 快捷统计：ApStatCard compact，三卡固定 3 列等分 -->
    <div :style="statsRowStyle">
      <ApStatCard size="compact" :label="$t('ws.k27')" :value="totalDocCount" icon="FileText" :color="cv('blue')" />
      <ApStatCard size="compact" :label="$t('ws.k227')" :value="spaceStore.mySpaces.length" icon="Folder" :color="cv('green')" />
      <ApStatCard size="compact" :label="$t('ws.k228')" :value="favTotal" icon="Star" :color="cv('orange')" />
    </div>

    <!-- 我的空间 -->
    <div :style="LayoutStyles.section">
      <div :style="{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: AppleSpace.md }">
        <SectionHeader :title="$t('ws.k232')" :subtitle="$t('vWorkspaceUi.spaceCount', { n: spaceStore.mySpaces.length })" align="left" :style="{ marginBottom: '0' }" />
        <div v-if="spaceStore.mySpaces.length > 1" :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.xs }">
          <ApButton v-if="!sortMode" variant="ghost" size="small" @click="enterSortMode">
            <ApIcon name="ArrowUpNarrowWide" :size="16" />
            {{ $t('ws.sort') }}
          </ApButton>
          <ApButton v-else variant="primary" size="small" :disabled="sortSaving" @click="exitSortMode">
            <ApIcon name="Check" :size="16" />
            {{ sortSaving ? $t('vWorkspaceUi.k27') : $t('vWorkspaceUi.k28') }}
          </ApButton>
        </div>
      </div>
      <div v-if="spaceStore.loading" :style="loadingStyle">
        <ApSkeleton variant="grid" :count="3" />
      </div>
      <ApEmpty v-else-if="!spaceStore.mySpaces.length" icon="Folder" :title="$t('ws.k233')" :description="$t('vWorkspaceUi.k26')" />
      <div v-else :style="spacesGridStyle">
        <ApEntityCard
          v-for="(space, idx) in displaySpaces"
          :key="space.id"
          :title="space.name"
          :subtitle="space.description || undefined"
          @click="!sortMode && navigateToSpace(space.id)"
        >
          <div :style="entityMetaRowStyle()">
            <span v-if="sortMode" :style="sequenceBadgeStyle()">
              {{ idx + 1 }}
            </span>
            <span :style="entityMetaItemStyle()">
              <ApIcon name="Users" :size="12" />
              {{ space.member_count ?? 0 }} {{ $t('vWorkspaceUi.k30') }}
            </span>
            <span :style="entityMetaItemStyle()">
              <ApIcon name="FileText" :size="12" />
              {{ space.doc_count ?? 0 }} {{ $t('vWorkspaceUi.k31') }}
            </span>
          </div>
          <!-- 操作位统一走 ApEntityCard 的 #actions（外层已 @click.stop，这里不必再写）；
               收藏开关本身是全站唯一实现，位置与样式不再各页自定。 -->
          <template #actions>
            <template v-if="sortMode">
              <ApButton
                variant="ghost"
                size="mini"
                icon-only
                :aria-label="$t('ws.k235')"
                :disabled="idx === 0"
                @click="moveSpace(idx, -1)"
              >
                <ApIcon name="ChevronUp" :size="16" />
              </ApButton>
              <ApButton
                variant="ghost"
                size="mini"
                icon-only
                :aria-label="$t('ws.k236')"
                :disabled="idx === displaySpaces.length - 1"
                @click="moveSpace(idx, 1)"
              >
                <ApIcon name="ChevronDown" :size="16" />
              </ApButton>
            </template>
            <ApFavoriteButton
              v-else
              :active="collectionStore.isCollected(2, space.id)"
              :noun="$t('lastAttrsUi.k9')"
              @toggle="toggleSpaceCollect(space.id)"
            />
          </template>
        </ApEntityCard>
      </div>
    </div>

    <!-- 最近访问 -->
    <div :style="LayoutStyles.section">
      <RecentDocs
        :docs="recentDocs"
        :loading="recentLoading"
        :error="recentError"
        @navigate="navigateToDoc"
      />
    </div>

    <!-- 收藏文档 -->
    <div :style="LayoutStyles.section">
      <FavoriteDocs
        :docs="favDocs"
        :loading="favLoading"
        @navigate="navigateToDoc"
      />
    </div>

    <!-- 创建/编辑空间 Modal -->
    <ApModal v-model="showModal" :title="$t('ws.k234')" size="medium" density="compact">
      <div :style="LayoutStyles.formGroup">
        <label :style="LayoutStyles.formLabel">{{ $t('ws.k224') }}</label>
        <ApInput v-model="form.name" :placeholder="$t('ws.k229')" />
      </div>
      <div :style="LayoutStyles.formGroup">
        <label :style="LayoutStyles.formLabel">{{ $t('ws.k225') }}</label>
        <ApTextarea v-model="form.description" :placeholder="$t('ws.k230')" :rows="3" />
      </div>
      <div :style="LayoutStyles.formGroup">
        <label :style="LayoutStyles.formLabel">{{ $t('ws.k24') }}</label>
        <ApInput v-model="form.tags" :placeholder="$t('ws.k231')" />
      </div>
      <div :style="LayoutStyles.formGroup">
        <label :style="LayoutStyles.formLabel">{{ $t('ws.k226') }}</label>
        <ApSelect v-model="form.visit_level" :options="visitLevelOptions" />
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('ws.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :disabled="!form.name.trim() || submitting" @click="handleSubmit">
          {{ submitting ? $t('vWorkspaceUi.k27') : $t('vWorkspaceUi.save') }}
        </ApButton>
      </template>
    </ApModal>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { appIntlLocale } from '@/i18n';
import { ref, shallowRef, computed, onMounted } from 'vue';
import { logger } from '@/utils/logger';
import { useRoute, useRouter } from 'vue-router';
import { cv, AppleType, AppleFontWeight, AppleSpace, AppleLayout, AppleRadius, appleCardGrid, appleFixedColumns } from '@/utils/core/apple-tokens';
import { LayoutStyles, loadingStyle, entityMetaRowStyle, entityMetaItemStyle } from '@/styles/layout/page-styles';
import { useBreakpoint } from '@/utils/responsive';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import ApEntityCard from '@/components/apple/ApEntityCard.vue';
import ApFavoriteButton from '@/components/apple/ApFavoriteButton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApModal from '@/components/apple/ApModal.vue';
import PageHeader from '@/components/layout/PageHeader.vue';
import SectionHeader from '@/components/layout/SectionHeader.vue';
import RecentDocs from '@/components/home/RecentDocs.vue';
import FavoriteDocs from '@/components/home/FavoriteDocs.vue';
import type { FavDoc } from '@/components/home/FavoriteDocs.vue';
import { useSpaceStore } from '@/stores/workspace/space';
import { useUIStore } from '@/stores/app/ui';
import { useFollowStore } from '@/stores/workspace/follow';
import { useCollectionStore } from '@/stores/workspace/collection';
import { getMyCollections } from '@/api/modules/space/collection';
import type { CollectionItem } from '@/api/modules/space/collection';
import { createSpace, sortSpaces } from '@/api/modules/space/space';
import type { SpaceOrder } from '@/api/modules/space/space';
import type { Space } from '@/types/models';
import { useRecentDocs } from '@/composables/ui/useRecentDocs';
import { getTimeGreeting } from '@/utils/format/time-group';


const router = useRouter();
const route = useRoute();
const spaceStore = useSpaceStore();
const { isMobile, isTablet } = useBreakpoint();
const ui = useUIStore();
const followStore = useFollowStore();
const { docs: recentDocs, loading: recentLoading, error: recentError, loadRecentDocs } = useRecentDocs();

const greetingResult = getTimeGreeting();
const showModal = shallowRef(false);
const submitting = shallowRef(false);
const favTotal = shallowRef(0);
const collectionStore = useCollectionStore();
const favDocs = ref<FavDoc[]>([]);
const favLoading = shallowRef(false);
const form = ref({ name: '', description: '', tags: '', visit_level: '0' });

const sortMode = shallowRef(false);
const sortSaving = shallowRef(false);
const localSpaces = ref<Space[]>([]);
const displaySpaces = computed(() => (sortMode.value ? localSpaces.value : spaceStore.mySpaces));

function enterSortMode() {
  localSpaces.value = [...spaceStore.mySpaces];
  sortMode.value = true;
}

async function exitSortMode() {
  if (sortSaving.value) return;
  sortSaving.value = true;
  try {
    const orders: SpaceOrder[] = localSpaces.value.map((s, idx) => ({ id: s.id, sequence: idx + 1 }));
    await sortSpaces(orders);
    ui.showToast(t('ws.k240'), 'success');
    sortMode.value = false;
    spaceStore.invalidateSpacesCache();
    await spaceStore.loadMySpaces();
  } catch {
    ui.showToast(t('ws.k241'), 'error');
  } finally {
    sortSaving.value = false;
  }
}

function moveSpace(idx: number, dir: -1 | 1) {
  const target = idx + dir;
  if (target < 0 || target >= localSpaces.value.length) return;
  const arr = [...localSpaces.value];
  const tmp = arr[idx]!;
  arr[idx] = arr[target]!;
  arr[target] = tmp;
  localSpaces.value = arr;
}

function sequenceBadgeStyle() {
  return {
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    minWidth: '18px',
    height: '18px',
    padding: `0 ${AppleSpace.xs}`,
    borderRadius: AppleRadius.pill,
    background: cv('blue'),
    color: cv('on-blue'),
    fontSize: AppleType.microUI.size,
    fontWeight: AppleFontWeight.semibold,
    fontFamily: AppleType.controlLabel.font,
  };
}

const visitLevelOptions = [
  { label: t('ws.k237'), value: '0' },
  { label: t('ws.k238'), value: '1' },
  { label: t('ws.k239'), value: '2' },
];

const currentDate = computed(() => {
  const now = new Date();
  return now.toLocaleDateString(appIntlLocale(), { year: 'numeric', month: 'long', day: 'numeric', weekday: 'long' });
});

const totalDocCount = computed(() =>
  spaceStore.mySpaces.reduce((sum, s) => sum + (s.doc_count ?? 0), 0),
);

onMounted(async () => {
  if (route.query.msg === 'no_permission') {
    ui.showToast(t('ws.k242'), 'error');
    router.replace({ path: '/', query: {} });
  }
  if (spaceStore.mySpaces.length === 0) {
    await spaceStore.loadMySpaces();
  }
  await Promise.allSettled([
    // 合并收藏文档列表与总数请求：一次调用同时获取 favDocs 与 favTotal
    loadFavDocs(),
    collectionStore.loadCollections({ type: 2, page: 1, page_size: 100 }).then(() => {
      collectionStore.items.forEach(item => collectionStore.collectedMap.set(`2-${item.resource_id}`, true));
    }).catch((e) => {
      logger.error('Failed to load collected spaces:', e);
    }),
    followStore.refreshStats(),
    loadRecentDocs(),
  ]);
});

async function loadFavDocs() {
  favLoading.value = true;
  try {
    const res = await getMyCollections({ type: 1, page: 1, page_size: 5 });
    if (res && res.data) {
      favDocs.value = res.data.map((item: CollectionItem) => ({
        id: item.resource_id,
        name: item.name || '',
        space_id: item.space_id,
        space_name: item.space_name,
      }));
      favTotal.value = res.total;
    }
  } catch (e) {
    logger.error('Failed to load favorite docs:', e);
    ui.showToast(t('ws.k243'), 'error');
  } finally {
    favLoading.value = false;
  }
}

function navigateToSpace(spaceId: number) {
  router.push({ path: `/space/${spaceId}`, query: { from: 'home' } });
}

function navigateToDoc(doc: { id: number; space_id?: number }) {
  if (doc.space_id) {
    router.push(`/space/${doc.space_id}/doc/${doc.id}`);
  } else {
    router.push(`/space/0/doc/${doc.id}`);
  }
}

async function toggleSpaceCollect(spaceId: number) {
  try {
    await collectionStore.toggle(2, spaceId);
  } catch (e) {
    logger.error('Failed to toggle space collection:', e);
    ui.showToast(t('ws.k244'), 'error');
  }
}

function openCreateSpace() {
  form.value = { name: '', description: '', tags: '', visit_level: '0' };
  showModal.value = true;
}

async function handleSubmit() {
  const name = form.value.name.trim();
  if (!name || submitting.value) return;
  if (name.length > 50) {
    ui.showToast(t('ws.k245'), 'error');
    return;
  }
  submitting.value = true;
  try {
    await createSpace({
      name: name,
      description: form.value.description.trim(),
      tags: form.value.tags.trim(),
      visit_level: form.value.visit_level,
    });
    ui.showToast(t('ws.k246'), 'success');
    showModal.value = false;
    spaceStore.invalidateSpacesCache();
    await spaceStore.loadMySpaces();
  } catch {
    ui.showToast(t('ws.k247'), 'error');
  } finally {
    submitting.value = false;
  }
}

// 三张统计卡固定 3 列等分：桌面 auto-fill 会剩一张孤儿卡独占一行（审查 §四.4），
// 窄屏上 stat 地板（200px）又会塌成竖条 —— 两端都不如直接 3 列（审查 §三.3 移动既定 3 列）
const statsRowStyle = {
  display: 'grid',
  gridTemplateColumns: appleFixedColumns(3),
  gap: AppleSpace.sm,
  marginBottom: AppleSpace.md,
};

// 空间卡与文档卡同档（entity 320），并按断点降列数：手机 1 列、平板 2 列、桌面 auto-fill。
// grid-auto-rows:1fr 让同区块的卡片等高——原先这里写了注释却没真设，所以卡片高矮不齐。
const spacesGridStyle = computed(() => {
  if (isMobile.value) {
    return {
      display: 'grid' as const,
      gridTemplateColumns: '1fr',
      gap: AppleSpace.md,
      gridAutoRows: '1fr', alignContent: 'start',
      alignItems: 'stretch' as const,
    };
  }
  if (isTablet.value) {
    return {
      display: 'grid' as const,
      gridTemplateColumns: appleFixedColumns(2),
      gap: AppleSpace.md,
      gridAutoRows: '1fr', alignContent: 'start',
      alignItems: 'stretch' as const,
    };
  }
  return appleCardGrid('entity');
});

</script>


