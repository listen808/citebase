<template>
  <ApPageContainer
    :title="$t('adminSys.k290')"
    :subtitle="$t('adminSys.k293')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >

    <AdminCrudPage
      :stats="stats"
      :loading="loading"
      :show-empty="!loading && !filteredTree.length"
      :empty-icon="'LockKeyhole'"
      :empty-text="$t('adminSys.k296')"
    >
      <template #filter-extra>
        <div :style="crudSearchStyle()">
          <ApInput v-model="searchText" :aria-label="$t('adminSys.k294')" :placeholder="$t('adminSys.k282')">
            <template #prefix>
              <ApIcon name="Search" :size="14" :color="cv('text-tertiary')" />
            </template>
          </ApInput>
        </div>
      </template>
      <template #actions>
        <ApButton variant="primary" size="small" @click="openCreateModal()">
          <ApIcon name="Plus" :size="16" />{{ $t('adminSys.k271') }}
        </ApButton>
      </template>

      <template #content>
        <div :style="groupContainerStyle">
      <ApCard v-for="node in filteredTree" :key="node.id" size="small">
        <div :style="groupHeaderStyle" @click="toggleGroup(node.id)">
          <div :style="groupHeaderLeftStyle">
            <div :style="iconBoxStyle(cv('purple'), 32)">
              <component :is="getIconComponent(node.icon)" :size="16" :color="cv('on-accent')" />
            </div>
            <div :style="groupHeaderInfoStyle">
              <span :style="groupHeaderNameStyle">{{ node.name }}</span>
              <!-- P2-13：「显示 · 排序 0」两个裸值没有表头解释，补 title 说明这组数的含义 -->
              <span :style="groupHeaderMetaStyle" :title="$t('adminSys.k397')">{{ node.is_display === 1 ? $t('common.visible') : $t('common.hidden') }} · {{ $t('admin.sort') }} {{ node.sequence || 0 }}</span>
            </div>
          </div>
          <div :style="groupHeaderRightStyle">
            <span :style="groupCountBadgeStyle">{{ (node.children?.length ?? 0) }}</span>
            <component :is="expandedGroups.has(node.id) ? icons['ChevronUp'] : icons['ChevronDown']" :size="16" :color="cv('text-tertiary')" />
          </div>
        </div>

        <div v-if="expandedGroups.has(node.id)" :style="groupBodyStyle">
          <div :style="groupActionsRowStyle">
            <ApButton variant="ghost" size="small" @click="openCreateModal(node)">
              <ApIcon name="Plus" :size="16" />{{ $t('adminSys.k272') }}
            </ApButton>
            <ApButton variant="ghost" size="small" icon-only :tooltip="$t('adminSys.k295')" :aria-label="$t('adminSys.k295')" @click="openEditModal(node)">
              <ApIcon name="Pencil" :size="16" />
            </ApButton>
            <ApButton variant="ghost" size="small" icon-only :tooltip="$t('adminSys.k292')" :aria-label="$t('adminSys.k292')" @click="openDeleteModal(node)">
              <ApIcon name="Trash2" :size="16" :style="{ color: cv('red') }" />
            </ApButton>
          </div>

          <div v-if="node.children?.length" :style="privilegeListStyle">
            <div
              v-for="child in filterChildren(node.children)"
              :key="child.id"
              :style="privilegeRowStyle"
            >
              <div :style="privilegeRowLeftStyle">
                <div :style="privilegeRowInfoStyle">
                  <span :style="privilegeRowNameStyle">{{ child.name }}</span>
                  <span :style="privilegeRowMetaStyle">
                    {{ child.controller }}.{{ child.action }}
                    <span v-if="child.target">→ {{ child.target }}</span>
                  </span>
                </div>
              </div>
              <div :style="privilegeRowActionsStyle">
                <ApButton variant="ghost" size="small" icon-only :tooltip="$t('adminSys.k295')" :aria-label="$t('adminSys.k295')" @click="openEditModal(child)">
                  <ApIcon name="Pencil" :size="16" />
                </ApButton>
                <ApButton variant="ghost" size="small" icon-only :tooltip="$t('adminSys.k292')" :aria-label="$t('adminSys.k292')" @click="openDeleteModal(child)">
                  <ApIcon name="Trash2" :size="16" :style="{ color: cv('red') }" />
                </ApButton>
              </div>
            </div>
          </div>
          <ApEmpty v-else icon="File" :style="{ padding: `${AppleSpace.lg} 0` }" :title="$t('adminSys.k291')" />
        </div>
      </ApCard>
    </div>
      </template>

      <template #modals>
    <ApModal :model-value="showFormModal" :title="formModalTitle" size="medium" density="compact" @update:model-value="(v: boolean) => { if (!v) closeFormModal() }">
      <div :style="formStyle">
        <ApFormItem :label="$t('adminSys.k273')" required :error="formErrors.name">
          <ApInput v-model="privilegeForm.name" :placeholder="$t('adminSys.k283')" />
        </ApFormItem>
        <ApFormItem :label="$t('adminSys.k274')" required :associate="false">
          <div :style="{ display: 'flex', gap: AppleSpace.sm }">
            <ApRadio v-model="privilegeForm.type" value="menu" :label="$t('adminSys.k275')" :disabled="isEdit" />
            <ApRadio v-model="privilegeForm.type" value="controller" :label="$t('adminSys.k276')" :disabled="isEdit" />
          </div>
        </ApFormItem>
        <ApFormItem v-if="privilegeForm.type === 'controller'" :label="$t('adminSys.k277')">
          <ApSelect
            :model-value="privilegeForm.parent_id || null"
            :options="menuOptions"
            :placeholder="$t('adminSys.k284')"
            @update:model-value="(v: unknown) => privilegeForm.parent_id = v as number | undefined"
          />
        </ApFormItem>
        <ApFormItem v-if="privilegeForm.type === 'controller'" :label="$t('adminSys.k276')">
          <ApInput v-model="privilegeForm.controller" :placeholder="$t('adminSys.k285')" />
        </ApFormItem>
        <ApFormItem v-if="privilegeForm.type === 'controller'" :label="$t('adminSys.actions')">
          <ApInput v-model="privilegeForm.action" :placeholder="$t('adminSys.k286')" />
        </ApFormItem>
        <div :style="formRowStyle">
          <ApFormItem :label="$t('adminSys.k279')" :style="{ flex: '1' }">
            <div :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.sm }">
              <ApInput v-model="privilegeForm.icon" :placeholder="$t('adminSys.k287')" :style="{ flex: '1' }" />
              <div v-if="privilegeForm.icon" :style="iconPreviewStyle">
                <component :is="getIconComponent(privilegeForm.icon)" :size="16" />
              </div>
            </div>
          </ApFormItem>
          <ApFormItem :label="$t('adminSys.sort')" :style="{ flex: '1' }">
            <ApInput :model-value="String(privilegeForm.sequence)" type="number" :placeholder="$t('adminSys.k288')" @update:model-value="(v: string) => privilegeForm.sequence = Number(v) || 0" />
          </ApFormItem>
        </div>
        <div :style="formRowStyle">
          <ApFormItem :label="$t('adminSys.k280')">
            <ApSwitch :model-value="privilegeForm.is_display === 1" @update:model-value="(v: boolean) => privilegeForm.is_display = v ? 1 : 0" />
          </ApFormItem>
          <ApFormItem v-if="privilegeForm.type === 'controller'" :label="$t('adminSys.k281')">
            <ApInput v-model="privilegeForm.target" :placeholder="$t('adminSys.k289')" />
          </ApFormItem>
        </div>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" :disabled="submitting" @click="closeFormModal">{{ $t('adminSys.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :disabled="submitting" @click="handleFormSubmit">{{ submitting ? $t('vAdminUi.k1') : $t('vAdminUi.save') }}</ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('adminSys.k292')"
      :message="deleteMessage"
      :confirm-text="$t('adminSys.delete')"
      :cancel-text="$t('adminSys.cancel')"
      type="danger"
      @confirm="handleDeleteConfirm"
    />
      </template>
    </AdminCrudPage>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted } from 'vue';
