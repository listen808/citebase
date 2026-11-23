<template>
  <ApPageContainer
    :title="$t('adminSys.k107')"
    :subtitle="$t('adminSys.k111')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    :show-back="true"
    back-to="/settings"
    compact
  >
    <AdminCrudPage
      v-model:keyword="keyword"
      :error-text="loadError"
      :stats="stats"
      density="compact"
      :filters="{ search: { placeholder: $t('vAdminUi.k22') } }"
      :loading="loading"
      :total="total"
      :page="page"
      :page-size="200"
      @retry="retryLoad"
      @search="handleSearch"
    >
      <template #actions>
        <ApButton variant="secondary" @click="showTemplateModal = true">
          <ApIcon name="Eye" :size="16" />{{ $t('adminSys.k83') }}
        </ApButton>
        <ApButton variant="primary" @click="openCreateModal">
          <ApIcon name="Plus" :size="16" />{{ $t('adminSys.k84') }}
        </ApButton>
      </template>

      <template #content>
        <div v-if="loading && !items.length" :style="stepLayoutStyle">
          <div v-for="i in 2" :key="i" :style="stepCardOuterStyle">
            <ApSkeleton :style="{ height: AppleSpace['200px'], borderRadius: AppleRadius.card }" />
          </div>
        </div>

        <ApEmpty v-else-if="!items.length" icon="Mail" :title="$t('adminSys.k108')" />

        <div v-else :style="stepLayoutStyle">
          <ApCard
            v-for="server in items"
            :key="server.id"
            :accent-color="server.is_used === 1 ? cv('green') : cv('border-light')"
            :style="stepCardOuterStyle"
          >
            <div :style="stepCardHeaderStyle">
              <div :style="stepHeaderLeftStyle">
                <div :style="iconBoxStyle(cv('blue'))">
                  <ApIcon name="Mail" :size="20" :color="cv('on-accent')" />
                </div>
                <div :style="stepHeaderInfoStyle">
                  <h3 :style="stepTitleStyle">{{ server.name }}</h3>
                  <div :style="stepMetaStyle">
                    <ApStatusBadge v-if="server.is_used === 1" status="default" :label="$t('adminSys.k95')" size="small" />
                    <ApStatusBadge v-else status="disabled" :label="$t('adminSys.k96')" size="small" />
                  </div>
                </div>
              </div>
            </div>

            <div :style="stepBodyStyle">
              <div :style="stepSectionStyle">
                <div :style="stepBadgeStyle">
                  <span :style="stepBadgeNumStyle">1</span>
                  <span :style="stepBadgeLabelStyle">{{ $t('adminSys.k85') }}</span>
                </div>
                <div :style="stepInfoGridStyle">
                  <div :style="stepInfoItemStyle">
                    <span :style="stepInfoLabelStyle">{{ $t('adminSys.k86') }}</span>
                    <span :style="stepInfoValueStyle">{{ server.host }}:{{ server.port }}</span>
                  </div>
                  <div :style="stepInfoItemStyle">
                    <span :style="stepInfoLabelStyle">{{ $t('adminSys.k87') }}</span>
                    <span :style="stepInfoValueStyle">{{ server.sender_address }}</span>
                  </div>
                  <div :style="stepInfoItemStyle">
                    <span :style="stepInfoLabelStyle">{{ $t('adminSys.k88') }}</span>
                    <span :style="stepInfoValueStyle">{{ server.sender_name || '-' }}</span>
                  </div>
                  <div :style="stepInfoItemStyle">
                    <span :style="stepInfoLabelStyle">SSL/TLS</span>
                    <span :style="stepInfoValueStyle">{{ server.is_ssl === 1 ? $t('adminForm.k34') : $t('vAdminUi.k19') }}</span>
                  </div>
                </div>
              </div>

              <div :style="stepSectionStyle">
                <div :style="stepBadgeStyle">
                  <span :style="stepBadgeNumStyle">2</span>
                  <span :style="stepBadgeLabelStyle">{{ $t('adminSys.k89') }}</span>
                </div>
                <div :style="testRowStyle">
                  <ApInput
                    :model-value="testForms[server.id!]?.to_emails || ''"
                    :placeholder="$t('adminSys.k103')"
                    :style="{ flex: '1' }"
                    @update:model-value="(v: string) => updateTestForm(server.id!, v)"
                  />
                  <!-- P1-13: 后端测试接口 password 为 binding:"required" 且密码字段不下发（json:"-"），
                       此前固定传空串导致按钮 100% 返回 400；改为要求补录密码后再发送 -->
                  <ApInput
                    :model-value="testForms[server.id!]?.password || ''"
                    type="password"
                    :placeholder="$t('adminSys.k104')"
                    :style="{ width: '220px', flexShrink: '0' }"
                    @update:model-value="(v: string) => updateTestPassword(server.id!, v)"
                  />
                  <ApButton
                    variant="primary"
                    size="small"
                    :loading="testSubmittingIds.has(server.id)"
                    @click="handleTestFromCard(server)"
                  >
                    <ApIcon name="Send" :size="16" />{{ $t('adminSys.k90') }}
                  </ApButton>
                </div>
                <div v-if="testResults[server.id!]" :style="testResultStyle">
                  <ApIcon v-if="testResults[server.id!] === 'success'" name="CheckCircle" :size="16" :color="cv('green')" />
                  <ApIcon v-else name="XCircle" :size="16" :color="cv('red')" />
                  <span :style="testResultTextStyle">{{ testResults[server.id!] === 'success' ? $t('vAdminUi.k20') : $t('vAdminUi.k21') }}</span>
                </div>
              </div>
            </div>

            <div :style="stepCardFooterStyle">
              <ApButton variant="ghost" size="small" @click="openEditModal(server)">
                <ApIcon name="Pencil" :size="16" />{{ $t('adminSys.edit') }}
              </ApButton>
              <ApButton
                v-if="server.is_used !== 1"
                variant="ghost"
                size="small"
                @click="handleSetDefault(server)"
              >
                <ApIcon name="Star" :size="16" />{{ $t('adminSys.k92') }}
              </ApButton>
              <ApButton variant="danger" size="small" @click="openDeleteModal(server)">
                <ApIcon name="Trash2" :size="16" />{{ $t('adminSys.delete') }}
              </ApButton>
            </div>
          </ApCard>
        </div>
      </template>

      <template #modals>
        <ApModal :model-value="showCreateModal || showEditModal" :title="isEdit ? $t('adminForm.k5') : $t('adminForm.k6')" size="medium" density="compact" @update:model-value="(v: boolean) => { if (!v) closeModals() }">
          <ApFormItem :label="$t('adminSys.name')" required :error="formErrors.name">
            <ApInput v-model="form.name" :placeholder="$t('adminSys.k105')" />
          </ApFormItem>
          <ApFormItem :label="$t('adminSys.k86')" required :error="formErrors.host">
            <ApInput v-model="form.host" placeholder="smtp.example.com" />
          </ApFormItem>
          <div :style="formRowStyle">
            <ApFormItem :label="$t('adminSys.k98')" required :error="formErrors.port" style="flex: 1">
              <ApInput v-model="form.port" placeholder="587" />
            </ApFormItem>
            <ApFormItem :label="$t('adminSys.k99')" required :error="formErrors.sender_address" style="flex: 1">
              <ApInput v-model="form.sender_address" type="email" placeholder="noreply@example.com" />
            </ApFormItem>
          </div>
          <div :style="formRowStyle">
            <ApFormItem :label="$t('adminSys.k73')" required :error="formErrors.username" style="flex: 1">
              <ApInput v-model="form.username" :placeholder="$t('adminSys.k106')" />
            </ApFormItem>
            <ApFormItem :label="$t('adminSys.k100')" :error="formErrors.password" style="flex: 1">
              <ApInput v-model="form.password" type="password" :placeholder="isEdit ? $t('adminForm.k43') : ''" />
            </ApFormItem>
          </div>
          <div :style="formRowStyle">
            <ApFormItem :label="$t('adminSys.k88')" style="flex: 1">
              <ApInput v-model="form.sender_name" :placeholder="siteBrandName" />
            </ApFormItem>
            <ApFormItem :label="$t('adminSys.k101')" style="flex: 1">
              <ApInput v-model="form.sender_title_prefix" :placeholder="`[${siteBrandName}]`" />
            </ApFormItem>
          </div>
          <ApFormItem :label="$t('adminSys.k102')">
            <ApCheckbox v-model="isSslChecked" />
          </ApFormItem>
          <template #footer>
            <ApButton variant="ghost" size="small" @click="closeModals">{{ $t('adminSys.cancel') }}</ApButton>
            <ApButton variant="primary" size="small" :disabled="submitting" @click="onSave">{{ submitting ? $t('vAdminUi.k1') : $t('vAdminUi.save') }}</ApButton>
          </template>
        </ApModal>

        <!-- P1-64: 原“发送测试”Modal（showTestModal/testForm/handleTest）全站无入口、testTarget 恒为 null，
             属死代码且带空指针风险，已删除；测试功能统一走卡片内联发送（handleTestFromCard） -->

        <ApConfirmModal
          v-model="showDeleteModal"
          :title="$t('adminSys.k109')"
          :message="deleteMessage"
          :confirm-text="$t('adminSys.delete')"
          type="danger"
          @confirm="handleDelete"
        />

        <ApModal v-model="showTemplateModal" :title="$t('adminSys.k110')" size="medium" density="compact">
          <div :style="templatePreviewStyle">
            <h3 :style="templateTitleStyle">{{ $t('adminSys.k93', { app: siteBrandName }) }}</h3>
            <!-- eslint-disable-next-line vue/no-v-html -- sanitizedEmailTemplateHtml 已在 computed 里走 DOMPurify.sanitize -->
            <div :style="templateBodyStyle" v-html="sanitizedEmailTemplateHtml" />
          </div>
          <template #footer>
            <ApButton variant="ghost" size="small" @click="showTemplateModal = false">{{ $t('adminSys.close') }}</ApButton>
          </template>
        </ApModal>
      </template>
    </AdminCrudPage>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { siteBrandName } from '@/utils/core/site-brand';
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, reactive, onMounted, computed } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import { AppleSpace, cv, AppleLayout, AppleType, AppleFont, AppleFontWeight, AppleRadius, appleCardGrid } from '@/utils/core/apple-tokens';
import { formRowStyle, iconBoxStyle } from '@/styles/layout/page-styles';
import { getEmails, createEmail, updateEmail, deleteEmail, setDefaultEmail, testEmail, type CreateEmailRequest, type UpdateEmailRequest } from '@/api/modules/content/email';
import type { EmailServer } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';
import { useAdminCrud } from '@/composables/shared/useAdminCrud';
import { validators } from '@/utils/core/validators';
import AdminCrudPage from '@/components/admin/AdminCrudPage.vue';
import type { StatConfig } from '@/components/admin/AdminCrudPage.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import ApCheckbox from '@/components/apple/ApCheckbox.vue';
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue';
import ApCard from '@/components/apple/ApCard.vue';
import DOMPurify from 'dompurify';

