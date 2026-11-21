<template>
  <div :style="toolbarStyle">
    <div :style="leftStyle">
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k38')" :aria-label="$t('docUi1.k38')" @click="$emit('insertMarkdown', 'undo')">
        <ApIcon name="Undo2" :size="16" />
      </ApButton>
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k39')" :aria-label="$t('docUi1.k39')" @click="$emit('insertMarkdown', 'redo')">
        <ApIcon name="Redo2" :size="16" />
      </ApButton>
      <div :style="miniDividerStyle" />
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k40')" :aria-label="$t('docUi1.k40')" @click="$emit('insertMarkdown', 'bold')">
        <ApIcon name="Bold" :size="16" />
      </ApButton>
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k41')" :aria-label="$t('docUi1.k41')" @click="$emit('insertMarkdown', 'italic')">
        <ApIcon name="Italic" :size="16" />
      </ApButton>
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k42')" :aria-label="$t('docUi1.k42')" @click="$emit('insertMarkdown', 'heading')">
        <ApIcon name="Heading" :size="16" />
      </ApButton>
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k43')" :aria-label="$t('docUi1.k43')" @click="$emit('insertMarkdown', 'link')">
        <ApIcon name="Link" :size="16" />
      </ApButton>
      <ApPopover placement="bottom" width="160px" :label="$t('docUi1.k35')">
        <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k44')" :aria-label="$t('docUi1.k44')">
          <ApIcon name="Code" :size="16" />
        </ApButton>
        <template #content>
          <div :style="langListStyle">
            <ApButton v-for="lang in popularLangs" :key="lang" variant="ghost" size="small" @click.stop="handleLangClick(lang)">{{ lang }}</ApButton>
          </div>
        </template>
      </ApPopover>
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k45')" :aria-label="$t('docUi1.k45')" @click="$emit('insertMarkdown', 'list')">
        <ApIcon name="List" :size="16" />
      </ApButton>
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k46')" :aria-label="$t('docUi1.k46')" @click="$emit('insertMarkdown', 'quote')">
        <ApIcon name="Quote" :size="16" />
      </ApButton>
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k47')" :aria-label="$t('docUi1.k47')" @click="$emit('insertMarkdown', 'table')">
        <ApIcon name="Table" :size="16" />
      </ApButton>
      <div :style="miniDividerStyle" />
      <ApPopover placement="bottom" width="180px" :label="$t('docUi1.k36')">
        <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k36')" :aria-label="$t('docUi1.k36')">
          <ApIcon name="Plus" :size="16" />
        </ApButton>
        <template #content>
          <div :style="menuStyle">
            <ApButton variant="ghost" size="small" @click.stop="$emit('insertMarkdown', 'callout')">
              <ApIcon name="MessageSquare" :size="16" />
              <span>{{ $t('docUi1.k24') }}</span>
            </ApButton>
            <ApButton variant="ghost" size="small" @click.stop="$emit('insertMarkdown', 'task')">
              <ApIcon name="CheckSquare" :size="16" />
              <span>{{ $t('docUi1.k25') }}</span>
            </ApButton>
            <ApButton variant="ghost" size="small" @click.stop="$emit('insertMarkdown', 'mermaid')">
              <ApIcon name="GitBranch" :size="16" />
              <span>{{ $t('docUi1.k26') }}</span>
            </ApButton>
            <ApButton variant="ghost" size="small" @click.stop="$emit('insertMarkdown', 'embed')">
              <ApIcon name="Link" :size="16" />
              <span>{{ $t('docUi1.k27') }}</span>
            </ApButton>
            <ApButton variant="ghost" size="small" @click.stop="$emit('insertMarkdown', 'math')">
              <ApIcon name="Sigma" :size="16" />
              <span>{{ $t('docUi1.k28') }}</span>
            </ApButton>
          </div>
        </template>
      </ApPopover>
      <div :style="miniDividerStyle" />
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k48')" :aria-label="$t('docUi1.k48')" @click="$emit('triggerImageUpload')">
        <ApIcon name="Image" :size="16" />
      </ApButton>
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k8')" :aria-label="$t('docUi1.k8')" @click="$emit('triggerAttachmentUpload')">
        <ApIcon name="Paperclip" :size="16" />
      </ApButton>
      <div :style="miniDividerStyle" />
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k5')" :aria-label="$t('docUi1.k5')" @click="$emit('showVersions')">
        <ApIcon name="History" :size="16" />
      </ApButton>
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k49')" :aria-label="$t('docUi1.k49')" @click="$emit('toggleViewMode')">
        <ApIcon :name="viewMode === 'preview' ? 'EyeOff' : 'Eye'" :size="16" />
      </ApButton>
      <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('docUi1.k50')" :aria-label="$t('docUi1.k50')" @click="$emit('showShortcuts')">
        <ApIcon name="Keyboard" :size="16" />
      </ApButton>
      <template v-if="aiStore.enabled">
      <ApPopover placement="bottom" width="140px" :label="$t('docUi1.k37')">
        <ApButton variant="ghost" size="mini" icon-only :aria-label="$t('docUi1.k37')">
          <ApIcon name="Zap" :size="16" :style="{ color: cv('blue') }" />
        </ApButton>
        <template #content>
          <div :style="menuStyle">
            <ApButton variant="ghost" size="small" @click.stop="$emit('aiWrite', 'open')">
              <ApIcon name="Zap" :size="16" />
              <span>{{ $t('docUi1.k29') }}</span>
            </ApButton>
            <div :style="dividerStyle" />
            <ApButton variant="ghost" size="small" @click.stop="$emit('aiWrite', 'rewrite')">
              <ApIcon name="RefreshCw" :size="16" />
              <span>{{ $t('docUi1.k30') }}</span>
            </ApButton>
            <ApButton variant="ghost" size="small" @click.stop="$emit('aiWrite', 'continue')">
              <ApIcon name="ArrowRight" :size="16" />
              <span>{{ $t('docUi1.k31') }}</span>
            </ApButton>
            <ApButton variant="ghost" size="small" @click.stop="$emit('aiWrite', 'translate')">
              <ApIcon name="Languages" :size="16" />
              <span>{{ $t('docUi1.k32') }}</span>
            </ApButton>
            <ApButton variant="ghost" size="small" @click.stop="$emit('aiWrite', 'summarize')">
              <ApIcon name="FileText" :size="16" />
              <span>{{ $t('docUi1.k33') }}</span>
            </ApButton>
          </div>
        </template>
      </ApPopover>
      </template>
    </div>

    <div :style="rightStyle">
      <span v-if="uploading" :style="uploadingStyle">
        <ApIcon name="Loader2" :size="12" :style="{ animation: 'spin var(--apple-spinner-duration) linear infinite' }" />
      </span>
      <div :style="miniDividerStyle" />
      <ApButton variant="ghost" size="small" :tooltip="$t('docUi1.k53')" :aria-label="$t('docUi1.k51')" @click="$emit('cancel')">
        <ApIcon name="X" :size="16" />
        <span>{{ $t('docUi1.cancel') }}</span>
      </ApButton>
      <ApButton variant="primary" size="small" :tooltip="$t('docUi1.k54')" :aria-label="$t('docUi1.k52')" @click="$emit('save')">
        <ApIcon name="Save" :size="16" />
        <span>{{ $t('docUi1.save') }}</span>
      </ApButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { AppleSpace, AppleZIndex, cv } from '@/utils/core/apple-tokens';
