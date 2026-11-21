<template>
  <div :style="panelStyle">
    <div
v-if="isEditor"
      :style="[uploadAreaStyle, isDragging ? uploadAreaActiveStyle : {}]"
      @click="triggerUpload"
      @dragover.prevent="handleDragOver"
      @dragenter.prevent="handleDragEnter"
      @dragleave.prevent="handleDragLeave"
      @drop.prevent="handleDrop"
    >
      <input
        ref="fileInputRef"
        type="file"
        :aria-label="type === 'images' ? '选择要添加的图片' : '选择要添加的附件'"
        :multiple="true"
        :style="fileInputStyle"
        @change="handleFileSelect"
      />
      <div :style="uploadIconStyle">
        <!-- 这里的 <Upload> 从来没有 import 过：Vue 解析不到组件就渲染成空，图标位一直是空白格。
             收到 ApIcon + 注册表后，名字写错会被门禁 I1 与类型一起拦下 -->
        <ApIcon name="Upload" :size="AppleIconSize.section" />
      </div>
      <p :style="uploadTextStyle">
        {{ type === 'images' ? '点击或拖拽上传图片' : $t('spaceUi.k68') }}
      </p>
    </div>

    <div v-if="uploadProgress > 0 && uploadProgress < 100" :style="progressStyle">
      <ApProgress :value="uploadProgress" :label="$t('spaceUi.k65')" :style="{ flex: 1 }" />
      <span :style="progressTextStyle">{{ uploadProgress }}%</span>
    </div>

    <ImageGrid
      v-if="type === 'images'"
      :attachments="attachments"
      :loading="loading"
      :is-editor="isEditor"
      @open-preview="openPreview"
      @request-delete="requestDelete"
    />

    <AttachmentList
      v-else
      :attachments="attachments"
      :loading="loading"
      :is-editor="isEditor"
      @request-delete="requestDelete"
    />

    <ApImageLightbox
      v-if="type === 'images'"
      v-model="showPreview"
      v-model:index="previewIndex"
      :items="previewItems"
      :label="$t('spaceUi.k66')"
    />

    <ApConfirmModal
      v-model="showDeleteConfirm"
      :title="$t('spaceUi.k67')"
      :message="type === 'images' ? '确定删除该图片吗？' : '确定删除该附件吗？'"
      :confirm-text="$t('spaceUi.delete')"
      type="danger"
      @confirm="confirmDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { shallowRef, computed, watch } from 'vue';
import { AppleIconSize, AppleRadius, AppleSpace, AppleTransition, AppleType, cv } from '@/utils/core/apple-tokens';
import { useAttachmentUpload } from '@/composables/editor/useAttachmentUpload';
import { formatSize, resolveFileUrl } from '@/utils/domain/file';
import type { Attachment } from '@/api/modules/document/attachment';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ImageGrid from '../action/ImageGrid.vue';
import AttachmentList from '../action/AttachmentList.vue';

const props = defineProps<{
  documentId: number;
  type: 'images' | 'attachments';
  isEditor: boolean;
}>();

const emit = defineEmits<{
  (e: 'insert-image', url: string): void;
  (e: 'insert-attachment', url: string, name: string): void;
}>();

const {
  attachments,
  loading,
  uploadProgress,
  isDragging,
  fileInputRef,
  load,
  triggerUpload,
  handleDragOver,
  handleDragEnter,
  handleDragLeave,
  handleDrop,
  handleFileSelect,
  deleteAttachmentById,
} = useAttachmentUpload(
  () => props.documentId,
  () => props.type,
  () => props.isEditor,
  emit
);

const showPreview = shallowRef(false);
const previewIndex = shallowRef(0);
// 翻页与说明行由 ApImageLightbox 负责，这里只把附件映射成"能看的图"
const previewItems = computed(() => attachments.value
  .filter(a => !!a.url)
  .map(a => ({ src: resolveFileUrl(a.url as string), name: a.name, meta: formatSize(a.size) })));
const showDeleteConfirm = shallowRef(false);
const deleteTarget = shallowRef<number | null>(null);

function openPreview(img: Attachment) {
  // 下标要在"能看的那批"里找：灯箱翻的就是这个过滤后的列表，用原数组下标会错位
  const visible = attachments.value.filter(a => !!a.url);
  previewIndex.value = Math.max(0, visible.findIndex(a => a.id === img.id));
  showPreview.value = true;
}



function requestDelete(id: number) {
  deleteTarget.value = id;
  showDeleteConfirm.value = true;
}

async function confirmDelete() {
  if (!deleteTarget.value) return;
  await deleteAttachmentById(deleteTarget.value);
  deleteTarget.value = null;
}

watch(() => [props.documentId, props.type], () => {
  load();
}, { immediate: true });

const panelStyle = {
  background: cv('surface-panel'),
  borderRadius: AppleRadius.medium,
  border: `1px solid ${cv('border')}`,
  padding: AppleSpace.md
};

const uploadAreaStyle = {
  border: `2px dashed ${cv('border')}`,
  borderRadius: AppleRadius.medium,
  padding: AppleSpace.xl,
  textAlign: 'center' as const,
  cursor: 'pointer',
  transition: AppleTransition.default,
  marginBottom: AppleSpace.md
};

const uploadAreaActiveStyle = {
  border: `2px dashed ${cv('blue')}`,
  background: cv('blue-light'),
};

const fileInputStyle = {
  display: 'none'
};

const uploadIconStyle = {
  marginBottom: AppleSpace.sm,
  color: cv('text-tertiary')
};

const uploadTextStyle = {
  color: cv('text-tertiary'),
  fontSize: AppleType.controlLabel.size,
  margin: '0'
};

const progressStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.md,
  marginBottom: AppleSpace.md
};

const progressTextStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  minWidth: '40px'
};

</script>