const ui = useUIStore();

const {
  loading, loadError, retryLoad, items, keyword, total, page,
  showCreateModal, showEditModal, showDeleteModal, editingItem,
  formErrors,
  fetchList, handleSearch, openCreate, openEdit, openDelete, closeModals,
  handleDelete, handleSubmit,
} = useAdminCrud<EmailServer>({
  fetchFn: async (p) => {
    const res = await getEmails({ page: p.page, page_size: 200, keyword: p.keyword || undefined });
    const data = res.data || [];
    return { data, total: res.total ?? data.length, page: p.page, page_size: 200 };
  },
  createFn: (d) => createEmail(d as unknown as CreateEmailRequest),
  updateFn: (d) => updateEmail(d.id!, d as unknown as UpdateEmailRequest),
  deleteFn: (id) => deleteEmail(id),
  itemName: t('adminForm.k17'),
  resetForm: () => { form.value = { name: '', host: '', port: '587', sender_address: '', sender_name: '', sender_title_prefix: '', username: '', password: '', is_ssl: 0 }; },
});

Object.assign(formErrors, { name: '', host: '', port: '', sender_address: '', username: '', password: '' });

const isEdit = computed(() => !!editingItem.value);
const submitting = shallowRef(false);
const deleteMessage = shallowRef('');