import { icons } from '@/utils/icons';
import ApIcon from '@/components/apple/ApIcon.vue';
import { cv, AppleLayout, AppleRadius, AppleSpace, AppleFont, AppleFontWeight, AppleType, AppleTransition } from '@/utils/core/apple-tokens';
import { crudSearchStyle, formStyle, formRowStyle, iconBoxStyle } from '@/styles/layout/page-styles';
import { useUIStore } from '@/stores/app/ui';
import { getPrivileges, createPrivilege, updatePrivilege, deletePrivilege } from '@/api/modules/user/role';
import type { PrivilegeNode } from '@/api/modules/user/role';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApSwitch from '@/components/apple/ApSwitch.vue';
import ApRadio from '@/components/apple/ApRadio.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApCard from '@/components/apple/ApCard.vue';
import AdminCrudPage from '@/components/admin/AdminCrudPage.vue';
import type { StatConfig } from '@/components/admin/AdminCrudPage.vue';

const ui = useUIStore();

const iconMap: Record<string, (typeof icons)[keyof typeof icons]> = {
  home: icons['Home'], folder: icons['Folder'], file: icons['FileText'], settings: icons['Settings'],
  users: icons['Users'], user: icons['User'], key: icons['Key'], eye: icons['Eye'], lock: icons['Lock'],
  globe: icons['Globe'], monitor: icons['Monitor'], mail: icons['Mail'], bell: icons['Bell'],
  image: icons['Image'], link: icons['Link'], database: icons['Database'], chart: icons['BarChart3'],
  message: icons['MessageSquare'], share: icons['Share2'], download: icons['Download'], upload: icons['Upload'],
  wrench: icons['Wrench'], zap: icons['Zap'], book: icons['BookOpen'], star: icons['Star'], archive: icons['Archive'],
  tag: icons['Tag'], hash: icons['Hash'], calendar: icons['Calendar'], clock: icons['Clock'], pin: icons['MapPin'],
  phone: icons['Phone'], atsign: icons['AtSign'], shield: icons['Shield'], shieldcheck: icons['ShieldCheck'],
  alert: icons['AlertCircle'], help: icons['HelpCircle'], activity: icons['Activity'],
};

