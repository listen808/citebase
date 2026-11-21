<template>
  <ApPageContainer
    :title="$t('adminSys.k317')"
    :subtitle="$t('adminSys.k321')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <AdminCrudPage
      v-model:keyword="keyword"
      v-model:view-mode="viewMode"
      :error-text="loadError"
      :stats="[]"
      :filters="{ search: { placeholder: $t('vAdminUi.k42') } }"
      :loading="loading"
      @retry="retryLoad"
      @search="handleSearch"
    >
      <template #stats="{ statsStyle }">
        <div :style="statsStyle">
          <ApStatCard :label="$t('adminSys.k312')" :value="items.length" :icon="'Shield'" :color="cv('blue')" />
          <!-- 内置 / 自定义 都在数"这一页的角色"，只是按来源切片 —— 同一功能、只是计数 → 用功能符号 Shield。
               原先 内置=ShieldCheck 与「文档协作」导航符号撞字，Shield/ShieldCheck/ShieldAlert 三支盾牌
               其实是同一功能（角色）的变体，统一收到 Shield。 -->
          <ApStatCard :label="$t('adminSys.k313')" :value="items.filter(r => r.type === 1).length" :icon="'Shield'" :color="cv('purple')" />
          <ApStatCard :label="$t('adminSys.k314')" :value="items.filter(r => r.type !== 1).length" :icon="'Shield'" :color="cv('orange')" />
        </div>
      </template>
      <template #actions>
        <ApButton variant="primary" size="small" @click="openCreateModal">
          <ApIcon name="Plus" :size="16" />{{ $t('adminSys.k304') }}
        </ApButton>
      </template>
      <template #content>
        <div v-if="loading && !items.length" :style="loadingStyle">
          <ApSkeleton variant="card" :count="3" />
        </div>
        <ApEmpty v-else-if="!filteredRoles.length" :icon="keyword ? 'Search' : 'Shield'" :title="keyword ? $t('uiText.k16') : $t('uiText.k17')" :description="keyword ? $t('uiText.k18') : $t('uiText.k19')" />
        <div v-else :style="roleGridStyle">
          <ApCard
            v-for="role in filteredRoles"
            :key="role.role_id"
            hoverable
            size="small"
            @click="openEditModal(role)"
          >
            <div :style="roleCardHeaderStyle">
              <div :style="roleIconWrapStyle(getRoleColor(role.type))">
                <component :is="getRoleIcon(role.type)" :size="20" :color="cv('on-accent')" />
              </div>
              <div :style="roleCardInfoStyle">
                <div :style="roleNameStyle">{{ role.name }}</div>
                <div :style="roleDescStyle">{{ role.description || (role.type === 1 ? $t('vAdminUi.k49') : $t('vAdminUi.k39')) }}</div>
              </div>
              <span :style="roleTypeBadgeStyle(role.type)">{{ role.type === 1 ? $t('vAdminUi.k50') : $t('vAdminUi.k40') }}</span>
            </div>
            <div :style="roleCardActionsStyle">
              <ApButton variant="ghost" size="small" @click.stop="viewRoleUsers(role)">
                <ApIcon name="UserCheck" :size="16" />{{ $t('adminSys.k305') }}
              </ApButton>
              <ApButton variant="ghost" size="small" @click.stop="openPrivilegeModal(role)">
                <ApIcon name="ShieldCheck" :size="16" />{{ $t('adminSys.k306') }}
              </ApButton>
              <ApButton variant="ghost" size="small" :tooltip="$t('adminSys.k322')" :aria-label="$t('adminSys.k322')" icon-only @click.stop="openEditModal(role)">
                <ApIcon name="Pencil" :size="16" />
              </ApButton>
              <ApButton v-if="role.type !== 1" variant="danger" size="small" :tooltip="$t('adminSys.k318')" :aria-label="$t('adminSys.k318')" icon-only @click.stop="openDeleteModal(role)">
                <ApIcon name="Trash2" :size="16" />
              </ApButton>
            </div>
          </ApCard>
        </div>
      </template>
      <template #modals>
        <ApConfirmModal
          v-model="showDeleteModal"
          :title="$t('adminSys.k318')"
          :message="deleteMessage"
          :confirm-text="$t('adminSys.delete')"
          :cancel-text="$t('adminSys.cancel')"
          type="danger"
          @confirm="handleDelete"
        />

        <ApModal :model-value="showCreateModal || showEditModal" :title="isEdit ? $t('uiText.k20') : $t('uiText.k21')" size="small" density="compact" @update:model-value="(v: boolean) => { if (!v) closeModals() }">
          <ApFormItem :label="$t('adminSys.k315')" required :error="formErrors.name">
            <ApInput v-model="nameForm.name" :placeholder="$t('adminSys.k316')" />
          </ApFormItem>
          <template #footer>
            <ApButton variant="ghost" size="small" @click="closeModals">{{ $t('adminSys.cancel') }}</ApButton>
            <ApButton variant="primary" size="small" :disabled="submitting" @click="onSave">{{ submitting ? $t('vAdminUi.k1') : $t('vAdminUi.save') }}</ApButton>
          </template>
        </ApModal>

        <ApModal v-model="showPrivModal" :title="$t('adminSys.k319')" size="medium" density="compact">
          <div v-if="privilegeLoading" :style="loadingStyle">
            <ApSkeleton variant="list" :lines="5" />
          </div>
          <div v-else>
            <div :style="{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: AppleSpace.xs }">
              <p :style="{ fontFamily: AppleFont.text, fontSize: AppleType.controlLabel.size, color: cv('text-secondary'), marginBottom: '0' }">{{ $t('adminSys.k307') }}</p>
              <div :style="{ display: 'flex', gap: AppleSpace.xs }">
                <ApButton variant="ghost" size="small" @click="selectAll">{{ $t('adminSys.selectAll') }}</ApButton>
                <ApButton variant="ghost" size="small" @click="invertSelection">{{ $t('adminSys.invert') }}</ApButton>
              </div>
            </div>
            <div :style="{ maxHeight: AppleSpace['400px'], overflowY: 'auto' }">
              <div v-for="node in privilegeTree" :key="node.id" :style="{ marginBottom: AppleSpace.xs }">
                <label :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.xs, fontWeight: String(AppleFontWeight.semibold), fontSize: AppleType.subhead.size, cursor: 'pointer' }">
                  <ApCheckbox :model-value="selectedPrivs.includes(node.id)" @update:model-value="(val: boolean) => onParentChange(node, val)" />
                  {{ node.name }}
                </label>
                <div v-if="node.children?.length" :style="{ paddingLeft: AppleSpace.lg }">
                  <div v-for="child in node.children" :key="child.id" :style="{ marginBottom: AppleSpace.xxs }">
                    <label :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.xs, fontWeight: String(AppleFontWeight.regular), fontSize: AppleType.controlLabel.size, cursor: 'pointer' }">
                      <ApCheckbox :model-value="selectedPrivs.includes(child.id)" @update:model-value="(val: boolean) => onChildChange(node, child, val)" />
                      {{ child.name }}
                    </label>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <template #footer>
            <ApButton variant="ghost" size="small" @click="showPrivModal = false">{{ $t('adminSys.cancel') }}</ApButton>
            <ApButton variant="primary" size="small" :disabled="submitting" @click="handlePrivSubmit">{{ submitting ? $t('vAdminUi.k1') : $t('vAdminUi.save') }}</ApButton>
          </template>
        </ApModal>

        <ApModal v-model="showPrivConfirmModal" :title="$t('adminSys.k320')" size="small" density="compact">
          <div :style="{ marginBottom: AppleSpace.md }">
            <p :style="{ fontFamily: AppleFont.text, fontSize: AppleType.bodyPrimary.size, color: cv('text-secondary'), margin: 0 }">
              {{ $t('adminSys.k310') }}
            </p>
          </div>
          <div :style="{ display: 'flex', flexDirection: 'column', gap: AppleSpace.sm, maxHeight: AppleSpace['300px'], overflowY: 'auto' }">
            <div v-if="selectedPrivs.filter(id => !originalPrivs.includes(id)).length > 0">
              <p :style="{ fontFamily: AppleFont.text, fontSize: AppleType.subhead.size, fontWeight: String(AppleFontWeight.semibold), color: cv('green'), margin: `0 0 ${AppleSpace.xs} 0` }">
                {{ $t('uiText.k22', { n: selectedPrivs.filter(id => !originalPrivs.includes(id)).length }) }}
              </p>
              <div :style="{ paddingLeft: AppleSpace.sm }">
                <p v-for="id in selectedPrivs.filter(id => !originalPrivs.includes(id))" :key="id" :style="{ fontFamily: AppleFont.text, fontSize: AppleType.microUI.size, color: cv('text-secondary'), margin: `${AppleSpace.xxs} 0` }">
                  {{ getPrivilegeName(id) }}
                </p>
              </div>
            </div>
            <div v-if="originalPrivs.filter(id => !selectedPrivs.includes(id)).length > 0">
              <p :style="{ fontFamily: AppleFont.text, fontSize: AppleType.subhead.size, fontWeight: String(AppleFontWeight.semibold), color: cv('red'), margin: `0 0 ${AppleSpace.xs} 0` }">
                {{ $t('uiText.k23', { n: originalPrivs.filter(id => !selectedPrivs.includes(id)).length }) }}
              </p>
              <div :style="{ paddingLeft: AppleSpace.sm }">
                <p v-for="id in originalPrivs.filter(id => !selectedPrivs.includes(id))" :key="id" :style="{ fontFamily: AppleFont.text, fontSize: AppleType.microUI.size, color: cv('text-secondary'), margin: `${AppleSpace.xxs} 0` }">
                  {{ getPrivilegeName(id) }}
                </p>
              </div>
            </div>
          </div>
          <template #footer>
            <ApButton variant="ghost" size="small" @click="cancelPrivConfirm">{{ $t('adminSys.k311') }}</ApButton>
            <ApButton variant="primary" size="small" :disabled="submitting" @click="confirmPrivSubmit">{{ submitting ? $t('vAdminUi.k1') : $t('vAdminUi.k41') }}</ApButton>
          </template>
        </ApModal>
      </template>
    </AdminCrudPage>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, onMounted, computed } from 'vue';
