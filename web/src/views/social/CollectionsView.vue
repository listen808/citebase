<template>
  <ApPageContainer
    :title="$t('social.k18')"
    :subtitle="$t('social.k20')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <!-- 一行装下"有多少 + 看哪些 + 怎么看 + 搜什么"：
         以前这三件事各占一行，2366 宽实测 chrome 217px，每行只用到版心的 10~13% -->
    <FilterBar
      v-model:view-mode="viewMode"
      v-model:search-value="searchKeyword"
      :show-search="true"
      :search-placeholder="$t('social.k23')"
    >
      <template #leading>
        <ApStatChip :icon="'Bookmark'" :color="cv('orange')" :label="$t('social.k15')" :value="totalCount" />
        <ApSegmentedControl v-model="activeTab" :options="tabs" size="small" :aria-label="$t('social.k22')" />
      </template>
    </FilterBar>

    <div v-if="loading" :style="compactLoadingStyle">
      <ApSkeleton variant="card" :lines="3" />
      <ApSkeleton variant="card" :lines="3" />
    </div>
    <ApEmpty
      v-else-if="items.length === 0"
      :type="searchKeyword ? 'search' : 'list'"
      :title="searchKeyword ? '未找到匹配的收藏内容' : '暂无收藏内容'"
      :description="searchKeyword ? '' : '去浏览空间，发现感兴趣的内容'"
    />

    <!-- 按空间分组的卡片视图 -->
    <template v-else-if="viewMode === 'card'">
      <div v-for="[spaceName, groupItems] in groupedBySpace" :key="spaceName" :style="compactGroupSectionStyle()">
        <div :style="compactGroupHeaderStyle()" @click="toggleGroup(spaceName)">
          <div :style="groupTitleWrapperStyle()">
            <ApIcon name="Folder" :size="14" :style="{ color: cv('blue') }" />
            <span :style="groupTitleStyle()">{{ spaceName }}</span>
            <span :style="groupCountStyle()">{{ groupItems.length }}</span>
          </div>
          <ApIcon
            name="ChevronDown"
            :size="14"
            :style="{
              color: cv('text-tertiary'),
              transition: AppleTransition.default,
              transform: collapsedGroups.has(spaceName) ? 'rotate(-90deg)' : 'rotate(0deg)',
            }"
          />
        </div>
        <div
          v-show="!collapsedGroups.has(spaceName)"
          :style="collectionGridStyle"
        >
          <ApEntityCard
            v-for="item in groupItems"
            :key="item.id"
            :title="item.name ?? $t('fallback.k77')"
            :icon="item.type === 1 ? 'FileText' : 'Table'"
            :icon-color="getItemColor(item)"
            @click="handleClick(item)"
          >
            <template #actions>
              <!-- 这里的项目必然已收藏，所以开关一律 active；红色斜线星（StarOff）读起来像出错，
                   且与首页空间卡那颗星不是同一种语言，统一交给 ApFavoriteButton -->
              <ApFavoriteButton active :noun="item.type === 1 ? $t('social.k95') : $t('social.k96')" @toggle="handleUnfavorite(item)" />
            </template>
            <ApStatusBadge
              v-if="item.type === 1"
              status="default"
              size="small"
              :label="$t('social.k16')"
            />
            <ApStatusBadge
              v-else
              status="enabled"
              size="small"
              :label="$t('social.k17')"
            />
            <span>{{ formatCollectionTime(item.create_time) }}</span>
          </ApEntityCard>
        </div>
      </div>
    </template>

    <!-- 表格视图 -->
    <ApTable v-else density="compact" :columns="columns" :data="filteredItems">
      <template #actions="{ row: item }">
        <div :style="actionGroupStyle">
          <ApButton variant="ghost" size="small" @click="handleTableRowClick(item)">{{ $t('social.k14') }}</ApButton>
          <!-- 取消收藏不是破坏性动作，原先用 variant="danger" 的「取消」文字按钮，
               与卡片视图那颗星是两种语言；这里也走同一个开关 -->
          <ApFavoriteButton active :noun="item.type === 1 ? $t('social.k95') : $t('social.k96')" @toggle="handleTableRowUnfavorite(item)" />
        </div>
      </template>
    </ApTable>

    <ApPagination
      v-if="!loading && items.length > 0"
      :page="page"
      :page-size="pageSize"
      :total="total"
      @change="changePage"
    />

    <ApConfirmModal
      v-model="showRemoveModal"
      :title="$t('social.k19')"
      :message="$t('vSocialUi.k1')"
      :confirm-text="$t('social.confirm')"
      type="danger"
      density="compact"
      @confirm="confirmRemove"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted, onBeforeUnmount, watch } from 'vue';
import { useRouter } from 'vue-router';
import { useCollectionStore } from '@/stores/workspace/collection';
import { useUIStore } from '@/stores/app/ui';
import { logger } from '@/utils/logger';
import { debounce } from '@/utils/core/shared';
import { cv, AppleLayout, AppleSpace, AppleTransition, AppleRadius, appleCardGrid } from '@/utils/core/apple-tokens';
import { groupTitleWrapperStyle, groupTitleStyle, groupCountStyle, compactLoadingStyle } from '@/styles/layout/page-styles';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApStatChip from '@/components/apple/ApStatChip.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApPagination from '@/components/apple/ApPagination.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApFavoriteButton from '@/components/apple/ApFavoriteButton.vue';
import ApTable from '@/components/apple/ApTable.vue';
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue';
import FilterBar from '@/components/layout/FilterBar.vue';
import type { CollectionItem } from '@/api/modules/space/collection';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import { formatDateTime } from '@/utils/format/format-time';

