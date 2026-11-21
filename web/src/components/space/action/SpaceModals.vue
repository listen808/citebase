<template>
  <ApModal v-model="showCreateModal" :title="$t('spaceUi.k50')" size="medium">
    <div :style="formGroupStyle">
      <label :style="labelStyle">{{ $t('spaceUi.name') }}</label>
      <ApInput v-model="createDocName" :placeholder="$t('spaceUi.k49')" @keyup.enter="$emit('createDoc')" />
    </div>
    <div :style="formGroupStyle">
      <label :style="labelStyle">{{ $t('spaceUi.type') }}</label>
      <div :style="radioGroupStyle">
        <ApRadio v-model="createDocType" :value="1" :label="$t('spaceUi.k47')" />
        <ApRadio v-model="createDocType" :value="2" :label="$t('spaceUi.k48')" />
      </div>
    </div>
    <div v-if="createDocType === 1" :style="formGroupStyle">
      <label :style="labelStyle">{{ $t('spaceUi.k42') }}</label>
      <!-- Category tabs -->
      <div :style="categoryTabsStyle">
        <ApSegmentedControl
          :model-value="activeCategory"
          :options="categoryOptions"
          size="small"
          :aria-label="$t('spaceUi.k57')"
          @update:model-value="(v: string | number) => activeCategory = String(v)"
        />
      </div>
      <!-- Template grid -->
      <div :style="templateGridStyle">
        <div
          v-for="tpl in filteredTemplates"
          :key="tpl.key"
          :style="templateCardStyle(selectedTemplateKey === tpl.key)"
          @click="selectedTemplateKey = tpl.key"
        >
          <!-- 模板图标走共享注册表 + ApIcon（与 AITemplatePicker 同一画法）：
               以前这一格渲染 docTemplates 里的 emoji，字号用 statValue —— 等于第三套图标族，
               门禁看不见（它只认 lucide 与 ApIcon），尺寸与描边都无从收口 -->
          <ApIcon :name="tpl.icon" :size="AppleIconSize.section" :style="templateIconStyle" />
          <span :style="templateNameStyle">{{ tpl.name }}</span>
          <span :style="templateDescStyle">{{ tpl.desc }}</span>
        </div>
      </div>
      <ApButton v-if="selectedTemplateKey" variant="outline" size="small" :style="{ marginTop: AppleSpace.sm }" @click="showTemplatePreview = true">{{ $t('spaceUi.k43') }}</ApButton>
    </div>
    <template #footer>
      <ApButton variant="ghost" size="small" @click="showCreateModal = false">{{ $t('spaceUi.cancel') }}</ApButton>
      <!-- 与 77 行的重命名同一条口径：名字为空就把按钮置灰，并把原因写在 tooltip 上
           （此前按钮恒可点，空名提交由 handler 静默 return，看着像没响应） -->
      <ApButton
        variant="primary"
        size="small"
        :disabled="!createDocName.trim() || creatingDoc"
        :tooltip="createDocName.trim() || creatingDoc ? '' : '请先输入文档名称'"
        @click="$emit('createDoc')"
      >{{ creatingDoc ? $t('spaceUi.k58') : $t('spaceUi.create2') }}</ApButton>
    </template>
  </ApModal>

  <!-- 2026-10-26：纯「关闭」的 footer 全部删除 —— ApModal 右上角自带 ×，两处关闭入口重复
       （用户报"弹窗有 X 有关闭，重复"）。带表单的弹窗（创建/重命名）footer 保留的是
       「取消 + 主操作」双按钮，属标准形态，不在删除之列。 -->
  <ApModal v-model="showTemplatePreview" :title="templatePreviewTitle" size="large">
    <div :style="templatePreviewStyle">
      <ApMarkdownPreview :content="templatePreviewContent" />
    </div>
  </ApModal>

  <ApConfirmModal
    v-model="showDeleteModal"
    :title="$t('spaceUi.k31')"
    :message="deleteMessage"
    :confirm-text="$t('spaceUi.delete')"
    type="danger"
    :require-input-confirm="true"
    :confirm-match-text="deletingDocName"
    :confirm-placeholder="`请输入「${deletingDocName}」确认删除`"
    @confirm="$emit('confirmDelete')"
  />

  <ApModal v-model="showRenameModal" :title="$t('spaceUi.k51')" size="small">
    <div :style="formGroupStyle">
      <label :style="labelStyle">{{ $t('spaceUi.name') }}</label>
      <ApInput v-model="renameDocName" :placeholder="$t('spaceUi.k49')" @keyup.enter="$emit('renameDoc')" />
    </div>
    <template #footer>
      <ApButton variant="ghost" size="small" @click="showRenameModal = false">{{ $t('spaceUi.cancel') }}</ApButton>
      <ApButton variant="primary" size="small" :disabled="!renameDocName.trim() || renamingDoc" @click="$emit('renameDoc')">{{ renamingDoc ? $t('spaceUi.k60') : $t('spaceUi.save') }}</ApButton>
    </template>
  </ApModal>

  <ApModal v-model="showShareModal" :title="$t('spaceUi.k52')" size="small">
    <p :style="shareDescStyle">{{ $t('spaceUi.k45') }}</p>
    <div :style="shareUrlBoxStyle">
      <ApInput :model-value="shareUrl" readonly :style="{ flex: 1 }" @click="($event.target as HTMLInputElement).select()" />
      <ApButton variant="primary" size="small" @click="$emit('copyShareUrl')">{{ $t('spaceUi.copy') }}</ApButton>
    </div>
  </ApModal>

  <ApModal v-model="showHistoryModal" :title="$t('spaceUi.k53')" size="large">
    <VersionHistory v-if="currentDocId" :document-id="currentDocId" @rollback="$emit('versionRollback', $event)" />
  </ApModal>

  <ApModal v-model="showImagesModal" :title="$t('spaceUi.k28')" size="large">
    <DocAttachmentPanel :document-id="currentDocId || 0" type="images" :is-editor="canEditInSpace" />
  </ApModal>

  <ApModal v-model="showAttachmentsModal" :title="$t('spaceUi.k29')" size="large">
    <DocAttachmentPanel :document-id="currentDocId || 0" type="attachments" :is-editor="canEditInSpace" />
  </ApModal>

  <TargetFolderPicker v-model="showMoveModal" :title="$t('spaceUi.k54')" :space-id="spaceId" :current-doc-id="movingDocId || undefined" @confirm="$emit('confirmMove', $event)" />
  <TargetFolderPicker v-model="showCopyModal" :title="$t('spaceUi.k55')" :space-id="spaceId" :current-doc-id="copyingDocId || undefined" @confirm="$emit('confirmCopy', $event)" />

  <ApModal v-model="showShortcutHelp" :title="$t('spaceUi.k56')" size="medium">
    <div :style="{ fontFamily: AppleFont.text, fontSize: AppleType.controlLabel.size, color: cv('text-secondary') }">
      <div v-for="item in shortcutList" :key="item.key" :style="{ display: 'flex', justifyContent: 'space-between', padding: `${AppleSpace.sm} 0`, borderBottom: `1px solid ${cv('border-light')}` }">
        <span>{{ item.desc }}</span>
        <kbd :style="{ padding: `${AppleSpace.xxs} ${AppleSpace.sm}`, borderRadius: AppleRadius.micro, background: cv('light-gray'), border: `1px solid ${cv('border')}`, fontSize: AppleType.microUI.size, fontFamily: AppleFont.mono }">{{ item.key }}</kbd>
      </div>
    </div>
  </ApModal>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { AppleFont, AppleRadius, AppleSpace, AppleType, AppleFontWeight, AppleTransition, cv, AppleLineHeight, AppleGridFloor, AppleIconSize } from '@/utils/core/apple-tokens';
