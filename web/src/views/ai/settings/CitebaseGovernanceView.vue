<template>
  <div :style="aiSubPageStyle">
    <div :style="filterBarStyle">
      <!-- 治理视图切换以前在统计区下面另起一行，并进这条工具行的开头 -->
      <ApSegmentedControl v-if="spaceId" v-model="tab" :options="tabOptions" size="small" :aria-label="$t('aiSet2.k202')" />
      <div :style="spaceSelectorStyle">
        <span :style="aiSubFilterLabelStyle">{{ $t('aiSet2.k180') }}</span>
        <ApSelect
          v-model="spaceId"
          :options="spaceOptions"
          :placeholder="$t('aiSet2.k12')"
          :searchable="true"
          @update:model-value="loadAll"
        />
      </div>
      <ApButton variant="secondary" size="small" :loading="loading" @click="loadAll">
        <ApIcon name="RotateCw" :size="AppleIconSize.control" /> {{ $t('aiSet2.refresh') }}
      </ApButton>
    </div>

    <div v-if="!spaceId" :style="aiSubSectionMetaStyle">{{ $t('aiSet2.k181') }}</div>
    <template v-else>
      <div :style="statsGridStyle">
        <ApStatCard :label="$t('aiSet2.k191')" :value="conflictPins.length" icon="ShieldAlert" :color="cv('red')" size="compact" />
        <ApStatCard :label="$t('aiSet2.k192')" :value="orphanPins.length" icon="Unlink" :color="cv('orange')" size="compact" />
        <ApStatCard :label="$t('aiSet2.k193')" :value="pendingReviews.length" icon="FileQuestion" :color="cv('blue')" size="compact" />
        <ApStatCard :label="$t('aiSet2.k194')" :value="openIssues.length" icon="Stethoscope" :color="cv('purple')" size="compact" />
      </div>

      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <div :style="aiSubCardNameRowStyle">
              <span :style="aiSubSectionTitleStyle">{{ tabTitle }}</span>
              <span :style="aiSubCountBadgeStyle">{{ tabRowCount }}</span>
            </div>
            <div :style="aiSubActionRowStyle">
              <ApButton v-if="tab === 'pins'" variant="primary" size="small" @click="openPinForm">
                <ApIcon name="Plus" :size="AppleIconSize.control" /> {{ $t('aiSet2.k182') }}
              </ApButton>
              <ApButton v-if="tab === 'lint'" variant="secondary" size="small" :loading="linting" @click="rerunLint">
                <ApIcon name="Stethoscope" :size="AppleIconSize.control" /> {{ $t('aiSet2.k183') }}
              </ApButton>
              <ApButton variant="secondary" size="small" :loading="exporting" @click="exportWikiZip">
                <ApIcon name="Download" :size="AppleIconSize.control" /> {{ $t('aiSet2.exportWiki') }}
              </ApButton>
              <ApSwitch v-if="tab !== 'links'" v-model="onlyOpen" :label="$t('aiSet2.k195')" />
            </div>
          </div>
        </template>

        <ApTable
          density="compact"
          :columns="currentColumns"
          :data="currentRows"
          :loading="loading"
          :empty-text="emptyText"
          empty-icon="ScrollText"
          :aria-label="tabTitle"
        >
          <template #actions="{ row }">
            <ApButton variant="ghost" size="small" @click="openPage(row.slug)">{{ $t('aiSet2.k184') }}</ApButton>
            <template v-if="tab === 'pins'">
              <ApButton
                variant="secondary"
                size="small"
                :loading="busyId === 'pin-check-' + asPin(row).id"
                @click="checkPin(asPin(row))"
              >
                {{ $t('aiSet2.k185') }}
              </ApButton>
              <ApButton
                v-if="asPin(row).status !== 'resolved'"
                variant="primary"
                size="small"
                :loading="busyId === 'pin-' + asPin(row).id"
                @click="resolvePin(asPin(row))"
              >
                {{ $t('aiSet2.k186') }}
              </ApButton>
            </template>
            <template v-else-if="tab === 'reviews'">
              <ApButton
                v-if="asReview(row).status === 'pending'"
                variant="primary"
                size="small"
                :loading="busyId === 'rev-' + asReview(row).id"
                @click="resolveReview(asReview(row), 'accepted')"
              >
                {{ $t('aiSet2.k187') }}
              </ApButton>
              <ApButton
                v-if="asReview(row).status === 'pending'"
                variant="danger"
                size="small"
                :loading="busyId === 'rev-' + asReview(row).id"
                @click="resolveReview(asReview(row), 'rejected')"
              >
                {{ $t('aiSet2.k188') }}
              </ApButton>
            </template>
          </template>
        </ApTable>
      </ApCard>
    </template>

    <ApModal v-model="showPinForm" :title="$t('aiSet2.k182')" size="medium">
      <div :style="formStyle">
        <ApFormItem :label="$t('aiSet2.k196')" required :hint="$t('vAiUi.k230')">
          <ApInput v-model="pinForm.page_slug" :aria-label="$t('aiSet2.k196')" :placeholder="$t('aiSet2.k200')" />
        </ApFormItem>
        <ApFormItem :label="$t('aiSet2.k197')" required :hint="$t('vAiUi.k231')">
          <ApTextarea v-model="pinForm.claim" :rows="3" :aria-label="$t('aiSet2.k203')" />
        </ApFormItem>
        <div :style="formRowStyle">
          <ApFormItem :label="$t('aiSet2.k198')" style="flex: 1">
            <ApInput v-model="pinForm.kind" :aria-label="$t('aiSet2.k204')" :placeholder="$t('aiSet2.k201')" />
          </ApFormItem>
          <ApFormItem :label="$t('aiSet2.k199')" style="flex: 1" :hint="$t('vAiUi.k232')">
            <ApInput v-model="pinForm.anchor" :aria-label="$t('aiSet2.k199')" />
          </ApFormItem>
        </div>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showPinForm = false">{{ $t('aiSet2.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="savingPin" @click="submitPin">{{ $t('aiSet2.create2') }}</ApButton>
      </template>
    </ApModal>

    <ApModal v-model="showPage" :title="pageDetail?.title || $t('fallback.k100')" size="large">
      <div v-if="pageLoading" :style="aiSubSectionMetaStyle">{{ $t('aiSet2.k190') }}</div>
      <template v-else-if="pageDetail">
        <div :style="pageMetaStyle">
          <ApStatusBadge :status="pageDetail.status === 'published' ? 'enabled' : 'default'" :label="pageDetail.status" size="small" />
          <span>{{ pageDetail.source_count }} {{ $t('vAiUi.k234') }}</span>
          <span>{{ $t('aiSet.k297', { version: pageDetail.version, base: pageDetail.base_version }) }}</span>
          <span>{{ pageDetail.provenance === 'human' ? $t('aiSet.k298') : $t('vAiUi.k233') }}</span>
        </div>
        <p :style="summaryStyle">{{ pageDetail.summary || $t('fallback.k112') }}</p>

        <div v-if="pageContradictions.length" :style="contradictionStyle">
          <div :style="contradictionTitleStyle">{{ $t('aiSet2.k248') }}</div>
          <ul :style="contradictionListStyle">
            <li v-for="(c, i) in pageContradictions" :key="i">{{ c }}</li>
          </ul>
        </div>

        <ApCard size="small">
          <template #header>
            <div :style="aiSubCardNameRowStyle">
              <span :style="aiSubSectionTitleStyle">{{ $t('aiSet2.k237') }}</span>
              <span :style="aiSubCountBadgeStyle">{{ pageHistory.length }}</span>
            </div>
            <ApButton
              variant="secondary"
              size="small"
              :disabled="!contradictionDraft.trim()"
              :loading="busyId === 'mark-contradiction'"
              @click="submitContradictions"
            >
              {{ $t('aiSet2.k244') }}
            </ApButton>
          </template>
          <ApTextarea
            v-model="contradictionDraft"
            :rows="2"
            :placeholder="$t('aiSet2.k245')"
            :aria-label="$t('aiSet2.k245')"
          />
          <div v-if="pageHistory.length" :style="historyTableStyle">
            <div v-for="h in pageHistory" :key="h.version" :style="historyRowStyle">
              <span :style="historyVersionStyle">v{{ h.version }}</span>
              <span :style="historySummaryStyle">{{ h.summary || '—' }}</span>
              <span :style="historyTimeStyle">{{ h.created_at?.slice(0, 19).replace('T', ' ') }}</span>
              <ApButton
                v-if="h.version !== pageDetail.version"
                variant="ghost"
                size="small"
                :loading="busyId === 'rollback-' + h.version"
                @click="rollbackTo(h.version)"
              >
                {{ $t('aiSet2.k238') }}
              </ApButton>
            </div>
          </div>
          <div v-else :style="aiSubSectionMetaStyle">{{ $t('aiSet2.k241') }}</div>
        </ApCard>

        <pre :style="contentStyle">{{ pageDetail.content }}</pre>
      </template>
    </ApModal>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { computed, onMounted, ref } from 'vue';