// P1-13: 测试需要补录密码（后端 password 必填且已存密码不下发）
const testForms = reactive<Record<number, { to_emails: string; password: string }>>({});
const testResults = reactive<Record<number, 'success' | 'fail'>>({});
const testSubmittingIds = reactive<Set<number>>(new Set());

function updateTestForm(serverId: number, value: string) {
  if (!testForms[serverId]) testForms[serverId] = { to_emails: '', password: '' };
  testForms[serverId].to_emails = value;
}

function updateTestPassword(serverId: number, value: string) {
  if (!testForms[serverId]) testForms[serverId] = { to_emails: '', password: '' };
  testForms[serverId].password = value;
}

async function handleTestFromCard(server: EmailServer) {
  const form = testForms[server.id!];
  if (!form?.to_emails?.trim()) {
    ui.showToast(t('adminSys.k115'), 'warning');
    return;
  }
  if (!validators.email(form.to_emails)) {
    ui.showToast(t('adminSys.k116'), 'warning');
    return;
  }
  // P1-13: 后端 TestEmailRequest.Password 为 binding:"required"，空密码必 400
  if (!form.password) {
    ui.showToast(t('adminSys.k117'), 'warning');
    return;
  }
  testSubmittingIds.add(server.id!);
  delete testResults[server.id!];
  try {
    await testEmail({
      name: server.name,
      host: server.host,
      port: server.port,
      sender_address: server.sender_address,
      sender_name: server.sender_name,
      sender_title_prefix: server.sender_title_prefix,
      username: server.username,
      password: form.password,
      is_ssl: server.is_ssl,
      to_emails: form.to_emails
    });
    testResults[server.id!] = 'success';
    ui.showToast(t('adminSys.k118'), 'success');
  } catch (e: unknown) {
    testResults[server.id!] = 'fail';
    // P1-13: 展示后端具体错误而不是笼统“发送失败”，避免误判为邮箱服务器故障
    const msg = e instanceof Error ? e.message : '';
    ui.showToast(msg ? t('adminForm.k44', { p1: msg }) : t('vAdminUi.k21'), 'error');
  } finally {
    testSubmittingIds.delete(server.id!);
  }
}

