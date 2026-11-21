<template>
  <div :style="aiSubPageStyle">
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Upload" :size="16" :color="cv('blue')" />
          <span :style="sectionTitleStyle">{{ $t('aiView.k150') }}</span>
        </div>
      </template>

      <ApFormItem id="import-file-field" :label="$t('aiView.k164')">
        <div :style="uploadRowStyle">
          <input
            id="import-file-field"
            ref="fileInputRef"
            type="file"
            :aria-label="$t('aiView.k172')"
            accept=".pdf,.docx,.xlsx,.md,.txt"
            :style="hiddenInputStyle"
            @change="handleFileChange"
          />
          <ApButton
            variant="secondary"
            size="small"
            :loading="uploading"
            @click="triggerFileInput"
          >
            <ApIcon name="Upload" :size="16" /> {{ $t('aiView.k151') }}
          </ApButton>
          <span v-if="selectedFile" :style="fileNameStyle">{{ selectedFile.name }}</span>
          <span v-else :style="uploadHintStyle">{{ $t('aiView.k152') }}</span>
          <ApButton
            v-if="selectedFile"
            variant="primary"
            size="small"
            :loading="uploading"
            @click="handleUpload"
          >
            <ApIcon name="Upload" :size="16" /> {{ $t('aiView.upload') }}
          </ApButton>
        </div>
      </ApFormItem>

      <ApFormItem :label="$t('aiView.k165')">
        <ApTextarea
          v-model="content"
          mono
          min-height="250px"
          :placeholder="$t('aiView.k170')"
        />
      </ApFormItem>

      <ApFormItem :label="$t('aiView.k166')" :associate="false">
        <ApSegmentedControl v-model="format" :options="formats" :aria-label="$t('aiView.k166')" />
      </ApFormItem>

      <div :style="compactActionRowStyle">
        <ApButton variant="primary" size="small" :loading="importing" :disabled="!content.trim()" @click="handleImport">
          <ApIcon name="Upload" :size="16" /> {{ $t('aiView.k154') }}
        </ApButton>
      </div>

      <ApCard v-if="result" size="small" :style="{ marginTop: AppleSpace.md }">
        <div :style="resultTitleStyle">
          <ApIcon name="CheckCircle" :size="14" :color="cv('green')" />
          {{ result.document.title }}
        </div>
        <div :style="resultMetaStyle">
          格式: {{ result.document.format || 'markdown' }} · {{ result.document.content.length }} 字 · 尚未写入空间
        </div>
        <div :style="{ ...compactActionRowStyle, marginTop: AppleSpace.sm }">
          <ApButton variant="primary" size="small" :loading="persisting" @click="persistResult">
            <ApIcon name="Download" :size="16" /> {{ $t('aiView.k155') }}
          </ApButton>
          <ApButton variant="ghost" size="small" @click="resetForm">{{ $t('aiView.k156') }}</ApButton>
        </div>
      </ApCard>
    </ApCard>

    <!-- 外部数据源连接器：后端 /ai/datasources/* 三个端点只解析不落库，
         解析结果单独展示，确认后再走上方既有导入路径生成文档。 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="statusRowStyle">
            <ApIcon name="Network" :size="16" :color="cv('teal')" />
            <span :style="sectionTitleStyle">{{ $t('aiView.k157') }}</span>
          </div>
          <ApSelect
            v-model="selectedSpaceId"
            :options="spaceOptions"
            :placeholder="$t('aiView.k17')"
            size="small"
            searchable
            :style="{ width: AI_TABLE_WIDTH.xxl }"
          />
        </div>
      </template>

      <ApFormItem :label="$t('aiView.k167')" :hint="$t('vAiUi.k22')">
        <div :style="searchBarStyle">
          <ApInput
            v-model="urlInput"
            size="small"
            placeholder="https://example.com/article"
            :style="{ flex: '1', minWidth: '40%' }"
          />
          <ApButton
            variant="secondary"
            size="small"
            :loading="scrapingUrl"
            :disabled="!urlInput.trim()"
            @click="handleScrapeUrl"
          >
            <ApIcon name="Globe" :size="16" /> {{ $t('aiView.k158') }}
          </ApButton>
        </div>
      </ApFormItem>

      <ApFormItem :label="$t('aiView.k168')" :hint="$t('vAiUi.k23')">
        <div :style="searchBarStyle">
          <ApInput
            v-model="siteUrlInput"
            size="small"
            placeholder="https://example.com/docs"
            :style="{ flex: '1', minWidth: '40%' }"
          />
          <ApInput
            v-model="maxPagesInput"
            size="small"
            type="number"
            :aria-label="$t('aiView.k173')"
            :placeholder="$t('aiView.k171')"
            :style="{ width: AI_TABLE_WIDTH.sm }"
          />
          <ApButton
            variant="secondary"
            size="small"
            :loading="scrapingSite"
            :disabled="!siteUrlInput.trim()"
            @click="handleScrapeSite"
          >
            <ApIcon name="Network" :size="16" /> {{ $t('aiView.k159') }}
          </ApButton>
        </div>
      </ApFormItem>

      <ApFormItem id="connector-file-field" :label="$t('aiView.k169')" :hint="$t('vAiUi.k24')">
        <div :style="uploadRowStyle">
          <input
            id="connector-file-field"
            ref="connectorFileInputRef"
            type="file"
            accept=".pdf,.docx,.xlsx,.csv,.md,.markdown,.txt,.html,.htm,.json"
            :style="hiddenInputStyle"
            @change="handleConnectorFileChange"
          />
          <ApButton variant="secondary" size="small" :loading="parsing" @click="triggerConnectorFileInput">
            <ApIcon name="FileText" :size="16" /> {{ $t('aiView.k151') }}
          </ApButton>
          <span v-if="selectedConnectorFile" :style="fileNameStyle">{{ selectedConnectorFile.name }}</span>
          <span v-else :style="uploadHintStyle">{{ $t('aiView.k160') }}</span>
          <ApButton
            v-if="selectedConnectorFile"
            variant="primary"
            size="small"
            :loading="parsing"
            @click="handleConnectorUpload"
          >
            <ApIcon name="Upload" :size="16" /> {{ $t('aiView.k161') }}
          </ApButton>
        </div>
      </ApFormItem>

      <ApCard v-if="connectorResult" size="small" :style="{ marginTop: AppleSpace.md }">
        <div :style="resultTitleStyle">
          <ApIcon name="CheckCircle" :size="14" :color="cv('green')" />
          {{ connectorResult.title || '（无标题）' }}
        </div>
        <div :style="metaStyle">
          <span :style="aiSubBadgeStyle">{{ connectorResult.kindLabel }}</span>
          <span :style="resultMetaStyle">{{ connectorResult.meta }}</span>
          <span :style="resultMetaStyle">{{ connectorResult.wordCount }} {{ $t('vAiUi.k25') }}</span>
        </div>
        <div v-if="connectorResult.pages.length" :style="{ ...listStyle(AppleSpace.sm), marginTop: AppleSpace.sm }">
          <div v-for="(page, idx) in connectorResult.pages" :key="page.url || idx" :style="metaItemStyle">
            <span :style="aiSubBadgeStyle">{{ idx + 1 }}</span>
            <span :style="fileNameStyle">{{ page.title || page.url }}</span>
            <span :style="resultMetaStyle">{{ page.word_count }} {{ $t('vAiUi.k25') }}</span>
          </div>
        </div>
        <div :style="{ ...aiSubBodyStyle, marginTop: AppleSpace.sm, cursor: 'default' }">{{ connectorPreviewText }}</div>
        <div :style="{ ...compactActionRowStyle, marginTop: AppleSpace.sm }">
          <ApButton
            variant="primary"
            size="small"
            :loading="importingParsed"
            :disabled="!connectorResult.content.trim()"
            @click="importConnectorResult"
          >
            <ApIcon name="Upload" :size="16" /> {{ $t('aiView.k162') }}
          </ApButton>
          <ApButton variant="ghost" size="small" @click="connectorResult = null">{{ $t('aiView.k163') }}</ApButton>
        </div>
      </ApCard>
    </ApCard>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { logger } from '@/utils/logger';
import ApIcon from '@/components/apple/ApIcon.vue';
import { cv, AppleFontWeight, AppleSpace, AppleType } from '@/utils/core/apple-tokens';
import {
  AI_TABLE_WIDTH,
  aiSubActionRowStyle,
  aiSubBadgeStyle,
  aiSubBodyStyle,
  aiSubPageStyle,
  aiSubSectionHeaderStyle,
  aiSubSectionTitleStyle,
  listStyle,
  metaItemStyle,
  metaStyle,
  searchBarStyle,
  statusRowStyle,
} from '@/styles/ai/ai-page-styles';
import {
  importDocument,
  importFile,
  parseConnectorFile,
  scrapeSite,
  scrapeUrl,
  type ImportDocumentResponse,
  type ScrapeSitePage,
} from '@/api/modules/ai/ai-import';
import { importDocuments } from '@/api/modules/document/document';
import { useSpaceStore } from '@/stores/workspace/space';
import { useUIStore } from '@/stores/app/ui';
import type { Space } from '@/types/models';
import ApCard from '@/components/apple/ApCard.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';

const router = useRouter();
const ui = useUIStore();
const spaceStore = useSpaceStore();

const content = ref('');
const format = ref('markdown');
const importing = ref(false);
const result = ref<ImportDocumentResponse | null>(null);
const fileInputRef = ref<HTMLInputElement | null>(null);
const selectedFile = ref<File | null>(null);
const uploading = ref(false);

const formats = [
  { label: 'Markdown', value: 'markdown' },
  { label: 'HTML', value: 'html' },
  { label: t('aiView.k174'), value: 'text' },
];

async function handleImport() {
  if (!content.value.trim()) return;
  importing.value = true;
  result.value = null;
  try {
    const res = await importDocument({ content: content.value, format: format.value });
    result.value = res;
    ui.showToast(t('aiView.k175'), 'success');
  } catch (e: unknown) {
    ui.showToast((e as { message?: string }).message || '导入失败', 'error');
  } finally {
    importing.value = false;
  }
}

const persisting = ref(false);

// /ai/import 只做转换，落库必须显式走 /spaces/:id/import
async function persistResult() {
  const doc = result.value?.document;
  if (!doc?.content?.trim()) return;
  const spaceId = currentSpaceId();
  if (!spaceId) {
    ui.showToast(t('aiView.k176'), 'error');
    return;
  }
  persisting.value = true;
  try {
    const r = await importDocuments(spaceId, [
      { path: doc.title || '导入文档', content: doc.content, format: 'md' },
    ]);
    if (r.created > 0) {
      ui.showToast(`已写入 ${r.created} 篇${r.skipped ? `，跳过 ${r.skipped}` : ''}`, 'success');
      result.value = null;
      router.push(`/space/${spaceId}`);
      return;
    }
    ui.showToast(r.errors?.length ? `写入失败：${r.errors[0]}` : '没有文档被写入', 'error');
  } catch (e: unknown) {
    reportFailure('写入空间', e);
  } finally {
    persisting.value = false;
  }
}

function resetForm() {
  content.value = '';
  result.value = null;
  selectedFile.value = null;
  if (fileInputRef.value) {
    fileInputRef.value.value = '';
  }
}

function triggerFileInput() {
  fileInputRef.value?.click();
}

function handleFileChange(e: Event) {
  const target = e.target as HTMLInputElement;
  const file = target.files?.[0];
  if (file) {
    selectedFile.value = file;
  }
}

async function handleUpload() {
  if (!selectedFile.value) return;
  uploading.value = true;
  result.value = null;
  try {
    const res = await importFile(selectedFile.value, currentSpaceId());
    result.value = res;
    ui.showToast(t('aiView.k177'), 'success');
    selectedFile.value = null;
    if (fileInputRef.value) {
      fileInputRef.value.value = '';
    }
  } catch (e: unknown) {
    logger.error('[AIImport] File upload failed:', e);
    ui.showToast((e as { message?: string }).message || '文件导入失败', 'error');
  } finally {
    uploading.value = false;
  }
}

// ===== 外部数据源连接器（URL 抓取 / 站点抓取 / 文件解析上传）=====
const spaces = ref<Space[]>([]);
const selectedSpaceId = ref<string | number | null>(null);
const spaceOptions = computed(() => spaces.value.map((s) => ({ label: s.name, value: s.id })));

const urlInput = ref('');
const siteUrlInput = ref('');
const maxPagesInput = ref('');
const scrapingUrl = ref(false);
const scrapingSite = ref(false);
const parsing = ref(false);
const importingParsed = ref(false);
const connectorFileInputRef = ref<HTMLInputElement | null>(null);
const selectedConnectorFile = ref<File | null>(null);

/** 连接器端点返回的是解析后的正文，没有 document 结构，故与 result 分开存 */
interface ConnectorPreview {
  kindLabel: string;
  title: string;
  meta: string;
  content: string;
  pages: ScrapeSitePage[];
  wordCount: number;
}
const connectorResult = ref<ConnectorPreview | null>(null);

