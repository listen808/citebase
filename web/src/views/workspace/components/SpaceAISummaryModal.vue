<template>
  <ApModal v-model="showModal" :title="$t('ws.k3')" size="medium">
    <div v-if="loading" :style="{ display: 'flex', justifyContent: 'center', padding: AppleSpace.lg }">
      <ApSkeleton variant="text" :lines="3" />
    </div>
    <ApEmpty v-else-if="!summaryText" type="list" :title="$t('ws.k4')" />
    <ApMarkdownPreview
      v-else
      :content="summaryText"
      :style="{ fontSize: AppleType.bodyPrimary.size, lineHeight: AppleLineHeight.relaxed, color: cv('text-primary') }"
    />
    <template #footer>
      <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('ws.close') }}</ApButton>
      <ApButton
        variant="primary"
        size="small"
        :disabled="!summaryText || loading"
        @click="handleCopy"
      >
        {{ $t('ws.copy') }}
      </ApButton>
    </template>
  </ApModal>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed } from 'vue';
import { AppleLineHeight, AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';
import ApModal from '@/components/apple/ApModal.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApMarkdownPreview from '@/components/apple/ApMarkdownPreview.vue';
import { useUIStore } from '@/stores/app/ui';
import { copyText } from '@/utils/clipboard';

interface Props {
  modelValue?: boolean;
  loading?: boolean;
  summaryText?: string;
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: false,
  loading: false,
  summaryText: '',
});

const emit = defineEmits<{
  'update:modelValue': [value: boolean];
}>();
const { t } = useI18n();

const ui = useUIStore();

const showModal = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
});

async function handleCopy() {
  if (!props.summaryText) return;
  const ok = await copyText(props.summaryText);
  ui.showToast(ok ? t('ws.k5') : t('ws.k6'), ok ? 'success' : 'error');
}
</script>