const isSslChecked = computed({
  get: () => form.value.is_ssl === 1,
  set: (val) => { form.value.is_ssl = val ? 1 : 0; }
});

const form = ref({
  name: '',
  host: '',
  port: '587',
  sender_address: '',
  sender_name: '',
  sender_title_prefix: '',
  username: '',
  password: '',
  is_ssl: 0
});

// P1-64: testForm/testFormErrors/handleTest（旧死代码测试弹窗三件套）已随弹窗一并移除

const stats = computed<StatConfig[]>(() => [
  { label: t('adminSys.k112'), value: items.value.length, icon: 'Server', color: cv('blue') },
  { label: t('adminSys.k113'), value: items.value.filter(s => s.is_used === 1).length, icon: 'Star', color: cv('yellow') },
  { label: t('adminSys.k114'), value: items.value.filter(s => s.is_used === 1).length, icon: 'CheckCircle', color: cv('green') },
]);

function openCreateModal() {
  form.value = {
    name: '',
    host: '',
    port: '587',
    sender_address: '',
    sender_name: '',
    sender_title_prefix: '',
    username: '',
    password: '',
    is_ssl: 0
  };
  openCreate();
}

function openEditModal(server: EmailServer) {
  form.value = {
    name: server.name,
    host: server.host,
    port: server.port,
    sender_address: server.sender_address,
    sender_name: server.sender_name || '',
    sender_title_prefix: server.sender_title_prefix || '',
    username: server.username,
    password: '',
    is_ssl: server.is_ssl
  };
  openEdit(server);
}

function openDeleteModal(server: EmailServer) {
  deleteMessage.value = t('sentUi.k21', { p1: server.name });
  openDelete(server);
}

async function onSave() {
  if (submitting.value) return;
  submitting.value = true;
  try {
    await handleSubmit(
      {
        name: form.value.name,
        host: form.value.host,
        port: form.value.port,
        sender_address: form.value.sender_address,
        sender_name: form.value.sender_name,
        sender_title_prefix: form.value.sender_title_prefix,
        // P1-65: 后端 Create/UpdateEmailRequest.Username 均为 binding:"required"，
        // placeholder 承诺“同发件人邮箱”但此前并未回填，留空必 400；提交时用 sender_address 兜底
        username: form.value.username?.trim() || form.value.sender_address,
        is_ssl: form.value.is_ssl,
        ...(form.value.password ? { password: form.value.password } : {})
      } as Partial<EmailServer> & Record<string, unknown>,
      (data) => {
        const errors: Record<string, string> = {};
        if (!data.name?.trim()) errors.name = t('adminForm.k37');
        if (!data.host?.trim()) errors.host = t('adminForm.k45');
        if (!validators.port(data.port as string)) errors.port = t('adminForm.k46');
        if (!data.sender_address || !validators.email(data.sender_address as string)) errors.sender_address = t('adminForm.k47');
        if (!isEdit.value && (!(data as Record<string, unknown>).password || !((data as Record<string, unknown>).password as string).length)) errors.password = t('adminForm.k48');
        return Object.keys(errors).length > 0 ? errors : null;
      }
    );
  } finally {
    submitting.value = false;
  }
}

async function handleSetDefault(server: EmailServer) {
  try {
    await setDefaultEmail(server.id);
    ui.showToast(t('adminSys.k119'), 'success');
    await fetchList();
  } catch (e: unknown) {
    ui.showToast((e instanceof Error ? e.message : '') || t('fallback.k64'), 'error');
  }
}

onMounted(() => fetchList());