function getIconComponent(iconName?: string) {
  if (!iconName) return icons['FileText'];
  return iconMap[iconName.toLowerCase()] || icons['FileText'];
}

const loading = shallowRef(false);
const submitting = shallowRef(false);
const treeData = ref<PrivilegeNode[]>([]);
const searchText = shallowRef('');
const expandedGroups = ref<Set<number>>(new Set());

const allPrivileges = computed(() => {
  const result: PrivilegeNode[] = [];
  for (const node of treeData.value) {
    result.push(node);
    if (node.children) result.push(...node.children);
  }
  return result;
});

const menuPrivileges = computed(() => treeData.value);
const controllerPrivileges = computed(() => {
  const result: PrivilegeNode[] = [];
  for (const node of treeData.value) {
    if (node.children) result.push(...node.children);
  }
  return result;
});

const stats = computed<StatConfig[]>(() => [
  { label: t('adminSys.k297'), value: allPrivileges.value.length, icon: 'LockKeyhole', color: cv('blue') },
  { label: t('adminSys.k298'), value: menuPrivileges.value.length, icon: 'Layout', color: cv('purple') },
  { label: t('adminSys.k299'), value: controllerPrivileges.value.length, icon: 'Code', color: cv('orange') },
]);

const filteredTree = computed(() => {
  if (!searchText.value.trim()) return treeData.value;
  const q = searchText.value.toLowerCase();
  return treeData.value
    .map(node => {
      const nameMatch = node.name.toLowerCase().includes(q);
      const filteredChildren = node.children?.filter(c =>
        c.name.toLowerCase().includes(q) || c.controller?.toLowerCase().includes(q) || c.action?.toLowerCase().includes(q)
      );
      if (nameMatch || (filteredChildren && filteredChildren.length > 0)) {
        return { ...node, children: filteredChildren || [] };
      }
      return null;
    })
    .filter(Boolean) as PrivilegeNode[];
});

function filterChildren(children: PrivilegeNode[]): PrivilegeNode[] {
  if (!searchText.value.trim()) return children;
  const q = searchText.value.toLowerCase();
  return children.filter(c =>
    c.name.toLowerCase().includes(q) || c.controller?.toLowerCase().includes(q) || c.action?.toLowerCase().includes(q)
  );
}

const menuOptions = computed(() =>
  treeData.value.map(n => ({ label: n.name, value: n.id }))
);

const showFormModal = shallowRef(false);
const isEdit = shallowRef(false);
const editingId = shallowRef<number | null>(null);
const formErrors = ref<Record<string, string>>({});

const privilegeForm = ref({
  name: '',
  type: 'controller' as string,
  parent_id: undefined as number | undefined,
  controller: '',
  action: '',
  icon: '',
  target: '',
  is_display: 1 as number,
  sequence: 0 as number,
});

const formModalTitle = computed(() => (isEdit.value ? t('uiText.k42') : t('uiText.k43')));

const showDeleteModal = shallowRef(false);
const deletingItem = shallowRef<PrivilegeNode | null>(null);
const deleteMessage = computed(() => (deletingItem.value?.children?.length
  ? t('uiText.k45', { p1: deletingItem.value?.name })
  : t('uiText.k44', { p1: deletingItem.value?.name })));