import { logger } from '@/utils/logger';
import { AppleGridFloor, AppleFontWeight, AppleRadius, AppleSpace, cv, AppleIconSize } from '@/utils/core/apple-tokens';
import ApIcon from '@/components/apple/ApIcon.vue';
import {
  AI_TABLE_WIDTH,
  aiSubActionRowStyle,
  aiSubCardNameRowStyle,
  aiSubCountBadgeStyle,
  aiSubFilterLabelStyle,
  aiSubPageStyle,
  aiSubSectionHeaderStyle,
  aiSubSectionMetaStyle,
  aiSubSectionTitleStyle,
  aiSubFilterStyle as spaceSelectorStyle,
  filterBarStyle,
  formRowStyle,
  formStyle,
} from '@/styles/ai/ai-page-styles';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import {
  checkWikiPins,
  createWikiPin,
  exportWiki,
  getWikiLinks,
  getWikiPage,
  listWikiLint,
  listWikiPageHistory,
  listWikiPins,
  listWikiReviews,
  markWikiContradictions,
  resolveWikiPin,
  resolveWikiReview,
  rollbackWikiPage,
  runWikiLint,
  type WikiLintIssue,
  type WikiLink,
  type WikiPin,
  type WikiReview,
} from '@/api/modules/ai/ai-wiki';
import { useSpaceStore } from '@/stores/workspace/space';
import { useUIStore } from '@/stores/app/ui';
import type { Space } from '@/types/models';

