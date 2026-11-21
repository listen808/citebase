<template>
  <div>
    <ApSegmentedControl
      v-model="activeTab"
      :options="tabOptions"
      :style="{ marginBottom: AppleSpace.md }"
    />

    <div v-if="loading" :style="loadingStyle">
      <ApSkeleton variant="card" :lines="3" />
    </div>

    <div v-else-if="activeTab === 'files'">
      <div v-if="files.length === 0" :style="emptyStyle">
        <ApIcon name="File" :size="AppleIconSize.empty" :style="{ color: cv('text-quaternary'), marginBottom: AppleSpace.sm }" />
        <p>{{ $t('spaceUi.k1') }}</p>
      </div>
      <div v-else :style="fileListStyle">
        <div v-for="att in files" :key="att.id" :style="fileCardStyle">
          <div :style="fileAvatarStyle">
            <ApIcon name="File" :size="AppleIconSize.listLead" :style="{ color: cv('on-accent') }" />
          </div>
          <div :style="fileBodyStyle">
            <div :style="fileTitleStyle" :title="att.name">{{ att.name }}</div>
            <div :style="fileMetaStyle">
              <span>{{ formatSize(att.size) }}</span>
              <span :style="timeStyle">{{ formatTime(att.create_time) }}</span>
            </div>
          </div>
          <div v-if="isEditor" :style="fileActionStyle">
            <a v-if="att.url" :href="getDownloadUrl(att.url)" target="_blank" :style="downloadLinkStyle" download>{{ $t('spaceUi.download') }}</a>
            <ApButton variant="danger" size="small" @click="$emit('request-delete', att.id)">{{ $t('spaceUi.delete') }}</ApButton>
          </div>
        </div>
      </div>
    </div>

    <div v-else>
      <div v-if="images.length === 0" :style="emptyStyle">
        <ApIcon name="Image" :size="AppleIconSize.empty" :style="{ color: cv('text-quaternary'), marginBottom: AppleSpace.sm }" />
        <p>{{ $t('spaceUi.k4') }}</p>
      </div>
      <div v-else :style="imgGridStyle">
        <div v-for="img in images" :key="img.id" :style="imgCardStyle">
          <div :style="imgContainerStyle" @click="openPreview(img)">
            <img
              v-if="img.url"
              :src="getDownloadUrl(img.url)"
              :alt="img.name"
              :style="imgThumbStyle"
              loading="lazy"
            />
            <!-- 缩略图缺位时的占位：32 是这格 64px 缩略图框里唯一合适的数（24 太小、48 顶到边），
                 属于「确有一-off 尺寸」的调用点，走 ApIcon 拿描边、数字留在门禁 I6 基线里 -->
            <ApIcon v-else name="Image" :size="AppleIconSize.status" :style="{ color: cv('text-quaternary') }" />
          </div>
          <div :style="imgNameStyle" :title="img.name">{{ img.name }}</div>
        </div>
      </div>
    </div>

    <ApImageLightbox v-model="previewOpen" :items="previewItems" :label="$t('spaceUi.k5')" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed, shallowRef } from 'vue';
import { AppleIconSize, cv, AppleSpace, AppleFont, AppleType, AppleRadius, AppleGridFloor } from '@/utils/core/apple-tokens';
import { loadingStyle, emptyStateStyle as emptyStyle, timeStyle, mediaFileListStyle as fileListStyle, mediaFileCardStyle as fileCardStyle, mediaFileAvatarStyle as fileAvatarStyle, mediaFileBodyStyle as fileBodyStyle, mediaFileTitleStyle as fileTitleStyle, mediaFileMetaStyle as fileMetaStyle, mediaFileActionStyle as fileActionStyle, mediaDownloadLinkStyle as downloadLinkStyle } from '@/styles/layout/page-styles';
import { formatSize, getDownloadUrl } from '@/utils/domain/file';
import { formatTime } from '@/utils/format/format-time';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApImageLightbox from '@/components/apple/ApImageLightbox.vue';
import type { Attachment } from '@/api/modules/document/attachment';

const props = defineProps<{
  attachments: Attachment[];
  loading: boolean;
  isEditor: boolean;
}>();

defineEmits<{
  (e: 'request-delete', id: number): void;
}>();
const { t } = useI18n();

const activeTab = shallowRef('files');

const tabOptions = [
  { label: t('spaceUi.k6'), value: 'files' },
  { label: t('spaceUi.k7'), value: 'images' },
];

const imageExtensions = new Set(['.jpg', '.jpeg', '.png', '.gif', '.svg', '.webp', '.bmp', '.ico', '.tiff']);

const files = computed(() =>
  props.attachments.filter(a => !imageExtensions.has(a.name.toLowerCase().slice(a.name.lastIndexOf('.'))))
);

const images = computed(() =>
  props.attachments.filter(a => imageExtensions.has(a.name.toLowerCase().slice(a.name.lastIndexOf('.'))))
);

const previewImage = shallowRef<Attachment | null>(null);

function openPreview(img: Attachment) {
  if (img.url) {
    previewImage.value = img;
  }
}

// Esc / 焦点 / 遮罩都归 ApImageLightbox，这里只留"在看哪一张"
const previewOpen = computed({
  get: () => !!previewImage.value,
  set: (open: boolean) => { if (!open) previewImage.value = null; },
});
const previewItems = computed(() => {
  const img = previewImage.value;
  return img?.url ? [{ src: getDownloadUrl(img.url), name: img.name }] : [];
});

const imgGridStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor.tile})`,
  gap: AppleSpace.md,
};

const imgCardStyle = {
  borderRadius: AppleRadius.card,
  overflow: 'hidden',
  background: cv('bg-secondary'),
  border: `1px solid ${cv('border-light')}`,
};

const imgContainerStyle = {
  width: '100%',
  aspectRatio: '1',
  overflow: 'hidden',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  background: cv('bg-tertiary'),
  cursor: 'pointer',
};

const imgThumbStyle = {
  width: '100%',
  height: '100%',
  objectFit: 'cover' as const,
};

const imgNameStyle = {
  padding: AppleSpace.sm,
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};


</script>