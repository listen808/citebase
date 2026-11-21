<template>
  <div :style="aiSubPageStyle">
    <!-- 编译入口 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="BookMarked" :size="18" :color="cv('blue')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('vAiUi.k142') }}</span>
        </div>
      </template>

      <div :style="formStyle">
        <div :style="fieldRowStyle">
          <div :style="fieldStyle">
            <div :style="labelStyle">{{ $t('vAiUi.k143') }}</div>
            <ApInput v-model="spaceId" :style="inputStyle" :aria-label="$t('vAiUi.k143')" :placeholder="$t('vAiUi.k158')" />
            <!-- 非法值不只在提交时拦：写在这里用户才知道为什么按钮点不动 -->
            <div v-if="spaceIdInvalid" :style="fieldErrorStyle">{{ $t('vAiUi.k144') }}</div>
          </div>
          <div :style="fieldStyle">
            <div :style="labelStyle">{{ $t('vAiUi.k145') }}</div>
            <ApInput v-model="concurrency" :style="inputStyle" :aria-label="$t('vAiUi.k145')" :placeholder="$t('vAiUi.k159')" />
          </div>
        </div>

        <div :style="fieldStyle">
          <div :style="labelStyle">{{ $t('vAiUi.k146') }}</div>
          <ApInput v-model="docIds" :style="inputStyle" :aria-label="$t('vAiUi.k166')" :placeholder="$t('vAiUi.k160')" />
          <div :style="hintStyle">
            {{ $t('vAiUi.k147') }}
          </div>
        </div>

        <div :style="aiSubActionRowStyle">
          <ApButton
            variant="primary"
            size="small"
            :loading="compiling"
            :disabled="!parsedSpaceId || !docIds.trim()"
            :tooltip="parsedSpaceId ? '' : '请先填写空间 ID'"
            @click="handleCompile"
          >
            <ApIcon name="Hammer" :size="16" /> {{ $t('vAiUi.k148') }}
          </ApButton>
          <ApButton variant="secondary" size="small" :loading="loadingPages" @click="loadPages">
            <ApIcon name="RefreshCw" :size="16" /> {{ $t('vAiUi.k149') }}
          </ApButton>
          <ApButton variant="secondary" size="small" :loading="runningLint" @click="handleLint">
            <ApIcon name="Stethoscope" :size="16" /> {{ $t('vAiUi.k150') }}
          </ApButton>
        </div>

        <div v-if="compileResult" :style="resultBannerStyle">
          新建 {{ compileResult.created.length }} 页 · 更新 {{ compileResult.updated.length }} 页 ·
          跳过 {{ compileResult.skipped }} 页（内容未变更）
        </div>
      </div>
    </ApCard>

    <!-- 页面列表 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Files" :size="18" :color="cv('indigo')" />
          <span :style="aiSubSectionTitleStyle">编译产物（{{ pages.length }}）</span>
        </div>
      </template>

      <ApSkeleton v-if="loadingPages" variant="card" :lines="3" />

      <ApEmpty
        v-else-if="pages.length === 0"
        icon="File"
        :title="$t('vAiUi.k162')"
        :description="$t('vAiUi.k168')"
      />

      <div v-else :style="tableWrapStyle">
        <div v-for="p in pages" :key="p.id" :style="rowStyle">
          <div :style="rowMainStyle">
            <div :style="rowTitleStyle">{{ p.title }}</div>
            <div :style="rowMetaStyle">
              <code :style="codeStyle">{{ p.slug }}</code>
              <span :style="chipStyle">{{ p.type }}</span>
              <span :style="chipStyle">{{ p.status }}</span>
              <span :style="chipStyle">{{ p.source_count }} {{ $t('vAiUi.k171') }}</span>
              <span v-if="p.provenance === 'human'" :style="humanChipStyle">{{ $t('vAiUi.k151') }}</span>
              <span v-if="p.superseded_by" :style="supersededChipStyle">{{ $t('vAiUi.k242', { p1: p.superseded_by }) }}</span>
            </div>
            <div v-if="p.summary" :style="rowSummaryStyle">{{ p.summary }}</div>
          </div>
          <div :style="rowActionsStyle">
            <ApButton
              v-if="p.status === 'draft'"
              variant="primary"
              size="small"
              :loading="isBusy(p.slug, 'publish')"
              :disabled="isBusy(p.slug)"
              @click="handlePublish(p.slug, p.base_version)"
            >
              {{ $t('vAiUi.k152') }}
            </ApButton>
            <ApButton
              variant="ghost"
              size="small"
              :loading="isBusy(p.slug, 'mark')"
              :disabled="isBusy(p.slug)"
              @click="handleMarkHuman(p.slug)"
            >
              {{ $t('vAiUi.k153') }}
            </ApButton>
            <ApButton
              v-if="p.provenance !== 'human' && !p.superseded_by && p.status !== 'draft'"
              variant="ghost"
              size="small"
              :loading="isBusy(p.slug, 'supersede')"
              :disabled="isBusy(p.slug)"
              @click="requestSupersede(p)"
            >{{ $t('vAiUi.k239') }}</ApButton>
            <ApButton variant="ghost" size="small" :disabled="isBusy(p.slug)" @click="handleRelated(p.slug)">{{ $t('vAiUi.k154') }}</ApButton>
            <ApButton variant="ghost" size="small" :disabled="isBusy(p.slug)" @click="requestDelete(p)">{{ $t('vAiUi.delete') }}</ApButton>
          </div>
        </div>
      </div>
    </ApCard>

    <!-- 检索验证：目录 ∪ 全文 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Search" :size="18" :color="cv('teal')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('vAiUi.k155') }}</span>
        </div>
      </template>
      <div :style="formStyle">
        <ApInput v-model="question" :style="inputStyle" :aria-label="$t('vAiUi.k167')" :placeholder="$t('vAiUi.k161')" />
        <div :style="aiSubActionRowStyle">
          <ApButton
            variant="primary"
            size="small"
            :loading="searching"
            :disabled="!parsedSpaceId || !question.trim()"
            @click="handleSearch"
          >
            <ApIcon name="Search" :size="16" /> {{ $t('vAiUi.k156') }}
          </ApButton>
        </div>
        <div v-if="hits.length > 0" :style="hitListStyle">
          <div v-for="h in hits" :key="h.slug" :style="hitStyle">
            <div :style="hitTitleStyle">
              {{ h.title }}
              <span :style="viaChipStyle(h.via)">{{ viaLabel(h.via) }}</span>
            </div>
            <div :style="rowSummaryStyle">{{ h.summary }}</div>
          </div>
        </div>
      </div>
    </ApCard>

    <!-- 洞察与体检 -->
    <div :style="twoColStyle">
      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="Lightbulb" :size="18" :color="cv('orange')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('vAiUi.k157') }}</span>
          </div>
        </template>
        <div :style="aiSubActionRowStyle">
          <ApButton variant="secondary" size="small" :loading="loadingInsights" @click="loadInsights">
            <ApIcon name="RefreshCw" :size="16" /> {{ $t('vAiUi.refresh') }}
          </ApButton>
        </div>
        <ApEmpty v-if="insights.length === 0" icon="Compass" :title="$t('vAiUi.k163')" :description="$t('vAiUi.k169')" />
        <div v-else :style="listStyle">
          <div v-for="(it, i) in insights" :key="i" :style="listItemStyle">
            <div :style="listItemTitleStyle">{{ kindLabel(it.kind) }} · {{ it.slugs.join(' ↔ ') }}</div>
            <div :style="rowSummaryStyle">{{ it.detail }}</div>
          </div>
        </div>
      </ApCard>

      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="AlertTriangle" :size="18" :color="cv('red')" />
            <span :style="aiSubSectionTitleStyle">体检问题（{{ lintIssues.length }}）</span>
          </div>
        </template>
        <ApEmpty v-if="lintIssues.length === 0" icon="Check" :title="$t('vAiUi.k164')" :description="$t('vAiUi.k170')" />
        <div v-else :style="listStyle">
          <div v-for="(it, i) in lintIssues" :key="i" :style="listItemStyle">
            <div :style="listItemTitleStyle">
              <span :style="severityChipStyle(it.severity)">{{ it.severity }}</span>
              {{ it.page_slug || $t('fallback.k117') }} · {{ it.category }}
            </div>
            <div :style="rowSummaryStyle">{{ it.detail }}</div>
          </div>
        </div>
      </ApCard>
    </div>

    <!-- 相关页面 -->
    <ApCard v-if="related.length > 0" size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="GitBranch" :size="18" :color="cv('purple')" />
          <span :style="aiSubSectionTitleStyle">相关页面 · {{ relatedSlug }}</span>
        </div>
      </template>
      <div :style="listStyle">
        <div v-for="r in related" :key="r.slug" :style="listItemStyle">
          <div :style="listItemTitleStyle">{{ r.title }}（{{ r.score.toFixed(2) }}）</div>
          <div :style="rowSummaryStyle">{{ r.reason }}</div>
        </div>
      </div>
    </ApCard>

    <!-- 删除原先直接发 DELETE，与旁边「相关」「标记人工确认」两颗同尺寸的 ghost 按钮并排，单击即执行 -->
    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('vAiUi.k165')"
      :message="deleteMessage"
      :confirm-text="$t('vAiUi.delete')"
      type="danger"
      @confirm="doDelete"
    />

    <!-- 取代：从当前空间的已发布页面里挑一个作为替代 -->
    <div v-if="showSupersedeModal" :style="overlayStyle" @click.self="showSupersedeModal = false">
      <div :style="modalStyle">
        <div :style="modalTitleStyle">{{ $t('vAiUi.k241') }} · {{ pendingSupersede?.title }}</div>
        <div :style="modalBodyStyle">
          <div :style="hintStyle">{{ $t('vAiUi.k243') }}</div>
          <ApInput
            v-model="supersedeFilter"
            :style="{ ...inputStyle, marginBottom: AppleSpace.sm }"
            :placeholder="$t('vAiUi.k244')"
          />
          <div v-if="supersedeCandidates.length === 0" :style="hintStyle">{{ $t('vAiUi.k162') }}</div>
          <div v-else :style="supersedeListStyle">
            <div
              v-for="c in supersedeCandidates"
              :key="c.slug"
              :style="supersedeItemStyle(c.slug === supersedeTarget)"
              @click="supersedeTarget = c.slug"
            >
              <div :style="rowTitleStyle">{{ c.title }}</div>
              <div :style="rowMetaStyle">
                <code :style="codeStyle">{{ c.slug }}</code>
                <span :style="chipStyle">{{ c.type }}</span>
              </div>
            </div>
          </div>
        </div>
        <div :style="modalFooterStyle">
          <ApButton variant="secondary" size="small" @click="showSupersedeModal = false">{{ $t('vAiUi.cancel') }}</ApButton>
          <ApButton
            variant="primary"
            size="small"
            :disabled="!supersedeTarget || isBusy(pendingSupersede?.slug ?? '', 'supersede')"
            :loading="isBusy(pendingSupersede?.slug ?? '', 'supersede')"
            @click="doSupersede"
          >{{ $t('vAiUi.k239') }}</ApButton>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed, ref } from 'vue'