const ui = useUIStore();
const spaceStore = useSpaceStore();

type GovernanceRow = Record<string, unknown> & { slug: string };

const spaces = ref<Space[]>([]);
const spaceId = ref<number | null>(null);
const loading = ref(false);
const linting = ref(false);
const savingPin = ref(false);
const exporting = ref(false);
const busyId = ref('');
const tab = ref('pins');
const onlyOpen = ref(true);

const pins = ref<WikiPin[]>([]);
const reviews = ref<WikiReview[]>([]);
const issues = ref<WikiLintIssue[]>([]);
const links = ref<WikiLink[]>([]);

const showPinForm = ref(false);
const pinForm = ref({ page_slug: '', claim: '', kind: '', anchor: '' });

const showPage = ref(false);
const pageLoading = ref(false);
const pageDetail = ref<Awaited<ReturnType<typeof getWikiPage>>['page'] | null>(null);
const pageHistory = ref<Awaited<ReturnType<typeof listWikiPageHistory>>['history']>([]);
const contradictionDraft = ref('');

// 矛盾条目的唯一来源是页面正文里的标记块（后端确定性写入），
// 这里只做解析展示，不与 Lint 表二次对账 —— 两处数据会各说各话。
const pageContradictions = computed(() => {
  const content = pageDetail.value?.content ?? '';
  const i = content.indexOf('<!-- contradiction: -->');
  if (i < 0) return [] as string[];
  const out: string[] = [];
  for (const raw of content.slice(i).split('\n')) {
    const line = raw.trim();
    if (line.startsWith('- ')) out.push(line.slice(2));
    else if (line && !line.startsWith('<!--')) break;
  }
  return out;
});