import { docTemplates, templateCategories } from '@/config/docTemplates';
import { formGroupStyle } from '@/styles/component/form-field-styles';
import ApButton from '@/components/apple/ApButton.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApRadio from '@/components/apple/ApRadio.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApMarkdownPreview from '@/components/apple/ApMarkdownPreview.vue';
import DocAttachmentPanel from '@/components/space/sidebar/DocAttachmentPanel.vue';
import VersionHistory from '@/components/document/VersionHistory.vue';
import TargetFolderPicker from '@/components/document/TargetFolderPicker.vue';

defineProps<{
  creatingDoc: boolean;
  renamingDoc: boolean;
  deleteMessage: string;
  deletingDocName: string;
  shareUrl: string;
  shortcutList: { key: string; desc: string }[];
  currentDocId: number | null;
  spaceId: number;
  canEditInSpace: boolean;
  movingDocId: number | null;
  copyingDocId: number | null;
}>();
defineEmits<{
  (e: 'createDoc'): void;
  (e: 'confirmDelete'): void;
  (e: 'renameDoc'): void;
  (e: 'copyShareUrl'): void;
  (e: 'versionRollback', version: number): void;
  (e: 'confirmMove', targetId: number): void;
  (e: 'confirmCopy', targetId: number): void;
}>();
const showCreateModal = defineModel<boolean>('showCreateModal', { default: false });
const showTemplatePreview = defineModel<boolean>('showTemplatePreview', { default: false });
const showDeleteModal = defineModel<boolean>('showDeleteModal', { default: false });