import { useAIStore } from '@/stores/ai/ai';
import ApButton from '@/components/apple/ApButton.vue';
import ApPopover from '@/components/apple/ApPopover.vue';

defineProps<{
  viewMode: 'edit' | 'preview' | 'split';
  viewModeLabel: string;
  uploading: boolean;
}>();

const emit = defineEmits<{
  (e: 'save'): void;
  (e: 'cancel'): void;
  (e: 'showVersions'): void;
  (e: 'showShortcuts'): void;
  (e: 'toggleViewMode'): void;
  (e: 'insertMarkdown', type: string): void;
  (e: 'triggerImageUpload'): void;
  (e: 'triggerAttachmentUpload'): void;
  (e: 'aiWrite', type: string): void;
}>();

const aiStore = useAIStore();


function handleLangClick(lang: string) {
  emit('insertMarkdown', 'code:' + lang);
}

const popularLangs = ['javascript', 'typescript', 'python', 'go', 'java', 'bash', 'sql', 'json', 'yaml', 'html', 'css', 'markdown'];

const toolbarStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `0 ${AppleSpace.sm}`,
  // minHeight 而不是 height：这条栏按 40px 写死过，触屏下按钮补到 44px 就被外层
  // 卡片（overflow:hidden）切掉 3px（390 宽实测「取消 / 保存」两处）。
  // 栏的高度应该由里面最高的控件决定，不是反过来把控件裁了。
  minHeight: '40px',
  borderBottom: `1px solid ${cv('border-light')}`,
  // 与宿主卡片（DocumentEditor 的 editorContainerStyle = surface-panel）同档：
  // 原先用 bg-tertiary（0.84 半透内嵌面），同页 chrome 里自成一档材质，
  // 玻璃主题下与身上的卡片都能看出色差 —— 卡片内部的条就该跟卡片同一个面。
  background: cv('surface-panel'),
  flexShrink: 0,
  position: 'sticky' as const,
  top: '0',
  zIndex: AppleZIndex.raised,
  gap: AppleSpace.xs,
};

const leftStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xxs,
  flex: '1 1 auto',
  minWidth: '0',
  overflowX: 'auto' as const,
  overflowY: 'hidden' as const,
  scrollbarWidth: 'none' as const,
};

const rightStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  flexShrink: 0,
};

const miniDividerStyle = {
  width: '1px',
  height: '14px',
  background: cv('border-light'),
  margin: `0 ${AppleSpace.xs}`,
};

const dividerStyle = {
  height: '1px',
  background: cv('border-light'),
  margin: `${AppleSpace.xs} 0`,
};


const uploadingStyle = {
  display: 'flex',
  alignItems: 'center',
  color: cv('blue'),
};

const menuStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  padding: `${AppleSpace.xs} 0`,
  gap: AppleSpace.xxs,
};

const langListStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  maxHeight: '240px',
  overflowY: 'auto' as const,
  padding: `${AppleSpace.xs} 0`,
  gap: AppleSpace.xxs,
};
</script>

<style scoped>
button[title*="保存"]:hover {
  background: var(--apple-blue);
  filter: brightness(1.1);
}
</style>