const PIN_KIND_LABELS: Record<string, string> = { fact: '事实', policy: '约定', metric: '指标' };
const PIN_STATUS_LABELS: Record<string, string> = {
  active: '生效', orphan: '锚点已失效', conflict: '与新源冲突', resolved: '已处理',
};
const REVIEW_STATUS_LABELS: Record<string, string> = { pending: '待审', accepted: '已接受', rejected: '已拒绝' };
const REVIEW_ACTION_LABELS: Record<string, string> = {
  create_page: '新建词条', deep_research: '需深入研究', skip: '建议跳过',
};

const spaceOptions = computed(() => spaces.value.map((s) => ({ label: s.name, value: s.id })));
const conflictPins = computed(() => pins.value.filter((p) => p.status === 'conflict'));
const orphanPins = computed(() => pins.value.filter((p) => p.status === 'orphan'));
const pendingReviews = computed(() => reviews.value.filter((r) => r.status === 'pending'));
const openIssues = computed(() => issues.value.filter((i) => i.status === 'open'));

const shownPins = computed(() => {
  if (!onlyOpen.value) return pins.value;
  return pins.value.filter((p) => p.status === 'conflict' || p.status === 'orphan' || p.status === 'active');
});
const shownReviews = computed(() => (onlyOpen.value ? pendingReviews.value : reviews.value));
const shownIssues = computed(() => (onlyOpen.value ? openIssues.value : issues.value));

const tabOptions = [
  { label: t('aiSet2.k205'), value: 'pins' },
  { label: t('aiSet2.k206'), value: 'reviews' },
  { label: t('aiSet2.k207'), value: 'lint' },
  { label: t('aiSet2.k208'), value: 'links' },
];

const tabTitle = computed(() => ({
  pins: '知识引脚', reviews: '待审条目', lint: '体检问题', links: '双向链接',
}[tab.value] ?? ''));

const pinColumns: TableColumn<GovernanceRow>[] = [
  { key: 'slug', title: t('aiSet2.k209'), width: AI_TABLE_WIDTH.xl, ellipsis: true },
  { key: 'claim', title: t('aiSet2.k197'), ellipsis: true },
  { key: 'kind', title: t('aiSet2.k198'), width: AI_TABLE_WIDTH.sm, render: (r) => PIN_KIND_LABELS[String(r.kind)] ?? String(r.kind ?? '—') },
  { key: 'status', title: t('aiSet2.status'), width: AI_TABLE_WIDTH.sm, render: (r) => PIN_STATUS_LABELS[String(r.status)] ?? String(r.status) },
  { key: 'provenance', title: t('aiSet2.k210'), width: AI_TABLE_WIDTH.sm },
];

const reviewColumns: TableColumn<GovernanceRow>[] = [
  { key: 'slug', title: t('aiSet2.k209'), width: AI_TABLE_WIDTH.xl, ellipsis: true },
  { key: 'reason', title: t('aiSet2.k211'), ellipsis: true },
  { key: 'action', title: t('aiSet2.k212'), width: AI_TABLE_WIDTH.md, render: (r) => REVIEW_ACTION_LABELS[String(r.action)] ?? String(r.action) },
  { key: 'status', title: t('aiSet2.status'), width: AI_TABLE_WIDTH.sm, render: (r) => REVIEW_STATUS_LABELS[String(r.status)] ?? String(r.status) },
  { key: 'base_version', title: t('aiSet2.k213'), width: AI_TABLE_WIDTH.sm, align: 'right' },
];

const lintColumns: TableColumn<GovernanceRow>[] = [
  { key: 'slug', title: t('aiSet2.k209'), width: AI_TABLE_WIDTH.xl, ellipsis: true },
  { key: 'detail', title: t('aiSet2.k45'), ellipsis: true },
  { key: 'category', title: t('aiSet2.k198'), width: AI_TABLE_WIDTH.md },
  { key: 'severity', title: t('aiSet2.k214'), width: AI_TABLE_WIDTH.sm },
  { key: 'status', title: t('aiSet2.status'), width: AI_TABLE_WIDTH.sm },
];

