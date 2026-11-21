<template>
  <div>
    <div v-if="loading" :style="loadingStyle">
      <ApSkeleton variant="grid" :count="4" />
    </div>
    <div v-else-if="attachments.length === 0" :style="emptyStyle">
      <ApIcon name="Image" :size="AppleIconSize.empty" :style="{ color: cv('text-quaternary'), marginBottom: AppleSpace.sm }" />
      <p>{{ $t('spaceUi.k4') }}</p>
    </div>
    <div v-else :style="gridStyle">
      <div v-for="img in attachments" :key="img.id" :style="imgCardStyle">
        <div :style="previewContainerStyle" @click="$emit('open-preview', img)" @mouseenter="hoveredImg = img.id" @mouseleave="hoveredImg = null">
          <img
            :src="resolveFileUrl(img.url)"
            :alt="img.name"
            :style="[thumbnailStyle, hoveredImg === img.id ? thumbnailHoverStyle : {}]"
            loading="lazy"
            @error="handleImageError"
          />
          <div :style="[overlayStyle, hoveredImg === img.id ? overlayVisibleStyle : {}]">
            <ApIcon name="Eye" :size="AppleIconSize.listLead" :style="{ color: cv('on-accent') }" />
            <span :style="overlayTextStyle">{{ $t('spaceUi.k39') }}</span>
          </div>
        </div>
        <div :style="imgMetaStyle">
          <span :style="imgNameStyle" :title="img.name">{{ img.name }}</span>
          <span :style="imgSizeStyle">{{ formatSize(img.size) }}</span>
        </div>
        <div v-if="isEditor" :style="imgActionRowStyle">
          <a :href="getDownloadUrl(img.url)" target="_blank" :style="downloadActionStyle" download>{{ $t('spaceUi.download') }}</a>
          <ApButton variant="danger" size="small" @click.stop="$emit('request-delete', img.id)">{{ $t('spaceUi.delete') }}</ApButton>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { AppleIconSize, cv, AppleType, AppleSpace, AppleTransition, AppleFontWeight } from '@/utils/core/apple-tokens';
import { loadingStyle, emptyStateStyle as emptyStyle, mediaImgGridStyle as imgGridStyle, mediaImgCardStyle as imgCardStyle, mediaImgMetaStyle as imgMetaStyle, mediaImgNameStyle as imgNameStyle, mediaImgSizeStyle as imgSizeStyle, mediaImgActionRowStyle as imgActionRowStyle, mediaDownloadActionStyle as downloadActionStyle } from '@/styles/layout/page-styles';
import { formatSize, resolveFileUrl, getDownloadUrl } from '@/utils/domain/file';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApButton from '@/components/apple/ApButton.vue';
import type { Attachment } from '@/api/modules/document/attachment';

defineProps<{
  attachments: Attachment[];
  loading: boolean;
  isEditor: boolean;
}>();

defineEmits<{
  (e: 'open-preview', img: Attachment): void;
  (e: 'request-delete', id: number): void;
}>();

const gridStyle = imgGridStyle;

function handleImageError(e: Event) {
  const target = e.target as HTMLImageElement;
  target.style.opacity = '0.5';
}

const hoveredImg = ref<number | null>(null);

const previewContainerStyle = {
  position: 'relative' as const,
  cursor: 'pointer',
  overflow: 'hidden',
};

const thumbnailStyle = {
  width: '100%',
  height: '140px',
  objectFit: 'cover' as const,
  display: 'block',
  transition: AppleTransition.smooth,
};

const thumbnailHoverStyle = {
  transform: 'scale(1.05)',
};

const overlayStyle = {
  position: 'absolute' as const,
  inset: '0',
  background: cv('overlay'),
  display: 'flex',
  flexDirection: 'column' as const,
  alignItems: 'center',
  justifyContent: 'center',
  gap: AppleSpace.sm,
  opacity: '0',
  transition: AppleTransition.fast,
};

const overlayVisibleStyle = {
  opacity: '1',
};

const overlayTextStyle = {
  color: cv('on-accent'),
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
};
</script>
