<template>
  <div :style="docContentStyle">
    <DocBreadcrumb
      v-if="(hasDocSelected || crumbs.length > 0) && !(isMobile && hasDocSelected)"
      :space-name="spaceName"
      :crumbs="crumbs"
      :has-back-button="hasBackButton"
      :has-doc-selected="hasDocSelected"
      @back="$emit('back')"
      @navigate-to-space-root="$emit('navigateToSpaceRoot')"
      @navigate-to-doc="(id: number) => $emit('navigateToDoc', id)"
    />

    <template v-if="loading">
      <div :style="loadingStyle">
        <ApSkeleton variant="card" :lines="3" />
      </div>
    </template>

    <template v-else-if="localDocument">
      <!-- Cover image -->
      <div v-if="localDocument.cover_image && !editing" :style="coverImageContainerStyle">
        <img :src="localDocument.cover_image" :style="coverImageStyle" :alt="$t('shell.k31')" decoding="async" />
      </div>
      <div :style="headerContainerStyle">
        <div :style="headerTopStyle">
          <h1 :style="docTitleStyle" data-ap-object-title>
            <span v-if="localDocument.icon" :style="docIconStyle">{{ localDocument.icon }}</span>
            {{ localDocument.name }}
          </h1>
          <div :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.xs }">
            <slot name="toolbar" />
            <!-- P2-15：阅读宽度切换原先挂在 meta 行末尾，读起来像个孤立的铅笔图标；归到标题右侧的操作组 -->
            <ApButton variant="ghost" size="mini" icon-only :tooltip="wideMode ? $t('uiCopy.toggleReadWidth') : $t('uiCopy.toggleFullWidth')" :aria-label="wideMode ? $t('uiCopy.toggleReadWidth') : $t('uiCopy.toggleFullWidth')" @click="wideMode = !wideMode">
              <ApMorphIcon :icon="wideMode ? Minimize2 : Maximize2" :size="16" />
            </ApButton>
            <ApButton variant="ghost" size="mini" icon-only :tooltip="showRightPanel ? $t('uiCopy.closePanel') : $t('uiCopy.openPanel')" :aria-label="showRightPanel ? $t('uiCopy.closePanel') : $t('uiCopy.openPanel')" @click="showRightPanel = !showRightPanel">
              <ApMorphIcon :icon="showRightPanel ? ChevronLeft : ChevronRight" :size="16" />
            </ApButton>
          </div>
        </div>
        <div v-if="!editing" :style="docMetaStyle">
          <span :style="metaLabelStyle">{{ $t('shell.k21') }}</span>
          <router-link
            v-if="creatorUserId"
            :to="`/user/${creatorUserId}`"
            :style="metaLinkStyle"
          >{{ localDocument.creator_name || localDocument.create_user_id }}</router-link>
          <span :style="metaSepStyle">·</span>
          <span :style="metaLabelStyle">{{ $t('shell.k22') }}</span>
          <span :style="metaTextStyle">{{ formatTime(localDocument.create_time) }}</span>
          <span :style="metaSepStyle">·</span>
          <span :style="metaLabelStyle">{{ $t('shell.k23') }}</span>
          <router-link
            v-if="editorUserId"
            :to="`/user/${editorUserId}`"
            :style="metaLinkStyle"
          >{{ localDocument.editor_name || localDocument.edit_user_id }}</router-link>
          <span :style="metaSepStyle">·</span>
          <span :style="metaLabelStyle">{{ $t('shell.k24') }}</span>
          <span :style="metaTextStyle">{{ formatTime(localDocument.update_time) }}</span>
          <span :style="metaSepStyle">·</span>
          <span :style="metaTextStyle">{{ wordCount }} {{ $t('cExtraUi.k7') }}</span>
          <span :style="metaSepStyle">·</span>
          <span :style="metaTextStyle">{{ $t('shell.readingTime', { m: readingTime }) }}</span>
        </div>
      </div>

      <!-- 编辑模式 -->
      <template v-if="editing">
        <div :style="editSplitStyle" @paste.capture="handleInlinePaste">
          <div :style="editPaneStyle">
            <MonacoEditor
              v-model="localEditContent"
              language="markdown"
              theme="vs"
              :space-id="localDocument.space_id"
              @image-paste="handleInlineImagePaste"
            />
          </div>
          <div :style="editDividerStyle" />
          <div :style="previewPaneStyle">
            <div :style="previewLabelStyle">
              <ApIcon name="Eye" :size="12" />
              {{ $t('shell.k25') }}
            </div>
            <div :style="previewScrollStyle">
              <ApMarkdownPreview :content="localEditContent" />
            </div>
          </div>
        </div>
        <div :style="editBarStyle">
    <span :style="editHintStyle">{{ $t('shell.k26') }}</span>
  </div>
      </template>

      <!-- 查看模式 -->
      <template v-else>
        <div :style="contentRowStyle">
          <div ref="mainRef" :style="docMainStyle" @scroll="onScroll" @mouseup="onMouseUp">
            <div :style="readingContainerStyle">
              <ApMarkdownPreview
                v-if="localDocument.type === 1 || localDocument.type === 2"
                ref="previewRef"
                :content="highlightedContent || ''"
              />

              <div v-if="!currentContent && localDocument.type === 2" :style="dirPlaceholderStyle">
                <ApIcon name="Folder" :size="AppleIconSize.empty" :style="{ color: cv('text-quaternary'), marginBottom: AppleSpace.md }" />
                <p>{{ $t('shell.k27') }}</p>
              </div>

              <!-- 空文档（审查 B20）：正文为空时整页只剩一条空白横带，像渲染失败 ——
                   0 字属实，但界面必须把"这是空的"说出来 -->
              <ApEmpty
                v-else-if="!currentContent && localDocument.type === 1"
                icon="FileText"
                :title="$t('shell.k89')"
                :description="$t('shell.k90')"
              />
            </div>
            <!-- 划词快捷菜单 Teleport 到 body：fixed 定位在文档阅读区 DOM 里会被祖先 overflow/transform 裁掉或偏移 -->
            <Teleport to="body">
            <div
              v-if="showSelectionMenu"
              ref="selectionMenuRef"
              class="apple-glass-layer"
              :style="selectionMenuStyle"
            >
              <ApButton variant="secondary" size="small" @click="handleCopySelection">{{ $t('shell.copy') }}</ApButton>
              <div :style="menuDividerStyle" />
              <ApButton variant="secondary" size="small" @click="handleAnnotate">
                <ApIcon name="MessageSquare" :size="AppleIconSize.control" />
                {{ $t('shell.k29') }}
              </ApButton>
            </div>
            </Teleport>
          </div>

          <div v-if="showRightPanel" :style="rightPanelStyle">
            <div :style="rightPanelInnerStyle">
              <DocProperties
                :document="localDocument"
                :content="currentContent"
                :word-count="wordCount"
                :reading-time="readingTime"
                @update:icon="handleIconUpdate"
                @update:cover-image="handleCoverUpdate"
                @navigate-to-doc="(id) => $emit('navigateToDoc', id)"
              />
              <DocCommentPanel ref="docCommentPanelRef" :document-id="localDocument?.id ?? null" :can-edit="props.canEdit" />
            </div>
          </div>
        </div>
      </template>
    </template>

    <template v-else>
      <div ref="mainRef" :style="docMainStyle" @scroll="onScroll">
        <slot name="spaceRoot" />
      </div>
    </template>

    <ApFloatingActionButton v-if="showBackTop && !editing" variant="ghost" :label="$t('shell.k30')" bottom="56px" right="20px" @click="scrollToTop">
      <ApIcon name="ArrowUp" :size="AppleIconSize.control" />
    </ApFloatingActionButton>

    <!-- 阅读进度条 -->
    <ReadingProgressBar v-if="!editing && localDocument" :progress="readingProgress" />

    <!-- 桌面端悬浮目录 -->
    <FloatingToc
      v-if="!isMobile && !editing"
      :content-ref="() => mainRef"
      :preview-ref="() => previewRef"
      :current-content="() => currentContent || ''"
    />

    <!-- 移动端浮动TOC按钮 -->
    <MobileTocFab
      v-if="isMobile && !editing"
      :content-ref="() => mainRef"
      :preview-ref="() => previewRef"
      :current-content="() => currentContent || ''"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, watch, onUnmounted, defineAsyncComponent } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import { ChevronLeft, ChevronRight, Maximize2, Minimize2 } from '@/components/apple/morph-icon-data';
