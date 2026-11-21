<template>
  <div :style="aiSubPageStyle">
    <!-- 顶层 Tab：模板 / 版本管理 -->
    <ApSegmentedControl v-model="topTab" :options="topTabs" />

    <!-- 版本管理 Tab -->
    <AIPromptVersionView v-if="topTab === 'versions'" />

    <!-- 模板管理 Tab -->
    <template v-else>
    <!-- 工具栏 -->
    <div :style="toolbarStyle">
      <ApButton variant="primary" size="small" @click="openCreateModal">
        <ApIcon name="Plus" :size="16" />
        {{ $t('aiView2.k106') }}
      </ApButton>
    </div>

    <!-- 模板列表 -->
    <div :style="listStyle">
      <ApSkeleton v-if="loading" :lines="3" />
      <ApEmpty v-else-if="templates.length === 0" icon="ClipboardList" :title="$t('aiView2.k121')" :description="$t('vAiUi.k54')" />
      <div v-for="tpl in templates" :key="String(tpl.id)" :style="cardStyle">
        <div :style="aiSubSectionHeaderStyle">
          <div :style="cardTitleRowStyle">
            <span :style="cardTitleStyle">{{ tpl.name }}</span>
            <div :style="badgeRowStyle">
              <span v-if="tpl.is_built_in" :style="builtinBadgeStyle">{{ $t('aiView2.k107') }}</span>
              <span :style="metaBadgeStyle">{{ tpl.query_type }}</span>
              <span v-if="tpl.is_default" :style="defaultBadgeStyle">{{ $t('aiView2.k108') }}</span>
              <span v-if="!tpl.is_built_in" :style="tpl.is_active ? activeBadgeStyle : inactiveBadgeStyle">{{ tpl.is_active ? $t('vAiUi.enable') : $t('vAiUi.k56') }}</span>
            </div>
          </div>
          <div :style="actionsStyle">
            <ApButton variant="ghost" size="small" @click="toggleExpand(String(tpl.id))">
              <ApIcon v-if="!expanded[String(tpl.id)]" name="ChevronDown" :size="16" />
              <ApIcon v-else name="ChevronUp" :size="16" />
              {{ expanded[String(tpl.id)] ? $t('vAiUi.collapse') : $t('vAiUi.expand') }}
            </ApButton>
            <template v-if="!tpl.is_built_in">
              <ApButton variant="ghost" size="small" @click="openEditModal(tpl)">
                <ApIcon name="Pencil" :size="16" />
                {{ $t('aiView2.edit') }}
              </ApButton>
              <ApButton variant="ghost" size="small" @click="confirmDelete(tpl)">
                <ApIcon name="Trash2" :size="16" />
                {{ $t('aiView2.delete') }}
              </ApButton>
            </template>
          </div>
        </div>

        <div v-if="tpl.description" :style="descStyle">{{ tpl.description }}</div>

        <div v-if="expanded[String(tpl.id)]" :style="cardBodyStyle">
          <div :style="cardSectionStyle">
            <div :style="sectionHeaderRowStyle">
              <span :style="labelStyle">System</span>
            </div>
            <pre :style="bodyStyle">{{ tpl.system_prompt || $t('fallback.k2') }}</pre>
          </div>
          <div :style="cardSectionStyle">
            <div :style="sectionHeaderRowStyle">
              <span :style="labelStyle">User</span>
            </div>
            <pre :style="bodyStyle">{{ tpl.user_template }}</pre>
          </div>
          <div v-if="!tpl.is_built_in" :style="cardBodyActionRowStyle">
            <ApButton
              variant="ghost"
              size="small"
              :loading="togglingId === tpl.id"
              :disabled="togglingId !== null"
              @click="toggleActive(tpl)"
            >
              {{ tpl.is_active ? $t('vAiUi.k56') : $t('vAiUi.enable') }}
            </ApButton>
            <ApButton v-if="!tpl.is_default" variant="ghost" size="small" @click="setDefault(tpl)">
              {{ $t('aiView2.k111') }}
            </ApButton>
          </div>
        </div>
      </div>
    </div>

    <!-- 创建/编辑 Modal -->
    <ApModal v-model="showFormModal" :title="editingTpl ? '编辑 Prompt 模板' : '新建 Prompt 模板'" size="medium" density="compact">
      <div :style="formGroupStyle">
        <label :style="aiSubLabelStyle">{{ $t('aiView2.name') }} <span :style="requiredStyle">*</span></label>
        <ApInput v-model="formData.name" :placeholder="$t('aiView2.k116')" />
      </div>
      <div :style="formGroupStyle">
        <label :style="aiSubLabelStyle">{{ $t('aiView2.k112') }} <span :style="requiredStyle">*</span></label>
        <ApInput v-model="formData.query_type" :placeholder="$t('aiView2.k117')" />
      </div>
      <div :style="formGroupStyle">
        <label :style="aiSubLabelStyle">{{ $t('aiView2.desc') }}</label>
        <ApInput v-model="formData.description" :placeholder="$t('aiView2.k118')" />
      </div>
      <div :style="formGroupStyle">
        <label :style="aiSubLabelStyle">System Prompt</label>
        <ApTextarea v-model="formData.system_prompt" :rows="4" :placeholder="$t('aiView2.k119')" />
      </div>
      <div :style="formGroupStyle">
        <label :style="aiSubLabelStyle">User Template <span :style="requiredStyle">*</span></label>
        <ApTextarea v-model="formData.user_template" :rows="6" :placeholder="$t('aiView2.k120')" />
      </div>
      <div :style="formRowStyle">
        <div :style="formCheckboxGroupStyle">
          <ApCheckbox v-model="formData.is_active" :label="$t('aiView2.enable')" />
          <ApCheckbox v-model="formData.is_default" :label="$t('aiView2.k111')" />
        </div>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showFormModal = false">{{ $t('aiView2.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :disabled="saving || !formData.name.trim() || !formData.query_type.trim() || !formData.user_template.trim()" @click="saveTemplate">
          {{ saving ? $t('vAiUi.k58') : $t('vAiUi.save') }}
        </ApButton>
      </template>
    </ApModal>

    <!-- 删除确认 -->
    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('aiView2.k122')"
      :message="deleteMessage"
      :confirm-text="$t('aiView2.delete')"
      type="danger"
      @confirm="doDelete"
    />
    </template>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { logger } from '@/utils/logger';