import ApSkeleton from '@/components/apple/ApSkeleton.vue'

import { cv, AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleType, AppleTransition, AppleGridFloor, AppleZIndex, appleCardGrid, resolvedColor } from '@/utils/core/apple-tokens';
import { withAlpha } from '@/utils/core/shared';
import { errorStyle } from '@/styles/component/form-field-styles'
import {
  aiSubPageStyle,
  aiSubActionRowStyle,
  aiSubSectionHeaderStyle,
  aiSubSectionTitleStyle,
} from '@/styles/ai/ai-page-styles'
import { useUIStore } from '@/stores/app/ui'
import {
  compileWiki,
  listWikiPages,
  publishWikiPage,
  markWikiPageHumanEdited,
  deleteWikiPage,
  supersedeWikiPage,
  searchWiki,
  getWikiRelated,
  getWikiInsights,
  runWikiLint,
  type WikiPage,
  type WikiHit,
  type WikiRelated,
  type WikiInsight,
  type WikiLintIssue,
  type CompileResult,
} from '@/api/modules/ai/ai-wiki'

const ui = useUIStore()

const spaceId = ref('')
const concurrency = ref('4')
const docIds = ref('')
const question = ref('')

const compiling = ref(false)
const loadingPages = ref(false)
const searching = ref(false)
const loadingInsights = ref(false)
const runningLint = ref(false)

