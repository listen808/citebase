<template>
  <div :style="settingsFormStyle">
    <ApFormItem :label="$t('user.k1')">
      <ApInput v-model="profileForm.username" disabled />
    </ApFormItem>
    <ApFormItem :label="$t('user.k2')">
      <ApInput v-model="profileForm.givenName" :placeholder="$t('user.k10')" />
    </ApFormItem>
    <ApFormItem :label="$t('user.k3')">
      <ApInput v-model="profileForm.email" :placeholder="$t('user.k11')" />
    </ApFormItem>
    <ApFormItem :label="$t('user.k4')">
      <ApInput v-model="profileForm.mobile" :placeholder="$t('user.k12')" />
    </ApFormItem>
    <ApFormItem :label="$t('user.k5')">
      <ApInput v-model="profileForm.department" :placeholder="$t('user.k13')" />
    </ApFormItem>
    <ApFormItem :label="$t('user.k6')">
      <ApInput v-model="profileForm.position" :placeholder="$t('user.k14')" />
    </ApFormItem>
    <ApFormItem :label="$t('user.k7')">
      <ApInput v-model="profileForm.location" :placeholder="$t('user.k15')" />
    </ApFormItem>
    <ApFormItem :label="$t('user.k8')">
      <ApInput v-model="profileForm.phone" :placeholder="$t('user.k16')" />
    </ApFormItem>
    <ApFormItem :label="$t('user.k9')">
      <ApInput v-model="profileForm.im" :placeholder="$t('user.k17')" @keyup.enter="handleSaveProfile" />
    </ApFormItem>
    <div :style="formActionsStyle">
      <ApButton variant="primary" size="small" :loading="profileLoading" :disabled="!hasChanges" @click="handleSaveProfile">
        {{ hasChanges ? $t('vUserUi.k1') : $t('vUserUi.k2') }}
      </ApButton>
    </div>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, onMounted, watch, computed } from 'vue';

import { useAuthStore } from '@/stores/auth/auth';
import { useUIStore } from '@/stores/app/ui';
import { formActionsStyle, settingsFormStyle } from '@/styles/layout/page-styles';
import { updateCurrentUser } from '@/api/modules/auth/auth';
import { validators, getValidationMessage } from '@/utils/core/validators';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApButton from '@/components/apple/ApButton.vue';

const auth = useAuthStore();
const ui = useUIStore();

const profileLoading = shallowRef(false);

const profileForm = ref({
  username: '',
  givenName: '',
  email: '',
  mobile: '',
  department: '',
  position: '',
  location: '',
  phone: '',
  im: '',
});

const originalForm = ref({
  givenName: '',
  email: '',
  mobile: '',
  department: '',
  position: '',
  location: '',
  phone: '',
  im: '',
});

const hasChanges = computed(() => {
  return (
    profileForm.value.givenName !== originalForm.value.givenName ||
    profileForm.value.email !== originalForm.value.email ||
    profileForm.value.mobile !== originalForm.value.mobile ||
    profileForm.value.department !== originalForm.value.department ||
    profileForm.value.position !== originalForm.value.position ||
    profileForm.value.location !== originalForm.value.location ||
    profileForm.value.phone !== originalForm.value.phone ||
    profileForm.value.im !== originalForm.value.im
  );
});

async function handleSaveProfile() {
  if (profileLoading.value) return;
  if (!hasChanges.value) {
    ui.showToast(t('user.k18'), 'info');
    return;
  }
  if (profileForm.value.email && !validators.email(profileForm.value.email)) {
    ui.showToast(getValidationMessage('email', profileForm.value.email), 'warning');
    return;
  }
  if (profileForm.value.mobile && !validators.phone(profileForm.value.mobile)) {
    ui.showToast(getValidationMessage('phone', profileForm.value.mobile), 'warning');
    return;
  }
  profileLoading.value = true;
  try {
    // 自助端点 PUT /auth/me：任意登录用户可调用（原 PUT /users/:id 需 user:modify 管理员权限，普通用户必定 403）
    await updateCurrentUser({
      given_name: profileForm.value.givenName,
      email: profileForm.value.email,
      mobile: profileForm.value.mobile,
      department: profileForm.value.department,
      position: profileForm.value.position,
      location: profileForm.value.location,
      phone: profileForm.value.phone,
      im: profileForm.value.im,
    });
    originalForm.value = {
      givenName: profileForm.value.givenName,
      email: profileForm.value.email,
      mobile: profileForm.value.mobile,
      department: profileForm.value.department,
      position: profileForm.value.position,
      location: profileForm.value.location,
      phone: profileForm.value.phone,
      im: profileForm.value.im,
    };
    ui.showToast(t('user.k19'), 'success');
    await auth.fetchUser();
  } catch {
    ui.showToast(t('user.k20'), 'error');
  } finally {
    profileLoading.value = false;
  }
}

function initForm() {
  if (auth.user) {
    const form = {
      username: auth.user.username,
      givenName: auth.user.given_name || '',
      email: auth.user.email || '',
      mobile: auth.user.mobile || '',
      department: auth.user.department || '',
      position: auth.user.position || '',
      location: auth.user.location || '',
      phone: auth.user.phone || '',
      im: auth.user.im || '',
    };
    profileForm.value = form;
    originalForm.value = {
      givenName: form.givenName,
      email: form.email,
      mobile: form.mobile,
      department: form.department,
      position: form.position,
      location: form.location,
      phone: form.phone,
      im: form.im,
    };
  }
}

onMounted(initForm);

watch(() => auth.user, initForm);
</script>