import { useRouter } from 'vue-router';
import { icons } from '@/utils/icons';
import ApIcon from '@/components/apple/ApIcon.vue';
import { cv, AppleLayout, AppleSpace, AppleFont, AppleFontWeight, AppleType, AppleRadius, appleCardGrid } from '@/utils/core/apple-tokens';
import { loadingStyle } from '@/styles/layout/page-styles';
import { useUIStore } from '@/stores/app/ui';
import { useAdminCrud } from '@/composables/shared/useAdminCrud';
import { getRoles, createRole, updateRole, deleteRole, getPrivileges, getRolePrivileges, assignPrivileges } from '@/api/modules/user/role';
import type { Role } from '@/types/models';
import type { PrivilegeNode } from '@/api/modules/user/role';
import AdminCrudPage from '@/components/admin/AdminCrudPage.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApCheckbox from '@/components/apple/ApCheckbox.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import ApCard from '@/components/apple/ApCard.vue';

const ui = useUIStore();
const router = useRouter();

type RoleItem = Role & { id: number };

const {
  loading, loadError, retryLoad, items,
  keyword, viewMode,
  showCreateModal, showEditModal, showDeleteModal, editingItem,
  formErrors,
  fetchList, handleSearch, openCreate, openEdit, openDelete, closeModals,
  handleDelete, handleSubmit,
} = useAdminCrud<RoleItem>({
  fetchFn: async () => {
    const data = await getRoles();
    const mapped = data.map(r => ({ ...r, id: r.role_id }));
    return { data: mapped, total: mapped.length, page: 1, page_size: 200 };
  },
  createFn: (d) => createRole(d.name!),
  updateFn: (d) => updateRole(d.id!, d.name!),
  deleteFn: (id) => deleteRole(id),
  itemName: t('uiText.k24'),
  resetForm: () => { nameForm.value = { name: '' }; },
});