// 三个写操作（发布 / 标记人工确认 / 删除）原先既没有二次确认也没有请求期锁，
// 双击就是两条 PUT/DELETE。锁按「哪一行的哪个动作」记：行与行互不相干，不必整表置灰。
const busy = ref<{ slug: string; action: string } | null>(null)
const pendingDelete = ref<WikiPage | null>(null)
const showDeleteModal = ref(false)

// 取代：从当前空间已发布页里挑替代页
const pendingSupersede = ref<WikiPage | null>(null)
const showSupersedeModal = ref(false)
const supersedeTarget = ref('')
const supersedeFilter = ref('')

/** 不传 action = 这一行有任何在途写操作，整行按钮都该锁住 */
function isBusy(slug: string, action?: string): boolean {
  const b = busy.value
  if (!b) return false
  return action ? b.slug === slug && b.action === action : b.slug === slug
}

const compileResult = ref<CompileResult | null>(null)
const pages = ref<WikiPage[]>([])
const hits = ref<WikiHit[]>([])
const related = ref<WikiRelated[]>([])
const relatedSlug = ref('')
const insights = ref<WikiInsight[]>([])
const lintIssues = ref<WikiLintIssue[]>([])

function parseIds(raw: string): number[] {
  return raw
    .split(/[,，\s]+/)
    .map((s) => s.trim())
    .filter(Boolean)
    .map((s) => Number(s))
    .filter((n) => Number.isFinite(n) && n > 0)
}

