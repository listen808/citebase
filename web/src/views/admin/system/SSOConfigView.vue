<template>
  <ApPageContainer
    :title="$t('adminSys.k226')"
    :subtitle="$t('adminSys.k235')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    :show-back="true"
    back-to="/settings"
    compact
  >

    <div v-if="activeTab === 'config'" :style="crudStatsRowStyle('compact')">
      <ApStatCard size="compact" :label="$t('adminSys.k226')" :value="items.length" :icon="'Fingerprint'" :color="cv('blue')" />
      <!-- 已启用 / OAuth / SAML 三张卡都在数"这一页的 SSO 配置"，只是按状态或协议切片 ——
           属「同一功能、只是计数」，一律用功能符号 Fingerprint；协议/状态的含义由标签与颜色表达。
           原先 已启用=CheckCircle、OAuth=Globe、SAML=Lock 会把 OIDC 与数据敏感性的身份符号借走（撞字）。 -->
      <!-- 这三张卡各数**不同对象**（已启用的条数 / OAuth 协议条数 / SAML 协议条数），
           统一成导航那枚 Fingerprint 会让四张卡只剩颜色差别、符号失去信息 —— 收口收过头了。
           选的三枚都不是别的功能的身份符：Globe 归 OIDC 提供方、Lock 归数据敏感性，这里不借用。 -->
      <ApStatCard size="compact" :label="$t('adminSys.k114')" :value="items.filter(c => c.is_used === 1).length" :icon="'BadgeCheck'" :color="cv('green')" />
      <ApStatCard size="compact" label="OAuth" :value="items.filter(c => detectProtocol(c.url) === 'OAuth').length" :icon="'Handshake'" :color="cv('blue')" />
      <ApStatCard size="compact" label="SAML" :value="items.filter(c => detectProtocol(c.url) === 'SAML').length" :icon="'FileKey'" :color="cv('orange')" />
    </div>

    <ApSegmentedControl
      v-model="activeTab"
      :options="tabs"
      :style="tabBarStyle"
    />

    <div v-if="activeTab === 'config'">
      <AdminCrudPage
        v-model:keyword="keyword"
        :error-text="loadError"
        :stats="[]"
        :loading="loading"
        :filters="{ search: { placeholder: $t('vAdminUi.k37') } }"
        :show-empty="!loading && !items.length"
        :empty-icon="'Inbox'"
        :empty-text="$t('adminSys.k233')"
        @retry="retryLoad"
        @search="handleSearch"
      >
        <template #actions>
          <ApButton variant="primary" size="small" @click="openCreateModal">
            <ApIcon name="Plus" :size="16" />{{ $t('adminSys.k214') }}
          </ApButton>
        </template>

        <template #content>
          <div v-if="loading" :style="{ ...appleCardGrid('entity'), marginBottom: AppleSpace.md }">
            <div v-for="i in 3" :key="i" :style="{ ...formSectionStyle(), height: AppleSpace['220px'] }">
              <ApSkeleton :style="{ height: AppleSpace.lg, width: '40%', marginBottom: AppleSpace.md }" />
              <ApSkeleton :style="{ height: AppleSpace.md, width: '80%', marginBottom: AppleSpace.sm }" />
              <ApSkeleton :style="{ height: AppleSpace.md, width: '60%', marginBottom: AppleSpace.sm }" />
              <ApSkeleton :style="{ height: AppleSpace.md, width: '70%' }" />
            </div>
          </div>
          <ApEmpty v-else-if="!items.length" :icon="'Inbox'" :title="$t('adminSys.k233')" />
          <div v-else :style="{ ...appleCardGrid('entity'), marginBottom: AppleSpace.md }">
            <ApCard
              v-for="cfg in items"
              :key="cfg.id"
              size="small"
              :style="[crudCardStyle(), crudCardInnerStyle()]"
              :accent-color="cfg.is_used === 1 ? cv('green') : cv('gray')"
            >
              <div :style="crudCardHeaderRowStyle()">
                <div :style="crudCardTitleRowStyle()">
                  <div :style="iconBoxStyle(protocolColor(detectProtocol(cfg.url)))">
                    <component :is="protocolIcon(detectProtocol(cfg.url))" :size="20" :color="cv('on-accent')" />
                  </div>
                  <div :style="crudCardTitleWrapStyle()">
                    <h3 :style="cardTitleStyle">{{ cfg.name }}</h3>
                    <div :style="cardMetaStyle">
                      <ApStatusBadge v-if="cfg.is_used === 1" status="enabled" :label="$t('adminSys.k114')" size="small" />
                      <ApStatusBadge v-else status="disabled" :label="$t('adminSys.k96')" size="small" />
                    </div>
                  </div>
                </div>
              </div>

              <div :style="crudCardBodyDetailStyle()">
                <div :style="crudInfoRowStyle()">
                  <span :style="crudInfoLabelStyle()">{{ $t('adminSys.k215') }}</span>
                  <span :style="[crudInfoValueStyle(), { color: protocolColor(detectProtocol(cfg.url)) }]">{{ detectProtocol(cfg.url) }}</span>
                </div>
                <div :style="crudInfoRowStyle()">
                  <span :style="crudInfoLabelStyle()">{{ $t('adminSys.k216') }}</span>
                  <span :style="[crudInfoValueStyle(), crudCodeStyle()]">{{ cfg.url }}</span>
                </div>
                <div :style="crudInfoRowStyle()">
                  <span :style="crudInfoLabelStyle()">{{ $t('adminSys.k217') }}</span>
                  <code :style="crudCodeStyle()">{{ cfg.username_prefix }}</code>
                </div>
              </div>

              <div :style="crudCardFooterStyle()">
                <ApButton variant="ghost" size="small" @click="handleSSOLogin(cfg)">
                  <ApIcon name="ExternalLink" :size="16" /> {{ $t('adminSys.k218') }}
                </ApButton>
                <ApButton variant="ghost" size="small" @click="openEditModal(cfg)">
                  <ApIcon name="Pencil" :size="16" /> {{ $t('adminSys.edit') }}
                </ApButton>
                <ApButton
                  v-if="cfg.is_used !== 1"
                  variant="ghost"
                  size="small"
                  @click="handleSetDefault(cfg)"
                >
                  <ApIcon name="Star" :size="16" /> {{ $t('adminSys.k92') }}
                </ApButton>
                <ApButton variant="danger" size="small" @click="openDeleteModal(cfg)">
                  <ApIcon name="Trash2" :size="16" /> {{ $t('adminSys.delete') }}
                </ApButton>
              </div>
            </ApCard>
          </div>
        </template>
      </AdminCrudPage>
    </div>

    <div v-if="activeTab === 'docs'" :style="docTabContentStyle">
      <div :style="docSectionStyle">
        <h2 :style="docSectionTitleStyle">{{ $t('adminSys.k219') }}</h2>
        <p :style="docBodyStyle">{{ $t('adminSys.k220') }}</p>
        <div :style="docCodeBlockStyle">
          <pre :style="docPreStyle">ldap:
  enabled: true
  host: ldap.example.com
  port: 389
  base_dn: "dc=example,dc=com"
  user_filter: "(uid=%s)"
  bind_dn: "cn=admin,dc=example,dc=com"</pre>
        </div>
      </div>

      <div :style="docSectionStyle">
        <h2 :style="docSectionTitleStyle">{{ $t('adminSys.k221') }}</h2>
        <p :style="docBodyStyle">{{ $t('adminSys.k222') }}</p>
        <div :style="docCodeBlockStyle">
          <pre :style="docPreStyle">oauth:
  enabled: true
  client_id: "your-client-id"
  client_secret: "your-client-secret"
  authorize_url: "https://auth.example.com/oauth/authorize"
  token_url: "https://auth.example.com/oauth/token"</pre>
        </div>
      </div>

      <div :style="docLinkRowStyle">
        <ApButton variant="secondary" @click="router.push('/settings/sso/doc')">
          <ApIcon name="BookOpen" :size="16" />{{ $t('adminSys.k223') }}
        </ApButton>
      </div>
    </div>

    <div v-if="activeTab === 'test'" :style="testTabContentStyle">
      <div :style="formSectionStyle()">
        <h3 :style="sectionTitleStyle">{{ $t('adminSys.k224') }}</h3>
        <p :style="sectionDescStyle()">{{ $t('adminSys.k225') }}</p>

        <div :style="formStyle">
          <ApFormItem :label="$t('adminSys.k226')">
            <ApSelect
              :model-value="testConfigId"
              :options="testConfigOptions"
              :placeholder="$t('adminSys.k230')"
              @update:model-value="(val) => testConfigId = val as number"
            />
          </ApFormItem>

          <ApFormItem :label="$t('adminSys.k227')">
            <ApInput
              :model-value="testUsername"
              :placeholder="$t('adminSys.k231')"
              @update:model-value="(val) => testUsername = val"
            />
          </ApFormItem>
        </div>

        <div :style="formActionsStyle">
          <ApButton variant="primary" :disabled="testing || !testConfigId" @click="handleTestConnection">
            {{ testing ? $t('vAdminUi.k33') : $t('vAdminUi.k34') }}
          </ApButton>
        </div>

        <div v-if="testResult" :style="testResultStyle">
          <div :style="testResultHeaderStyle">
            <ApIcon v-if="testResult.success" name="CheckCircle" :size="20" :color="cv('green')" />
            <ApIcon v-else name="XCircle" :size="20" :color="cv('red')" />
            <span :style="testResultTitleStyle">{{ testResult.success ? $t('vAdminUi.k35') : $t('vAdminUi.k36') }}</span>
          </div>
          <p :style="testResultMsgStyle">{{ testResult.message }}</p>
        </div>
      </div>
    </div>

    <ApModal :model-value="showCreateModal || showEditModal" :title="isEdit ? $t('adminForm.k3') : $t('adminForm.k4')" size="medium" density="compact" @update:model-value="(v: boolean) => { if (!v) closeModals() }">
      <ApFormItem :label="$t('adminSys.name')" required :error="formErrors.name">
        <ApInput v-model="form.name" :placeholder="$t('adminSys.k232')" />
      </ApFormItem>
      <ApFormItem :label="$t('adminSys.k228')" required :error="formErrors.url">
        <ApInput v-model="form.url" placeholder="https://sso.example.com/authorize" />
      </ApFormItem>
      <ApFormItem :label="$t('adminSys.k217')" required :error="formErrors.username_prefix">
        <ApInput v-model="form.username_prefix" placeholder="sso_" />
      </ApFormItem>
      <ApFormItem :label="$t('adminSys.k229')" :error="formErrors.ext_data">
        <ApTextarea v-model="form.ext_data" placeholder='{"callback_url": "https://your-app.com/callback", "username_field": "preferred_username"}' :rows="3" />
      </ApFormItem>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="closeModals">{{ $t('adminSys.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :disabled="submitting" @click="onSave">{{ submitting ? $t('vAdminUi.k1') : $t('vAdminUi.save') }}</ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('adminSys.k234')"
      :message="deleteMessage"
      :confirm-text="$t('adminSys.delete')"
      type="danger"
      density="compact"
      @confirm="handleDelete"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, onMounted, computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { icons } from '@/utils/icons';
import ApIcon from '@/components/apple/ApIcon.vue';
import { AppleSpace, AppleType, AppleRadius, AppleFont, AppleFontWeight, cv, AppleLayout, AppleLineHeight, appleCardGrid } from '@/utils/core/apple-tokens';
import { crudStatsRowStyle, crudCardStyle, crudCardInnerStyle, crudCardHeaderRowStyle, crudCardTitleRowStyle, crudCardTitleWrapStyle, crudCardBodyDetailStyle, crudInfoRowStyle, crudInfoLabelStyle, crudInfoValueStyle, crudCardFooterStyle, crudCodeStyle, cardTitleStyle, cardMetaStyle, iconBoxStyle, formSectionStyle, formStyle, formActionsStyle, sectionTitleStyle, sectionDescStyle, tabBarStyle } from '@/styles/layout/page-styles';
import { getSSOConfigs, createSSO, updateSSO, deleteSSO, setDefaultSSO, initiateSSOLogin } from '@/api/modules/auth/sso';
import type { SSOConfig } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';
import { useAdminCrud } from '@/composables/shared/useAdminCrud';
import { validators } from '@/utils/core/validators';
import { isSafeRedirect } from '@/utils/core/shared';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import AdminCrudPage from '@/components/admin/AdminCrudPage.vue';

const ui = useUIStore();
const router = useRouter();
const route = useRoute();

// 初始档读 ?tab=（文档页标签栏跳回时带上，审计 B10）；非法值回落 config
const tabQuery = route.query.tab;
const activeTab = ref<'config' | 'docs' | 'test'>(
  tabQuery === 'docs' || tabQuery === 'test' ? tabQuery : 'config',
);
const tabs = [
  { value: 'config' as const, label: t('adminSys.k189') },
  { value: 'docs' as const, label: t('adminSys.k236') },
  { value: 'test' as const, label: t('adminSys.test') },
];

const {
  loading, loadError, retryLoad, items, keyword,
  showCreateModal, showEditModal, showDeleteModal, editingItem,
  formErrors,
  fetchList, handleSearch, openCreate, openEdit, openDelete, closeModals,
  handleDelete, handleSubmit,
} = useAdminCrud<SSOConfig>({
  fetchFn: async (p) => {
    const res = await getSSOConfigs({ page: p.page, page_size: 200, keyword: p.keyword || undefined });
    const data = res.data || [];
    return { data, total: res.total ?? data.length, page: p.page, page_size: 200 };
  },
  createFn: (d) => createSSO({ name: d.name!, url: d.url!, username_prefix: d.username_prefix!, ext_data: d.ext_data || undefined }),
  updateFn: (d) => updateSSO(d.id!, { name: d.name!, url: d.url!, username_prefix: d.username_prefix!, ext_data: d.ext_data || undefined }),
  deleteFn: (id) => deleteSSO(id),
  itemName: t('adminForm.k16'),
  resetForm: () => { form.value = { name: '', url: '', username_prefix: '', ext_data: '' }; },
});

Object.assign(formErrors, { name: '', url: '', username_prefix: '', ext_data: '' });

const isEdit = computed(() => !!editingItem.value);
const submitting = shallowRef(false);
const deleteMessage = shallowRef('');
const form = ref<{ name: string; url: string; username_prefix: string; ext_data: string }>({ name: '', url: '', username_prefix: '', ext_data: '' });

const testConfigId = ref<number | null>(null);
const testUsername = ref('');
const testing = shallowRef(false);
const testResult = shallowRef<{ success: boolean; message: string } | null>(null);

const testConfigOptions = computed(() =>
  items.value.map(c => ({ label: c.name, value: c.id }))
);

const docTabContentStyle = {
  paddingTop: 0,
};

const docSectionStyle = {
  marginBottom: AppleSpace.xl,
};

const docSectionTitleStyle = {
  fontFamily: AppleType.utilityHeading.font,
  fontSize: AppleType.utilityHeading.size,
  fontWeight: String(AppleType.utilityHeading.weight),
  color: cv('text-primary'),
  margin: `0 0 ${AppleSpace.md} 0`,
};

const docBodyStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-secondary'),
  lineHeight: String(AppleType.bodyPrimary.lineHeight),
  margin: `0 0 ${AppleSpace.md} 0`,
};

const docCodeBlockStyle = {
  background: cv('bg-tertiary'),
  border: `1px solid ${cv('border-light')}`,
  borderRadius: AppleRadius.medium,
  padding: AppleSpace.md,
  overflowX: 'auto' as const,
};

const docPreStyle = {
  margin: '0',
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUI.size,
  color: cv('text-primary'),
  lineHeight: AppleLineHeight.spacious,
  whiteSpace: 'pre' as const,
};

const docLinkRowStyle = {
  paddingTop: 0,
};

const testTabContentStyle = {
  paddingTop: 0,
  maxWidth: AppleLayout.cardWide,
};

const testResultStyle = {
  marginTop: AppleSpace.lg,
  padding: AppleSpace.lg,
  borderRadius: AppleRadius.card,
  background: cv('surface-panel'),
  border: `1px solid ${cv('border-light')}`,
};

const testResultHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  marginBottom: AppleSpace.sm,
};

const testResultTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
};

const testResultMsgStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-secondary'),
  margin: '0',
};

function openCreateModal() {
  form.value = { name: '', url: '', username_prefix: '', ext_data: '' };
  openCreate();
}

function openEditModal(cfg: SSOConfig) {
  form.value = { name: cfg.name, url: cfg.url, username_prefix: cfg.username_prefix, ext_data: cfg.ext_data || '' };
  openEdit(cfg);
}

function openDeleteModal(cfg: SSOConfig) {
  deleteMessage.value = t('sentUi.k21', { p1: cfg.name });
  openDelete(cfg);
}

async function onSave() {
  if (submitting.value) return;
  if (form.value.ext_data?.trim()) {
    try {
      JSON.parse(form.value.ext_data);
    } catch {
      ui.showToast(t('adminSys.k239'), 'error');
      formErrors.ext_data = t('adminForm.k36');
      return;
    }
  }
  submitting.value = true;
  try {
    await handleSubmit(
      { name: form.value.name, url: form.value.url, username_prefix: form.value.username_prefix, ext_data: form.value.ext_data } as Partial<SSOConfig>,
      (data) => {
        const errors: Record<string, string> = {};
        if (!data.name?.trim()) errors.name = t('adminForm.k37');
        if (!data.url?.trim()) errors.url = t('adminForm.k38');
        else if (!validators.url(data.url)) errors.url = t('adminForm.k39');
        if (!data.username_prefix?.trim()) errors.username_prefix = t('adminForm.k40');
        else if (!validators.usernamePrefix(data.username_prefix)) errors.username_prefix = t('adminForm.k41');
        if (data.ext_data?.trim()) {
          try {
            JSON.parse(data.ext_data);
          } catch {
            errors.ext_data = t('adminForm.k36');
          }
        }
        return Object.keys(errors).length > 0 ? errors : null;
      }
    );
  } finally {
    submitting.value = false;
  }
}