const linkColumns: TableColumn<GovernanceRow>[] = [
  { key: 'slug', title: t('aiSet2.k215'), width: AI_TABLE_WIDTH.xl, ellipsis: true },
  { key: 'to_slug', title: t('aiSet2.k216'), ellipsis: true },
  { key: 'link_type', title: t('aiSet2.k217'), width: AI_TABLE_WIDTH.md },
  { key: 'weight', title: t('aiSet2.k218'), width: AI_TABLE_WIDTH.sm, align: 'right' },
];

const columnsByTab = computed<Record<string, TableColumn<GovernanceRow>[]>>(() => ({
  pins: pinColumns, reviews: reviewColumns, lint: lintColumns, links: linkColumns,
}));

const rowsByTab = computed<Record<string, GovernanceRow[]>>(() => ({
  pins: shownPins.value.map((p) => ({ ...p, slug: p.page_slug })) as unknown as GovernanceRow[],
  reviews: shownReviews.value.map((r) => ({ ...r, slug: r.page_slug })) as unknown as GovernanceRow[],
  lint: shownIssues.value.map((i) => ({ ...i, slug: i.page_slug })) as unknown as GovernanceRow[],
  links: links.value.map((l) => ({ ...l, slug: l.from_slug })) as unknown as GovernanceRow[],
}));

const currentColumns = computed(() => columnsByTab.value[tab.value] ?? []);
const currentRows = computed(() => rowsByTab.value[tab.value] ?? []);
const tabRowCount = computed(() => currentRows.value.length);

// 表体是四个 tab 共用的一个 ApTable，行数据本身就是对应的 Wiki* 对象
// （只多了一个 slug 别名），所以这里在边界处还原类型，而不是模板里逐处断言。
const asPin = (row: GovernanceRow) => row as unknown as WikiPin;
const asReview = (row: GovernanceRow) => row as unknown as WikiReview;

const emptyText = computed(() => {
  // 筛选致空与真的没数据是两件事：前者要告诉用户关掉「只看待处理」再看，
  // 否则界面只是在假装"这里什么都没有"。
  if (onlyOpen.value && tab.value !== 'links') {
    const fullCount =
      tab.value === 'pins' ? pins.value.length : tab.value === 'reviews' ? reviews.value.length : issues.value.length;
    if (fullCount > 0) return t('aiSet2.k236');
  }
  return ({
    pins: onlyOpen.value ? t('aiSet2.k231') : t('aiSet2.k232'),
    reviews: t('aiSet2.k233'),
    lint: t('aiSet2.k234'),
    links: t('aiSet2.k235'),
  } as Record<string, string>)[tab.value] ?? t('common.noData');
});

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
    const first = spaces.value[0];
    if (first && !spaceId.value) {
      spaceId.value = first.id;
      await loadAll();
    }
  } catch (e) {
    logger.error('Failed to load spaces:', e);
    ui.showToast(t('aiSet2.k25'), 'error');
  }
}

async function loadAll() {
  const sid = spaceId.value;
  if (!sid) return;
  loading.value = true;
  try {
    const [p, r, l, k] = await Promise.all([
      listWikiPins(sid, ''),
      listWikiReviews(sid, ''),
      // 空串是有意的：后端 ListBySpace 把空 status 当"不过滤"，而 /ai/wiki/lint 的
      // handler 用 DefaultQuery("status","open")——参数缺省才会只给 open。
      // 取全量才能在本地真正实现"只看待处理 / 看全部"两种视图。
      listWikiLint(sid, ''),
      getWikiLinks(sid),
    ]);
    pins.value = p?.pins ?? [];
    reviews.value = r?.reviews ?? [];
    issues.value = l?.issues ?? [];
    links.value = k?.links ?? [];
  } catch (e) {
    logger.error('Failed to load wiki governance:', e);
    ui.showToast(t('aiSet2.k220'), 'error');
  } finally {
    loading.value = false;
  }
}