import { ref, reactive, computed, onMounted } from 'vue';
import { useBreakpoint } from '@/utils/responsive';

const { isMobile } = useBreakpoint();
import ApIcon from '@/components/apple/ApIcon.vue';
import AIPromptVersionView from './settings/AIPromptVersionView.vue';

import { cv, AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleType, appleFixedColumns } from '@/utils/core/apple-tokens';
import { aiSubPageStyle, aiSubLabelStyle, formGroupStyle, listStyle as sharedListStyle, aiSubSectionStyle, aiSubSectionHeaderStyle, aiSubCardNameRowStyle, aiSubCardNameStyle, aiSubBodyStyle, aiSubActionRowStyle } from '@/styles/ai/ai-page-styles';
import { descStyle as sharedDescStyle } from '@/styles/layout/page-styles';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import { textToSQLApi } from '@/api/modules/ai/ai-data';
import type { CreatePromptTemplateRequest, PromptTemplateItem } from '@/api/modules/ai/ai-data';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();

const loading = ref(true);

const topTab = ref('templates');
const topTabs = [
  { value: 'templates', label: t('aiView2.k123'), icon: 'FileText' },
  { value: 'versions', label: t('aiView2.k124'), icon: 'GitBranch' },
];

const templates = ref<PromptTemplateItem[]>([]);
const expanded = ref<Record<string, boolean>>({});
const showFormModal = ref(false);
const showDeleteModal = ref(false);
const saving = ref(false);
const editingTpl = ref<PromptTemplateItem | null>(null);
const deleting = ref(false);
const deletingTpl = ref<PromptTemplateItem | null>(null);
const togglingId = ref<number | string | null>(null);