/** 空间 ID 只接受正整数：空、负数、小数、非数字一律算非法（原先会被直接 Number 化发进 API） */
const parsedSpaceId = computed(() => {
  const raw = spaceId.value.trim()
  if (!raw) return 0
  const n = Number(raw)
  return Number.isInteger(n) && n > 0 ? n : 0
})

const spaceIdInvalid = computed(() => spaceId.value.trim() !== '' && parsedSpaceId.value === 0)

function currentSpaceId(): number {
  return parsedSpaceId.value
}

/** 提交路径上的守卫：非法时给一次可见反馈再中止，而不是静默 return 或把 0/负数发出去 */
const { t } = useI18n();
function ensureSpaceId(): number {
  const sid = currentSpaceId()
  if (!sid) {
    ui.showToast(t('vAiUi.k179'), 'warning')
  }
  return sid
}

async function handleCompile() {
  const sid = currentSpaceId()
  const ids = parseIds(docIds.value)
  if (!sid || ids.length === 0) {
    ui.showToast(t('vAiUi.k180'), 'error')
    return
  }
  compiling.value = true
  try {
    compileResult.value = await compileWiki({
      space_id: sid,
      doc_ids: ids,
      concurrency: Number(concurrency.value) || 4,
    })
    ui.showToast(t('vAiUi.k181'), 'success')
    await loadPages()
  } catch (e: unknown) {
    ui.showToast(e instanceof Error ? e.message: t('vAiUi.k172'), 'error')
  } finally {
    compiling.value = false
  }
}

async function loadPages() {
  const sid = ensureSpaceId()
  if (!sid) return
  loadingPages.value = true
  try {
    const res = await listWikiPages(sid)
    pages.value = res.pages ?? []
  } catch (e: unknown) {
    ui.showToast(e instanceof Error ? e.message: t('vAiUi.k173'), 'error')
  } finally {
    loadingPages.value = false
  }
}

async function handlePublish(slug: string, baseVersion: number) {
  if (isBusy(slug)) return
  const sid = ensureSpaceId()
  if (!sid) return
  busy.value = { slug, action: 'publish' }
  try {
    await publishWikiPage(sid, slug, baseVersion)
    ui.showToast(t('vAiUi.k182'), 'success')
    await loadPages()
  } catch (e: unknown) {
    ui.showToast(e instanceof Error ? e.message: t('vAiUi.k174'), 'error')
  } finally {
    busy.value = null
  }
}

async function handleMarkHuman(slug: string) {
  if (isBusy(slug)) return
  const sid = ensureSpaceId()
  if (!sid) return
  busy.value = { slug, action: 'mark' }
  try {
    await markWikiPageHumanEdited(sid, slug)
    ui.showToast(t('vAiUi.k183'), 'success')
    await loadPages()
  } catch (e: unknown) {
    ui.showToast(e instanceof Error ? e.message: t('vAiUi.k175'), 'error')
  } finally {
    busy.value = null
  }
}