Object.assign(formErrors, { name: '' });

const isEdit = computed(() => !!editingItem.value);
const submitting = shallowRef(false);
const nameForm = ref({ name: '' });
const showPrivModal = shallowRef(false);
const showPrivConfirmModal = shallowRef(false);
const privilegeLoading = shallowRef(false);
const privilegeTree = ref<PrivilegeNode[]>([]);
const selectedPrivs = ref<number[]>([]);
const originalPrivs = ref<number[]>([]);
const currentRoleId = shallowRef<number | null>(null);
const deleteMessage = shallowRef('');

const filteredRoles = computed(() => {
  const q = keyword.value.trim().toLowerCase();
  if (!q) return items.value;
  return items.value.filter(r => r.name.toLowerCase().includes(q));
});

const roleGridStyle = appleCardGrid('entity');

const roleCardHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  padding: AppleSpace.md,
  paddingBottom: AppleSpace.sm,
};

function roleIconWrapStyle(color: string) {
  return {
    width: AppleSpace.lg,
    height: AppleSpace.lg,
    borderRadius: AppleRadius.medium,
    background: color,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: '0',
  };
}

const roleCardInfoStyle = {
  flex: '1',
  minWidth: '0',
};

const roleNameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
};

const roleDescStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-tertiary'),
  marginTop: AppleSpace.xxs,
};

function roleTypeBadgeStyle(type: number) {
  return {
    fontFamily: AppleFont.text,
    fontSize: AppleType.microUI.size,
    padding: `${AppleSpace.xxs} ${AppleSpace.xs}`,
    borderRadius: AppleRadius.pill,
    background: type === 1 ? cv('bg-secondary') : cv('bg-tertiary'),
    color: cv('text-secondary'),
    flexShrink: '0',
  };
}

const roleCardActionsStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
  padding: `${AppleSpace.xs} ${AppleSpace.md}`,
  borderTop: `1px solid ${cv('border-light')}`,
};

