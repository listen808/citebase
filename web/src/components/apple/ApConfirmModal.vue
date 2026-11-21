<template>
  <ApModal
    :model-value="modelValue"
    size="small"
    :density="density"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <template #title>
      {{ title }}
    </template>
    
    <div :style="contentStyle">
      <div :style="iconContainerStyle">
        <component :is="iconComponent" :style="iconStyle" />
      </div>
      <p :style="messageStyle">{{ message }}</p>

      <div v-if="requireInputConfirm" :style="confirmInputStyle">
        <ApInput
          :model-value="confirmInput"
          :placeholder="confirmPlaceholder"
          @update:model-value="confirmInput = $event"
          @keydown.enter="handleConfirmEnter"
        />
      </div>
    </div>

    <template #footer>
      <ApButton variant="ghost" size="small" @click="handleCancel">{{ cancelText || $t('common.cancel') }}</ApButton>
      <ApButton
        variant="danger"
        size="small"
        :disabled="requireInputConfirm && confirmInput !== matchText"
        @click="handleConfirm"
      >
        {{ confirmText }}
      </ApButton>
    </template>
  </ApModal>
</template>

<script setup lang="ts">
import {computed, shallowRef, watch} from 'vue';
import { AlertTriangle, Trash2 } from 'lucide-vue-next';
import { AppleLayout, AppleFont, AppleSpace, AppleType, AppleRadius, cv } from '@/utils/core/apple-tokens';
import type { AppleDensity } from '@/utils/core/apple-tokens';
import ApButton from '@/components/apple/ApButton.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApInput from '@/components/apple/ApInput.vue';

const props = withDefaults(defineProps<{
  modelValue: boolean;
  title?: string;
  message?: string;
  type?: 'warning' | 'danger';
  requireInputConfirm?: boolean;
  confirmText?: string;
  cancelText?: string;
  confirmPlaceholder?: string;
  confirmMatchText?: string;
  density?: AppleDensity;
}>(), {
  cancelText: '',
  density: 'default',
});

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void;
  (e: 'confirm'): void;
}>();

const confirmInput = shallowRef('');

watch(() => props.modelValue, (val) => {
  if (!val) confirmInput.value = '';
});

const iconComponent = computed(() => {
  return props.type === 'danger' ? Trash2 : AlertTriangle;
});

const matchText = computed(() => props.confirmMatchText || props.confirmText || '');

function handleConfirm() {
  if (props.requireInputConfirm && confirmInput.value !== matchText.value) return;
  emit('confirm');
  emit('update:modelValue', false);
}

// IME 守卫：敲确认词是中文时，选字回车不该触发确认
function handleConfirmEnter(e: KeyboardEvent) {
  if (e.isComposing || e.keyCode === 229) return;
  handleConfirm();
}

function handleCancel() {
  emit('update:modelValue', false);
}

const contentStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  alignItems: 'center',
  gap: AppleSpace.md,
  padding: `${AppleSpace.md} ${AppleSpace.xl}`,
};

const iconContainerStyle = computed(() => ({
  width: AppleLayout.iconBoxLarge,
  height: AppleLayout.iconBoxLarge,
  borderRadius: AppleRadius.circle,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  background: props.type === 'danger' ? cv('red-light') : cv('orange-light'),
}));

const iconStyle = computed(() => ({
  color: props.type === 'danger' ? cv('red') : cv('orange'),
}));

const messageStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.bodyPrimary.size,
  lineHeight: String(AppleType.bodyPrimary.lineHeight),
  color: cv('text-primary'),
  textAlign: 'center' as const,
  margin: 0,
};

const confirmInputStyle = {
  width: '100%',
  marginTop: AppleSpace.sm,
};
</script>