function openPinForm() {
  pinForm.value = { page_slug: '', claim: '', kind: '', anchor: '' };
  showPinForm.value = true;
}

async function submitPin() {
  const sid = spaceId.value;
  const slug = pinForm.value.page_slug.trim();
  const claim = pinForm.value.claim.trim();
  if (!sid || !slug || !claim) {
    ui.showToast(t('aiSet2.k221'), 'warning');
    return;
  }
  savingPin.value = true;
  try {
    await createWikiPin({
      space_id: sid,
      page_slug: slug,
      claim,
      kind: pinForm.value.kind.trim() || undefined,
      anchor: pinForm.value.anchor.trim() || undefined,
    });
    ui.showToast(t('aiSet2.k222'), 'success');
    showPinForm.value = false;
    await loadAll();
  } catch (e) {
    logger.error('Failed to create wiki pin:', e);
    ui.showToast(t('aiSet2.k223'), 'error');
  } finally {
    savingPin.value = false;
  }
}

async function checkPin(pin: WikiPin) {
  const sid = spaceId.value;
  if (!sid) return;
  busyId.value = 'pin-check-' + pin.id;
  try {
    const res = await checkWikiPins(sid, pin.page_slug);
    const bad = (res?.checks ?? []).filter((c) => c.status && c.status !== 'ok');
    if (!res?.checks?.length) ui.showToast(t('aiSet2.k224'), 'warning');
    else if (bad.length === 0) ui.showToast(t('aiSet2.k225'), 'success');
    else ui.showToast(t('sentUi.k45', { p1: bad.length }), 'warning');
    await loadAll();
  } catch (e) {
    logger.error('Failed to check wiki pins:', e);
    ui.showToast(t('aiSet2.k226'), 'error');
  } finally {
    busyId.value = '';
  }
}

async function resolvePin(pin: WikiPin) {
  busyId.value = 'pin-' + pin.id;
  try {
    await resolveWikiPin(pin.id);
    ui.showToast(t('aiSet2.k227'), 'success');
    await loadAll();
  } catch (e) {
    logger.error('Failed to resolve wiki pin:', e);
    ui.showToast(t('aiSet2.k228'), 'error');
  } finally {
    busyId.value = '';
  }
}

async function resolveReview(review: WikiReview, action: 'accepted' | 'rejected') {
  busyId.value = 'rev-' + review.id;
  try {
    await resolveWikiReview(review.id, action);
    ui.showToast(action === 'accepted' ? '已接受' : '已拒绝', 'success');
    await loadAll();
  } catch (e) {
    // 批准走乐观锁发布：基线版本不匹配时后端会拒，必须把这话传给用户而不是只说"失败"
    logger.error('Failed to resolve wiki review:', e);
    ui.showToast(e instanceof Error && e.message ? e.message: t('aiSet2.k219'), 'error');
  } finally {
    busyId.value = '';
  }
}

async function rerunLint() {
  const sid = spaceId.value;
  if (!sid) return;
  linting.value = true;
  try {
    const res = await runWikiLint(sid);
    ui.showToast(t('sentUi.k40', { p1: res?.total ?? 0 }), 'success');
    await loadAll();
  } catch (e) {
    logger.error('Failed to run wiki lint:', e);
    ui.showToast(t('aiSet2.k229'), 'error');
  } finally {
    linting.value = false;
  }
}

async function openPage(slug: string) {
  const sid = spaceId.value;
  if (!sid || !slug) return;
  pageDetail.value = null;
  pageHistory.value = [];
  contradictionDraft.value = '';
  showPage.value = true;
  pageLoading.value = true;
  try {
    const [res, hist] = await Promise.all([
      getWikiPage(sid, slug),
      listWikiPageHistory(sid, slug).catch(() => null),
    ]);
    pageDetail.value = res?.page ?? null;
    pageHistory.value = hist?.history ?? [];
  } catch (e) {
    logger.error('Failed to load wiki page:', e);
    ui.showToast(t('aiSet2.k230'), 'error');
    showPage.value = false;
  } finally {
    pageLoading.value = false;
  }
}