const showRenameModal = defineModel<boolean>('showRenameModal', { default: false });
const showShareModal = defineModel<boolean>('showShareModal', { default: false });
const showHistoryModal = defineModel<boolean>('showHistoryModal', { default: false });
const showImagesModal = defineModel<boolean>('showImagesModal', { default: false });
const showAttachmentsModal = defineModel<boolean>('showAttachmentsModal', { default: false });
const showMoveModal = defineModel<boolean>('showMoveModal', { default: false });
const showCopyModal = defineModel<boolean>('showCopyModal', { default: false });
const showShortcutHelp = defineModel<boolean>('showShortcutHelp', { default: false });
const createDocName = defineModel<string>('createDocName', { default: '' });
const createDocType = defineModel<number>('createDocType', { default: 1 });
const selectedTemplateKey = defineModel<string>('selectedTemplateKey', { default: '' });
const renameDocName = defineModel<string>('renameDocName', { default: '' });

const templatePreviewTitle = computed(() => {
  const t = docTemplates.find(t => t.key === selectedTemplateKey.value);
  return t ? `模板预览 - ${t.name}` : '模板预览';
});

const templatePreviewContent = computed(() => {
  const t = docTemplates.find(t => t.key === selectedTemplateKey.value);
  return t?.content || '';
});

const activeCategory = ref('all');
const categoryOptions = templateCategories.map((c) => ({ label: c.label, value: c.key }));
const filteredTemplates = computed(() => {
  if (activeCategory.value === 'all') return docTemplates;
  return docTemplates.filter(t => t.category === activeCategory.value);
});

const labelStyle = { display: 'block', fontFamily: AppleFont.text, fontSize: AppleType.microUI.size, color: cv('text-tertiary'), marginBottom: `${AppleSpace.xs}` };
const radioGroupStyle = { display: 'flex', gap: AppleSpace.md };
const categoryTabsStyle = { display: 'flex', gap: AppleSpace.xs, marginBottom: AppleSpace.sm, flexWrap: 'wrap' as const };

const templateGridStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor.tile})`,
  gap: AppleSpace.sm,
  maxHeight: '240px',
  overflowY: 'auto' as const,
  padding: `${AppleSpace.xs} 0`,
};
function templateCardStyle(selected: boolean) {
  return {
    display: 'flex',
    flexDirection: 'column' as const,
    alignItems: 'center',
    gap: AppleSpace.xxs,
    padding: AppleSpace.sm,
    borderRadius: AppleRadius.small,
    border: selected ? `2px solid ${cv('blue')}` : `1px solid ${cv('border-light')}`,
    background: selected ? cv('blue-selection') : cv('surface-field'),
    cursor: 'pointer',
    transition: AppleTransition.fast,
    textAlign: 'center' as const,
  };
}
const templateIconStyle = { color: cv('text-secondary'), lineHeight: '1' };
const templateNameStyle = {
  fontSize: AppleType.microUI.size,
  fontFamily: AppleFont.text,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
};
const templateDescStyle = {
  fontSize: AppleType.legalMeta.size,
  fontFamily: AppleFont.text,
  color: cv('text-quaternary'),
  lineHeight: AppleLineHeight.display,
};
const templatePreviewStyle = {
  maxHeight: '60vh',
  overflowY: 'auto' as const,
};

const shareDescStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-secondary'),
  marginBottom: AppleSpace.md,
};

const shareUrlBoxStyle = {
  display: 'flex',
  gap: AppleSpace.sm,
  alignItems: 'center',
};
</script>
