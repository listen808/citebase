<template>
  <ApPageContainer
    :title="$t('adminSys.k19')"
    :subtitle="$t('adminSys.k23')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <!-- 页头操作位与备份/恢复同档：ghost 刷新在前、primary 主操作在后 -->
    <template #actions>
      <ApButton variant="ghost" size="small" :loading="loading" @click="loadKeys">
        <ApIcon name="RefreshCw" :size="16" />{{ $t('adminSys.refresh') }}
      </ApButton>
      <ApButton variant="primary" size="small" @click="showCreateModal = true">
        <ApIcon name="Plus" :size="16" />{{ $t('adminSys.k2') }}
      </ApButton>
    </template>

    <div :style="PageLayout.sectionGap">
      <ApCard size="small">
        <template #header>
          <div :style="statusRowStyle">
            <ApIcon name="KeyRound" :size="18" :color="cv('blue')" />
            <span :style="sectionTitleStyle">{{ $t('adminSys.k1') }}</span>
          </div>
        </template>

        <div :style="toolbarStyle">
          <span :style="metaItemStyle">{{ $t('common.totalCount', { n: total }) }}</span>
        </div>

        <!-- 长说明收进折叠区：页头已有「新建密钥」，说明不该与主操作抢同一屏注意力（P1-9） -->
        <details :style="noteDetailsStyle">
          <summary :style="noteSummaryStyle">{{ $t('adminSys.k396') }}</summary>
          <p :style="descStyle">{{ $t('adminSys.k4') }}</p>
        </details>

        <ApTable
          density="compact"
          row-key="id"
          :columns="columns"
          :data="keys"
          :loading="loading"
          hide-header-when-empty
        >
          <template #empty>
            <ApEmpty :icon="'Key'" :title="$t('adminSys.k394')" :description="$t('adminSys.k395')" />
          </template>
          <template #actions="{ row }">
            <!-- 置灰本身不解释原因：非 active（已吊销/已过期）的密钥按下去没反应，
                 看起来像按钮坏了。禁用态把原因写在 tooltip 上（同 UserManageView 的用法）。 -->
            <ApButton
              variant="danger"
              size="small"
              :disabled="(row as unknown as ApiKeyRecord).status !== 'active'"
              :tooltip="(row as unknown as ApiKeyRecord).status !== 'active' ? $t('uiText.k34') : ''"
              @click="handleRevoke(row as unknown as ApiKeyRecord)"
            >
              {{ $t('adminSys.k5') }}
            </ApButton>
          </template>
        </ApTable>
      </ApCard>
    </div>

    <!-- ==================== 新建密钥 ==================== -->
    <ApModal v-model="showCreateModal" :title="$t('adminSys.k20')" size="medium">
      <div :style="formStyle">
        <ApInput v-model="appName" :label="$t('adminSys.k14')" :placeholder="$t('adminSys.k17')" required />
        <div :style="layoutContentGridTwoColumn">
          <ApInput v-model="rateLimit" type="number" :label="$t('adminSys.k15')" placeholder="60" />
          <ApInput v-model="expiresInDays" type="number" :label="$t('adminSys.k16')" :placeholder="$t('adminSys.k18')" />
        </div>
        <p :style="descStyle">{{ $t('adminSys.k6') }}</p>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showCreateModal = false">{{ $t('adminSys.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="creating" @click="handleCreate">
          {{ $t('adminSys.k8') }}
        </ApButton>
      </template>
    </ApModal>

    <!-- ==================== 明文密钥（仅一次） ==================== -->
    <ApModal v-model="showSecretModal" :title="$t('adminSys.k21')" size="medium" :close-on-overlay="false">
      <div :style="formStyle">
        <p :style="statusIndicatorLargeStyle('warning')">
          {{ $t('adminSys.k9') }}
        </p>
        <div :style="userCardRowStyle">
          <span :style="crudCodeStyle()">{{ createdKey?.api_key }}</span>
          <ApButton variant="outline" size="small" @click="handleCopy">
            <ApIcon name="Copy" :size="16" />{{ $t('adminSys.copy') }}
          </ApButton>
        </div>
        <p :style="metaItemStyle">
          {{ $t('uiText.k39', { p1: createdKey?.app_name, p2: createdKey?.rate_limit, p3: formatTime(createdKey?.expires_at) }) }}
        </p>
        <p :style="descStyle">
          {{ $t('adminSys.k11') }} <span :style="crudCodeStyle()">X-API-Key</span> {{ $t('adminSys.k12') }}
        </p>
      </div>
      <template #footer>
        <ApButton variant="primary" size="small" @click="closeSecretModal">{{ $t('adminSys.k13') }}</ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showRevokeModal"
      :title="$t('adminSys.k22')"
      :message="$t('uiText.k41', { p1: revokingItem?.app_name, p2: revokingItem?.id })"
      type="danger"
      :confirm-text="$t('adminSys.k5')"
      :loading="revoking"
      @confirm="confirmRevoke"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { computed, onMounted, ref, shallowRef } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';