// aiSubBodyStyle 自带 maxHeight + overflowY，但整篇长文塞进 DOM 仍会拖慢渲染：预览截断，导入用全文
const PREVIEW_LIMIT = 5000;
const connectorPreviewText = computed(() => {
  const full = connectorResult.value?.content || '';
  if (full.length <= PREVIEW_LIMIT) return full;
  return `${full.slice(0, PREVIEW_LIMIT)}\n\n…（预览已截断，导入使用全文）`;
});

function currentSpaceId(): number {
  return selectedSpaceId.value == null ? 0 : Number(selectedSpaceId.value);
}

function reportFailure(action: string, e: unknown) {
  logger.error(`[AIImport] ${action} failed:`, e);
  ui.showToast((e as { message?: string }).message || `${action}失败`, 'error');
}

async function handleScrapeUrl() {
  const url = urlInput.value.trim();
  if (!url) return;
  scrapingUrl.value = true;
  connectorResult.value = null;
  try {
    const res = await scrapeUrl(url, currentSpaceId());
    connectorResult.value = {
      kindLabel: 'URL 抓取',
      title: res.title || res.url,
      meta: res.url,
      content: res.content || '',
      pages: [],
      wordCount: res.word_count || 0,
    };
    ui.showToast(t('aiView.k178'), 'success');
  } catch (e: unknown) {
    reportFailure('URL 抓取', e);
  } finally {
    scrapingUrl.value = false;
  }
}

