<template>
  <div :style="panelStyle">
    <div :style="headerStyle" @click="expanded = !expanded">
      <!-- 2026-10-26：与 DocCommentPanel 的面板头同构（图标+标题左、chevron 右）——
           此前 Properties 无图标而 Comments 有（MessageSquare），同屏两个折叠面板两套语言。 -->
      <div :style="headerLeftStyle">
        <ApIcon name="SlidersHorizontal" :size="14" :style="{ color: cv('text-tertiary') }" />
        <span :style="titleStyle">{{ $t('shell.k63') }}</span>
      </div>
      <ApButton variant="ghost" size="mini" icon-only tabindex="-1" :aria-label="$t('shell.k81')">
        <ApIcon name="ChevronRight" :size="16" :style="chevronStyle" />
      </ApButton>
    </div>

    <div v-show="expanded" :style="bodyStyle">
      <!-- 基本信息 -->
      <div :style="metaGridStyle">
        <div :style="metaItemStyle">
          <span :style="metaLabelStyle">{{ $t('shell.k64') }}</span>
          <span :style="metaValueStyle">{{ document?.type === 1 ? '页面' : '目录' }}</span>
        </div>
        <div :style="metaItemStyle">
          <span :style="metaLabelStyle">{{ $t('shell.k65') }}</span>
          <router-link v-if="document?.created_by" :to="`/user/${document.created_by}`" :style="linkStyle">
            {{ document?.creator_name || '-' }}
          </router-link>
          <span v-else :style="metaValueStyle">-</span>
        </div>
        <div :style="metaItemStyle">
          <span :style="metaLabelStyle">{{ $t('shell.k66') }}</span>
          <span :style="metaValueStyle">{{ formatTime(document?.create_time) }}</span>
        </div>
        <div :style="metaItemStyle">
          <span :style="metaLabelStyle">{{ $t('shell.k67') }}</span>
          <router-link v-if="document?.edit_user_id" :to="`/user/${document.edit_user_id}`" :style="linkStyle">
            {{ document?.editor_name || '-' }}
          </router-link>
          <span v-else :style="metaValueStyle">-</span>
        </div>
        <div :style="metaItemStyle">
          <span :style="metaLabelStyle">{{ $t('shell.k68') }}</span>
          <span :style="metaValueStyle">{{ formatTimeAgo(document?.update_time) }}</span>
        </div>
        <div :style="metaItemStyle">
          <span :style="metaLabelStyle">{{ $t('shell.k69') }}</span>
          <span :style="metaValueStyle">{{ wordCount.toLocaleString() }}</span>
        </div>
        <div :style="metaItemStyle">
          <span :style="metaLabelStyle">{{ $t('shell.k70') }}</span>
          <span :style="metaValueStyle">约{{ readingTime }}分钟</span>
        </div>
      </div>

      <div :style="dividerStyle" />

      <!-- 外观 -->
      <div :style="sectionStyle">
        <span :style="sectionTitleStyle">{{ $t('shell.k71') }}</span>
        <div :style="iconRowStyle">
          <span :style="metaLabelStyle">{{ $t('shell.k72') }}</span>
          <div :style="iconPreviewRowStyle">
            <span :style="iconPreviewStyle">{{ document?.icon || '📄' }}</span>
            <ApInput
              v-model="localIcon"
              size="small"
              :placeholder="$t('shell.k78')"
              :maxlength="4"
              @change="handleIconChange"
            />
          </div>
        </div>
        <div :style="iconRowStyle">
          <span :style="metaLabelStyle">{{ $t('shell.k31') }}</span>
          <ApInput
            v-model="localCoverImage"
            size="small"
            :placeholder="$t('shell.k79')"
            @change="handleCoverChange"
          />
        </div>
      </div>

      <div :style="dividerStyle" />

      <!-- 标签 -->
      <div :style="sectionStyle">
        <div :style="tagHeaderStyle">
          <span :style="sectionTitleStyle">{{ $t('shell.k73') }}</span>
          <!-- AI 自动标签走 admin-only 接口（/ai/auto-tag），非管理员隐藏入口避免 403；
               AI 未启用时同样隐藏 -->
          <template v-if="auth.isAdmin && aiStore.enabled">
            <ApButton v-if="!aiLoading" variant="ghost" size="mini" icon-only :tooltip="$t('shell.k82')" :aria-label="$t('shell.k82')" @click.stop="handleAITag">
              <ApIcon name="Zap" :size="16" />
            </ApButton>
            <ApButton v-else variant="ghost" size="mini" icon-only disabled :aria-label="$t('shell.k83')">
              <ApIcon name="Loader2" :size="16" :style="{ animation: 'spin var(--apple-spinner-duration) linear infinite' }" />
            </ApButton>
          </template>
        </div>
        <div :style="tagListStyle">
          <span v-for="(tag, idx) in tags" :key="idx" :style="tagBadgeStyle">
            {{ tag }}
            <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('shell.k84')" :aria-label="$t('shell.k84')" @click.stop="removeTag(idx)">
              <ApIcon name="X" :size="16" />
            </ApButton>
          </span>
          <ApInput
            v-model="newTagInput"
            size="small"
            :placeholder="$t('shell.k80')"
            :maxlength="20"
            @keydown.enter.prevent="addTag"
            @keydown.backspace="handleBackspace"
          />
        </div>
      </div>

      <div :style="dividerStyle" />

      <!-- 访问统计 -->
      <div :style="sectionStyle">
        <span :style="sectionTitleStyle">{{ $t('shell.k74') }}</span>
        <div v-if="viewStats" :style="statsGridStyle">
          <div :style="statItemStyle">
            <span :style="statNumberStyle">{{ viewStats.total_views }}</span>
            <span :style="statLabelStyle">{{ $t('shell.k75') }}</span>
          </div>
          <div :style="statItemStyle">
            <span :style="statNumberStyle">{{ viewStats.unique_visitors }}</span>
            <span :style="statLabelStyle">{{ $t('shell.k76') }}</span>
          </div>
          <div :style="statItemStyle">
            <span :style="statNumberStyle">{{ viewStats.today_views }}</span>
            <span :style="statLabelStyle">{{ $t('shell.k77') }}</span>
          </div>
          <div :style="statItemStyle">
            <span :style="statNumberStyle">{{ viewStats.week_views }}</span>
            <span :style="statLabelStyle">{{ $t('shell.k61') }}</span>
          </div>
        </div>
        <div v-else :style="statLoadingStyle">
          <ApSkeleton variant="text" :lines="2" />
        </div>
      </div>

      <!-- 反向链接 -->
      <template v-if="backlinks.length > 0">
        <div :style="dividerStyle" />
        <div :style="sectionStyle">
          <span :style="sectionTitleStyle">反向链接 ({{ backlinks.length }})</span>
          <div :style="linksListStyle">
            <div v-for="link in backlinks" :key="link.source_doc_id" :style="linkItemStyle" @click="$emit('navigateToDoc', link.source_doc_id)">
              <ApIcon name="FileText" :size="14" :style="{ color: cv('blue'), flexShrink: 0 }" />
              <span :style="linkNameStyle">{{ link.source_doc_name }}</span>
            </div>
          </div>
        </div>
      </template>

      <!-- 引用文档 -->
      <template v-if="forwardLinks.length > 0">
        <div :style="dividerStyle" />
        <div :style="sectionStyle">
          <span :style="sectionTitleStyle">引用文档 ({{ forwardLinks.length }})</span>
          <div :style="linksListStyle">
            <div v-for="link in forwardLinks" :key="link.target_doc_id" :style="linkItemStyle" @click="$emit('navigateToDoc', link.target_doc_id)">
              <ApIcon name="FileText" :size="14" :style="{ color: cv('blue'), flexShrink: 0 }" />
              <span :style="linkNameStyle">{{ link.target_doc_name }}</span>
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, watch, computed } from 'vue';
import { appleFixedColumns, cv, AppleRadius, AppleSpace, AppleType, AppleFont, AppleTransition, AppleFontWeight } from '@/utils/core/apple-tokens';
import { formatTime, formatTimeAgo } from '@/utils/format/format-time';
import { autoTag } from '@/api/modules/ai/ai-knowledge';
import { getDocumentStats, getBacklinks, getForwardLinks, type DocumentViewStats } from '@/api/modules/document/document';
import { getDocumentTags, setDocumentTags } from '@/api/modules/document/tags';
import { useUIStore } from '@/stores/app/ui';
import { useAuthStore } from '@/stores/auth/auth';
import { useAIStore } from '@/stores/ai/ai';
import type { Document, BacklinkResult, ForwardLinkResult } from '@/types/models';
import ApButton from '@/components/apple/ApButton.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';

