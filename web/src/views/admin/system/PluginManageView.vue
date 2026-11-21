<template>
  <ApPageContainer
    :title="$t('adminSys.k199')"
    :subtitle="$t('adminSys.k201')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <AdminCrudPage
      v-model:keyword="searchQuery"
      :error-text="loadError"
      :stats="stats"
      density="compact"
      :loading="loading && !filteredPlugins.length"
      :filters="{ search: { placeholder: $t('vAdminUi.k30') } }"
      :show-empty="!loading && !filteredPlugins.length"
      :empty-icon="searchQuery ? 'Search' : 'Puzzle'"
      :empty-text="emptyPluginText"
      @retry="retryLoad"
    >
      <template #actions>
        <ApButton variant="primary" @click="openCreateModal">
          <ApIcon name="Plus" :size="16" />{{ $t('adminSys.k188') }}
        </ApButton>
      </template>

      <template #content>
        <div v-if="loading && !filteredPlugins.length" :style="loadingStyle">
          <ApSkeleton variant="card" :count="4" />
        </div>
        <ApEmpty v-else-if="!filteredPlugins.length" :icon="searchQuery ? 'Search' : 'Puzzle'" :title="emptyPluginText" />
        <div v-else :style="pluginGridStyle">
          <ApCard v-for="plugin in filteredPlugins" :key="plugin.id">
            <div :style="cardHeaderStyle">
              <div :style="iconBoxStyle(cv('purple'), 36)">
                <ApIcon name="Puzzle" :size="20" :color="cv('on-accent')" />
              </div>
              <div :style="cardHeaderInfoStyle">
                <div :style="cardTitleRowStyle">
                  <span :style="cardTitleStyle">{{ plugin.title }}</span>
                  <span v-if="plugin.plugin_key" :style="versionBadgeStyle">{{ plugin.plugin_key }}</span>
                </div>
              </div>
              <!-- P1-14: 启用状态由后端 is_used 判定，走专用 toggle 接口 -->
              <ApSwitch
                :model-value="plugin.is_used === 1"
                @update:model-value="() => handleToggle(plugin)"
              />
            </div>

            <p v-if="plugin.description" :style="cardDescStyle">{{ plugin.description }}</p>

            <div :style="cardFooterStyle">
              <span :style="cardDateStyle">{{ plugin.created_at }}</span>
              <div :style="cardActionsStyle">
                <ApButton variant="ghost" size="small" @click="openEditModal(plugin)">
                  <ApIcon name="Settings" :size="16" />{{ $t('adminSys.k189') }}
                </ApButton>
                <ApButton variant="danger" size="small" @click="openDeleteModal(plugin)">
                  <ApIcon name="Trash2" :size="16" />{{ $t('adminSys.k190') }}
                </ApButton>
              </div>
            </div>
          </ApCard>
        </div>
      </template>

      <template #modals>
        <ApModal :model-value="showCreateModal || showEditModal" :title="isEdit ? $t('uiText.k27') : $t('uiText.k28')" size="medium" density="compact" @update:model-value="(v: boolean) => { if (!v) closeModals() }">
          <div :style="formStyle">
            <!-- P1-14: 字段对齐后端 CreatePluginRequest/UpdatePluginRequest：plugin_key/title 必填，配置走 conf_value -->
            <ApFormItem :label="$t('adminSys.k191')" required :error="formErrors.plugin_key">
              <ApInput v-model="pluginForm.plugin_key" :placeholder="$t('adminSys.k196')" :disabled="isEdit" />
            </ApFormItem>
            <ApFormItem :label="$t('adminSys.k192')" required :error="formErrors.title">
              <ApInput v-model="pluginForm.title" :placeholder="$t('adminSys.k197')" />
            </ApFormItem>
            <ApFormItem :label="$t('adminSys.desc')">
              <ApInput v-model="pluginForm.description" :placeholder="$t('adminSys.k198')" />
            </ApFormItem>
            <ApFormItem :label="$t('adminSys.k194')">
              <ApInput v-model="pluginForm.cover_url" placeholder="https://..." />
            </ApFormItem>
            <ApFormItem :label="$t('adminSys.k195')" :error="formErrors.config">
              <ApTextarea v-model="pluginForm.conf_value" placeholder='{"key": "value"}' :rows="4" />
            </ApFormItem>
          </div>
          <template #footer>
            <ApButton variant="ghost" size="small" @click="closeModals">{{ $t('adminSys.cancel') }}</ApButton>
            <ApButton variant="primary" size="small" :disabled="submitting" @click="onSave">{{ submitting ? $t('vAdminUi.k1') : $t('vAdminUi.save') }}</ApButton>
          </template>
        </ApModal>

        <ApConfirmModal
          v-model="showDeleteModal"
          :title="$t('adminSys.k200')"
          :message="deleteMessage"
          :confirm-text="$t('adminSys.k190')"
          :cancel-text="$t('adminSys.cancel')"
          type="danger"
          @confirm="handleDelete"
        />
      </template>
    </AdminCrudPage>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import { cv, AppleLayout, AppleSpace, AppleFont, AppleFontWeight, AppleType, AppleLineHeight, appleCardGrid } from '@/utils/core/apple-tokens';