async function handleScrapeSite() {
  const url = siteUrlInput.value.trim();
  if (!url) return;
  scrapingSite.value = true;
  connectorResult.value = null;
  try {
    const requested = Number(maxPagesInput.value);
    const maxPages = Number.isFinite(requested) && requested > 0 ? Math.min(Math.trunc(requested), 50) : 0;
    const res = await scrapeSite(url, currentSpaceId(), maxPages);
    // 后端 pages 是未初始化切片，抓不到任何页时返回 null
    const pages = res.pages || [];
    if (!pages.length) {
      ui.showToast(t('aiView.k179'), 'error');
      return;
    }
    connectorResult.value = {
      kindLabel: '站点抓取',
      title: pages[0]?.title || url,
      meta: `${res.total ?? pages.length} 页 · 入口 ${url}`,
      content: pages.map((p) => `# ${p.title || p.url}\n\n${p.content}`).join('\n\n---\n\n'),
      pages,
      wordCount: pages.reduce((sum, p) => sum + (p.word_count || 0), 0),
    };
    ui.showToast(t('sentUi.k37', { p1: pages.length }), 'success');
  } catch (e: unknown) {
    reportFailure('站点抓取', e);
  } finally {
    scrapingSite.value = false;
  }
}

function triggerConnectorFileInput() {
  connectorFileInputRef.value?.click();
}

