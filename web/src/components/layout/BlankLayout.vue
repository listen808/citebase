<template>
  <div :style="layoutStyle">
    <!-- 公开页（登录/安装/分享）也要有 main 地标；display:contents 只给语义，不额外造盒子 -->
    <main :style="{ display: 'contents' }">
      <RouterView />
    </main>
    <ApFooter pinned />
  </div>
</template>

<script setup lang="ts">
import { cv, AppleFont } from '@/utils/core/apple-tokens';
import ApFooter from '@/components/apple/ApFooter.vue';

const layoutStyle = {
  display: 'flex' as const,
  flexDirection: 'column' as const,
  minHeight: '100dvh',
  background: cv('surface-canvas'),
  fontFamily: AppleFont.text,
  WebkitFontSmoothing: 'antialiased',
  MozOsxFontSmoothing: 'grayscale',
};
</script>
