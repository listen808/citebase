<template>
  <div v-if="total > pageSize" :style="wrapperStyle" role="navigation" :aria-label="$t('pagination.label')">
    <ApButton
      variant="outline"
      size="small"
      :disabled="page <= 1"
      :aria-label="$t('pagination.previous')"
      @click="changePage(page - 1)"
    >
      {{ $t('pagination.previous') }}
    </ApButton>
    
    <!-- 添加页码按钮 -->
    <div :style="pageNumbersStyle">
      <ApButton
        v-for="pageNum in visiblePages"
        :key="pageNum"
        :variant="pageNum === page ? 'primary' : 'outline'"
        size="small"
        :aria-current="pageNum === page ? 'page' : undefined"
        :aria-label="$t('pagination.page', { page: pageNum })"
        @click="changePage(pageNum)"
      >
        {{ pageNum }}
      </ApButton>
      <span v-if="showEllipsis" :style="ellipsisStyle">...</span>
      <ApButton
        v-if="showLastPage"
        :variant="totalPages === page ? 'primary' : 'outline'"
        size="small"
        :aria-current="totalPages === page ? 'page' : undefined"
        :aria-label="$t('pagination.page', { page: totalPages })"
        @click="changePage(totalPages)"
      >
        {{ totalPages }}
      </ApButton>
    </div>
    
    <span :style="pageInfoStyle">
      {{ page }} / {{ totalPages }}
    </span>
    
    <!-- 添加跳转输入 -->
    <div :style="jumpInputStyle">
      <ApInput
        :model-value="jumpPage !== null ? String(jumpPage) : ''"
        :aria-label="$t('pagination.jumpAria')"
        type="number"
        :placeholder="$t('pagination.pageNumber')"
        @update:model-value="jumpPage = $event ? Number($event) : null"
        @keyup.enter="handleJump"
      />
      <ApButton variant="outline" size="small" @click="handleJump">{{ $t('pagination.jump') }}</ApButton>
    </div>
    
    <ApButton
      variant="outline"
      size="small"
      :disabled="page >= totalPages"
      :aria-label="$t('pagination.next')"
      @click="changePage(page + 1)"
    >
      {{ $t('pagination.next') }}
    </ApButton>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed, shallowRef } from 'vue';
import { AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';
import { useUIStore } from '@/stores/app/ui';
import ApButton from '@/components/apple/ApButton.vue';
import ApInput from '@/components/apple/ApInput.vue';

const props = defineProps<{
  page: number;
  pageSize: number;
  total: number;
}>();

const emit = defineEmits<{
  (e: 'change', page: number): void;
}>();
const { t } = useI18n();

const ui = useUIStore();
const totalPages = computed(() => Math.ceil(props.total / props.pageSize));
const jumpPage = shallowRef<number | null>(null);

// 计算可见页码（最多显示5个）
const visiblePages = computed(() => {
  const pages: number[] = [];
  const maxVisible = 5;
  let start = Math.max(1, props.page - Math.floor(maxVisible / 2));
  const end = Math.min(totalPages.value, start + maxVisible - 1);
  
  if (end - start < maxVisible - 1) {
    start = Math.max(1, end - maxVisible + 1);
  }
  
  for (let i = start; i <= end; i++) {
    if (showLastPage.value && i === totalPages.value) continue;
    pages.push(i);
  }
  
  return pages;
});

const showEllipsis = computed(() => {
  return totalPages.value > 7 && props.page < totalPages.value - 3;
});

const showLastPage = computed(() => {
  return totalPages.value > 7 && props.page < totalPages.value - 3;
});

function changePage(newPage: number) {
  if (newPage >= 1 && newPage <= totalPages.value) {
    emit('change', newPage);
  }
}

function handleJump() {
  if (!jumpPage.value) return;
  if (jumpPage.value >= 1 && jumpPage.value <= totalPages.value) {
    changePage(jumpPage.value);
    jumpPage.value = null;
  } else {
    ui.showToast(t('sentUi.k68', { p1: totalPages.value }), 'warning');
  }
}

const wrapperStyle = computed(() => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  // 一条分页有 prev + 最多 5 个页码 + 页码信息 + 跳转输入 + next，窄屏必然放不下；
  // 不换行时整条会溢出视口（外层容器的 flexWrap 管不到 flex 子项的固有宽度）
  flexWrap: 'wrap' as const,
  gap: AppleSpace.md,
  rowGap: AppleSpace.sm,
  padding: AppleSpace.md,
  borderTop: `1px solid ${cv('border-light')}` }));

const pageInfoStyle = computed(() => ({
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-secondary') }));

// 新增样式
const pageNumbersStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
  alignItems: 'center' };

const ellipsisStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-tertiary'),
  padding: `0 ${AppleSpace.xs}` };

const jumpInputStyle = {
  display: 'flex',
  gap: AppleSpace.sm,
  alignItems: 'center'
};
</script>