import ApMorphIcon from '@/components/apple/ApMorphIcon.vue';
import { AppleFont, AppleIconSize, AppleLineHeight, AppleRadius, AppleSpace, AppleType, cv, menuSurfaceStyle, primarySurfaceStyle } from '@/utils/core/apple-tokens';
import { loadingStyle } from '@/styles/layout/page-styles';
import { readingAreaStyle } from '@/styles/component/doc-styles';
import { logger } from '@/utils/logger';
import { copyText } from '@/utils/clipboard';
import { formatTime } from '@/utils/format/format-time';
import { countWords, estimateReadingTime } from '@/utils/format/reading-time';
import ApMarkdownPreview from '@/components/apple/ApMarkdownPreview.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApFloatingActionButton from '@/components/apple/ApFloatingActionButton.vue';
import DocBreadcrumb from '@/components/space/reader/DocBreadcrumb.vue';
import FloatingToc from '@/components/space/reader/FloatingToc.vue';
import MobileTocFab from '@/components/space/reader/MobileTocFab.vue';
import ReadingProgressBar from '@/components/space/reader/ReadingProgressBar.vue';
import DocCommentPanel from '@/components/space/sidebar/DocCommentPanel.vue';
import DocProperties from '@/components/space/sidebar/DocProperties.vue';