async function handleSetDefault(cfg: SSOConfig) {
  try {
    await setDefaultSSO(cfg.id);
    ui.showToast(t('adminSys.k119'), 'success');
    await fetchList();
  } catch (e: unknown) {
    ui.showToast((e instanceof Error ? e.message : '') || t('fallback.k64'), 'error');
  }
}

async function handleSSOLogin(cfg: SSOConfig) {
  try {
    const res = await initiateSSOLogin(cfg.id);
    if (!isSafeRedirect(res.redirect_url)) {
      ui.showToast(t('adminSys.k240'), 'error');
      return;
    }
    window.location.href = res.redirect_url;
  } catch (e: unknown) {
    ui.showToast((e instanceof Error ? e.message : '') || t('fallback.k40'), 'error');
  }
}

async function handleTestConnection() {
  if (!testConfigId.value || testing.value) return;
  testing.value = true;
  testResult.value = null;
  try {
    const cfg = items.value.find(c => c.id === testConfigId.value);
    if (!cfg) {
      testResult.value = { success: false, message: t('adminSys.k238') };
      return;
    }
    await initiateSSOLogin(cfg.id);
    testResult.value = { success: true, message: t('adminForm.k42') };
  } catch (e: unknown) {
    testResult.value = { success: false, message: (e instanceof Error ? e.message : '') || t('fallback.k105') };
  } finally {
    testing.value = false;
  }
}

function detectProtocol(url: string): string {
  const lower = url.toLowerCase();
  if (lower.includes('saml')) return 'SAML';
  if (lower.includes('ldap')) return 'LDAP';
  return 'OAuth';
}

function protocolColor(protocol: string): string {
  const map: Record<string, string> = {
    OAuth: cv('blue'),
    SAML: cv('orange'),
    LDAP: cv('purple'),
  };
  return map[protocol] || cv('text-secondary');
}

function protocolIcon(protocol: string) {
  const map: Record<string, (typeof icons)[keyof typeof icons]> = {
    OAuth: icons['Globe'],
    SAML: icons['Lock'],
    LDAP: icons['KeyRound'],
  };
  return map[protocol] || icons['Globe'];
}

onMounted(() => fetchList());
</script>