const props = withDefaults(defineProps<{
  document: Document | null;
  content?: string;
  wordCount?: number;
  readingTime?: number;
}>(), {
  wordCount: 0,
  readingTime: 0,
});

const emit = defineEmits<{
  (e: 'update:icon', value: string): void;
  (e: 'update:coverImage', value: string): void;
  (e: 'navigateToDoc', id: number): void;
}>();
const { t } = useI18n();

const ui = useUIStore();
const auth = useAuthStore();
const aiStore = useAIStore();

const expanded = shallowRef(false);
const tags = ref<string[]>([]);
const newTagInput = shallowRef('');
const aiLoading = shallowRef(false);
const viewStats = shallowRef<DocumentViewStats | null>(null);
const localIcon = shallowRef(props.document?.icon || '');
const localCoverImage = shallowRef(props.document?.cover_image || '');
const backlinks = ref<BacklinkResult[]>([]);
const forwardLinks = ref<ForwardLinkResult[]>([]);

watch(() => props.document?.icon, (val) => { localIcon.value = val || ''; });
watch(() => props.document?.cover_image, (val) => { localCoverImage.value = val || ''; });

function handleIconChange() {
  emit('update:icon', localIcon.value);
}

function handleCoverChange() {
  emit('update:coverImage', localCoverImage.value);
}

