<template>
  <ApModal v-model="showRecoverModal" :title="$t('ws.k197')" size="small">
    <p :style="modalTextStyle">
      {{ $t('ws.k188') }}
    </p>
    <template #footer>
      <ApButton variant="ghost" size="small" @click="$emit('discard')">{{ $t('ws.k189') }}</ApButton>
      <ApButton variant="primary" size="small" @click="$emit('recover')">{{ $t('ws.k190') }}</ApButton>
    </template>
  </ApModal>

  <ApModal v-model="showUnsavedModal" :title="$t('ws.tip')" size="small">
    <p :style="modalTextStyle">
      {{ $t('ws.k191') }}
    </p>
    <template #footer>
      <ApButton variant="ghost" size="small" @click="$emit('continueEditing')">{{ $t('ws.k192') }}</ApButton>
      <ApButton variant="secondary" size="small" @click="$emit('confirmCancel')">{{ $t('ws.k193') }}</ApButton>
    </template>
  </ApModal>

  <ApModal v-model="showChangeLogModal" :title="$t('ws.k199')" size="small">
    <div :style="changeLogContainerStyle">
      <ApTextarea
        v-model="changeLogInput"
        :label="$t('ws.k195')"
        :rows="3"
        :style="{ width: '100%' }"
      />
      <ApCheckbox v-model="skipChangeLog" :label="$t('ws.k196')" />
    </div>
    <template #footer>
      <ApButton variant="ghost" size="small" @click="$emit('cancelSave')">{{ $t('ws.cancel') }}</ApButton>
      <ApButton variant="primary" size="small" @click="$emit('confirmSave')">{{ $t('ws.k194') }}</ApButton>
    </template>
  </ApModal>
</template>

<script setup lang="ts">
import { AppleFont, AppleSpace, AppleType, cv, AppleLineHeight } from '@/utils/core/apple-tokens';
import ApModal from '@/components/apple/ApModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import ApCheckbox from '@/components/apple/ApCheckbox.vue';

defineEmits<{
  (e: 'recover'): void;
  (e: 'discard'): void;
  (e: 'continueEditing'): void;
  (e: 'confirmCancel'): void;
  (e: 'confirmSave'): void;
  (e: 'cancelSave'): void;
}>();
const showRecoverModal = defineModel<boolean>('showRecoverModal', { default: false });
const showUnsavedModal = defineModel<boolean>('showUnsavedModal', { default: false });
const showChangeLogModal = defineModel<boolean>('showChangeLogModal', { default: false });
const changeLogInput = defineModel<string>('changeLogInput', { default: '' });
const skipChangeLog = defineModel<boolean>('skipChangeLog');

const modalTextStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.bodyPrimary.size,
  lineHeight: AppleLineHeight.spacious,
  color: cv('text-secondary'),
  margin: '0',
};

const changeLogContainerStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
};
</script>