import { loadingStyle, formStyle, iconBoxStyle, badgeBaseStyle } from '@/styles/layout/page-styles';
import { useUIStore } from '@/stores/app/ui';
import { useAdminCrud } from '@/composables/shared/useAdminCrud';
// P1-14: 原视图用 name/version/author/enabled/tags/config 等旧字段，与后端
// PluginResponse{id,plugin_key,title,description,cover_url,conf_value,is_used} 完全错位：
// 创建必 400、编辑不落库、开关恒关。全部改为后端真实契约。
import { getPlugins, createPlugin, updatePlugin, updatePluginConfig, togglePlugin, deletePlugin, type CreatePluginRequest, type UpdatePluginRequest } from '@/api/modules/content/plugin';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import ApSwitch from '@/components/apple/ApSwitch.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApCard from '@/components/apple/ApCard.vue';
import AdminCrudPage from '@/components/admin/AdminCrudPage.vue';
import type { StatConfig } from '@/components/admin/AdminCrudPage.vue';

const ui = useUIStore();

interface PluginItem {
  id: number;
  plugin_key: string;
  title: string;
  description: string;
  cover_url: string;
  conf_value: string;
  is_used: number;
  created_at: string;
  updated_at: string;
}

const {
  loading, loadError, retryLoad, items,
  showCreateModal, showEditModal, showDeleteModal, editingItem,
  formErrors,
  fetchList, openCreate, openEdit, openDelete, closeModals,
  handleDelete, handleSubmit,
} = useAdminCrud<PluginItem>({
  fetchFn: async (p) => {
    const res = await getPlugins({ page: p.page, page_size: 200 });
    const data = (res.data || []) as unknown as PluginItem[];
    return { data, total: res.total ?? data.length, page: p.page, page_size: 200 };
  },
  createFn: (d) => createPlugin({
    plugin_key: d.plugin_key ?? '',
    title: d.title ?? '',
    description: d.description,
    cover_url: d.cover_url,
    conf_value: d.conf_value,
    is_used: 0,
  } as CreatePluginRequest),
  updateFn: async (d) => {
    const id = d.id!;
    await updatePlugin(id, {
      title: d.title,
      description: d.description,
      cover_url: d.cover_url,
    } as UpdatePluginRequest);
    // 配置有变化才写 conf_value（后端为独立端点且要求非空，清空时落 "{}"）
    if (d.conf_value !== undefined && d.conf_value !== editingItem.value?.conf_value) {
      await updatePluginConfig(id, d.conf_value.trim() || '{}');
    }
  },
  deleteFn: (id) => deletePlugin(id),
  itemName: t('uiText.k29'),
  resetForm: () => { pluginForm.value = { plugin_key: '', title: '', description: '', cover_url: '', conf_value: '' }; },
});

Object.assign(formErrors, { plugin_key: '', title: '', config: '' });

const isEdit = computed(() => !!editingItem.value);
const submitting = shallowRef(false);
const searchQuery = ref('');
const deleteMessage = shallowRef('');

const pluginForm = ref({
  plugin_key: '',
  title: '',
  description: '',
  cover_url: '',
  conf_value: '',
});

