<template>
  <div>
    <div :style="headerStyle">
      <slot name="icon" />
      <h1 :style="titleStyle" data-ap-page-title>{{ title }}</h1>
      <p v-if="subtitle" :style="subtitleStyle">{{ subtitle }}</p>
    </div>
    <form :style="formStyle" @submit.prevent="handleSubmit">
      <ApInput
        v-model="username"
        :label="$t('auth.username')"
        :placeholder="$t('auth.usernamePlaceholder')"
        autocomplete="username"
        @keyup.enter="handleSubmit"
      />
      <ApInput
        v-model="password"
        :label="$t('auth.password')"
        type="password"
        :placeholder="$t('auth.passwordPlaceholder')"
        autocomplete="current-password"
        @keyup.enter="handleSubmit"
      />
      <slot name="form-extra" />
      <ApButton
        variant="primary"
        size="large"
        :disabled="loading"
        :style="submitBtnStyle"
        @click="handleSubmit"
      >
        {{ loading ? $t('auth.loggingIn') : $t('auth.login') }}
      </ApButton>
      <slot />
    </form>
    <p v-if="error" :style="errorStyle" role="alert">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { cv, AppleType, AppleSpace } from '@/utils/core/apple-tokens';
import ApInput from '@/components/apple/ApInput.vue';
import ApButton from '@/components/apple/ApButton.vue';

withDefaults(defineProps<{
  title: string;
  subtitle?: string;
  loading?: boolean;
  error?: string;
}>(), {
  loading: false,
});

const emit = defineEmits<{
  submit: [username: string, password: string];
}>();

const username = ref('');
const password = ref('');

function handleSubmit() {
  emit('submit', username.value, password.value);
}

function clearPassword() {
  password.value = '';
}

defineExpose({ clearPassword });

const headerStyle = {
  textAlign: 'center' as const,
  marginBottom: AppleSpace.xl,
};

const titleStyle = {
  fontFamily: AppleType.cardProductTitle.font,
  fontSize: AppleType.cardProductTitle.size,
  fontWeight: String(AppleType.cardProductTitle.weight),
  lineHeight: String(AppleType.cardProductTitle.lineHeight),
  color: cv('text-primary'),
  margin: `0 0 ${AppleSpace.sm}`,
};

const subtitleStyle = {
  fontFamily: AppleType.subhead.font,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleType.subhead.weight),
  lineHeight: String(AppleType.subhead.lineHeight),
  color: cv('text-tertiary'),
  margin: '0',
};

const formStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.lg,
  textAlign: 'left' as const,
};

const submitBtnStyle = {
  width: '100%',
  marginTop: AppleSpace.sm,
};

const errorStyle = {
  marginTop: AppleSpace.md,
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('red'),
};
</script>