import { logger } from '@/utils/logger';
import { AppleLayout, AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';
import {
  crudCodeStyle,
  descStyle,
  formStyle,
  layoutContentGridTwoColumn,
  metaItemStyle,
  PageLayout,
  sectionTitleStyle,
  statusIndicatorLargeStyle,
  statusRowStyle,
  toolbarStyle,
  userCardRowStyle,
} from '@/styles/layout/page-styles';
import {
  createApiKey,
  listApiKeys,
  revokeApiKey,
  type ApiKeyRecord,
  type CreateApiKeyResponse,
} from '@/api/modules/system/api-key';
import { copyText } from '@/utils/clipboard';
import { formatTime } from '@/utils/format/format-time';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();

const keys = ref<ApiKeyRecord[]>([]);
const total = ref(0);
const loading = shallowRef(false);
const creating = shallowRef(false);
const revoking = shallowRef(false);

const showCreateModal = ref(false);
const showSecretModal = ref(false);
const showRevokeModal = ref(false);
const createdKey = ref<CreateApiKeyResponse | null>(null);
const revokingItem = ref<ApiKeyRecord | null>(null);

const appName = ref('');
const rateLimit = ref('');
const expiresInDays = ref('');

const statusLabel = (status: string) => (status === 'active' ? t('uiText.k35') : status === 'revoked' ? t('uiText.k36') : status);

const columns = computed(() => [
  { key: 'id', title: 'ID' },
  { key: 'app_name', title: t('adminSys.k14'), ellipsis: true },
  { key: 'status', title: t('adminSys.status'), render: (row: ApiKeyRecord) => statusLabel(row.status) },
  {
    key: 'rate_limit',
    title: t('adminSys.k25'),
    align: 'right' as const,
    render: (row: ApiKeyRecord) => t('uiText.k40', { p1: row.rate_limit }),
  },
  { key: 'expires_at', title: t('adminSys.k26'), render: (row: ApiKeyRecord) => formatTime(row.expires_at) },
  { key: 'last_used_at', title: t('adminSys.k27'), render: (row: ApiKeyRecord) => formatTime(row.last_used_at) },
  { key: 'created_at', title: t('adminSys.k28'), render: (row: ApiKeyRecord) => formatTime(row.created_at) },
]);

async function loadKeys() {
  loading.value = true;
  try {
    const result = await listApiKeys();
    keys.value = result?.api_keys ?? [];
    total.value = result?.total ?? keys.value.length;
  } catch (e) {
    logger.error('Failed to load api keys:', e);
    ui.showToast((e as Error).message || t('fallback.k29'), 'error');
  } finally {
    loading.value = false;
  }
}

async function handleCreate() {
  if (!appName.value.trim()) {
    ui.showToast(t('adminSys.k29'), 'warning');
    return;
  }
  creating.value = true;
  try {
    createdKey.value = await createApiKey({
      app_name: appName.value.trim(),
      rate_limit: Number(rateLimit.value) || 0,
      expires_in_days: Number(expiresInDays.value) || 0,
    });
    showCreateModal.value = false;
    showSecretModal.value = true;
    appName.value = '';
    rateLimit.value = '';
    expiresInDays.value = '';
    await loadKeys();
  } catch (e) {
    logger.error('Failed to create api key:', e);
    ui.showToast((e as Error).message || t('fallback.k16'), 'error');
  } finally {
    creating.value = false;
  }
}

async function handleCopy() {
  const raw = createdKey.value?.api_key;
  if (!raw) return;
  const ok = await copyText(raw);
  ui.showToast(ok ? t('uiText.k37') : t('uiText.k38'), ok ? 'success' : 'error');
}

function closeSecretModal() {
  showSecretModal.value = false;
  // 明文只展示一次，关闭即从内存丢弃
  createdKey.value = null;
}

function handleRevoke(row: ApiKeyRecord) {
  revokingItem.value = row;
  showRevokeModal.value = true;
}

async function confirmRevoke() {
  const item = revokingItem.value;
  if (!item) return;
  revoking.value = true;
  try {
    await revokeApiKey(item.id);
    ui.showToast(t('adminSys.k30'), 'success');
    showRevokeModal.value = false;
    revokingItem.value = null;
    await loadKeys();
  } catch (e) {
    logger.error('Failed to revoke api key:', e);
    ui.showToast((e as Error).message || t('fallback.k41'), 'error');
  } finally {
    revoking.value = false;
  }
}

// P1-9：折叠说明的标题行，展开后的正文复用 descStyle
const noteDetailsStyle = { marginBottom: AppleSpace.sm };
const noteSummaryStyle = {
  cursor: 'pointer',
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
};

onMounted(() => loadKeys());
</script>