const stats = computed<StatConfig[]>(() => [
  { label: t('adminSys.k202'), value: items.value.length, icon: 'Puzzle', color: cv('blue') },
  { label: t('adminSys.k114'), value: items.value.filter(p => p.is_used === 1).length, icon: 'Power', color: cv('green') },
  { label: t('adminSys.k203'), value: items.value.filter(p => p.is_used !== 1).length, icon: 'PowerOff', color: cv('gray') },
]);

const filteredPlugins = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return items.value;
  return items.value.filter(p =>
    (p.title || '').toLowerCase().includes(q) ||
    (p.description || '').toLowerCase().includes(q) ||
    (p.plugin_key || '').toLowerCase().includes(q)
  );
});

// 空态那句话在模板里用了两处（AdminCrudPage 的 empty-text 与兜底的 ApEmpty），
// 各写一份中文就会一份改了另一份没改 —— 收成一个 computed。
const emptyPluginText = computed(() => (searchQuery.value ? t('adminSys.k392') : t('adminSys.k393')));

const pluginGridStyle = appleCardGrid('panel');

const cardHeaderStyle = {
  display: 'flex',
  alignItems: 'flex-start',
  gap: AppleSpace.sm,
};

const cardHeaderInfoStyle = {
  flex: '1',
  minWidth: '0',
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
};

const cardTitleRowStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
};

const cardTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const versionBadgeStyle = {
  ...badgeBaseStyle,
  background: cv('bg-tertiary'),
  color: cv('text-secondary'),
};

const cardDescStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-secondary'),
  lineHeight: AppleLineHeight.loose,
  margin: '0',
  display: '-webkit-box',
  WebkitLineClamp: 2,
  WebkitBoxOrient: 'vertical' as const,
  overflow: 'hidden',
};

const cardFooterStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  paddingTop: AppleSpace.xs,
  marginTop: AppleSpace.sm,
  borderTop: `1px solid ${cv('border-light')}`,
};

const cardDateStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

const cardActionsStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
};

function openCreateModal() {
  pluginForm.value = { plugin_key: '', title: '', description: '', cover_url: '', conf_value: '' };
  openCreate();
}

function openEditModal(plugin: PluginItem) {
  pluginForm.value = {
    plugin_key: plugin.plugin_key || '',
    title: plugin.title || '',
    description: plugin.description || '',
    cover_url: plugin.cover_url || '',
    conf_value: plugin.conf_value || '',
  };
  openEdit(plugin);
}

function openDeleteModal(plugin: PluginItem) {
  deleteMessage.value = t('sentUi.k24', { p1: plugin.title });
  openDelete(plugin);
}

// P1-14: 启用/停用走后端专用 toggle 端点，成功后刷新列表以拿到最新 is_used
async function handleToggle(plugin: PluginItem) {
  try {
    await togglePlugin(plugin.id);
    await fetchList();
    ui.showToast(plugin.is_used === 1 ? t('uiText.k30') : t('uiText.k31'), 'success');
  } catch (e: unknown) {
    ui.showToast((e as Error).message || t('fallback.k64'), 'error');
  }
}

async function onSave() {
  if (submitting.value) return;
  submitting.value = true;
  try {
    const form = pluginForm.value;
    const confValue = form.conf_value.trim();
    if (confValue) {
      try {
        JSON.parse(confValue);
      } catch (e: unknown) {
        const message = e instanceof Error ? e.message: t('adminSys.k204');
        formErrors.config = t('uiText.k32', { p1: message });
        return;
      }
    }
    const payload = {
      plugin_key: form.plugin_key,
      title: form.title,
      description: form.description,
      cover_url: form.cover_url,
      conf_value: confValue,
    };
    await handleSubmit(
      payload as Partial<PluginItem>,
      (data) => {
        const errors: Record<string, string> = {};
        if (!data.plugin_key?.trim()) errors.plugin_key = t('uiText.k33');
        if (!data.title?.trim()) errors.title = t('vAdminUi.k31');
        return Object.keys(errors).length > 0 ? errors : null;
      }
    );
  } catch (e: unknown) {
    ui.showToast((e as Error).message || t('fallback.k8'), 'error');
  } finally {
    submitting.value = false;
  }
}

onMounted(() => { fetchList(); });
</script>