const deleteMessage = computed(() => `确定要删除模板「${deletingTpl.value?.name ?? ''}」吗？此操作不可撤销。`);

const formData = reactive<CreatePromptTemplateRequest>({
  name: '',
  query_type: '',
  system_prompt: '',
  user_template: '',
  description: '',
  is_active: true,
  is_default: false,
  sort: 0,
  space_id: null,
});

async function loadTemplates() {
  loading.value = true;
  try {
    const [dbRes, builtinRes] = await Promise.allSettled([
      textToSQLApi.listPromptTemplates(),
      textToSQLApi.listBuiltInPromptTemplates(),
    ]);

    let dbList: PromptTemplateItem[] = [];
    if (dbRes.status === 'fulfilled') {
      const r = dbRes.value as unknown as { templates?: PromptTemplateItem[] };
      dbList = Array.isArray(r?.templates) ? r.templates! : Array.isArray(dbRes.value) ? dbRes.value : [];
    }

    let builtinList: PromptTemplateItem[] = [];
    if (builtinRes.status === 'fulfilled') {
      const r = builtinRes.value as unknown as { templates?: PromptTemplateItem[] };
      builtinList = Array.isArray(r?.templates) ? r.templates! : Array.isArray(builtinRes.value) ? builtinRes.value : [];
    }

    templates.value = [...builtinList, ...dbList];
  } catch (e) {
    logger.error('Failed to load prompt templates:', e);
    ui.showToast(t('aiView2.k125'), 'warning');
  } finally {
    loading.value = false;
  }
}

onMounted(() => loadTemplates());

function toggleExpand(id: string) {
  expanded.value[id] = !expanded.value[id];
}

async function toggleActive(tpl: PromptTemplateItem) {
  // 请求期间锁住：原先双击会发两次 PUT，第二次带的是"翻转回原值"的意图，
  // 界面最后停在起点而两次请求都报成功 —— 用户看到的是"点了没反应"。
  // 锁的形态照 CitebaseCompileView 的 busy/isBusy（按 id 记，别的数据仍可操作）。
  if (togglingId.value !== null) return;
  togglingId.value = tpl.id;
  try {
    await textToSQLApi.togglePromptTemplate(tpl.id, !tpl.is_active);
    tpl.is_active = !tpl.is_active;
    ui.showToast(tpl.is_active ? '模板已启用' : '模板已停用', 'success');
  } catch (e) {
    logger.error('Failed to toggle template:', e);
    ui.showToast(t('aiView2.k126'), 'error');
  } finally {
    togglingId.value = null;
  }
}

async function setDefault(tpl: PromptTemplateItem) {
  try {
    await textToSQLApi.setDefaultPromptTemplate(tpl.id, tpl.space_id ?? undefined);
    templates.value.forEach(t => { t.is_default = (t.id === tpl.id); });
    ui.showToast(t('aiView2.k127'), 'success');
  } catch (e) {
    logger.error('Failed to set default template:', e);
    ui.showToast(t('aiView2.k126'), 'error');
  }
}

function openCreateModal() {
  editingTpl.value = null;
  Object.assign(formData, {
    name: '',
    query_type: 'generate',
    system_prompt: '',
    user_template: '',
    description: '',
    is_active: true,
    is_default: false,
    sort: 0,
    space_id: null,
  });
  showFormModal.value = true;
}

function openEditModal(tpl: PromptTemplateItem) {
  editingTpl.value = tpl;
  Object.assign(formData, {
    name: tpl.name,
    query_type: tpl.query_type,
    system_prompt: tpl.system_prompt || '',
    user_template: tpl.user_template,
    description: tpl.description || '',
    is_active: tpl.is_active,
    is_default: tpl.is_default,
    sort: tpl.sort,
    space_id: tpl.space_id,
  });
  showFormModal.value = true;
}

