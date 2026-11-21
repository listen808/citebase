<template>
  <ApPageContainer no-scroll>
    <ApErrorPage
      :icon="icons.ServerCrash"
      icon-color="red"
      status-code="500"
      :title="$t('vCommonUi.k16')"
      :description="$t('vCommonUi.k17')"
      :primary-label="$t('lastAttrsUi.k7')"
      :secondary-label="$t('lastAttrsUi.retry')"
      :on-secondary-action="handleRetry"
      :on-primary-action="() => router.push('/')"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router';
// ApErrorPage 的 icon 是 Component、经 :is 渲染，必须传注册表组件（字符串会被当成组件渲染失败）。
import { icons } from '@/utils/icons';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApErrorPage from '@/components/apple/ApErrorPage.vue';

const router = useRouter();
const route = useRoute();

function handleRetry() {
  // router.onError 兜底到这页时带 from（原目标）——重试应回到原处重新加载
  // 懒加载 chunk，而不是刷新错误页自己；直接输入 URL 到达时（无 from）维持整页刷新。
  const from = route.query.from;
  if (typeof from === 'string' && from.startsWith('/') && !from.startsWith('//')) {
    router.push(from);
  } else {
    window.location.reload();
  }
}
</script>