function requestDelete(page: WikiPage) {
  pendingDelete.value = page
  showDeleteModal.value = true
}

const deleteMessage = computed(() => {
  const page = pendingDelete.value
  if (!page) return ''
  return `确定删除知识页面「${page.title}」（${page.slug}）？删除后需要重新编译才能再生成。`
})

async function doDelete() {
  const page = pendingDelete.value
  pendingDelete.value = null
  if (!page) return
  await handleDelete(page.slug)
}

async function handleDelete(slug: string) {
  if (isBusy(slug)) return
  const sid = ensureSpaceId()
  if (!sid) return
  busy.value = { slug, action: 'delete' }
  try {
    await deleteWikiPage(sid, slug)
    ui.showToast(t('vAiUi.k184'), 'success')
    await loadPages()
  } catch (e: unknown) {
    ui.showToast(e instanceof Error ? e.message: t('vAiUi.k176'), 'error')
  } finally {
    busy.value = null
  }
}

function requestSupersede(page: WikiPage) {
  pendingSupersede.value = page
  supersedeTarget.value = ''
  supersedeFilter.value = ''
  showSupersedeModal.value = true
}

const supersedeCandidates = computed(() => {
  const q = supersedeFilter.value.trim().toLowerCase()
  return pages.value.filter(
    (p) =>
      p.slug !== pendingSupersede.value?.slug &&
      p.status === 'published' &&
      !p.superseded_by &&
      (q === '' || p.slug.toLowerCase().includes(q) || p.title.toLowerCase().includes(q)),
  )
})

async function doSupersede() {
  const page = pendingSupersede.value
  const target = supersedeTarget.value
  if (!page || !target) return
  showSupersedeModal.value = false
  const sid = ensureSpaceId()
  if (!sid) return
  if (isBusy(page.slug)) return
  busy.value = { slug: page.slug, action: 'supersede' }
  try {
    await supersedeWikiPage(sid, page.slug, target)
    ui.showToast(t('vAiUi.k245'), 'success')
    await loadPages()
  } catch (e: unknown) {
    ui.showToast(e instanceof Error ? e.message: t('vAiUi.k246'), 'error')
  } finally {
    busy.value = null
  }
}

async function handleSearch() {
  const sid = ensureSpaceId()
  if (!sid) return
  searching.value = true
  try {
    const res = await searchWiki(sid, question.value.trim())
    hits.value = res.hits ?? []
  } catch (e: unknown) {
    ui.showToast(e instanceof Error ? e.message: t('vAiUi.k177'), 'error')
  } finally {
    searching.value = false
  }
}

async function handleRelated(slug: string) {
  const sid = ensureSpaceId()
  if (!sid) return
  try {
    const res = await getWikiRelated(sid, slug)
    related.value = res.related ?? []
    relatedSlug.value = slug
  } catch (e: unknown) {
    ui.showToast(e instanceof Error ? e.message: t('vAiUi.k173'), 'error')
  }
}

async function loadInsights() {
  const sid = ensureSpaceId()
  if (!sid) return
  loadingInsights.value = true
  try {
    const res = await getWikiInsights(sid)
    insights.value = res.insights ?? []
  } catch (e: unknown) {
    ui.showToast(e instanceof Error ? e.message: t('vAiUi.k173'), 'error')
  } finally {
    loadingInsights.value = false
  }
}

async function handleLint() {
  const sid = ensureSpaceId()
  if (!sid) return
  runningLint.value = true
  try {
    const res = await runWikiLint(sid)
    lintIssues.value = res.issues ?? []
    ui.showToast(t('sentUi.k40', { p1: lintIssues.value.length }), 'success')
  } catch (e: unknown) {
    ui.showToast(e instanceof Error ? e.message: t('vAiUi.k178'), 'error')
  } finally {
    runningLint.value = false
  }
}

function viaLabel(via: string) {
  if (via === 'directory') return '目录'
  if (via === 'fulltext') return '全文'
  return '双路'
}

function kindLabel(kind: string) {
  if (kind === 'surprising') return '意外连接'
  if (kind === 'bridge') return '桥节点'
  return '知识缺口'
}

const formStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
}

const fieldRowStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.stat})`,
  gap: AppleSpace.md,
}

const fieldStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
}

const labelStyle = {
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-secondary'),
}

const inputStyle = {
  width: '100%',
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.small,
  border: `1px solid ${cv('border-light')}`,
  background: cv('surface-panel'),
  color: cv('text-primary'),
  fontSize: AppleType.bodyPrimary.size,
  fontFamily: AppleFont.text,
  transition: AppleTransition.default,
}

const fieldErrorStyle = errorStyle()

const hintStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
}

const resultBannerStyle = {
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.small,
  background: cv('green-light'),
  color: cv('green'),
  fontSize: AppleType.bodyPrimary.size,
}

const tableWrapStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
}

const rowStyle = {
  display: 'flex',
  alignItems: 'flex-start',
  justifyContent: 'space-between',
  gap: AppleSpace.md,
  padding: `${AppleSpace.sm} 0`,
  borderBottom: `1px solid ${cv('border-light')}`,
}

const rowMainStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
  minWidth: 0,
  flex: 1,
}

const rowTitleStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
}

const rowMetaStyle = {
  display: 'flex',
  flexWrap: 'wrap' as const,
  gap: AppleSpace.xs,
  alignItems: 'center',
}

const chipStyle = {
  padding: `1px ${AppleSpace.xs}`,
  borderRadius: AppleRadius.pill,
  background: cv('bg-tertiary'),
  color: cv('text-secondary'),
  fontSize: AppleType.microUI.size,
}

const humanChipStyle = {
  ...chipStyle,
  background: cv('orange-light'),
  color: cv('orange'),
}

const codeStyle = {
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
}

const rowSummaryStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
}

const rowActionsStyle = {
  display: 'flex',
  flexWrap: 'wrap' as const,
  gap: AppleSpace.xs,
  flexShrink: 0,
}

const hitListStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
}

const hitStyle = {
  padding: AppleSpace.sm,
  borderRadius: AppleRadius.small,
  border: `1px solid ${cv('border-light')}`,
}

const hitTitleStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
}

const viaChipStyle = (via: string) => ({
  padding: `1px ${AppleSpace.xs}`,
  borderRadius: AppleRadius.pill,
  fontSize: AppleType.microUI.size,
  background: via === 'both' ? cv('blue-light') : cv('bg-tertiary'),
  color: via === 'both' ? cv('blue') : cv('text-secondary'),
})

const twoColStyle = appleCardGrid('entity', AppleSpace.md)

const listStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
  marginTop: AppleSpace.sm,
}

const listItemStyle = {
  padding: AppleSpace.sm,
  borderRadius: AppleRadius.small,
  border: `1px solid ${cv('border-light')}`,
}

const listItemTitleStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  flexWrap: 'wrap' as const,
}

const severityChipStyle = (severity: string) => ({
  padding: `1px ${AppleSpace.xs}`,
  borderRadius: AppleRadius.pill,
  fontSize: AppleType.microUI.size,
  background: severity === 'warning' ? cv('orange-light') : cv('bg-tertiary'),
  color: severity === 'warning' ? cv('orange') : cv('text-secondary'),
})

const supersededChipStyle = {
  ...chipStyle,
  background: cv('red-light'),
  color: cv('red'),
}

const overlayStyle = {
  position: 'fixed' as const,
  inset: 0,
  background: withAlpha(resolvedColor('surface-chrome'), 0.4),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  zIndex: AppleZIndex.modal,
}

const modalStyle = {
  background: cv('surface-panel'),
  borderRadius: AppleRadius.medium,
  padding: AppleSpace.lg,
  width: '100%',
  maxWidth: 480,
  maxHeight: '80vh',
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
}

const modalTitleStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
}

const modalBodyStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
  overflowY: 'auto' as const,
}

const modalFooterStyle = {
  display: 'flex',
  justifyContent: 'flex-end' as const,
  gap: AppleSpace.sm,
}

const supersedeListStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
}

const supersedeItemStyle = (selected: boolean) => ({
  ...listItemStyle,
  border: `1px solid ${selected ? cv('blue') : cv('border-light')}`,
  background: selected ? cv('blue-light') : 'transparent',
  cursor: 'pointer' as const,
})
</script>
