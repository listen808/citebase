<template>
  <ApPageContainer
    :title="$t('user.k21')"
    :subtitle="$t('user.k62')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <template #actions>
      <ApStatusBadge
        :status="auth.isSuperAdmin ? 'online' : auth.isAdmin ? 'enabled' : 'default'"
        size="small"
        :label="getRoleName(auth.user?.role_id)"
      />
    </template>

    <PageContentCard
      v-model:active-tab="activeTab"
      :tabs="tabItems"
      :wrap-with-card="true"
    >
      <ProfileForm v-if="activeTab === 'profile'" />
      <PasswordSection v-if="activeTab === 'password'" />
      <PreferenceSection v-if="activeTab === 'preferences'" />
    </PageContentCard>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { shallowRef } from 'vue';
import { useAuthStore } from '@/stores/auth/auth';
import { AppleLayout } from '@/utils/core/apple-tokens';
import { getRoleName } from '@/utils/core/constants';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue';
import PageContentCard from '@/components/layout/PageContentCard.vue';
import ProfileForm from './ProfileForm.vue';
import PasswordSection from './sections/PasswordSection.vue';
import PreferenceSection from './sections/PreferenceSection.vue';

const auth = useAuthStore();
const activeTab = shallowRef('profile');

const tabItems = [
  { key: 'profile', label: t('user.k22') },
  { key: 'password', label: t('user.k23') },
  { key: 'preferences', label: t('user.k24') },
];
</script>




