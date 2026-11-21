<template>
  <ApPageContainer
    :title="$t('ws.k142')"
    :subtitle="$t('ws.k145')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <template #actions>
      <ApSelect
        v-model="selectedSpaceId"
        :options="spaceOptions"
        :placeholder="$t('ws.k73')"
        size="small"
        searchable
      />
      <ApButton variant="secondary" size="small" :loading="loading" @click="reload">
        <ApIcon name="RefreshCw" :size="16" /> {{ $t('ws.refresh') }}
      </ApButton>
    </template>

    <div :style="toolbarStyle">
      <ApSegmentedControl v-model="viewMode" :options="viewOptions" size="small" />
      <ApSelect
        v-if="viewMode === 'list'"
        v-model="statusFilter"
        :options="statusOptions"
        size="small"
      />
    </div>

    <ApEmpty
      v-if="loadError"
      type="error"
      :title="$t('ws.k143')"
      :description="loadError"
      :action-text="$t('lastAttrsUi.retry')"
      @action="reload"
    />

    <!-- 列表视图 -->
    <ApCard v-else-if="viewMode === 'list'" size="small">
      <ApTable :columns="columns" :data="listItems" :loading="loading">
        <template #empty>
          <ApEmpty icon="FileText" :title="$t('ws.k144')" :description="$t('vWorkspaceUi.k14')" />
        </template>
        <template #actions="{ row }">
          <div :style="actionGroupStyle">
            <ApButton variant="ghost" size="small" @click="openDocument(row)">
              {{ $t('ws.k138') }}
            </ApButton>
            <ApButton variant="ghost" size="small" @click="editDocument(row)">
              {{ $t('ws.edit') }}
            </ApButton>
          </div>
        </template>
      </ApTable>
      <div v-if="total > 0" :style="{ marginTop: AppleSpace.md }">
        <ApPagination :page="page" :page-size="pageSize" :total="total" @change="handlePageChange" />
      </div>
    </ApCard>

    <!-- 看板视图：按发布状态分列 -->
    <div v-else-if="viewMode === 'kanban'" :style="layoutContentGridTwoColumn">
      <ApCard
        v-for="column in kanbanColumns"
        :key="column.key"
        size="small"
        :accent-color="column.accent"
      >
        <template #header>
          <div :style="cardHeaderStyle">
            <span :style="sectionTitleStyle">{{ column.title }}</span>
            <span :style="totalStyle">{{ column.items.length }}</span>
          </div>
        </template>
        <div v-if="loading" :style="loadingTextStyle">{{ $t('ws.k140') }}</div>
        <ApEmpty v-else-if="column.items.length === 0" icon="FileText" :title="`暂无${column.title}文档`" />
        <div v-else :style="listStyle(AppleSpace.sm)">
          <router-link
            v-for="item in column.items"
            :key="item.id"
            :to="{ name: 'document', params: { spaceId: item.space_id || activeSpaceId, docId: item.id } }"
            :style="[listItemStyle, { textDecoration: 'none' }]"
          >
            <div :style="cardTitleStyle">{{ item.name }}</div>
            <div :style="cardMetaStyle">
              <span :style="metaItemStyle">
                <ApIcon :name="item.type === 2 ? 'Folder' : 'FileText'" :size="14" />
                {{ item.type === 2 ? $t('ws.docTypeDirectory') : $t('ws.docTypePage') }}
              </span>
              <span :style="timeStyle">{{ formatTimeAgo(item.update_time) }}</span>
            </div>
          </router-link>
        </div>
      </ApCard>
    </div>

    <!-- 画廊视图 -->
    <div v-else :style="appleCardGrid('entity')">
      <ApCard
        v-for="item in galleryItems"
        :key="item.id"
        size="small"
        hoverable
        @click="openDocument(item)"
      >
        <div :style="galleryTileStyle(item)">
          <ApIcon :name="item.type === 2 ? 'Folder' : 'FileText'" :size="20" :color="galleryAccent(item)" />
        </div>
        <div :style="cardTitleStyle">{{ item.name }}</div>
        <div :style="cardMetaStyle">
          <ApStatusBadge
            :status="item.status === 1 ? 'online' : 'default'"
            size="small"
            :label="item.status === 1 ? $t('ws.k153') : $t('ws.k152')"
          />
          <span :style="timeStyle">{{ item.space_name || currentSpaceName }}</span>
        </div>
      </ApCard>
      <ApEmpty v-if="!loading && galleryItems.length === 0" icon="FileText" :title="$t('ws.k144')" />
    </div>

    <p :style="{ ...metaItemStyle, marginTop: AppleSpace.lg }">
      {{ $t('ws.k141') }}
    </p>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { computed, onMounted, ref, shallowRef, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { logger } from '@/utils/logger';
import { AppleLayout, AppleRadius, AppleSpace, cv, appleCardGrid } from '@/utils/core/apple-tokens';
import { formatTimeAgo } from '@/utils/format/format-time';
import {
  actionGroupStyle,
  cardHeaderStyle,
  cardMetaStyle,
  cardTitleStyle,
  layoutContentGridTwoColumn,
  listItemStyle,
  listStyle,
  loadingTextStyle,
  metaItemStyle,
  sectionTitleStyle,
  timeStyle,
  toolbarStyle,
  totalStyle,
} from '@/styles/layout/page-styles';
import {
  getDocumentGalleryView,
  getDocumentKanbanView,
  getDocumentListView,
  type DocumentKanbanGroups,
  type DocumentViewItem,
} from '@/api/modules/document/document-view';
import { getMySpaces } from '@/api/modules/space/space';
import type { Space } from '@/types/models';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import type { SelectOption } from '@/components/apple/ApSelect.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApTable from '@/components/apple/ApTable.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApPagination from '@/components/apple/ApPagination.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue';

type ViewMode = 'list' | 'kanban' | 'gallery';

const route = useRoute();
const router = useRouter();

const viewOptions = [
  { label: t('ws.k146'), value: 'list' },
  { label: t('ws.k147'), value: 'kanban' },
  { label: t('ws.k148'), value: 'gallery' },
];

const statusOptions: SelectOption[] = [
  { label: t('ws.k149'), value: -1 },
  { label: t('ws.k150'), value: 0 },
  { label: t('ws.k151'), value: 1 },
];

const viewMode = ref<ViewMode>('list');
// ApSelect / ApSegmentedControl 的 v-model 载荷是 string | number，统一用宽类型 + Number() 收口
const statusFilter = ref<number | string>(-1);
const selectedSpaceId = ref<number | string>(Number(route.params.spaceId) || 0);
const spaces = shallowRef<Space[]>([]);
const listItems = shallowRef<DocumentViewItem[]>([]);
const galleryItems = shallowRef<DocumentViewItem[]>([]);
const kanban = shallowRef<DocumentKanbanGroups>({ draft: [], published: [] });
const total = shallowRef(0);
const page = shallowRef(1);
const pageSize = 20;
const loading = shallowRef(false);
const loadError = shallowRef('');
// 首次解析空间时会改写 selectedSpaceId，避免 watcher 与 onMounted 各发一次请求
const booting = shallowRef(true);

const activeSpaceId = computed(() => Number(selectedSpaceId.value) || 0);

const spaceOptions = computed<SelectOption[]>(() =>
  spaces.value.map((s) => ({ label: s.name, value: s.id })),
);

const currentSpaceName = computed(
  () => spaces.value.find((s) => s.id === activeSpaceId.value)?.name || '',
);

const kanbanColumns = computed(() => [
  { key: 'draft', title: t('ws.k152'), items: kanban.value.draft || [], accent: cv('orange') },
  { key: 'published', title: t('ws.k153'), items: kanban.value.published || [], accent: cv('green') },
]);

const columns = computed<TableColumn<DocumentViewItem>[]>(() => [
  { key: 'name', title: t('ws.k154'), ellipsis: true, render: (row) => row.name },
  { key: 'type', title: t('ws.type'), width: '10%', render: (row) => (row.type === 2 ? t('ws.docTypeDirectory') : t('ws.docTypePage')) },
  {
    key: 'status',
    title: t('ws.k156'),
    width: '14%',
    render: (row) => (row.status === 1 ? t('ws.k153') : t('ws.k152')),
  },
  { key: 'space_name', title: t('ws.k33'), width: '16%', render: () => currentSpaceName.value || '--' },
  {
    key: 'update_time',
    title: t('ws.k157'),
    width: '16%',
    render: (row) => formatTimeAgo(row.update_time),
  },
]);

function galleryAccent(item: DocumentViewItem) {
  return item.status === 1 ? cv('green') : cv('orange');
}

function galleryTileStyle(item: DocumentViewItem) {
  return {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    height: AppleLayout.avatarSizeLarge,
    borderRadius: AppleRadius.control,
    background: cv(item.status === 1 ? 'green-bg-subtle' : 'orange-bg-subtle'),
    marginBottom: AppleSpace.sm,
  };
}

async function loadSpaces() {
  try {
    const res = await getMySpaces({ page: 1, page_size: 200 });
    spaces.value = res.data || [];
    const first = spaces.value[0];
    if (!activeSpaceId.value && first) {
      selectedSpaceId.value = first.id;
    }
  } catch (error) {
    logger.error('[DocumentViews] load spaces failed:', error);
  }
}

async function loadData() {
  loadError.value = '';
  if (!activeSpaceId.value) {
    listItems.value = [];
    galleryItems.value = [];
    kanban.value = { draft: [], published: [] };
    total.value = 0;
    return;
  }
  loading.value = true;
  try {
    if (viewMode.value === 'list') {
      const res = await getDocumentListView({
        space_id: activeSpaceId.value,
        status: Number(statusFilter.value) as -1 | 0 | 1,
        page: page.value,
        page_size: pageSize,
      });
      listItems.value = res.data || [];
      total.value = res.total || 0;
    } else if (viewMode.value === 'kanban') {
      kanban.value = await getDocumentKanbanView(activeSpaceId.value);
    } else {
      galleryItems.value = (await getDocumentGalleryView(activeSpaceId.value)) || [];
    }
  } catch (error: unknown) {
    logger.error('[DocumentViews] load failed:', error);
    loadError.value = t('sentUi.k61');
  } finally {
    loading.value = false;
  }
}

function reload() {
  page.value = 1;
  loadData();
}

function handlePageChange(next: number) {
  page.value = next;
  loadData();
}

function openDocument(item: DocumentViewItem) {
  router.push({
    name: 'document',
    params: { spaceId: item.space_id || activeSpaceId.value, docId: item.id },
  });
}

function editDocument(item: DocumentViewItem) {
  router.push({
    name: 'document-edit',
    params: { spaceId: item.space_id || activeSpaceId.value, docId: item.id },
  });
}

watch([viewMode, statusFilter], () => {
  page.value = 1;
  loadData();
});
watch(selectedSpaceId, () => {
  if (!booting.value) reload();
});

onMounted(async () => {
  await loadSpaces();
  booting.value = false;
  loadData();
});
</script>
