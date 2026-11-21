<template>
  <ApPageContainer no-scroll>
    <ApErrorPage
      :icon="icons.SearchX"
      icon-color="blue"
      status-code="404"
      :title="$t('vCommonUi.k14')"
      :description="$t('vCommonUi.k15')"
      :primary-label="$t('lastAttrsUi.k7')"
      :secondary-label="$t('lastAttrsUi.k8')"
      :on-secondary-action="() => router.back()"
      :on-primary-action="() => router.push('/')"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';
// ApErrorPage 的 icon 是 Component、经 :is 渲染，必须传注册表组件（字符串会被当成组件渲染失败）。
import { icons } from '@/utils/icons';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApErrorPage from '@/components/apple/ApErrorPage.vue';

const router = useRouter();
</script>