function toggleGroup(id: number) {
  const next = new Set(expandedGroups.value);
  if (next.has(id)) {
    next.delete(id);
  } else {
    next.add(id);
  }
  expandedGroups.value = next;
}

async function loadTree() {
  loading.value = true;
  try {
    treeData.value = await getPrivileges();
    const allIds = treeData.value.map(n => n.id);
    expandedGroups.value = new Set(allIds);
  } catch {
    ui.showToast(t('adminSys.k300'), 'error');
  } finally {
    loading.value = false;
  }
}

function openCreateModal(parent?: PrivilegeNode) {
  isEdit.value = false;
  editingId.value = null;
  formErrors.value = {};
  privilegeForm.value = {
    name: '',
    type: parent ? 'controller' : 'menu',
    parent_id: parent?.id,
    controller: '',
    action: '',
    icon: '',
    target: '',
    is_display: 1,
    sequence: 0,
  };
  showFormModal.value = true;
}

function openEditModal(item: PrivilegeNode) {
  isEdit.value = true;
  editingId.value = item.id;
  formErrors.value = {};
  privilegeForm.value = {
    name: item.name,
    type: item.type || 'menu',
    parent_id: item.parent_id || undefined,
    controller: item.controller || '',
    action: item.action || '',
    icon: item.icon || '',
    target: item.target || '',
    is_display: item.is_display ?? 1,
    sequence: item.sequence ?? 0,
  };
  showFormModal.value = true;
}

function closeFormModal() {
  showFormModal.value = false;
  editingId.value = null;
}

function openDeleteModal(item: PrivilegeNode) {
  deletingItem.value = item;
  showDeleteModal.value = true;
}

async function handleFormSubmit() {
  if (submitting.value) return;
  const errors: Record<string, string> = {};
  if (!privilegeForm.value.name.trim()) errors.name = t('uiText.k46');
  if (Object.keys(errors).length > 0) {
    formErrors.value = errors;
    return;
  }
  submitting.value = true;
  try {
    if (isEdit.value && editingId.value) {
      await updatePrivilege(editingId.value, { ...privilegeForm.value });
      ui.showToast(t('adminSys.k301'), 'success');
    } else {
      await createPrivilege({ ...privilegeForm.value });
      ui.showToast(t('adminSys.k302'), 'success');
    }
    closeFormModal();
    await loadTree();
  } catch (e: unknown) {
    ui.showToast((e as Error).message || t('fallback.k64'), 'error');
  } finally {
    submitting.value = false;
  }
}

async function handleDeleteConfirm() {
  if (!deletingItem.value) return;
  try {
    await deletePrivilege(deletingItem.value.id);
    ui.showToast(t('adminSys.k303'), 'success');
    showDeleteModal.value = false;
    await loadTree();
  } catch (e: unknown) {
    ui.showToast((e as Error).message || t('fallback.k20'), 'error');
  }
}

onMounted(() => { loadTree(); });

const groupContainerStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
};

const groupHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `${AppleSpace.md} ${AppleSpace.lg}`,
  cursor: 'pointer',
  transition: AppleTransition.default,
};

const groupHeaderLeftStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.md,
  flex: '1',
  minWidth: '0',
};

const groupHeaderInfoStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
  minWidth: '0',
};

const groupHeaderNameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
};

const groupHeaderMetaStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

const groupHeaderRightStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  flexShrink: '0',
};

const groupCountBadgeStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-secondary'),
  background: cv('bg-tertiary'),
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.pill,
};

const groupBodyStyle = {
  borderTop: `1px solid ${cv('border-light')}`,
  padding: `${AppleSpace.md} ${AppleSpace.lg}`,
  background: cv('bg-secondary'),
};

const groupActionsRowStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
  marginBottom: AppleSpace.md,
};

const privilegeListStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
};

const privilegeRowStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  borderRadius: AppleRadius.small,
  background: cv('surface-panel'),
  // P1-1：半透明行脱离表头后同色于画布，补发丝边维持行边界
  border: `1px solid ${cv('border-light')}`,
  gap: AppleSpace.md,
};

const privilegeRowLeftStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.md,
  flex: '1',
  minWidth: '0',
};

const privilegeRowInfoStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
  minWidth: '0',
};

const privilegeRowNameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
};

const privilegeRowMetaStyle = {
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

const privilegeRowActionsStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  flexShrink: '0',
};

const iconPreviewStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  width: AppleSpace.xl,
  height: AppleSpace.xl,
  borderRadius: AppleRadius.small,
  background: cv('bg-tertiary'),
  color: cv('text-secondary'),
};
</script>