async function loadViewStats(docId: number) {
  viewStats.value = null;
  try {
    viewStats.value = await getDocumentStats(docId);
  } catch { /* silent */ }
}

async function loadTags(docId: number) {
  try {
    const res = await getDocumentTags(docId);
    tags.value = res.tags || [];
  } catch {
    tags.value = [];
  }
}

async function persistTags() {
  if (!props.document?.id) return;
  try {
    await setDocumentTags(props.document.id, tags.value.join(','));
  } catch {
    ui.showToast(t('shell.k85'), 'error');
  }
}

async function loadLinks() {
  if (!props.document?.id) {
    backlinks.value = [];
    forwardLinks.value = [];
    return;
  }
  try {
    const results = await Promise.allSettled([
      getBacklinks(props.document.id),
      getForwardLinks(props.document.id),
    ]);
    backlinks.value = results[0].status === 'fulfilled' ? (results[0].value || []) : [];
    forwardLinks.value = results[1].status === 'fulfilled' ? (results[1].value || []) : [];
  } catch {
    backlinks.value = [];
    forwardLinks.value = [];
  }
}

watch(() => props.document?.id, (newId) => {
  if (newId) {
    loadViewStats(newId);
    loadLinks();
    loadTags(newId);
  } else {
    viewStats.value = null;
    backlinks.value = [];
    forwardLinks.value = [];
    tags.value = [];
  }
}, { immediate: true });

function addTag() {
  const tag = newTagInput.value.trim();
  if (!tag || tags.value.includes(tag)) return;
  tags.value.push(tag);
  newTagInput.value = '';
  persistTags();
}

function removeTag(idx: number) {
  tags.value.splice(idx, 1);
  persistTags();
}

function handleBackspace(_e: KeyboardEvent) {
  if (!newTagInput.value && tags.value.length > 0) {
    tags.value.pop();
  }
}

async function handleAITag() {
  const text = props.content || props.document?.name || '';
  if (!text) return;
  aiLoading.value = true;
  try {
    const res = await autoTag({ content: text });
    if (res.tags?.tags) {
      const newTags = res.tags.tags.filter((t: string) => !tags.value.includes(t));
      tags.value.push(...newTags);
      ui.showToast(t('sentUi.k70', { p1: newTags.length }));
      persistTags();
    }
  } catch {
    ui.showToast(t('shell.k86'), 'error');
  } finally {
    aiLoading.value = false;
  }
}

