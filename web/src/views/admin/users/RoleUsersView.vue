<template>
  <ApPageContainer
    :title="pageTitle"
    :subtitle="$t('uiText.k47', { p1: total })"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    :breadcrumb-parent="{ label: $t('vAdminUi.k44'), path: '/roles' }"
    :show-back="true"
    back-to="/roles"
    compact
  >
    <AdminCrudPage
      :stats="[]"
      :loading="loading"
      :total="total"
      :page="page"
      :page-size="pageSize"
      :show-empty="!loading && !users.length"
      empty-icon="User"
      :empty-text="$t('adminSys.k332')"
      @page-change="changePage"
    >
      <template #filter-extra>
        <span :style="countBadgeStyle">{{ total }} {{ $t('vAdminUi.k43') }}</span>
      </template>
      <template #actions>
        <ApButton variant="primary" size="small" @click="showAddModal = true">
          <ApIcon name="UserPlus" :size="16" />{{ $t('adminSys.k325') }}
        </ApButton>
      </template>

      <template #content>
        <div :style="appleCardGrid('entity')">
          <ApCard
            v-for="user in users"
            :key="user.id"
            hoverable
            size="small"
            @mouseenter="hoveredUserId = user.id"
            @mouseleave="hoveredUserId = 0"
            @touchstart="hoveredUserId = user.id"
          >
            <div :style="userCardRowStyle">
              <ApAvatar :text="avatarLetter(user)" size="small" :aria-label="user.given_name || user.username" />
              <div :style="userInfoStyle">
                <span :style="userNameStyle">{{ user.given_name || user.username }}</span>
                <span v-if="user.given_name" :style="userGivenNameStyle">@{{ user.username }}</span>
              </div>
              <div v-if="user.id !== 1" :style="cardActionsStyle">
                <ApButton
                  v-if="hoveredUserId === user.id"
                  variant="ghost"
                  size="small"
                  @click="openResetModal()"
                >
                  <ApIcon name="RotateCcw" :size="16" />{{ $t('adminSys.reset') }}
                </ApButton>
              </div>
              <div v-else :style="cardActionsStyle">
                <span :style="adminBadgeStyle">{{ $t('adminSys.builtinAccount') }}</span>
              </div>
            </div>
          </ApCard>
        </div>
      </template>

      <template #modals>
        <ApConfirmModal
          v-model="showResetModal"
          :title="$t('adminSys.k330')"
          :message="resetMessage"
          :confirm-text="$t('adminSys.k331')"
          :cancel-text="$t('adminSys.cancel')"
          type="danger"
          @confirm="handleResetRole"
        />

        <ApModal
          :model-value="showAddModal"
          :title="$t('adminSys.k325')"
          size="medium"
          density="compact"
          @update:model-value="(v: boolean) => { if (!v) showAddModal = false }"
        >
          <ApFormItem :label="$t('adminSys.k73')">
            <ApInput
              :model-value="addUsername"
              :placeholder="$t('adminSys.k329')"
              @update:model-value="(val: string) => addUsername = val"
              @keyup.enter="handleAddUser"
            />
          </ApFormItem>
          <template #footer>
            <ApButton variant="ghost" size="small" :disabled="adding" @click="showAddModal = false">{{ $t('adminSys.cancel') }}</ApButton>
            <ApButton variant="primary" size="small" :loading="adding" @click="handleAddUser">{{ $t('adminSys.add') }}</ApButton>
          </template>
        </ApModal>
      </template>
    </AdminCrudPage>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, watch } from 'vue';
import { useRoute } from 'vue-router';
import ApIcon from '@/components/apple/ApIcon.vue';
import { cv, AppleLayout, AppleRadius, AppleFont, AppleFontWeight, AppleSpace, AppleType, AppleInputSize, appleCardGrid } from '@/utils/core/apple-tokens';
import { getRoleUsers, resetRoleUsers, addUserToRole, type RoleUser } from '@/api/modules/user/role';
import { useUIStore } from '@/stores/app/ui';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApAvatar from '@/components/apple/ApAvatar.vue';
import ApCard from '@/components/apple/ApCard.vue';
import AdminCrudPage from '@/components/admin/AdminCrudPage.vue';

const ui = useUIStore();
const route = useRoute();
const roleId = computed(() => Number(route.params.roleId));

const users = ref<RoleUser[]>([]);
const total = shallowRef(0);
const page = shallowRef(1);
const pageSize = shallowRef(20);
const loading = shallowRef(false);
const roleName = shallowRef('');
const hoveredUserId = shallowRef(0);

const showAddModal = shallowRef(false);
const addUsername = ref('');
const adding = shallowRef(false);

const pageTitle = computed(() => {
  return roleName.value ? t('uiText.k48', { p1: roleName.value }) : t('uiText.k49');
});

const showResetModal = shallowRef(false);
const resetting = shallowRef(false);

const resetMessage = computed(() => {
  return t('uiText.k50', { p1: roleName.value });
});

const countBadgeStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-secondary'),
  background: cv('bg-tertiary'),
  padding: `${AppleSpace.xxs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.medium,
};

const userCardRowStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  width: '100%',
  minWidth: '0',
};

const userInfoStyle = {
  flex: '1',
  minWidth: '0',
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
};

const userNameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const userGivenNameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const cardActionsStyle = {
  flexShrink: '0',
  display: 'flex',
  alignItems: 'center',
  minHeight: AppleInputSize.small.height,
};

const adminBadgeStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('orange'),
  background: cv('orange-bg-subtle'),
  padding: `${AppleSpace.xxs} ${AppleSpace.xs}`,
  borderRadius: AppleRadius.small,
};

function avatarLetter(user: RoleUser): string {
  const name = user.given_name || user.username;
  return name.charAt(0).toUpperCase();
}

function changePage(p: number) {
  page.value = p;
  loadUsers();
}

async function loadUsers() {
  if (!roleId.value) return;
  loading.value = true;
  try {
    const res = await getRoleUsers(roleId.value, page.value, pageSize.value);
    users.value = res.data;
    total.value = res.total;
    page.value = res.page;
    pageSize.value = res.page_size;
    roleName.value = res.role_name || '';
  } catch {
    users.value = [];
    ui.showToast(t('adminSys.k333'), 'error');
  } finally {
    loading.value = false;
  }
}

function openResetModal() {
  showResetModal.value = true;
}

async function handleResetRole() {
  if (resetting.value || !roleId.value) return;
  resetting.value = true;
  try {
    await resetRoleUsers(roleId.value);
    ui.showToast(t('sentUi.k28', { p1: roleName.value }), 'success');
    showResetModal.value = false;
    await loadUsers();
  } catch (e: unknown) {
    ui.showToast((e as Error).message || t('fallback.k108'), 'error');
  } finally {
    resetting.value = false;
  }
}

async function handleAddUser() {
  if (adding.value || !roleId.value) return;
  const username = addUsername.value.trim();
  if (!username) {
    ui.showToast(t('adminSys.k334'), 'warning');
    return;
  }
  adding.value = true;
  try {
    await addUserToRole(roleId.value, username);
    ui.showToast(t('sentUi.k29', { p1: username, p2: roleName.value }), 'success');
    showAddModal.value = false;
    addUsername.value = '';
    page.value = 1;
    await loadUsers();
  } catch (e: unknown) {
    ui.showToast((e as Error).message || t('fallback.k88'), 'error');
  } finally {
    adding.value = false;
  }
}

watch(roleId, () => {
  page.value = 1;
  loadUsers();
}, { immediate: true });
</script>