const showTemplateModal = shallowRef(false);
// HTML 邮件客户端不支持 CSS 变量，必须使用十六进制色值字面量
// 名字取站点展示名（后台「系统设置 → 基础」那两行改的就是它），不再是这里抄的一份产品名。
const emailTemplateHtml = computed(() => `<div style="font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Helvetica Neue', Helvetica, Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 24px; background: #ffffff; border-radius: 12px;">
  <div style="text-align: center; padding: 20px 0; border-bottom: 1px solid #d2d2d7;">
    <h2 style="color: #0071e3; margin: 0; font-size: 20px;">${siteBrandName.value}</h2>
  </div>
  <div style="padding: 24px 0;">
    <p style="color: #1d1d1f; font-size: 16px; margin: 0 0 16px 0;">您好，</p>
    <p style="color: #86868b; font-size: 15px; line-height: 1.6; margin: 0 0 16px 0;">
      文档 <strong style="color: #0071e3;">【文档标题】</strong> 已被更新，请及时查看。
    </p>
    <div style="background: #f5f5f7; border-radius: 8px; padding: 16px; margin: 16px 0;">
      <p style="margin: 4px 0; color: #6e6e73; font-size: 14px;">文档地址</p>
      <p style="margin: 4px 0; color: #0071e3; font-size: 14px;">
        <a href="[document_url]" style="color: #0071e3; text-decoration: none;">[document_url]</a>
      </p>
    </div>
    <div style="text-align: center; margin-top: 24px;">
      <a href="[document_url]" style="display: inline-block; background: #0071e3; color: #ffffff; padding: 10px 24px; border-radius: 8px; text-decoration: none; font-size: 15px; font-weight: 500;">查看文档</a>
    </div>
  </div>
  <div style="border-top: 1px solid #d2d2d7; padding-top: 16px; text-align: center;">
    <p style="color: #6e6e73; font-size: 12px; margin: 0;">此邮件由 ${siteBrandName.value} 系统自动发送，请勿回复。</p>
    <p style="color: #6e6e73; font-size: 12px; margin: 4px 0 0 0;">${t('share.poweredBy', { app: siteBrandName.value })}</p>
  </div>
</div>`);

const sanitizedEmailTemplateHtml = computed(() => DOMPurify.sanitize(emailTemplateHtml.value));

const templatePreviewStyle = { fontFamily: AppleFont.text };
const templateTitleStyle = { fontFamily: AppleType.subhead.font, fontSize: AppleType.subhead.size, fontWeight: String(AppleType.subhead.weight), color: cv('text-primary'), margin: `0 0 ${AppleSpace.sm} 0`, paddingBottom: AppleSpace.sm, borderBottom: `1px solid ${cv('border-light')}` };
const templateBodyStyle = { padding: `${AppleSpace.sm} 0`, maxHeight: AppleSpace['400px'], overflowY: 'auto' as const };

const stepLayoutStyle = appleCardGrid('panel');

const stepCardOuterStyle = {
  overflow: 'hidden',
};

const stepCardHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  borderBottom: `1px solid ${cv('border-light')}`,
};

const stepHeaderLeftStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  flex: '1',
  minWidth: '0',
};

const stepHeaderInfoStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
  minWidth: '0',
};

const stepTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  margin: '0',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const stepMetaStyle = {
  display: 'flex',
  gap: AppleSpace.sm,
  alignItems: 'center',
};

const stepCardFooterStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'flex-end',
  gap: AppleSpace.xs,
  padding: `${AppleSpace.xs} ${AppleSpace.md}`,
  borderTop: `1px solid ${cv('border-light')}`,
};

const stepBodyStyle = {
  padding: AppleSpace.md,
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.lg,
};

const stepSectionStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
};

const stepBadgeStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
};

const stepBadgeNumStyle = {
  width: AppleSpace.md,
  height: AppleSpace.md,
  borderRadius: AppleRadius.circle,
  background: cv('blue'),
  color: cv('on-blue'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
  flexShrink: '0',
};

const stepBadgeLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
};

const stepInfoGridStyle = {
  display: 'grid',
  gridTemplateColumns: '1fr 1fr',
  gap: AppleSpace.sm,
};

const stepInfoItemStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
  padding: AppleSpace.xs,
  background: cv('bg-secondary'),
  borderRadius: AppleRadius.medium,
};

const stepInfoLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
};

const stepInfoValueStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const testRowStyle = {
  display: 'flex',
  gap: AppleSpace.sm,
  alignItems: 'center',
};

const testResultStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
};

const testResultTextStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
};
</script>