import { useUIStore } from '@/stores/app/ui';
import { updateDocument } from '@/api/modules/document/document';
import { useEditorUpload } from '@/composables/editor/useEditorUpload';
import { useClickOutsideClose } from '@/composables/ui/useClickOutsideClose';
import { useEscapeClose } from '@/composables/ui/useEscapeClose';
import { storageGetBoolean, storageSetBoolean, StorageKeys } from '@/utils/core/storage';
import type { Document } from '@/types/models';

const props = withDefaults(defineProps<{
  currentDocument: Document | null;
  currentContent: string;
  spaceName: string;
  crumbs: { id: number; name: string }[];
  hasBackButton: boolean;
  hasDocSelected: boolean;
  isMobile: boolean;
  loading?: boolean;
  editing?: boolean;
  saving?: boolean;
  editContent?: string;
  highlightKeyword?: string;
  canEdit?: boolean;
}>(), {
  loading: false,
  editing: false,
  saving: false,
  editContent: '',
  highlightKeyword: '',
  canEdit: true,
});

const emit = defineEmits<{
  (e: 'back'): void;
  (e: 'navigateToSpaceRoot'): void;
  (e: 'navigateToDoc', id: number): void;
  (e: 'save', content: string): void;
  (e: 'cancel'): void;
  (e: 'update:editContent', value: string): void;
}>();
// Monaco 体积约 4MB，仅在进入编辑态时加载，阅读页首屏不拉
const MonacoEditor = defineAsyncComponent(() => import('@/components/document/MonacoEditor.vue'));
const { t } = useI18n();

const uiStore = useUIStore();

const previewRef = ref<InstanceType<typeof ApMarkdownPreview> | null>(null);
const mainRef = ref<HTMLElement | null>(null);
const docCommentPanelRef = ref<InstanceType<typeof DocCommentPanel> | null>(null);
const showBackTop = shallowRef(false);
const readingProgress = shallowRef(0);
const localEditContent = shallowRef(props.editContent || props.currentContent);
const localDocument = shallowRef<Document | null>(props.currentDocument);
const showSelectionMenu = shallowRef(false);

// Image paste support for inline editing
const { handleImagePaste: handleInlineImagePaste, handleParentPaste: handleInlinePaste } = useEditorUpload(
  props.currentDocument?.id || 0,
  (text: string) => { localEditContent.value = (localEditContent.value || '') + text; },
);

// 高亮关键词处理
import { highlightMarkdownContent } from '@/utils/domain/search-highlight';

const highlightedContent = computed(() => {
  if (!props.highlightKeyword || !props.currentContent) {
    return props.currentContent;
  }
  return highlightMarkdownContent(props.currentContent, [props.highlightKeyword]);
});