function handleConnectorFileChange(e: Event) {
  const target = e.target as HTMLInputElement;
  const file = target.files?.[0];
  if (file) {
    selectedConnectorFile.value = file;
  }
}

async function handleConnectorUpload() {
  if (!selectedConnectorFile.value) return;
  parsing.value = true;
  connectorResult.value = null;
  try {
    const res = await parseConnectorFile(selectedConnectorFile.value, currentSpaceId());
    connectorResult.value = {
      kindLabel: '文件解析',
      title: res.filename,
      meta: `${res.filename} · 服务端解析`,
      content: res.content || '',
      pages: [],
      wordCount: res.word_count || 0,
    };
    ui.showToast(t('aiView.k180'), 'success');
  } catch (e: unknown) {
    reportFailure('文件解析', e);
  } finally {
    parsing.value = false;
  }
}

async function importConnectorResult() {
  const content = connectorResult.value?.content;
  if (!content || !content.trim()) return;
  importingParsed.value = true;
  result.value = null;
  try {
    // 连接器只解析不入库，落库复用既有 /ai/import 结果展示路径
    result.value = await importDocument({ content, format: 'markdown' });
    ui.showToast(t('aiView.k181'), 'success');
  } catch (e: unknown) {
    reportFailure('导入解析内容', e);
  } finally {
    importingParsed.value = false;
  }
}

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
    const first = spaces.value[0];
    if (first) selectedSpaceId.value = first.id;
  } catch (e: unknown) {
    logger.error('[AIImport] Failed to load spaces:', e);
  }
}

onMounted(loadSpaces);

const sectionTitleStyle = aiSubSectionTitleStyle;
const compactActionRowStyle = { ...aiSubActionRowStyle, gap: AppleSpace.sm };

const resultTitleStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  marginBottom: AppleSpace.xs,
};

const resultMetaStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-secondary'),
};

const uploadRowStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  flexWrap: 'wrap' as const,
};

const hiddenInputStyle = {
  display: 'none',
};

const fileNameStyle = {
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-primary'),
  fontWeight: String(AppleFontWeight.medium),
};

const uploadHintStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
};
</script>