async function saveTemplate() {
  if (!formData.name.trim() || !formData.query_type.trim() || !formData.user_template.trim()) {
    ui.showToast(t('aiView2.k128'), 'warning');
    return;
  }
  saving.value = true;
  try {
    if (editingTpl.value) {
      await textToSQLApi.updatePromptTemplate(editingTpl.value.id, formData);
      ui.showToast(t('aiView2.k129'), 'success');
    } else {
      await textToSQLApi.createPromptTemplate(formData);
      ui.showToast(t('aiView2.k130'), 'success');
    }
    showFormModal.value = false;
    await loadTemplates();
  } catch (e) {
    logger.error('Failed to save template:', e);
    ui.showToast(t('aiView2.k131'), 'error');
  } finally {
    saving.value = false;
  }
}

function confirmDelete(tpl: PromptTemplateItem) {
  deletingTpl.value = tpl;
  showDeleteModal.value = true;
}

async function doDelete() {
  if (!deletingTpl.value || deleting.value) return;
  deleting.value = true;
  try {
    await textToSQLApi.deletePromptTemplate(deletingTpl.value.id);
    ui.showToast(t('aiView2.k132'), 'success');
    await loadTemplates();
  } catch (e) {
    logger.error('Failed to delete template:', e);
    ui.showToast(t('aiView2.k133'), 'error');
  } finally {
    deleting.value = false;
    deletingTpl.value = null;
  }
}

// ===== 样式 =====
const listStyle = sharedListStyle();
const cardStyle = aiSubSectionStyle;
const cardTitleRowStyle = { display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap' as const, gap: AppleSpace.sm };
const cardTitleStyle = aiSubCardNameStyle;
const badgeRowStyle = { ...aiSubCardNameRowStyle, gap: AppleSpace.xs };
const cardBodyStyle = { marginTop: AppleSpace.sm };
const bodyStyle = aiSubBodyStyle;
const cardSectionStyle = { marginBottom: AppleSpace.sm };
const sectionHeaderRowStyle = { display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: AppleSpace.xs };
const labelStyle = aiSubLabelStyle;
const descStyle = { ...sharedDescStyle, color: cv('text-tertiary'), marginBottom: AppleSpace.sm };

const metaBadgeStyle = { fontFamily: AppleFont.mono, fontSize: AppleType.legalMeta.size, color: cv('text-quaternary'), backgroundColor: cv('bg-secondary'), padding: `${AppleSpace.xs} ${AppleSpace.sm}`, borderRadius: AppleRadius.pill };
const defaultBadgeStyle = { fontSize: AppleType.legalMeta.size, padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, backgroundColor: cv('green-light'), color: cv('green'), fontWeight: String(AppleFontWeight.medium) };
const activeBadgeStyle = { fontSize: AppleType.legalMeta.size, padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, backgroundColor: cv('green-light'), color: cv('green') };
const inactiveBadgeStyle = { fontSize: AppleType.legalMeta.size, padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, backgroundColor: cv('bg-tertiary'), color: cv('text-quaternary') };
const builtinBadgeStyle = { fontSize: AppleType.legalMeta.size, padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, backgroundColor: cv('blue-light'), color: cv('blue'), fontWeight: String(AppleFontWeight.medium) };

const toolbarStyle = { display: 'flex', justifyContent: 'flex-end', marginBottom: AppleSpace.md };
const actionsStyle = { display: 'flex', gap: AppleSpace.xs, flexShrink: 0 };

const requiredStyle = { color: cv('red') };
const cardBodyActionRowStyle = { ...aiSubActionRowStyle, marginTop: AppleSpace.md, paddingTop: AppleSpace.sm, borderTop: `1px solid ${cv('border-light')}` };
const formRowStyle = computed(() => ({
  display: 'grid',
  gridTemplateColumns: isMobile.value ? '1fr' : appleFixedColumns(2),
  gap: AppleSpace.md,
  marginBottom: AppleSpace.md,
}));
const formCheckboxGroupStyle = { display: 'flex', gap: AppleSpace.md };
</script>
