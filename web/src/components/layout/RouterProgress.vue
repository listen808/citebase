<template>
  <ApProgress v-show="visible" :value="progress" :label="$t('layoutUi.k3')" size="hair" edge />
</template>

<script setup lang="ts">
import { shallowRef, watch, onBeforeUnmount } from 'vue';
import { useRouter } from 'vue-router';
import { useUIStore } from '@/stores/app/ui';

const visible = shallowRef(false);
const progress = shallowRef(0);
const ui = useUIStore();

const router = useRouter();
let timer: ReturnType<typeof setTimeout> | null = null;
let interval: ReturnType<typeof setInterval> | null = null;

function start() {
  visible.value = true;
  progress.value = 0;

  if (timer) clearTimeout(timer);
  if (interval) clearInterval(interval);

  interval = setInterval(() => {
    progress.value = Math.min(progress.value + Math.random() * 15, 90);
  }, 200);
}

function done() {
  if (interval) clearInterval(interval);
  progress.value = 100;

  timer = setTimeout(() => {
    visible.value = false;
    progress.value = 0;
  }, 150);
}

router.beforeEach(() => {
  start();
  return true;
});

router.afterEach(() => {
  done();
});

router.onError(() => {
  done();
});

watch(() => ui.globalLoading, (loading) => {
  if (loading) {
    start();
  } else {
    done();
  }
});

onBeforeUnmount(() => {
  if (timer) clearTimeout(timer);
  if (interval) clearInterval(interval);
});
</script>
