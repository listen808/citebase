<template>
  <div :style="toolbarStyle">
    <div :style="mainStyle">
      <template v-if="editing">
        <ApButton variant="ghost" size="small" @click="$emit('cancel')">
          <ApIcon name="X" :size="16" />
          <span>{{ $t('spaceUi.cancel') }}</span>
        </ApButton>
        <ApButton variant="primary" size="small" :loading="saving" @click="$emit('save')">
          <ApIcon name="Save" :size="16" />
          <span>{{ $t('spaceUi.save') }}</span>
        </ApButton>
      </template>
      <template v-else>
        <ApButton v-if="isEditor" variant="primary" size="small" :aria-label="$t('spaceUi.edit')" @click="$emit('edit')">
          <ApIcon name="Pencil" :size="16" />
          <span v-if="!isMobile">{{ $t('spaceUi.edit') }}</span>
        </ApButton>
        <ApFavoriteButton
          :labeled="!isMobile"
          size="small"
          :icon-size="16"
          :active="collected"
          :noun="$t('lastAttrsUi.k10')"
          @toggle="$emit('collect')"
        />
        <ApButton variant="ghost" size="small" :aria-label="$t('spaceUi.export')" @click="showExportModal = true">
          <ApIcon name="Download" :size="16" />
          <span v-if="!isMobile">{{ $t('spaceUi.export') }}</span>
        </ApButton>
        <ApButton v-if="isEditor" variant="ghost" size="small" :aria-label="$t('spaceUi.import')" @click="$emit('import')">
          <ApIcon name="Upload" :size="16" />
          <span v-if="!isMobile">{{ $t('spaceUi.import') }}</span>
        </ApButton>

      <!-- 2026-10-26：固定宽（180/240px）在移动端字号下放不下「Attachment management」全文 + 图标
           （菜单项 white-space:nowrap），溢出把菜单撑出内部横向滚动、长文案被截断。
           max-content 随内容自适应；视口越界由 calcPosition 的双侧 clamp 兜住。 -->
      <ApPopover v-model:open="moreOpen" placement="bottom" width="max-content">
        <ApButton variant="ghost" size="small">
          <ApIcon name="MoreHorizontal" :size="16" />
          <span>{{ $t('spaceUi.more') }}</span>
        </ApButton>
        <template #content>
          <div :style="menuStyle">
            <ApButton variant="ghost" size="small" :style="{ width: '100%', justifyContent: 'flex-start' }" @click="closeAndEmit('share')">
              <ApIcon name="Share2" :size="16" />
              <span>{{ $t('spaceUi.share') }}</span>
            </ApButton>
            <ApButton variant="ghost" size="small" :style="{ width: '100%', justifyContent: 'flex-start' }" @click="closeAndEmit('history')">
              <ApIcon name="Clock" :size="16" />
              <span>{{ $t('spaceUi.k26') }}</span>
            </ApButton>
            <ApButton v-if="aiStore.enabled" variant="ghost" size="small" :style="{ width: '100%', justifyContent: 'flex-start' }" @click="closeAndEmit('aiSummary')">
              <ApIcon name="Zap" :size="16" />
              <span>{{ $t('spaceUi.k27') }}</span>
            </ApButton>
            <ApButton variant="ghost" size="small" :style="{ width: '100%', justifyContent: 'flex-start' }" @click="closeAndEmit('images')">
              <ApIcon name="Image" :size="16" />
              <span>{{ $t('spaceUi.k28') }}</span>
            </ApButton>
            <ApButton variant="ghost" size="small" :style="{ width: '100%', justifyContent: 'flex-start' }" @click="closeAndEmit('attachments')">
              <ApIcon name="Paperclip" :size="16" />
              <span>{{ $t('spaceUi.k29') }}</span>
            </ApButton>
            <div :style="dividerStyle" />
            <ApButton variant="ghost" size="small" :style="{ width: '100%', justifyContent: 'flex-start' }" @click="closeAndEmit('fullscreen')">
              <ApIcon :name="fullscreen ? 'Minimize2' : 'Maximize2'" :size="16" />
              <span>{{ fullscreen ? $t('spaceUi.k37') : $t('spaceUi.k38') }}</span>
            </ApButton>
            <ApButton variant="ghost" size="small" :style="{ width: '100%', justifyContent: 'flex-start' }" @click="closeAndEmit('print')">
              <ApIcon name="Printer" :size="16" />
              <span>{{ $t('spaceUi.k30') }}</span>
            </ApButton>
            <template v-if="isEditor">
              <div :style="dividerStyle" />
              <ApButton variant="danger" size="small" :style="{ width: '100%', justifyContent: 'flex-start' }" @click="showDeleteConfirm = true; moreOpen = false">
                <ApIcon name="Trash2" :size="16" />
                <span>{{ $t('spaceUi.k31') }}</span>
              </ApButton>
            </template>
          </div>
        </template>
      </ApPopover>
      </template>
    </div>

    <ApConfirmModal
      v-model="showDeleteConfirm"
      :title="$t('spaceUi.k31')"
      :message="$t('cExtraUi.k9')"
      :confirm-text="$t('spaceUi.delete')"
      type="danger"
      @confirm="$emit('delete')"
    />

    <ApModal v-model="showExportModal" size="small">
      <template #title>{{ $t('spaceUi.k32') }}</template>
      <div :style="exportOptionsStyle">
        <div v-bind="keyboardClick(() => doExport('exportPdf'))" :style="exportItemStyle">
          <div :style="exportIconStyle('pdf')">
            <ApIcon name="FileText" :size="20" />
          </div>
          <div :style="exportTextStyle">
            <div :style="exportNameStyle">PDF</div>
            <div :style="exportDescStyle">{{ $t('spaceUi.k33') }}</div>
          </div>
        </div>
        <div v-bind="keyboardClick(() => doExport('exportWord'))" :style="exportItemStyle">
          <div :style="exportIconStyle('word')">
            <ApIcon name="FileText" :size="20" />
          </div>
          <div :style="exportTextStyle">
            <div :style="exportNameStyle">Word</div>
            <div :style="exportDescStyle">{{ $t('spaceUi.k34') }}</div>
          </div>
        </div>
        <div v-bind="keyboardClick(() => doExport('exportHtml'))" :style="exportItemStyle">
          <div :style="exportIconStyle('html')">
            <ApIcon name="FileText" :size="20" />
          </div>
          <div :style="exportTextStyle">
            <div :style="exportNameStyle">HTML</div>
            <div :style="exportDescStyle">{{ $t('spaceUi.k35') }}</div>
          </div>
        </div>
        <div v-bind="keyboardClick(() => doExport('exportMd'))" :style="exportItemStyle">
          <div :style="exportIconStyle('md')">
            <ApIcon name="FileText" :size="20" />
          </div>
          <div :style="exportTextStyle">
            <div :style="exportNameStyle">Markdown</div>
            <div :style="exportDescStyle">{{ $t('spaceUi.k36') }}</div>
          </div>
        </div>
      </div>
    </ApModal>
  </div>
