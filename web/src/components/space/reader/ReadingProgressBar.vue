<template>
  <ApProgress :value="progress" :label="$t('spaceUi.k64')" size="hair" edge />
</template>

<script setup lang="ts">
defineProps<{
  progress: number;
}>();
</script>
