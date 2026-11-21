<template>
  <ApPageContainer
    :title="$t('ws.k284')"
    :subtitle="$t('ws.k290')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <template #actions>
      <ApButton variant="primary" size="small" @click="openHierarchyModal()">
        <ApIcon name="Plus" :size="16" /> {{ $t('ws.k257') }}
      </ApButton>
    </template>

    <div :style="listStyle()">
      <!-- 标签池 -->
      <ApCard size="small">
        <template #header>
          <div :style="headerRowStyle">
            <div :style="actionRowStyle">
              <ApIcon name="Tags" :size="18" :color="cv('blue')" />
              <span :style="sectionTitleStyle">{{ $t('ws.k258') }}</span>
              <span :style="countBadgeStyle">{{ tagPool.length }}</span>
            </div>
            <ApButton variant="ghost" size="small" :loading="poolLoading" @click="loadPool">{{ $t('ws.refresh') }}</ApButton>
          </div>
        </template>

        <!-- P2-2：全站工具区同一条规则——筛选在左、搜索贴右 -->
        <div :style="toolbarStyle">
          <div :style="filterStyle">
            <span :style="filterLabelStyle">{{ $t('ws.k259') }}</span>
            <ApSelect v-model="poolLimit" :options="poolLimitOptions" size="small" @update:model-value="loadPool" />
          </div>
          <div :style="[searchBoxStyle, { marginLeft: 'auto' }]">
            <ApSearch v-model="keyword" :aria-label="$t('ws.k291')" :placeholder="$t('ws.k280')" @search="loadPool" />
          </div>
        </div>

        <div v-if="tagPool.length" :style="tagChipRowStyle">
          <span
            v-for="name in tagPool"
            :key="name"
            :style="[tagChipStyle, { cursor: 'pointer' }]"
            @click="selectTag(name)"
          >
            <ApIcon v-if="name === selectedTag" name="Check" :size="12" :color="cv('blue')" />
            {{ name }}
            <span v-if="statCount(name) !== null" :style="countBadgeStyle">{{ statCount(name) }}</span>
          </span>
        </div>
        <ApEmpty
          v-else-if="!poolLoading"
          icon="Search"
          :title="$t('ws.k285')"
          :description="$t('vWorkspaceUi.k33')"
        />
      </ApCard>

      <!-- 使用统计 -->
      <ApCard size="small">
        <template #header>
          <div :style="headerRowStyle">
            <div :style="actionRowStyle">
              <ApIcon name="BarChart3" :size="18" :color="cv('green')" />
              <span :style="sectionTitleStyle">{{ $t('ws.k260') }}</span>
              <span :style="aiSubSectionMetaStyle">{{ $t('ws.k261') }}</span>
            </div>
            <div :style="filterStyle">
              <ApSelect v-model="statsLimit" :options="statsLimitOptions" size="small" @update:model-value="loadStats" />
              <ApButton variant="ghost" size="small" :loading="statsLoading" @click="loadStats">{{ $t('ws.refresh') }}</ApButton>
            </div>
          </div>
        </template>

        <div v-if="stats.length" :style="rankListStyle()">
          <div
            v-for="(item, index) in stats"
            :key="item.tag_name"
            :style="[rankItemStyle(index + 1), { cursor: 'pointer' }]"
            @click="selectTag(item.tag_name)"
          >
            <span :style="rankNumberStyle(index + 1)">{{ index + 1 }}</span>
            <span :style="rankNameStyle()">{{ item.tag_name }}</span>
            <ApProgress :value="percentOf(item.count)" :label="$t('ws.k277')" :style="{ width: '25%' }" />
            <span :style="rankCountStyle()">{{ item.count }}</span>
          </div>
        </div>
        <ApEmpty v-else-if="!statsLoading" icon="FileText" :title="$t('ws.k286')" :description="$t('vWorkspaceUi.k34')" />
      </ApCard>

      <!-- 层级树 -->
      <ApCard size="small">
        <template #header>
          <div :style="headerRowStyle">
            <div :style="actionRowStyle">
              <ApIcon name="ListTree" :size="18" :color="cv('purple')" />
              <span :style="sectionTitleStyle">{{ $t('ws.k262') }}</span>
              <span :style="countBadgeStyle">{{ flatTree.length }}</span>
            </div>
            <div :style="filterStyle">
              <span :style="filterLabelStyle">{{ $t('ws.k263') }}</span>
              <ApSelect v-model="spaceId" :options="spaceOptions" size="small" searchable @update:model-value="loadTree" />
              <ApButton variant="ghost" size="small" :loading="treeLoading" @click="loadTree">{{ $t('ws.refresh') }}</ApButton>
            </div>
          </div>
        </template>

        <div v-if="flatTree.length" :style="listStyle(AppleSpace.sm)">
          <div
            v-for="node in flatTree"
            :key="`${node.depth}-${node.tag_name}`"
            :style="[statusRowStyle, { paddingLeft: 'calc(' + AppleSpace.sm + ' * ' + node.depth + ')' }]"
          >
            <ApIcon v-if="node.depth > 0" name="CornerDownRight" :size="14" :color="cv('text-tertiary')" />
            <ApIcon v-else name="Hash" :size="14" :color="cv('blue')" />
            <span :style="nodeNameStyle">{{ node.tag_name }}</span>
            <span v-if="statCount(node.tag_name) !== null" :style="countBadgeStyle">{{ statCount(node.tag_name) }}</span>
            <span v-if="node.parent" :style="aiSubBadgeStyle">父：{{ node.parent }}</span>
            <div :style="[actionGroupStyle, { marginLeft: 'auto' }]">
              <ApButton variant="ghost" size="mini" @click="selectTag(node.tag_name)">{{ $t('ws.detail') }}</ApButton>
              <ApButton
                v-if="node.parent"
                variant="danger"
                size="mini"
                @click="askDetach(node.tag_name, node.parent)"
              >
                {{ $t('ws.k264') }}
              </ApButton>
            </div>
          </div>
        </div>
        <ApEmpty
          v-else-if="!treeLoading"
          icon="GitBranch"
          :title="$t('ws.k287')"
          :description="$t('vWorkspaceUi.k35')"
        />
      </ApCard>

      <!-- 标签详情 -->
      <ApCard size="small">
        <template #header>
          <div :style="headerRowStyle">
            <div :style="actionRowStyle">
              <ApIcon name="Info" :size="18" :color="cv('orange')" />
              <span :style="sectionTitleStyle">{{ $t('ws.k265') }}</span>
              <span v-if="selectedTag" :style="aiSubBadgeStyle">{{ selectedTag }}</span>
            </div>
            <div :style="filterStyle">
              <ApSelect v-model="docLimit" :options="docLimitOptions" size="small" @update:model-value="loadDetail" />
              <ApButton variant="secondary" size="small" :disabled="!selectedTag" @click="openHierarchyModal(selectedTag)">
                <ApIcon name="Plus" :size="16" /> {{ $t('ws.k266') }}
              </ApButton>
            </div>
          </div>
        </template>

        <ApEmpty v-if="!selectedTag" icon="FileText" :title="$t('ws.k288')" :description="$t('vWorkspaceUi.k36')" />
        <template v-else>
          <div :style="sectionStyle">
            <div :style="headerRowStyle">
              <span :style="sectionTitleStyle">{{ $t('ws.k267') }}</span>
              <ApButton v-if="parentItem" variant="danger" size="mini" @click="askDetach(selectedTag, parentItem.parent_tag)">
                {{ $t('ws.k268') }}
              </ApButton>
            </div>
            <p v-if="parentItem" :style="noteStyle">
              {{ parentItem.parent_tag }}
              <span :style="aiSubBadgeStyle">关系 ID {{ parentItem.id }}</span>
              <span :style="aiSubBadgeStyle">{{ parentItem.space_id === 0 ? '全局' : '空间 #' + parentItem.space_id }}</span>
            </p>
            <p v-else-if="parentLoading" :style="loadingStyle">{{ $t('ws.k269') }}</p>
            <p v-else :style="noteStyle">{{ $t('ws.k270') }}</p>
          </div>

          <div :style="sectionStyle">
            <div :style="headerRowStyle">
              <span :style="sectionTitleStyle">子标签（{{ children.length }}）</span>
              <ApButton variant="ghost" size="mini" :loading="childrenLoading" @click="loadChildren">{{ $t('ws.refresh') }}</ApButton>
            </div>
            <div v-if="children.length" :style="tagChipRowStyle">
              <span v-for="edge in children" :key="edge.id" :style="[tagChipStyle, { cursor: 'pointer' }]" @click="askDetach(edge.child_tag, selectedTag)">
                {{ edge.child_tag }}
                <ApIcon name="X" :size="12" :color="cv('red')" />
              </span>
            </div>
            <p v-else :style="noteStyle">{{ $t('ws.k271') }}</p>
          </div>

          <div :style="sectionStyle">
            <div :style="headerRowStyle">
              <span :style="sectionTitleStyle">使用该标签的文档（{{ docs.length }}）</span>
              <ApButton variant="ghost" size="mini" :loading="docsLoading" @click="loadDocs">{{ $t('ws.refresh') }}</ApButton>
            </div>
            <div v-if="docs.length" :style="listStyle(AppleSpace.sm)">
              <div v-for="doc in docs" :key="doc.id" :style="statusRowStyle">
                <ApIcon name="FileText" :size="14" :color="cv('blue')" />
                <span :style="nodeNameStyle">{{ doc.name || ('文档 #' + doc.id) }}</span>
                <span :style="aiSubBadgeStyle">{{ doc.space_id ? $t('vWorkspaceUi.k38') + doc.space_id : $t('vWorkspaceUi.k39') }}</span>
                <div :style="[actionGroupStyle, { marginLeft: 'auto' }]">
                  <ApButton variant="ghost" size="mini" :disabled="!doc.space_id" @click="openDocument(doc)">{{ $t('ws.k138') }}</ApButton>
                  <ApButton
                    v-if="parentItem"
                    variant="secondary"
                    size="mini"
                    @click="handleAppendParentTag(doc)"
                  >
                    {{ $t('ws.k272') }}
                  </ApButton>
                  <ApButton variant="danger" size="mini" @click="askRemoveTagFromDocument(doc)">{{ $t('ws.k273') }}</ApButton>
                </div>
              </div>
            </div>
            <p v-else-if="!docsLoading" :style="noteStyle">{{ $t('ws.k274') }}</p>
          </div>

          <p :style="noteStyle">
            后端只返回文档 ID，标题与空间需逐个调用文档详情接口解析（最多解析 {{ DOC_RESOLVE_LIMIT }} 篇）。
          </p>
        </template>
      </ApCard>
    </div>

    <!-- 新建层级关系 -->
    <ApModal v-model="showModal" :title="$t('ws.k289')" size="medium">
      <div :style="formStyle">
        <ApFormItem :label="$t('ws.k267')" required :error="parentError">
          <ApInput v-model="form.parent_tag" :placeholder="$t('ws.k281')" :maxlength="100" />
        </ApFormItem>
        <ApFormItem :label="$t('ws.k278')" required :error="childError">
          <ApInput v-model="form.child_tag" :placeholder="$t('ws.k282')" :maxlength="100" />
        </ApFormItem>
        <ApFormItem :label="$t('ws.k279')" :hint="$t('vWorkspaceUi.k37')">
          <ApSelect v-model="form.space_id" :options="spaceOptions" :placeholder="$t('ws.k283')" searchable />
        </ApFormItem>
        <p :style="noteStyle">
          {{ $t('ws.k275') }}
        </p>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('ws.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="creating" @click="handleCreateHierarchy">{{ $t('ws.k276') }}</ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showConfirm"
      :title="confirmTitle"
      :message="confirmMessage"
      type="danger"
      :confirm-text="$t('ws.confirm')"
      :loading="confirming"
      @confirm="runConfirm"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { logger } from '@/utils/logger';
import { cv, AppleLayout, AppleSpace } from '@/utils/core/apple-tokens';
import { getApiErrorStatus } from '@/api/client';
import {
  listStyle,
  sectionStyle,
  toolbarStyle,
  formStyle,
  noteStyle,
  loadingStyle,
  actionGroupStyle,
  searchBoxStyle,
  statusRowStyle,
  rankListStyle,
  rankItemStyle,
  rankNumberStyle,
  rankNameStyle,
  rankCountStyle,
  aiSubSectionHeaderStyle as headerRowStyle,
  aiSubSectionTitleStyle as sectionTitleStyle,
  aiSubSectionMetaStyle,
  aiSubCountBadgeStyle as countBadgeStyle,
  aiSubFilterStyle as filterStyle,
  aiSubFilterLabelStyle as filterLabelStyle,
  aiSubTagRowStyle as tagChipRowStyle,
  aiSubTagStyle as tagChipStyle,
  aiSubBadgeStyle,
  aiSubCardNameStyle as nodeNameStyle,
  aiSubActionRowStyle as actionRowStyle,
} from '@/styles/ai/ai-page-styles';
import {
  addDocumentTags,
  createTagHierarchy,
  deleteTagHierarchy,
  getAllTags,
  getDocumentsByTag,
  getTagChildren,
  getTagHierarchyTree,
  getTagParent,
  getTagStats,
  removeDocumentTags,
  type TagHierarchyItem,
  type TagStat,
  type TagTreeNode,
} from '@/api/modules/document/tags';
import { getDocumentByID } from '@/api/modules/document/document';
import { useSpaceStore } from '@/stores/workspace/space';
import { useUIStore } from '@/stores/app/ui';
import type { Space } from '@/types/models';

const router = useRouter();
const ui = useUIStore();
const spaceStore = useSpaceStore();

/** 详情区最多解析多少篇文档的标题（每篇一次文档详情请求） */
const DOC_RESOLVE_LIMIT = 20;

const spaces = ref<Space[]>([]);
const tagPool = ref<string[]>([]);
const stats = ref<TagStat[]>([]);
const tree = ref<TagTreeNode[]>([]);
const children = ref<TagHierarchyItem[]>([]);
const parentItem = ref<TagHierarchyItem | null>(null);
const docs = ref<Array<{ id: number; name?: string; space_id?: number }>>([]);

const keyword = ref('');
const poolLimit = ref<number | string>(100);
const statsLimit = ref<number | string>(20);
const docLimit = ref<number | string>(50);
/** 层级查询范围：0 = 全部关系（后端 space_id=0 走「不过滤」分支） */
const spaceId = ref<number | string>(0);
const selectedTag = ref('');

const poolLoading = ref(false);
const statsLoading = ref(false);
const treeLoading = ref(false);
const parentLoading = ref(false);
const childrenLoading = ref(false);
const docsLoading = ref(false);

const showModal = ref(false);
const creating = ref(false);
const parentError = ref('');
const childError = ref('');
const form = ref({ parent_tag: '', child_tag: '', space_id: 0 as number | string });

const showConfirm = ref(false);
const confirming = ref(false);
const confirmTitle = ref('解除标签层级关系');
const confirmMessage = ref('');
let confirmAction: (() => Promise<void>) | null = null;

const poolLimitOptions = [
  { label: '50', value: 50 },
  { label: '100', value: 100 },
  { label: '200', value: 200 },
];

const statsLimitOptions = [
  { label: 'Top 10', value: 10 },
  { label: 'Top 20', value: 20 },
  { label: 'Top 50', value: 50 },
  { label: 'Top 100', value: 100 },
];

const docLimitOptions = [
  { label: t('ws.k292'), value: 50 },
  { label: t('ws.k293'), value: 100 },
  { label: t('ws.k294'), value: 200 },
];

const spaceOptions = computed(() => [
  { label: t('ws.k295'), value: 0 },
  ...spaces.value.map((s) => ({ label: s.name, value: s.id })),
]);

/** 层级树展平为带缩进的行（后端不返回边的 ID，解除关系时按子标签反查） */
const flatTree = computed(() => {
  const rows: Array<{ tag_name: string; depth: number; parent?: string }> = [];
  const walk = (nodes: TagTreeNode[], depth: number, parent?: string) => {
    for (const node of nodes ?? []) {
      rows.push({ tag_name: node.tag_name, depth, parent });
      if (node.children?.length) walk(node.children, depth + 1, node.tag_name);
    }
  };
  walk(tree.value, 0);
  return rows;
});

const statsMap = computed(() => {
  const map = new Map<string, number>();
  stats.value.forEach((s) => map.set(s.tag_name, s.count));
  return map;
});

const maxCount = computed(() => stats.value.reduce((acc, s) => Math.max(acc, s.count), 0));

function statCount(name: string): number | null {
  return statsMap.value.get(name) ?? null;
}

function percentOf(count: number): number {
  if (!maxCount.value) return 0;
  return Math.round((count / maxCount.value) * 100);
}

async function loadPool() {
  poolLoading.value = true;
  try {
    const res = await getAllTags({ keyword: keyword.value.trim(), limit: Number(poolLimit.value) });
    tagPool.value = res ?? [];
  } catch (e) {
    logger.error('Failed to load tag pool:', e);
    ui.showToast((e as Error).message || t('fallback.k35'), 'error');
  } finally {
    poolLoading.value = false;
  }
}

async function loadStats() {
  statsLoading.value = true;
  try {
    const res = await getTagStats(Number(statsLimit.value) || 20);
    stats.value = res ?? [];
  } catch (e) {
    logger.error('Failed to load tag stats:', e);
    ui.showToast((e as Error).message || t('fallback.k36'), 'error');
  } finally {
    statsLoading.value = false;
  }
}

async function loadTree() {
  treeLoading.value = true;
  try {
    const res = await getTagHierarchyTree(Number(spaceId.value) || 0);
    tree.value = res ?? [];
  } catch (e) {
    logger.error('Failed to load tag tree:', e);
    ui.showToast((e as Error).message || t('fallback.k30'), 'error');
  } finally {
    treeLoading.value = false;
  }
}

async function loadParent() {
  parentItem.value = null;
  if (!selectedTag.value) return;
  parentLoading.value = true;
  try {
    parentItem.value = await getTagParent(selectedTag.value);
  } catch (e) {
    // 后端无父标签时返回 404（no parent found），属正常态
    if (getApiErrorStatus(e) !== 404) logger.error('Failed to load tag parent:', e);
  } finally {
    parentLoading.value = false;
  }
}

async function loadChildren() {
  children.value = [];
  if (!selectedTag.value) return;
  childrenLoading.value = true;
  try {
    const res = await getTagChildren(selectedTag.value, Number(spaceId.value) || 0);
    children.value = res ?? [];
  } catch (e) {
    logger.error('Failed to load tag children:', e);
    ui.showToast((e as Error).message || t('fallback.k28'), 'error');
  } finally {
    childrenLoading.value = false;
  }
}

async function loadDocs() {
  docs.value = [];
  if (!selectedTag.value) return;
  docsLoading.value = true;
  try {
    const res = await getDocumentsByTag(selectedTag.value, Number(docLimit.value) || 50);
    const ids = res?.document_ids ?? [];
    const settled = await Promise.allSettled(
      ids.slice(0, DOC_RESOLVE_LIMIT).map((id) => getDocumentByID(id))
    );
    docs.value = ids.map((id, index) => {
      const r = settled[index];
      return {
        id,
        name: r && r.status === 'fulfilled' ? r.value?.name : undefined,
        space_id: r && r.status === 'fulfilled' ? r.value?.space_id : undefined,
      };
    });
  } catch (e) {
    logger.error('Failed to load documents by tag:', e);
    ui.showToast((e as Error).message || t('fallback.k34'), 'error');
  } finally {
    docsLoading.value = false;
  }
}

function selectTag(name: string) {
  selectedTag.value = name;
  loadDetail();
}

function loadDetail() {
  loadParent();
  loadChildren();
  loadDocs();
}

function openDocument(doc: { id: number; space_id?: number }) {
  if (!doc.space_id) return;
  router.push(`/space/${doc.space_id}/doc/${doc.id}`);
}

function openHierarchyModal(parentTag?: string) {
  form.value = {
    parent_tag: parentTag ?? '',
    child_tag: '',
    space_id: Number(spaceId.value) || 0,
  };
  parentError.value = '';
  childError.value = '';
  showModal.value = true;
}

function validateHierarchy(): boolean {
  const parent = form.value.parent_tag.trim();
  const child = form.value.child_tag.trim();
  parentError.value = !parent ? '请填写父标签' : parent.length > 100 ? '父标签最长 100 字符' : '';
  childError.value = !child
    ? '请填写子标签'
    : child.length > 100
      ? '子标签最长 100 字符'
      : child === parent
        ? '父标签与子标签不能相同'
        : '';
  return !parentError.value && !childError.value;
}

async function handleCreateHierarchy() {
  if (!validateHierarchy()) return;
  creating.value = true;
  try {
    await createTagHierarchy({
      parent_tag: form.value.parent_tag.trim(),
      child_tag: form.value.child_tag.trim(),
      space_id: Number(form.value.space_id) || 0,
    });
    ui.showToast(t('ws.k296'), 'success');
    showModal.value = false;
    await Promise.all([loadTree(), loadPool()]);
    if (selectedTag.value === form.value.parent_tag) await loadChildren();
  } catch (e) {
    logger.error('Failed to create tag hierarchy:', e);
    // 409 = 子标签已有父标签；循环引用后端返回 400
    ui.showToast((e as Error).message || t('fallback.k17'), 'error');
  } finally {
    creating.value = false;
  }
}

/** 边的记录 ID 只在 children/parent 接口里返回，树节点没有，故按子标签反查 */
async function resolveEdgeId(childTag: string): Promise<number | null> {
  try {
    const edge = await getTagParent(childTag);
    return edge?.id ?? null;
  } catch (e) {
    if (getApiErrorStatus(e) !== 404) logger.error('Failed to resolve hierarchy edge:', e);
    return null;
  }
}

function askDetach(childTag: string, parentTag: string) {
  confirmTitle.value = t('sentUi.k63');
  confirmMessage.value = t('sentUi.k65', { p1: parentTag, p2: childTag });
  confirmAction = async () => {
    const id = await resolveEdgeId(childTag);
    if (!id) {
      ui.showToast(t('ws.k297'), 'error');
      return;
    }
    await deleteTagHierarchy(id);
    ui.showToast(t('ws.k298'), 'success');
    await loadTree();
    if (selectedTag.value === parentTag || selectedTag.value === childTag) await loadDetail();
  };
  showConfirm.value = true;
}

function askRemoveTagFromDocument(doc: { id: number; name?: string }) {
  confirmTitle.value = t('sentUi.k64');
  confirmMessage.value = t('sentUi.k66', { p1: doc.name || t('fallback.k65') + doc.id, p2: selectedTag.value });
  confirmAction = async () => {
    await removeDocumentTags(doc.id, selectedTag.value);
    ui.showToast(t('ws.k299'), 'success');
    await Promise.all([loadDocs(), loadStats(), loadPool()]);
  };
  showConfirm.value = true;
}

async function handleAppendParentTag(doc: { id: number; name?: string }) {
  if (!parentItem.value) return;
  try {
    await addDocumentTags(doc.id, parentItem.value.parent_tag);
    ui.showToast(t('sentUi.k67', { p1: parentItem.value.parent_tag }), 'success');
    await Promise.all([loadStats(), loadPool()]);
  } catch (e) {
    logger.error('Failed to append tag to document:', e);
    ui.showToast((e as Error).message || t('fallback.k106'), 'error');
  }
}

async function runConfirm() {
  if (!confirmAction) return;
  confirming.value = true;
  try {
    await confirmAction();
    showConfirm.value = false;
    confirmAction = null;
  } catch (e) {
    logger.error('Confirm action failed:', e);
    ui.showToast((e as Error).message || t('fallback.k64'), 'error');
  } finally {
    confirming.value = false;
  }
}

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
  } catch (e) {
    logger.error('Failed to load spaces:', e);
  }
}

onMounted(() => {
  loadSpaces();
  loadPool();
  loadStats();
  loadTree();
});
</script>
