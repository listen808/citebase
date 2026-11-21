<template>
  <div :style="settingsFormStyle">
      <ApFormItem :label="$t('user.k27')">
        <ApInput
          v-model="passwordForm.currentPassword"
          type="password"
          :placeholder="$t('user.k30')"
        />
      </ApFormItem>
      <ApFormItem :label="$t('user.k28')">
        <ApInput
          v-model="passwordForm.newPassword"
          type="password"
          :placeholder="$t('user.k31')"
        />
      </ApFormItem>
      <ApFormItem :label="$t('user.k29')">
        <ApInput
          v-model="passwordForm.confirmPassword"
          type="password"
          :placeholder="$t('user.k32')"
          @keyup.enter="handleSavePassword"
        />
        <div v-if="passwordForm.confirmPassword && !passwordsMatch" :style="mismatchStyle">
          {{ $t('user.k25') }}
        </div>
      </ApFormItem>
      <div :style="formActionsStyle">
        <ApButton variant="primary" size="small" :loading="passwordLoading" :disabled="!passwordsMatch || !passwordForm.currentPassword" @click="handleSavePassword">
          {{ $t('user.k26') }}
        </ApButton>
      </div>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed } from 'vue';
import { useUIStore } from '@/stores/app/ui';
import { AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';
import { changeOwnPassword } from '@/api/modules/auth/auth';
import { getApiErrorStatus } from '@/api/client';
import { validators, getValidationMessage } from '@/utils/core/validators';
import { formActionsStyle, settingsFormStyle } from '@/styles/layout/page-styles';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApButton from '@/components/apple/ApButton.vue';

const ui = useUIStore();

const passwordLoading = shallowRef(false);

const passwordForm = ref({
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
});

const passwordsMatch = computed(() => {
  if (!passwordForm.value.confirmPassword) return true;
  return passwordForm.value.newPassword === passwordForm.value.confirmPassword;
});

const mismatchStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('red'),
  marginTop: AppleSpace.xs,
};

async function handleSavePassword() {
  if (!passwordForm.value.currentPassword) {
    ui.showToast(t('user.k30'), 'warning');
    return;
  }
  if (!validators.password(passwordForm.value.newPassword)) {
    ui.showToast(getValidationMessage('password', passwordForm.value.newPassword), 'warning');
    return;
  }
  if (passwordForm.value.newPassword !== passwordForm.value.confirmPassword) {
    ui.showToast(t('user.k33'), 'warning');
    return;
  }
  if (passwordLoading.value) return;
  passwordLoading.value = true;
  try {
    // 自助端点 PUT /auth/me/password：任意登录用户可改自己密码
    // （原 PUT /users/:id/password 走 user:password 管理员权限，普通用户必定 403）
    await changeOwnPassword(
      passwordForm.value.currentPassword,
      passwordForm.value.newPassword
    );
    ui.showToast(t('user.k34'), 'success');
    passwordForm.value = {
      currentPassword: '',
      newPassword: '',
      confirmPassword: '',
    };
  } catch (error: unknown) {
    const err = error as { message?: string };
    // P1-2: 状态码经 ApiError.status 判断（401/400 分支恢复生效）
    const status = getApiErrorStatus(error);
    if (status === 401 || status === 400) {
      ui.showToast(t('user.k35'), 'error');
      passwordForm.value.currentPassword = '';
    } else {
      ui.showToast(err?.message || t('fallback.k47'), 'error');
    }
  } finally {
    passwordLoading.value = false;
  }
}
</script>