// 滚动到高亮位置
watch(() => props.highlightKeyword, (newKeyword) => {
  if (newKeyword && mainRef.value) {
    setTimeout(() => {
      const firstMark = mainRef.value?.querySelector('mark.highlight');
      if (firstMark) {
        firstMark.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    }, 100);
  }
}, { immediate: true });

// 滚动优化：使用 requestAnimationFrame
let scrollRAF: number | null = null;

watch(() => props.editing, (editing) => {
  if (editing) {
    localEditContent.value = props.editContent || props.currentContent;
  }
});

// 监听文档切换或数据更新，重置状态
// 使用 { immediate: true } 确保初始值正确同步，避免只监听 id 导致异步加载的完整数据无法更新
watch(() => props.currentDocument, (newDoc) => {
  localDocument.value = newDoc;
  localEditContent.value = props.editContent || props.currentContent;
  showSelectionMenu.value = false;
  resetScroll();
}, { immediate: true });

function onScroll() {
  if (scrollRAF !== null) {
    cancelAnimationFrame(scrollRAF);
  }
  scrollRAF = requestAnimationFrame(() => {
    if (mainRef.value) {
      showBackTop.value = mainRef.value.scrollTop > 300;
      const { scrollTop, scrollHeight, clientHeight } = mainRef.value;
      const scrollableHeight = scrollHeight - clientHeight;
      readingProgress.value = scrollableHeight > 0 ? (scrollTop / scrollableHeight) * 100 : 0;
    }
    showSelectionMenu.value = false;
    scrollRAF = null;
  });
}

function scrollToTop() {
  if (mainRef.value) {
    mainRef.value.scrollTo({ top: 0, behavior: 'smooth' });
  }
}

// 划词快捷菜单

const selectionMenuRef = ref<HTMLElement | null>(null);
const selectionMenuPos = shallowRef({ top: 0, left: 0 });

// 点别处收起走全站唯一实现（原先这里自己挂了一份常驻的 mousedown capture 监听；
// 共用版只在开着的时候挂，全站几十个浮层不再各自留一个空转监听）
useClickOutsideClose(
  () => showSelectionMenu.value,
  () => { showSelectionMenu.value = false; },
  () => [selectionMenuRef.value],
);
// 选区工具条也要能 Esc 收起（此前只有点别处，键盘用户出不来）
useEscapeClose(() => showSelectionMenu.value, () => { showSelectionMenu.value = false; });

onUnmounted(() => {
  if (scrollRAF !== null) {
    cancelAnimationFrame(scrollRAF);
  }
});

function onMouseUp(e: MouseEvent) {
  if (selectionMenuRef.value?.contains(e.target as Node)) {
    return;
  }
  const selection = window.getSelection();
  const text = selection?.toString().trim() || '';
  if (text && text.length > 0) {
    const range = selection?.getRangeAt(0);
    if (range) {
      const rect = range.getBoundingClientRect();
      selectionMenuPos.value = {
        top: rect.top - 8,
        left: rect.left + rect.width / 2,
      };
    }
    showSelectionMenu.value = true;
  } else {
    showSelectionMenu.value = false;
  }
}

function handleCopySelection() {
  const selection = window.getSelection();
  const text = selection?.toString() || '';
  if (text) {
    copyText(text);
  }
  showSelectionMenu.value = false;
  selection?.removeAllRanges();
}

function handleAnnotate() {
  const selection = window.getSelection();
  const text = selection?.toString().trim() || '';
  if (!text) return;
  showSelectionMenu.value = false;
  selection?.removeAllRanges();
  // Copy selected text to clipboard and open comment panel
  copyText(text);
  showRightPanel.value = true;
  uiStore.showToast(t('shell.k32'), 'info');
}

async function handleIconUpdate(icon: string) {
  if (!localDocument.value) return;
  try {
    await updateDocument(localDocument.value.id, { icon });
    if (localDocument.value) localDocument.value = { ...localDocument.value, icon };
    uiStore.showToast(t('shell.k33'), 'success');
  } catch {
    uiStore.showToast(t('shell.k34'), 'error');
  }
}

async function handleCoverUpdate(coverImage: string) {
  if (!localDocument.value) return;
  try {
    await updateDocument(localDocument.value.id, { cover_image: coverImage });
    if (localDocument.value) localDocument.value = { ...localDocument.value, cover_image: coverImage };
    uiStore.showToast(t('shell.k35'), 'success');
  } catch {
    uiStore.showToast(t('shell.k36'), 'error');
  }
}

function handleSave() {
  emit('save', localEditContent.value);
}

function resetScroll() {
  if (mainRef.value) {
    mainRef.value.scrollTop = 0;
  }
}

const wordCount = computed(() => countWords(props.currentContent));
const readingTime = computed(() => estimateReadingTime(props.currentContent));

const creatorUserId = computed(() =>
  props.currentDocument?.created_by || props.currentDocument?.create_user_id
);

const editorUserId = computed(() =>
  props.currentDocument?.editor_id || props.currentDocument?.edit_user_id
);

defineExpose({ previewRef, resetScroll, wordCount, readingTime, handleSave });

const metaSepStyle = { fontFamily: AppleFont.text, color: cv('text-quaternary'), fontSize: AppleType.controlLabel.size, lineHeight: AppleLineHeight.default };

const metaLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-quaternary'),
  lineHeight: AppleLineHeight.default,
};

const metaLinkStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('blue'),
  textDecoration: 'none',
  cursor: 'pointer',
  // 作者名是原子单位：meta 行本身 flexWrap 允许条目间换行，但单个名字不该被劈成两行
  whiteSpace: 'nowrap' as const,
  lineHeight: AppleLineHeight.default,
};

const metaTextStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-quaternary'),
  lineHeight: AppleLineHeight.default,
};

const docContentStyle = {
  flex: '1',
  minHeight: '0',
  width: '100%',
  position: 'relative' as const,
  display: 'flex',
  flexDirection: 'column' as const,
};

const docTitleStyle = {
  fontFamily: AppleType.sectionTitle.font,
  fontSize: AppleType.linkActionHeading.size,
  fontWeight: String(AppleType.linkActionHeading.weight),
  lineHeight: AppleLineHeight.snug,
  color: cv('text-primary'),
  margin: '0',
  letterSpacing: AppleType.linkActionHeading.letterSpacing,
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  // P1-6：原先没有 minWidth/flex，窄视口下 h1 被操作行挤到 21px，中文逐字竖排。
  // 允许它占满剩余宽度并正常换行，操作行由 headerTopStyle 的 flexWrap 换到下一行。
  flex: '1 1 auto',
  minWidth: '0',
  flexWrap: 'wrap' as const,
  overflowWrap: 'break-word' as const,
};

const docIconStyle = {
  fontSize: '1.2em',
  lineHeight: '1',
  flexShrink: 0,
};

const coverImageContainerStyle = {
  width: '100%',
  marginBottom: AppleSpace.sm,
  borderRadius: AppleRadius.card,
  overflow: 'hidden',
  maxHeight: '200px',
};

const coverImageStyle = {
  width: '100%',
  height: '200px',
  objectFit: 'cover' as const,
  display: 'block',
};

const docMetaStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
  alignItems: 'center',
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-quaternary'),
  marginTop: '0',
  flexWrap: 'wrap' as const,
  lineHeight: AppleLineHeight.default,
};

const headerTopStyle = {
  display: 'flex',
  alignItems: 'flex-start',
  justifyContent: 'space-between',
  gap: AppleSpace.sm,
  // P1-6：390 宽时标题与操作行挤不下，允许操作行掉到下一行，而不是把标题压成竖排
  flexWrap: 'wrap' as const,
};

const dirPlaceholderStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  alignItems: 'center',
  justifyContent: 'center',
  padding: `${AppleSpace['3xl']} 0`,
  color: cv('text-quaternary'),
  fontFamily: AppleFont.text,
  fontSize: AppleType.bodyPrimary.size,
};

// 阅读宽度：默认走 680 行度量（长文可读性），全宽是用户显式选择且必须被记住。
// 之前这里有个 if (!wideMode.value) 强制改回 true 的兜底，等于把选了窄栏的用户每次重置。
const wideMode = shallowRef(storageGetBoolean(StorageKeys.READER_WIDE_MODE, false));

