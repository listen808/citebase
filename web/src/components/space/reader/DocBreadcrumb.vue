<template>
  <div :style="rootStyle">
    <ApButton v-if="hasBackButton" variant="outline" size="small" @click="$emit('back')">
      <ApIcon name="ArrowLeft" :size="16" /> {{ $t('spaceUi.back') }}
    </ApButton>
    <ApBreadcrumb
      :items="breadcrumbItems"
      :always-interactive="false"
      :style="breadcrumbWrapperStyle"
      @navigate="onNavigate"
    />
    <span v-if="editMode && breadcrumbItems.length > 0" :style="editLabelStyle">{{ $t('spaceUi.edit') }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleType, AppleLineHeight, cv } from '@/utils/core/apple-tokens';
import ApBreadcrumb from '@/components/apple/ApBreadcrumb.vue';
import ApButton from '@/components/apple/ApButton.vue';
import type { BreadcrumbItem as ApBreadcrumbItem } from '@/components/apple/ApBreadcrumb.vue';

interface BreadcrumbItem {
  id: number;
  name: string;
}

const props = withDefaults(defineProps<{
  spaceName: string;
  crumbs: BreadcrumbItem[];
  hasBackButton: boolean;
  editMode?: boolean;
}>(), {
  editMode: false,
});

const emit = defineEmits<{
  back: [];
  'navigate-to-space-root': [];
  'navigate-to-doc': [id: number];
}>();

const breadcrumbItems = computed<ApBreadcrumbItem[]>(() => {
  let resolvedSpaceName = props.spaceName;
  let crumbsToProcess = props.crumbs;
  
  if ((!resolvedSpaceName || resolvedSpaceName === '空间') && crumbsToProcess.length > 0 && crumbsToProcess[0]) {
    resolvedSpaceName = crumbsToProcess[0].name;
    crumbsToProcess = crumbsToProcess.slice(1);
  }
  
  if (!resolvedSpaceName) {
    return [];
  }
  
  const items: ApBreadcrumbItem[] = [
    { label: resolvedSpaceName, path: '__space__' },
  ];
  const filteredCrumbs = crumbsToProcess.filter((crumb, index) => {
    if (index === 0 && crumb.name === resolvedSpaceName) {
      return false;
    }
    return true;
  });
  for (const crumb of filteredCrumbs) {
    items.push({ label: crumb.name, path: `__doc__${crumb.id}` });
  }
  return items;
});

function onNavigate(item: ApBreadcrumbItem) {
  if (item.path === '__space__') {
    emit('navigate-to-space-root');
  } else if (item.path?.startsWith('__doc__')) {
    const id = Number(item.path.slice(7));
    emit('navigate-to-doc', id);
  }
}

const rootStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  flexWrap: 'wrap' as const,
};

// 阅读区这一条不要"面包屑与 H1 之间"的那 8px（它上面没有 H1），
// 但保留它自己的上下 16px 呼吸 —— 合并实现后节奏默认走容器那一档，这里显式覆盖回来。
const breadcrumbWrapperStyle = {
  marginBottom: 0,
  padding: `${AppleSpace.md} 0`,
};

const editLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-tertiary'),
  padding: `${AppleSpace.xxs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.small,
  background: cv('bg-secondary'),
  lineHeight: AppleLineHeight.display,
};
</script>