const router = useRouter();
/** 统计行与全站统一：compact 网格（auto-fit 收缩空轨道，单卡不铺满整行） */
const collectionStore = useCollectionStore();
const ui = useUIStore();
const items = computed(() => collectionStore.items);
const total = computed(() => collectionStore.total);
const loading = computed(() => collectionStore.loading);
const page = shallowRef(1);
const pageSize = 20;
const activeTab = shallowRef(0);
const searchKeyword = shallowRef('');
const viewMode = shallowRef<'card' | 'table'>('card');

const showRemoveModal = shallowRef(false);
const removeTarget = ref<CollectionItem | null>(null);
const collapsedGroups = ref<Set<string>>(new Set());
let loadAbortController: AbortController | null = null;

const tabs = [
  { label: t('social.all'), value: 0 },
  { label: t('social.k16'), value: 1 },
  { label: t('social.k17'), value: 2 },
];

const totalCount = computed(() => collectionStore.total);

function getItemColor(item: CollectionItem) {
  return item.type === 1 ? cv('blue') : cv('green');
}

function toggleGroup(spaceName: string) {
  const set = new Set(collapsedGroups.value);
  if (set.has(spaceName)) {
    set.delete(spaceName);
  } else {
    set.add(spaceName);
  }
  collapsedGroups.value = set;
}

const groupedBySpace = computed(() => {
  const groups = new Map<string, CollectionItem[]>();
  filteredItems.value.forEach((item) => {
    const spaceName = item.space_name || t('fallback.k72');
    if (!groups.has(spaceName)) {
      groups.set(spaceName, []);
    }
    groups.get(spaceName)!.push(item);
  });
  return Array.from(groups.entries());
});

async function loadData() {
  loadAbortController?.abort();
  loadAbortController = new AbortController();
  const params: Record<string, number | string> = { page: page.value, page_size: pageSize };
  if (activeTab.value !== 0) params.type = activeTab.value;
  if (searchKeyword.value.trim()) params.keyword = searchKeyword.value.trim();
  try {
    await collectionStore.loadCollections(params, loadAbortController.signal);
  } catch (e) {
    if ((e as Error)?.name === 'AbortError') return;
    ui.showToast(t('social.k28'), 'error');
    logger.error('Failed to load collections:', e);
  }
}

const debouncedSearch = debounce(() => loadData(), 300);

function handleClick(item: CollectionItem) {
  if (item.type === 1) {
    router.push(`/space/${item.space_id || 0}/doc/${item.resource_id}`);
  } else {
    router.push(`/space/${item.resource_id}`);
  }
}

async function handleUnfavorite(item: CollectionItem) {
  removeTarget.value = item;
  showRemoveModal.value = true;
}

async function confirmRemove() {
  const item = removeTarget.value;
  if (!item) return;
  try {
    await collectionStore.toggle(item.type, item.resource_id);
    ui.showToast(t('social.k29'), 'success');
    showRemoveModal.value = false;
    await loadData();
  } catch (e) {
    ui.showToast(t('social.k30'), 'error');
    logger.error('Failed to remove collection:', e);
  }
}

watch(activeTab, () => { page.value = 1; loadData(); });
watch(searchKeyword, () => { page.value = 1; debouncedSearch(); });
onMounted(() => { loadData(); });
onBeforeUnmount(() => {
  loadAbortController?.abort();
  debouncedSearch.cancel();
});

function renderType(item: CollectionItem) {
  return item.type === 1 ? '文档' : '空间';
}

function renderSpaceName(item: CollectionItem) {
  return item.space_name || '-';
}

// P2-9：toLocaleString 的月份/日期不补零，卡片与表格里会出现 2026/10/9 与 2026/10/10 混排
function formatCollectionTime(input: Parameters<typeof formatDateTime>[0]) {
  return formatDateTime(input, 'YYYY/MM/DD HH:mm');
}

function renderTime(item: CollectionItem) {
  return formatCollectionTime(item.create_time);
}

const filteredItems = computed(() => items.value);

function changePage(newPage: number) {
  page.value = newPage;
  loadData();
}

const columns: TableColumn<CollectionItem>[] = [
  { key: 'name', title: t('social.name') },
  { key: 'type', title: t('social.type'), render: renderType },
  { key: 'space_name', title: t('social.k26'), render: renderSpaceName },
  { key: 'create_time', title: t('social.k27'), render: renderTime },
];

const actionGroupStyle = { display: 'flex', gap: AppleSpace.xs };

function handleTableRowClick(row: CollectionItem) { handleClick(row); }
function handleTableRowUnfavorite(row: CollectionItem) { handleUnfavorite(row); }

function compactGroupSectionStyle() {
  return {
    marginBottom: AppleSpace.md,
  };
}

function compactGroupHeaderStyle() {
  return {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between' as const,
    padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
    background: cv('bg-secondary'),
    borderRadius: AppleRadius.control,
    marginBottom: AppleSpace.sm,
    cursor: 'pointer',
    transition: AppleTransition.smooth,
  };
}

// 「卡片」档就是 entity 档网格，和首页最近文档、空间卡同一档地板、同一个条目组件。
// 原来这一档渲染的是通栏行列表（还带 3px 左缘条），跟「卡片」这个名字对不上，
// 也和首页的卡片完全不是一回事。
const collectionGridStyle = appleCardGrid('entity', AppleSpace.sm);

</script>