// ===== Styles =====
const panelStyle = {
  marginTop: AppleSpace.xxs,
  padding: `0 ${AppleSpace.sm}`,
};

const headerStyle = {
  display: 'flex', alignItems: 'center', justifyContent: 'space-between',
  padding: `${AppleSpace.xs} 0`, cursor: 'pointer', userSelect: 'none' as const,
};

const headerLeftStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.xs,
};

const titleStyle = {
  fontSize: AppleType.controlLabel.size, fontWeight: AppleFontWeight.semibold,
  color: cv('text-primary'), fontFamily: AppleFont.text,
};

const chevronStyle = computed(() => ({
  color: cv('text-tertiary'),
  transition: AppleTransition.fast,
  transform: expanded.value ? 'rotate(90deg)' : 'rotate(0deg)',
}));

const bodyStyle = { paddingBottom: AppleSpace.xs };

const metaGridStyle = { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: AppleSpace['6px'] };
const metaItemStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace['1px'] };
const metaLabelStyle = { fontSize: AppleType.microUI.size, color: cv('text-quaternary'), fontFamily: AppleFont.text };
const metaValueStyle = {
  fontSize: AppleType.microUI.size, color: cv('text-primary'), fontFamily: AppleFont.text,
  overflow: 'hidden' as const, textOverflow: 'ellipsis' as const, whiteSpace: 'nowrap' as const,
};

const linkStyle = {
  fontSize: AppleType.microUI.size, color: cv('blue'), textDecoration: 'none',
  fontFamily: AppleFont.text, cursor: 'pointer',
};

const dividerStyle = { height: '1px', background: cv('border-light'), margin: `${AppleSpace.xs} 0` };

const sectionStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xs };
const sectionTitleStyle = { fontSize: AppleType.microUI.size, color: cv('text-quaternary'), fontFamily: AppleFont.text };

const tagHeaderStyle = { display: 'flex', alignItems: 'center', justifyContent: 'space-between' };
const tagListStyle = { display: 'flex', flexWrap: 'wrap' as const, gap: AppleSpace.xs, alignItems: 'center' };
const tagBadgeStyle = {
  display: 'inline-flex', alignItems: 'center', gap: AppleSpace.xxs,
  padding: `${AppleSpace.xxs} ${AppleSpace.xs}`, fontSize: AppleType.microUI.size,
  color: cv('blue'), background: cv('bg-tertiary'), borderRadius: AppleRadius.circle, fontFamily: AppleFont.text,
};


const iconRowStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xs };
const iconPreviewRowStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.xs };
const iconPreviewStyle = {
  fontSize: AppleType.statValue.size, letterSpacing: AppleType.statValue.letterSpacing, lineHeight: '1', width: '28px', height: '28px',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  background: cv('bg-secondary'), borderRadius: AppleRadius.small, flexShrink: 0,
};


const statsGridStyle = { display: 'grid', gridTemplateColumns: appleFixedColumns(2), gap: AppleSpace.xs };
const statItemStyle = {
  display: 'flex', flexDirection: 'column' as const, alignItems: 'center' as const,
  padding: `${AppleSpace.xs} 0`, background: cv('bg-secondary'), borderRadius: AppleRadius.small,
};
const statNumberStyle = {
  fontSize: AppleType.bodyEmphasis.size, fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'), fontFamily: AppleFont.text,
};
const statLabelStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-quaternary'), fontFamily: AppleFont.text, marginTop: AppleSpace.xxs };
const statLoadingStyle = { fontSize: AppleType.microUI.size, color: cv('text-quaternary'), fontFamily: AppleFont.text, padding: `${AppleSpace.xs} 0` };

const linksListStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xs };
const linkItemStyle = {
  display: 'flex', alignItems: 'center' as const, gap: AppleSpace.sm,
  padding: `${AppleSpace.xs} 0`, cursor: 'pointer', color: cv('blue'),
  fontFamily: AppleFont.text, fontSize: AppleType.microUI.size, transition: AppleTransition.default,
};
const linkNameStyle = { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const };
</script>