function getRoleColor(type: number): string {
  if (type === 1) return cv('purple');
  if (type === 2) return cv('blue');
  return cv('text-tertiary');
}

function getRoleIcon(type: number) {
  if (type === 1) return icons['Crown'];
  if (type === 2) return icons['Shield'];
  return icons['User'];
}

function openCreateModal() {
  nameForm.value = { name: '' };
  openCreate();
}

function openEditModal(role: RoleItem) {
  nameForm.value = { name: role.name };
  openEdit(role);
}

function openDeleteModal(role: RoleItem) {
  if (role.type === 1) return;
  deleteMessage.value = t('sentUi.k27', { p1: role.name });
  openDelete(role);
}

async function openPrivilegeModal(role: Role) {
  currentRoleId.value = role.role_id;
  privilegeLoading.value = true;
  showPrivModal.value = true;
  try {
    const [tree, privs] = await Promise.all([getPrivileges(), getRolePrivileges(currentRoleId.value!)]);
    privilegeTree.value = tree;
    selectedPrivs.value = [...privs];
    originalPrivs.value = [...privs];
  } catch {
    ui.showToast(t('adminSys.k300'), 'error');
  } finally {
    privilegeLoading.value = false;
  }
}

async function onSave() {
  if (submitting.value) return;
  submitting.value = true;
  try {
    await handleSubmit(
      { name: nameForm.value.name } as Partial<RoleItem>,
      (data) => {
        const errors: Record<string, string> = {};
        if (!data.name?.trim()) errors.name = t('uiText.k25');
        return Object.keys(errors).length > 0 ? errors : null;
      }
    );
  } finally {
    submitting.value = false;
  }
}

async function handlePrivSubmit() {
  if (!currentRoleId.value || submitting.value) return;

  const added = selectedPrivs.value.filter(id => !originalPrivs.value.includes(id));
  const removed = originalPrivs.value.filter(id => !selectedPrivs.value.includes(id));

  if (added.length === 0 && removed.length === 0) {
    ui.showToast(t('adminSys.k323'), 'info');
    showPrivModal.value = false;
    return;
  }

  showPrivModal.value = false;
  showPrivConfirmModal.value = true;
}

async function confirmPrivSubmit() {
  if (!currentRoleId.value || submitting.value) return;
  submitting.value = true;
  try {
    await assignPrivileges(currentRoleId.value, [...selectedPrivs.value]);
    ui.showToast(t('adminSys.k324'), 'success');
    showPrivConfirmModal.value = false;
  } catch (e: unknown) {
    ui.showToast((e as Error).message || t('fallback.k64'), 'error');
  } finally {
    submitting.value = false;
  }
}

function cancelPrivConfirm() {
  showPrivConfirmModal.value = false;
  showPrivModal.value = true;
}

async function viewRoleUsers(role: RoleItem) {
  router.push(`/roles/${role.role_id}/users`);
}

function allPrivilegeIds(): number[] {
  const ids: number[] = [];
  for (const node of privilegeTree.value) {
    ids.push(node.id);
    if (node.children) {
      for (const child of node.children) ids.push(child.id);
    }
  }
  return ids;
}

function selectAll() {
  selectedPrivs.value = allPrivilegeIds();
}

function invertSelection() {
  const all = allPrivilegeIds();
  const set = new Set(selectedPrivs.value);
  selectedPrivs.value = all.filter(id => !set.has(id));
}

function onParentChange(node: PrivilegeNode, checked: boolean) {
  const childIds = (node.children || []).map(c => c.id);
  if (checked) {
    const set = new Set(selectedPrivs.value);
    set.add(node.id);
    for (const cid of childIds) set.add(cid);
    selectedPrivs.value = [...set];
  } else {
    const removeSet = new Set([node.id, ...childIds]);
    selectedPrivs.value = selectedPrivs.value.filter(id => !removeSet.has(id));
  }
}

function onChildChange(node: PrivilegeNode, child: PrivilegeNode, checked: boolean) {
  const set = new Set(selectedPrivs.value);
  if (checked) {
    set.add(child.id);
    const childIds = (node.children || []).map(c => c.id);
    if (childIds.every(cid => set.has(cid))) {
      set.add(node.id);
    }
  } else {
    set.delete(child.id);
    set.delete(node.id);
  }
  selectedPrivs.value = [...set];
}

onMounted(() => { fetchList(); });

function getPrivilegeName(privId: number): string {
  for (const node of privilegeTree.value) {
    if (node.id === privId) return node.name;
    if (node.children) {
      for (const child of node.children) {
        if (child.id === privId) return child.name;
      }
    }
  }
  return t('uiText.k26', { p1: privId });
}
</script>
