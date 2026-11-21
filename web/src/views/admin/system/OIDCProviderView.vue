<template>
  <ApPageContainer
    :title="$t('adminSys.k173')"
    :subtitle="$t('adminSys.k175')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <div :style="crudStatsRowStyle('compact')">
      <ApStatCard size="compact" :label="$t('adminSys.k156')" :value="providers.length" :icon="'Globe'" :color="cv('blue')" />
      <!-- 已启用 是"这一页的提供方"里开着的那部分，同一功能只是计数 → 用功能符号 Globe。
           自动创建 / 失败 保留各自符号：它们不是"该功能数了一遍"，而是"需处置的运维信号"，
           且 UserPlus / AlertTriangle 不是任何功能的身份符号，借走不产生撞字。 -->
      <ApStatCard size="compact" :label="$t('adminSys.k114')" :value="enabledCount" :icon="'Globe'" :color="cv('green')" />
      <ApStatCard size="compact" :label="$t('adminSys.k157')" :value="autoCreateCount" :icon="'UserPlus'" :color="cv('purple')" />
      <ApStatCard size="compact" :label="$t('adminSys.k158')" :value="failedProviders.length" :icon="'AlertTriangle'" :color="cv('orange')" />
    </div>

    <ApError
      v-if="loadError"
      :message="loadError"
      show-retry
      @retry="loadProviders"
    />

    <!-- 创建/更新时后端把初始化失败只记成 warning 并照样返回成功，
         失败原因只能从 init_error 读，所以这块必须显式存在，否则管理员会以为配好了 -->
    <ApCard v-if="failedProviders.length" size="small" :style="failureCardStyle">
      <template #header>
        <div :style="failureHeaderStyle">
          <ApIcon name="AlertTriangle" :size="16" :color="cv('orange')" />
          <span :style="sectionTitleStyle">{{ $t('adminSys.k153') }}</span>
        </div>
      </template>
      <ul :style="failureListStyle">
        <li v-for="p in failedProviders" :key="'fail-' + p.id" :style="failureItemStyle">
          <strong :style="failureNameStyle">{{ p.display_name || p.name }}</strong>
          <span :style="failureReasonStyle">{{ p.init_error }}</span>
        </li>
      </ul>
    </ApCard>

    <AdminCrudPage
      v-if="!loadError"
      :stats="[]"
      :loading="loading"
      :filters="{ search: { placeholder: $t('vAdminUi.k29') } }"
      @search="keyword = $event"
    >
      <template #actions>
        <ApButton variant="primary" size="small" @click="openCreate">
          <ApIcon name="Plus" :size="16" /> {{ $t('adminSys.k154') }}
        </ApButton>
      </template>

      <template #content>
        <ApTable
          density="compact"
          :columns="columns"
          :data="filtered"
          :loading="loading"
          :empty-text="$t('adminSys.k177')"
          :empty-icon="'Globe'"
          :aria-label="$t('adminSys.k176')"
          hide-header-when-empty
        >
          <template #actions="{ row }">
            <ApButton variant="ghost" size="small" @click="openEdit(row as unknown as OIDCProviderConfig)">
              {{ $t('adminSys.edit') }}
            </ApButton>
            <ApButton
              variant="secondary"
              size="small"
              :loading="busyId === (row as unknown as OIDCProviderConfig).id"
              @click="toggleEnabled(row as unknown as OIDCProviderConfig)"
            >
              {{ (row as unknown as OIDCProviderConfig).enabled ? $t('vAdminUi.k27') : $t('vAdminUi.enable') }}
            </ApButton>
            <ApButton
              variant="danger"
              size="small"
              @click="askDelete(row as unknown as OIDCProviderConfig)"
            >
              <ApIcon name="Trash2" :size="16" /> {{ $t('adminSys.delete') }}
            </ApButton>
          </template>
        </ApTable>
      </template>
    </AdminCrudPage>

    <ApModal v-model="showModal" :title="editingId ? $t('adminForm.k1') : $t('adminForm.k2')" size="medium">
      <div :style="formStyle">
        <div :style="formRowStyle">
          <ApFormItem :label="$t('adminSys.k159')" required :error="errors.name" style="flex: 1">
            <ApInput v-model="form.name" :placeholder="$t('adminSys.k167')" :maxlength="50" />
          </ApFormItem>
          <ApFormItem :label="$t('adminSys.k160')" style="flex: 1">
            <ApInput v-model="form.display_name" :placeholder="$t('adminSys.k168')" />
          </ApFormItem>
        </div>

        <div :style="formRowStyle">
          <ApFormItem :label="$t('adminSys.type')" required style="flex: 1">
            <ApSelect v-model="form.type" :options="typeOptions" />
          </ApFormItem>
          <ApFormItem :label="$t('adminSys.k162')" style="flex: 1">
            <ApInput v-model="form.sort_order" type="number" :placeholder="$t('adminSys.k169')" />
          </ApFormItem>
        </div>

        <div :style="formRowStyle">
          <ApFormItem label="Client ID" required style="flex: 1">
            <ApInput v-model="form.client_id" :placeholder="$t('adminSys.k170')" />
          </ApFormItem>
          <ApFormItem
            label="Client Secret"
            :required="!editingId"
            :hint="$t('vAdminUi.k24')"
            :error="errors.client_secret"
            style="flex: 1"
          >
            <ApInput v-model="form.client_secret" type="password" :placeholder="editingId ? $t('adminForm.k22') : $t('adminForm.k23')" />
          </ApFormItem>
        </div>

        <ApFormItem
          label="Issuer URL"
          :hint="$t('vAdminUi.k25')"
        >
          <ApInput v-model="form.issuer_url" placeholder="https://idp.example.com/realms/main" />
        </ApFormItem>

        <div :style="formRowStyle">
          <ApFormItem label="Authorization URL" :error="errors.endpoint" style="flex: 1">
            <ApInput v-model="form.auth_url" placeholder="https://idp.example.com/authorize" />
          </ApFormItem>
          <ApFormItem label="Token URL" style="flex: 1">
            <ApInput v-model="form.token_url" placeholder="https://idp.example.com/token" />
          </ApFormItem>
        </div>

        <ApFormItem label="UserInfo URL">
          <ApInput v-model="form.user_info_url" :placeholder="$t('adminSys.k171')" />
        </ApFormItem>

        <div :style="formRowStyle">
          <ApFormItem label="Scopes" :hint="$t('vAdminUi.k26')" style="flex: 1">
            <ApInput v-model="form.scopes" placeholder="openid,profile,email" />
          </ApFormItem>
          <ApFormItem :label="$t('adminSys.k163')" style="flex: 1">
            <ApSelect v-model="form.default_role" :options="roleOptions" />
          </ApFormItem>
        </div>

        <ApFormItem :label="$t('adminSys.k164')">
          <ApInput v-model="form.icon_url" :placeholder="$t('adminSys.k172')" />
        </ApFormItem>

        <div :style="switchRowStyle">
          <ApSwitch v-model="form.enabled" :label="$t('adminSys.k165')" />
          <ApSwitch v-model="form.auto_create" :label="$t('adminSys.k166')" />
        </div>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('adminSys.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="saving" @click="handleSave">{{ $t('adminSys.save') }}</ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('adminSys.k174')"
      :message="$t('adminForm.k24', { p1: deletingItem?.display_name || deletingItem?.name || '' })"
      type="danger"
      :confirm-text="$t('adminSys.delete')"
      @confirm="confirmDelete"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { computed, onMounted, ref } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';

import { logger } from '@/utils/logger';
import { AppleFontWeight, AppleLayout, AppleSpace, cv } from '@/utils/core/apple-tokens';
import { crudStatsRowStyle, formRowStyle, formStyle, sectionTitleStyle } from '@/styles/layout/page-styles';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApError from '@/components/apple/ApError.vue';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import ApSwitch from '@/components/apple/ApSwitch.vue';
import ApTable from '@/components/apple/ApTable.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApInput from '@/components/apple/ApInput.vue';
import AdminCrudPage from '@/components/admin/AdminCrudPage.vue';
import {
  createOIDCProvider,
  deleteOIDCProvider,
  getOIDCProviderAdmin,
  updateOIDCProvider,
  type OIDCProviderConfig,
} from '@/api/modules/auth/sso';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();

const providers = ref<OIDCProviderConfig[]>([]);
const loading = ref(false);
const saving = ref(false);
const busyId = ref<number | null>(null);
const loadError = ref<string | null>(null);
const keyword = ref('');

const showModal = ref(false);
const showDeleteModal = ref(false);
const editingId = ref<number | null>(null);
const deletingItem = ref<OIDCProviderConfig | null>(null);

const typeOptions = [
  { label: t('adminSys.k178'), value: 'oidc' },
  { label: 'Google', value: 'google' },
  { label: 'GitHub', value: 'github' },
  { label: 'Azure AD', value: 'azure' },
  { label: 'Keycloak', value: 'keycloak' },
];

// 后端 resolveRoleID 只认 admin/editor，其余一律落 viewer，所以这里不给自由输入
const roleOptions = [
  { label: t('adminSys.k179'), value: 'viewer' },
  { label: t('adminSys.k180'), value: 'editor' },
  { label: t('adminSys.k181'), value: 'admin' },
];

const emptyForm = () => ({
  name: '',
  display_name: '',
  type: 'oidc',
  client_id: '',
  client_secret: '',
  issuer_url: '',
  auth_url: '',
  token_url: '',
  user_info_url: '',
  scopes: '',
  default_role: 'viewer',
  icon_url: '',
  sort_order: '0',
  enabled: true,
  auto_create: true,
});

const form = ref(emptyForm());
const errors = ref<{ name?: string; client_secret?: string; endpoint?: string }>({});

const enabledCount = computed(() => providers.value.filter((p) => p.enabled).length);
const autoCreateCount = computed(() => providers.value.filter((p) => p.auto_create).length);
const failedProviders = computed(() => providers.value.filter((p) => p.enabled && p.init_error));

const filtered = computed(() => {
  const q = keyword.value.trim().toLowerCase();
  if (!q) return providers.value;
  return providers.value.filter(
    (p) => p.name.toLowerCase().includes(q) || (p.display_name || '').toLowerCase().includes(q) || p.client_id.toLowerCase().includes(q),
  );
});

const columns: TableColumn<OIDCProviderConfig>[] = [
  { key: 'display_name', title: t('adminSys.k160'), ellipsis: true, render: (row) => row.display_name || row.name },
  { key: 'name', title: t('adminSys.k159'), width: '20%', ellipsis: true },
  { key: 'type', title: t('adminSys.type'), width: '10%' },
  { key: 'client_id', title: 'Client ID', width: '20%', ellipsis: true },
  { key: 'issuer_url', title: 'Issuer', ellipsis: true, render: (row) => row.issuer_url || `${row.auth_url ? t('adminForm.k25') : '—'}` },
  {
    key: 'enabled',
    title: t('adminSys.status'),
    width: '120px',
    render: (row) => (row.enabled ? (row.init_error ? t('adminForm.k26') : t('adminForm.k34')) : t('adminForm.k35')),
  },
  { key: 'default_role', title: t('adminSys.k182'), width: '110px' },
  { key: 'sort_order', title: t('adminSys.sort'), width: '70px', align: 'right' },
];

async function loadProviders() {
  loading.value = true;
  loadError.value = null;
  try {
    providers.value = (await getOIDCProviderAdmin()) ?? [];
  } catch (e) {
    logger.error('Failed to load OIDC providers:', e);
    loadError.value = t('sentUi.k22');
    providers.value = [];
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingId.value = null;
  form.value = emptyForm();
  errors.value = {};
  showModal.value = true;
}

function openEdit(p: OIDCProviderConfig) {
  editingId.value = p.id;
  form.value = {
    name: p.name,
    display_name: p.display_name || '',
    type: p.type || 'oidc',
    client_id: p.client_id,
    // 后端不回显 secret，留空即"保持原值"（UpdateProvider 只在非空时覆盖）
    client_secret: '',
    issuer_url: p.issuer_url || '',
    auth_url: p.auth_url || '',
    token_url: p.token_url || '',
    user_info_url: p.user_info_url || '',
    scopes: p.scopes || '',
    default_role: p.default_role || 'viewer',
    icon_url: p.icon_url || '',
    sort_order: String(p.sort_order ?? 0),
    enabled: p.enabled,
    auto_create: p.auto_create,
  };
  errors.value = {};
  showModal.value = true;
}

function validate(): boolean {
  const f = form.value;
  const next: typeof errors.value = {};
  if (!f.name.trim()) next.name = t('adminForm.k27');
  if (!editingId.value && !f.client_secret) next.client_secret = t('adminForm.k28');
  // 与后端 initProvider 的判据一致：issuer_url 走发现，否则 auth_url + token_url 必填
  if (!f.issuer_url.trim() && (!f.auth_url.trim() || !f.token_url.trim())) {
    next.endpoint = t('adminForm.k29');
  }
  errors.value = next;
  return Object.keys(next).length === 0;
}

function buildPayload() {
  const f = form.value;
  return {
    name: f.name.trim(),
    display_name: f.display_name.trim(),
    type: f.type as 'oidc' | 'google' | 'github' | 'azure' | 'keycloak',
    client_id: f.client_id.trim(),
    client_secret: f.client_secret,
    issuer_url: f.issuer_url.trim(),
    auth_url: f.auth_url.trim(),
    token_url: f.token_url.trim(),
    user_info_url: f.user_info_url.trim(),
    scopes: f.scopes.trim(),
    enabled: f.enabled,
    auto_create: f.auto_create,
    default_role: f.default_role,
    icon_url: f.icon_url.trim(),
    sort_order: Number(f.sort_order) || 0,
  };
}

async function handleSave() {
  if (!validate()) return;
  saving.value = true;
  const payload = buildPayload();
  try {
    if (editingId.value) {
      const res = await updateOIDCProvider(editingId.value, payload);
      reportInitOutcome(res?.init_error, t('adminForm.k30'));
    } else {
      const res = await createOIDCProvider(payload);
      reportInitOutcome(res?.init_error, t('adminForm.k31'));
    }
    showModal.value = false;
    await loadProviders();
  } catch (e) {
    logger.error('Failed to save OIDC provider:', e);
    ui.showToast(t('adminSys.k184'), 'error');
  } finally {
    saving.value = false;
  }
}

// 保存请求成功不等于能用：初始化失败时后端仍返回 2xx，所以要把 init_error 说出来
function reportInitOutcome(initError?: string, verb = t('adminForm.k32')) {
  if (initError) {
    logger.warn('[OIDC] provider saved but initialization failed:', initError);
    ui.showToast(t('sentUi.k23', { p1: verb, p2: initError }), 'warning');
    return;
  }
  ui.showToast(`${verb}${form.value.enabled ? '' : t('adminForm.k33')}`, 'success');
}

async function toggleEnabled(p: OIDCProviderConfig) {
  busyId.value = p.id;
  try {
    const res = await updateOIDCProvider(p.id, {
      name: p.name,
      display_name: p.display_name,
      type: p.type as 'oidc' | 'google' | 'github' | 'azure' | 'keycloak',
      client_id: p.client_id,
      // 不改动 secret：留空即后端"保持原值"
      issuer_url: p.issuer_url,
      auth_url: p.auth_url,
      token_url: p.token_url,
      user_info_url: p.user_info_url,
      scopes: p.scopes,
      enabled: !p.enabled,
      auto_create: p.auto_create,
      default_role: p.default_role,
      icon_url: p.icon_url,
      sort_order: p.sort_order,
    });
    reportInitOutcome(res?.init_error, p.enabled ? t('adminForm.k35') : t('adminForm.k34'));
    await loadProviders();
  } catch (e) {
    logger.error('Failed to toggle OIDC provider:', e);
    ui.showToast(t('adminSys.k185'), 'error');
  } finally {
    busyId.value = null;
  }
}

function askDelete(p: OIDCProviderConfig) {
  deletingItem.value = p;
  showDeleteModal.value = true;
}

async function confirmDelete() {
  const target = deletingItem.value;
  if (!target?.id) return;
  try {
    await deleteOIDCProvider(target.id);
    ui.showToast(t('adminSys.k186'), 'success');
    showDeleteModal.value = false;
    deletingItem.value = null;
    await loadProviders();
  } catch (e) {
    logger.error('Failed to delete OIDC provider:', e);
    ui.showToast(t('adminSys.k187'), 'error');
  }
}

onMounted(loadProviders);

const failureCardStyle = { marginBottom: AppleSpace.md };

const failureHeaderStyle = {
  display: 'flex',
  alignItems: 'center' as const,
  gap: AppleSpace.xs,
};

const failureListStyle = {
  margin: '0',
  paddingLeft: AppleSpace.lg,
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
};

const failureItemStyle = {
  display: 'flex',
  flexWrap: 'wrap' as const,
  gap: AppleSpace.xs,
};

const failureNameStyle = { fontWeight: String(AppleFontWeight.semibold) };

const failureReasonStyle = { wordBreak: 'break-all' as const };

const switchRowStyle = {
  display: 'flex',
  gap: AppleSpace.xl,
  flexWrap: 'wrap' as const,
};
</script>