// 标题行必须和正文落在同一列：正文是「680 行度量 + 左右 xl 内边距」的一张纸，
// 标题若只按 680 加 md 内边距居中，文字列会比正文左移 20px（实测 h1 539 / 正文 519）。
// 这里让标题容器与纸同宽同内边距，标题文字才和正文第一列严格对齐。
const readingMaxWidth = computed(() => (wideMode.value ? '100%' : `calc(${readingAreaStyle.maxWidth} + ${AppleSpace.xl} * 2)`));
const headerContainerStyle = computed(() => ({
  padding: wideMode.value ? `0 ${AppleSpace.lg}` : `0 ${AppleSpace.xl}`,
  maxWidth: readingMaxWidth.value,
  margin: '0 auto',
  width: '100%',
  flexShrink: '0',
  overflow: 'hidden' as const,
}));

const docMainStyle = { flex: '1 1 0%', minWidth: '0', minHeight: '0', overflowY: 'auto' as const, overflowX: 'hidden' as const, padding: `${AppleSpace.xs} ${AppleSpace.md}` };
const showRightPanel = shallowRef(false);

watch(wideMode, (val) => {
  storageSetBoolean(StorageKeys.READER_WIDE_MODE, val);
});

const contentRowStyle = {
  display: 'flex',
  flexDirection: 'row' as const,
  flex: '1 1 0%',
  minHeight: '0',
  overflow: 'hidden',
};

const rightPanelStyle = {
  width: '260px',
  flexShrink: 0,
  borderLeft: `1px solid ${cv('border-light')}`,
  background: cv('surface-panel'),
  overflow: 'hidden',
  display: 'flex',
  flexDirection: 'column' as const,
};

const rightPanelInnerStyle = {
  width: '260px',
  display: 'flex',
  flexDirection: 'column' as const,
  overflow: 'hidden',
};

// 窄栏走全站唯一的阅读度量（680px，styles/component/doc-styles.ts），
// 不再自带一个 800px 的私有常量——ShareView / SSODocView 用的是同一个值。
// 这一张纸是**本页的主对象**：底色与 elevation 成对取自 primarySurfaceStyle（§12 越大的面越厚），
// 内边距从度量里额外让出来，保证 680px 说的是文字栏而不是纸的宽度。
const readingContainerStyle = computed(() => primarySurfaceStyle({
  ...readingAreaStyle,
  maxWidth: wideMode.value ? '100%' : `calc(${readingAreaStyle.maxWidth} + ${AppleSpace.xl} * 2)`,
  padding: wideMode.value ? `0 ${AppleSpace.lg}` : AppleSpace.xl,
  margin: `${AppleSpace.lg} auto`,
  width: '100%',
}));

const editSplitStyle = {
  display: 'flex',
  flex: '1',
  minHeight: '0',
  overflow: 'hidden',
  borderTop: `1px solid ${cv('border')}`,
  marginTop: AppleSpace.sm,
};

const editPaneStyle = {
  flex: '1',
  minWidth: '0',
  overflow: 'hidden',
};

const editDividerStyle = {
  width: '1px',
  background: cv('border-light'),
  flexShrink: 0,
};

const previewPaneStyle = {
  flex: '1',
  minWidth: '0',
  display: 'flex',
  flexDirection: 'column' as const,
  background: cv('surface-panel'),
};

const previewLabelStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  fontSize: AppleType.microUI.size,
  fontFamily: AppleFont.text,
  color: cv('text-tertiary'),
  borderBottom: `1px solid ${cv('border-light')}`,
  flexShrink: 0,
};

const previewScrollStyle = {
  flex: '1',
  overflowY: 'auto' as const,
  overflowX: 'hidden' as const,
};

const editBarStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  borderTop: `1px solid ${cv('border')}`,
  background: cv('bg-secondary'),
  flexShrink: 0,
};

const editHintStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

const selectionMenuStyle = computed(() => ({
  position: 'fixed' as const,
  top: `${selectionMenuPos.value.top}px`,
  left: `${selectionMenuPos.value.left}px`,
  transform: 'translate(-50%, -100%)',
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xxs,
  ...menuSurfaceStyle(),
  padding: AppleSpace.xs,
  pointerEvents: 'auto' as const,
}));

const menuDividerStyle = {
  width: '1px',
  height: '16px',
  background: cv('border'),
};
</script>

<style scoped>
[style*="cursor: pointer"]:hover {
  background: var(--apple-bg-tertiary);
  color: var(--apple-text-primary);
}
</style>