</template>

<script setup lang="ts">
import { shallowRef } from 'vue';
import { AppleFontWeight, AppleSpace, AppleTiming, AppleType, AppleRadius, cv, cssType } from '@/utils/core/apple-tokens';
import { keyboardClick } from '@/composables/ui/useKeyboardClick';
import ApButton from '@/components/apple/ApButton.vue';
import ApFavoriteButton from '@/components/apple/ApFavoriteButton.vue';
import ApPopover from '@/components/apple/ApPopover.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import { useAIStore } from '@/stores/ai/ai';
import { useBreakpoint } from '@/utils/responsive';

defineProps<{
  isEditor: boolean;
  collected: boolean;
  editing?: boolean;
  saving?: boolean;
  fullscreen?: boolean;
}>();

const emit = defineEmits<{
  (e: 'edit'): void;
  (e: 'images'): void;
  (e: 'attachments'): void;
  (e: 'collect'): void;
  (e: 'share'): void;
  (e: 'export'): void;
  (e: 'exportHtml'): void;
  (e: 'exportWord'): void;
  (e: 'exportPdf'): void;
  (e: 'exportMd'): void;
  (e: 'import'): void;
  (e: 'history'): void;
  (e: 'aiSummary'): void;
  (e: 'fullscreen'): void;
  (e: 'print'): void;
  (e: 'delete'): void;
  (e: 'save'): void;
  (e: 'cancel'): void;
}>();

const { isMobile } = useBreakpoint();

const aiStore = useAIStore();

const showDeleteConfirm = shallowRef(false);
const moreOpen = shallowRef(false);
const showExportModal = shallowRef(false);

/** 工具栏动作名全集：与 defineEmits 一一对应，模板里只允许传这些名字 */
type ToolbarEvent =
  | 'share'
  | 'history'
  | 'aiSummary'
  | 'images'
  | 'attachments'
  | 'fullscreen'
  | 'print'
  | 'exportPdf'
  | 'exportWord'
  | 'exportHtml'
  | 'exportMd';

/** defineEmits 生成的是重载签名，动态事件名过不了重载；这里收窄成单签名再转发，名字仍受 ToolbarEvent 约束 */
const emitEvent = emit as (event: ToolbarEvent) => void;

function closeAndEmit(event: ToolbarEvent) {
  moreOpen.value = false;
  emitEvent(event);
}

function doExport(event: ToolbarEvent) {
  showExportModal.value = false;
  emitEvent(event);
}

const toolbarStyle = {
  marginTop: AppleSpace.xs,
};

const mainStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
  flexWrap: 'nowrap' as const,
};

const menuStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
  maxHeight: '70vh',
  overflowY: 'auto' as const,
};

const dividerStyle = {
  height: '1px',
  background: cv('border-light'),
  margin: `${AppleSpace.xs} 0`,
};

const exportOptionsStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
  padding: `${AppleSpace.xs} 0`,
};

const exportItemStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.md,
  padding: `${AppleSpace.md} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.medium,
  cursor: 'pointer',
  border: `1px solid ${cv('border-light')}`,
  transition: `background ${AppleTiming.default}, border-color ${AppleTiming.default}, box-shadow ${AppleTiming.default}`,
};

function exportIconStyle(type: 'pdf' | 'word' | 'html' | 'md') {
  const colors: Record<string, string> = {
    pdf: cv('red'),
    word: cv('blue'),
    html: cv('orange'),
    md: cv('green'),
  };
  return {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: '40px',
    height: '40px',
    borderRadius: AppleRadius.medium,
    background: colors[type] + '15',
    color: colors[type],
    flexShrink: 0,
  };
}

const exportTextStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
};

const exportNameStyle = {
  ...cssType(AppleType.bodyPrimary),
  fontWeight: AppleFontWeight.semibold,
  color: cv('text-primary'),
};

const exportDescStyle = {
  ...cssType(AppleType.microUI),
  color: cv('text-secondary'),
};
</script>