async function rollbackTo(version: number) {
  const sid = spaceId.value;
  const slug = pageDetail.value?.slug;
  if (!sid || !slug) return;
  busyId.value = 'rollback-' + version;
  try {
    await rollbackWikiPage(sid, slug, version);
    ui.showToast(t('aiSet2.k242'), 'success');
    const [res, hist] = await Promise.all([getWikiPage(sid, slug), listWikiPageHistory(sid, slug)]);
    pageDetail.value = res?.page ?? null;
    pageHistory.value = hist?.history ?? [];
  } catch (e) {
    logger.error('Failed to rollback wiki page:', e);
    ui.showToast(t('aiSet2.k243'), 'error');
  } finally {
    busyId.value = '';
  }
}

async function submitContradictions() {
  const sid = spaceId.value;
  const slug = pageDetail.value?.slug;
  const items = contradictionDraft.value.split('\n').map((s) => s.trim()).filter(Boolean);
  if (!sid || !slug || items.length === 0) return;
  busyId.value = 'mark-contradiction';
  try {
    await markWikiContradictions(sid, slug, items);
    ui.showToast(t('aiSet2.k246'), 'success');
    contradictionDraft.value = '';
    const [res, hist] = await Promise.all([getWikiPage(sid, slug), listWikiPageHistory(sid, slug)]);
    pageDetail.value = res?.page ?? null;
    pageHistory.value = hist?.history ?? [];
    await loadAll();
  } catch (e) {
    logger.error('Failed to mark wiki contradictions:', e);
    ui.showToast(t('aiSet2.k247'), 'error');
  } finally {
    busyId.value = '';
  }
}

async function exportWikiZip() {
  const sid = spaceId.value;
  if (!sid || exporting.value) return;
  exporting.value = true;
  try {
    const blob = await exportWiki(sid, 'zip');
    const url = window.URL.createObjectURL(blob as Blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `wiki-space-${sid}.zip`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    window.URL.revokeObjectURL(url);
    ui.showToast(t('aiSet2.exported'), 'success');
  } catch (e) {
    logger.error('Failed to export wiki:', e);
    ui.showToast(t('aiSet2.exportFailed'), 'error');
  } finally {
    exporting.value = false;
  }
}

// 空间列表来自 useSpaceStore，与其它 AI 子页一致
const statsGridStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.stat})`,
  gap: AppleSpace.sm,
  marginTop: AppleSpace.sm,
};

const pageMetaStyle = {
  display: 'flex',
  flexWrap: 'wrap' as const,
  gap: AppleSpace.md,
  marginBottom: AppleSpace.sm,
};

const summaryStyle = { margin: `0 0 ${AppleSpace.sm}` };

const contradictionStyle = {
  border: `1px solid ${cv('red')}`,
  borderRadius: AppleRadius.panel,
  padding: AppleSpace.sm,
  marginBottom: AppleSpace.sm,
};

const contradictionTitleStyle = {
  fontWeight: AppleFontWeight.semibold,
  color: cv('red'),
  marginBottom: AppleSpace.xs,
};

const contradictionListStyle = {
  margin: 0,
  paddingLeft: AppleSpace.md,
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
};

const historyTableStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
  marginTop: AppleSpace.sm,
};

const historyRowStyle = {
  display: 'flex',
  alignItems: 'center' as const,
  gap: AppleSpace.sm,
  flexWrap: 'wrap' as const,
};

const historyVersionStyle = {
  fontWeight: AppleFontWeight.semibold,
  minWidth: 44,
};

const historySummaryStyle = {
  flex: 1,
  color: cv('text-tertiary'),
};

const historyTimeStyle = {
  color: cv('text-quaternary'),
  fontVariantNumeric: 'tabular-nums',
};

const contentStyle = {
  margin: '0',
  whiteSpace: 'pre-wrap' as const,
  wordBreak: 'break-word' as const,
  maxHeight: '60vh',
  overflowY: 'auto' as const,
};

onMounted(loadSpaces);
</script>
