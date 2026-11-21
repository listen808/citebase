<template>
  <ApPageContainer
    :title="$t('adminSys.k360')"
    :subtitle="$t('adminSys.k365')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <AdminCrudPage
      v-model:keyword="keyword"
      v-model:view-mode="viewMode"
      :error-text="loadError"
      :stats="[]"
      density="compact"
      :filters="{ search: { placeholder: $t('vAdminUi.k48') }, viewToggle: true }"
      :loading="loading"
      @retry="retryLoad"
      @search="handleSearch"
    >
      <!-- 只有 1 个统计：不再独占一行卡片面，换成工具条行首的紧凑计数（同一条数据、同一档数值） -->
      <template #filter-extra>
        <ApStatChip :icon="'Users'" :color="cv('blue')" :label="$t('adminSys.k338')" :value="total" />
        <!-- 定宽：ApSelect 根是 width:100%，不兜住会把整条工具条撑到换行 -->
        <div :style="filterSelectGroupStyle()">
          <ApSelect
            v-model="roleFilter"
            :options="roleOptions"
            :label="$t('adminSys.k339')"
            @update:model-value="() => { page = 1; fetchList(); }"
          />
        </div>
      </template>
      <template #actions>
        <ApButton variant="ghost" size="small" :disabled="exporting" @click="handleExportCSV">
          <ApIcon name="Download" :size="16" />{{ exporting ? $t('vAdminUi.k46') : $t('vAdminUi.k47') }}
        </ApButton>
        <ApButton variant="primary" size="small" @click="openCreateModal">
          <ApIcon name="Plus" :size="16" />{{ $t('adminSys.k335') }}
        </ApButton>
      </template>
      <template #content>

    <!-- 卡片视图 (桌面卡片 + 移动端列表共用) -->
    <template v-if="viewMode === 'card' || isMobile">
      <div v-if="loading && !items.length" :style="loadingStyle">
        <ApSkeleton variant="grid" :count="3" />
      </div>
      <ApEmpty v-else-if="!items.length" icon="Users" :title="$t('adminSys.k361')" :description="$t('vAdminUi.k45')" />
      <div v-else :style="userListStyle()">
        <div
          v-for="user in items"
          :key="user.id"
          :style="userRowStyle(getUserStatus(user) !== 'online')"
          @mouseenter="hoverId = user.id"
          @mouseleave="hoverId = null"
          @touchstart="hoverId = user.id"
          @click="isMobile && openDetailModal(user)"
        >
          <div :style="userRowLeftStyle">
            <ApAvatar
              :text="(user.given_name || user.username || '').slice(0, 1).toUpperCase()"
              size="medium"
              :online="isUserOnline(user.id)"
            />
            <div :style="userRowInfoStyle">
              <div :style="userRowNameStyle">
                <span :style="userRowUsernameStyle">{{ user.given_name || user.username }}</span>
                <span v-if="user.given_name" :style="userRowGivenNameStyle">@{{ user.username }}</span>
              </div>
              <!-- P2-10：两列都空时不再逐行补"暂无信息"，整行缺少元信息本身就是空的 -->
              <div v-if="user.email || user.department" :style="userRowMetaStyle">{{ user.email || user.department }}</div>
            </div>
          </div>
          <div :style="userRowRightStyle">
            <span :style="[badgeBaseStyle, roleBadgeStyle(user.role_id)]">{{ getRoleName(user.role_id) }}</span>
            <ApStatusBadge
              :status="getUserStatusBadge(getUserStatus(user))"
              :label="getUserStatusLabel(user)"
              size="small"
            />
            <!-- 行操作常显：只在 hover 出现时，静态页看起来"没有编辑/禁用入口"（P2-10） -->
            <div v-if="viewMode === 'card'" :style="actionGroupStyle">
              <ApButton variant="ghost" size="mini" :aria-label="$t('adminSys.k366')" :tooltip="$t('adminSys.k366')" icon-only @click="openDetailModal(user)"><ApIcon name="Eye" :size="16" /></ApButton>
              <ApButton variant="ghost" size="mini" :aria-label="$t('adminSys.k367')" :tooltip="$t('adminSys.k367')" icon-only @click="openEditModal(user)"><ApIcon name="Pencil" :size="16" /></ApButton>
              <!-- P1-73: 后端无旧密码的改密仅超管可用（403 否则），入口按能力隐藏 -->
              <ApButton v-if="auth.isSuperAdmin" variant="ghost" size="mini" :aria-label="$t('adminSys.k368')" :tooltip="$t('adminSys.k368')" icon-only @click="openPasswordModal(user)"><ApIcon name="Key" :size="16" /></ApButton>
              <ApButton v-if="user.role_id !== ROLE.SUPER_ADMIN" variant="ghost" size="mini" :aria-label="$t('adminSys.k369')" :tooltip="$t('adminSys.k369')" icon-only @click="handleToggle(user)"><ApIcon name="Power" :size="16" /></ApButton>
              <ApButton v-if="user.role_id !== ROLE.SUPER_ADMIN" variant="danger" size="mini" :aria-label="$t('adminSys.k362')" :tooltip="$t('adminSys.k362')" icon-only @click="openDeleteModal(user)"><ApIcon name="Trash2" :size="16" /></ApButton>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- 表格视图 (桌面端) -->
    <template v-else>
      <ApTable :columns="userColumns" :data="items" :loading="loading && !items.length" density="compact">
        <template #actions="{ row }">
          <div :style="actionGroupStyle">
            <ApButton variant="ghost" size="mini" @click="openDetailModal(row)"><ApIcon name="Eye" :size="16" /> {{ $t('adminSys.k336') }}</ApButton>
            <ApButton variant="ghost" size="mini" @click="openEditModal(row)"><ApIcon name="Pencil" :size="16" /> {{ $t('adminSys.edit') }}</ApButton>
            <ApButton v-if="auth.isSuperAdmin" variant="ghost" size="mini" @click="openPasswordModal(row)"><ApIcon name="Key" :size="16" /> {{ $t('adminSys.k337') }}</ApButton>
            <ApButton
              v-if="row.role_id !== ROLE.SUPER_ADMIN"
              variant="ghost"
              size="mini"
              @click="handleToggle(row)"
            ><ApIcon name="Power" :size="16" /> {{ row.is_forbidden === 1 ? $t('vAdminUi.enable') : $t('vAdminUi.disable') }}</ApButton>
            <ApButton
              v-if="row.role_id !== ROLE.SUPER_ADMIN"
              variant="danger"
              size="mini"
              @click="openDeleteModal(row)"
            ><ApIcon name="Trash2" :size="16" />{{ $t('adminSys.delete') }}</ApButton>
          </div>
        </template>
      </ApTable>
    </template>

    <div v-if="total > 0" :style="crudPaginationStyle('compact')">
      <span :style="totalStyle">{{ $t('common.totalCount', { n: total }) }}</span>
      <ApPagination :page="page" :page-size="pageSize" :total="total" @change="changePage" />
    </div>
      </template>
      <template #modals>
    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('adminSys.k362')"
      :message="deleteMessage"
      :confirm-text="$t('adminSys.delete')"
      :cancel-text="$t('adminSys.cancel')"
      type="danger"
      @confirm="handleDelete"
    />

    <!-- 创建/编辑用户 Modal -->
    <ApModal :model-value="showCreateModal || showEditModal" :title="isEdit ? $t('uiText.k1') : $t('uiText.k2')" size="medium" density="compact" @update:model-value="(v: boolean) => { if (!v) closeModals() }">
      <ApFormItem :label="$t('adminSys.k73')" required :error="formErrors.username">
        <ApInput v-model="form.username" :placeholder="$t('adminSys.k352')" :disabled="isEdit" />
      </ApFormItem>
      <ApFormItem v-if="!isEdit" :label="$t('adminSys.k340')" required :error="formErrors.password">
        <ApInput v-model="form.password" type="password" :placeholder="$t('adminSys.k353')" />
      </ApFormItem>
      <ApFormItem v-if="isEdit && auth.isSuperAdmin" :label="$t('adminSys.k341')">
        <ApInput v-model="form.password" type="password" :placeholder="$t('adminSys.k354')" />
      </ApFormItem>
      <div :style="formRowStyle">
        <ApFormItem :label="$t('adminSys.k342')" :style="{ flex: 1 }">
          <ApInput v-model="form.given_name" :placeholder="$t('adminSys.k355')" />
        </ApFormItem>
        <ApFormItem :label="$t('adminSys.k343')" :style="{ flex: 1 }" :error="formErrors.email">
          <ApInput v-model="form.email" type="email" placeholder="email@example.com" />
        </ApFormItem>
      </div>
      <div :style="formRowStyle">
        <ApFormItem :label="$t('adminSys.k344')" :style="{ flex: 1 }" :error="formErrors.mobile">
          <ApInput v-model="form.mobile" :placeholder="$t('adminSys.k344')" />
        </ApFormItem>
        <ApFormItem :label="$t('adminSys.k345')" :style="{ flex: 1 }">
          <ApInput v-model="form.department" :placeholder="$t('adminSys.k356')" />
        </ApFormItem>
      </div>
      <div :style="formRowStyle">
        <ApFormItem :label="$t('adminSys.k346')" :style="{ flex: 1 }">
          <ApInput v-model="form.position" :placeholder="$t('adminSys.k346')" />
        </ApFormItem>
        <ApFormItem :label="$t('adminSys.k347')" :style="{ flex: 1 }">
          <ApInput v-model="form.phone" :placeholder="$t('adminSys.k357')" />
        </ApFormItem>
      </div>
      <div :style="formRowStyle">
        <ApFormItem :label="$t('adminSys.k348')" :style="{ flex: 1 }">
          <ApInput v-model="form.location" :placeholder="$t('adminSys.k358')" />
        </ApFormItem>
        <ApFormItem :label="$t('adminSys.k349')" :style="{ flex: 1 }">
          <ApInput v-model="form.im" :placeholder="$t('adminSys.k359')" />
        </ApFormItem>
      </div>
      <div :style="formRowStyle">
        <ApFormItem :label="$t('adminSys.k350')" required :style="{ flex: 1 }">
          <ApSelect v-model="form.role_id" :options="roleSelectOptions" />
        </ApFormItem>
        <div :style="{ flex: 1 }" />
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" :disabled="submitting" @click="closeModals">{{ $t('adminSys.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :disabled="submitting" @click="onSave">{{ submitting ? $t('vAdminUi.k1') : $t('vAdminUi.save') }}</ApButton>
      </template>
    </ApModal>

    <!-- 修改密码 Modal -->
    <ApModal v-model="showPwdModal" :title="$t('adminSys.k363')" size="small" density="compact">
      <ApFormItem :label="$t('adminSys.k351')" required>
        <ApInput v-model="pwdForm.new_password" type="password" :placeholder="$t('adminSys.k353')" />
      </ApFormItem>
      <template #footer>
        <ApButton variant="ghost" size="small" :disabled="submitting" @click="showPwdModal = false">{{ $t('adminSys.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :disabled="!pwdForm.new_password || submitting" @click="handleChangePwd">{{ submitting ? $t('vAdminUi.k1') : $t('vAdminUi.save') }}</ApButton>
      </template>
    </ApModal>
    <ApConfirmModal
      v-model="showToggleModal"
      :title="$t('uiText.k15', { p1: toggleAction })"
      :message="toggleConfirmMessage"
      :confirm-text="toggleAction"
      type="danger"
      @confirm="confirmToggle"
    />

    <!-- 用户详情 Modal -->
    <ApModal :model-value="showDetailModal" :title="$t('adminSys.k364')" size="medium" density="compact" @update:model-value="(v: boolean) => { if (!v) showDetailModal = false }">
      <ApDescriptionList :items="detailItems" />
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showDetailModal = false">{{ $t('adminSys.close') }}</ApButton>
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
import ApIcon from '@/components/apple/ApIcon.vue';
import { cv, AppleLayout, AppleFont, AppleFontWeight, AppleType, AppleSpace, AppleRadius, AppleTransition, AppleInteractionState } from '@/utils/core/apple-tokens';
import { loadingStyle, crudPaginationStyle, badgeBaseStyle, totalStyle, formRowStyle, actionGroupStyle } from '@/styles/layout/page-styles';
import { useAuthStore } from '@/stores/auth/auth';
import { useUIStore } from '@/stores/app/ui';
import { useAdminCrud } from '@/composables/shared/useAdminCrud';
import { useBreakpoint } from '@/utils/responsive';
import { getUsers, createUser, updateUser, deleteUser, changePassword, toggleUserForbidden, exportUsersCSV } from '@/api/modules/user/user';
import AdminCrudPage from '@/components/admin/AdminCrudPage.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue'
import ApAvatar from '@/components/apple/ApAvatar.vue'
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue'
import ApModal from '@/components/apple/ApModal.vue'
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue'
import ApButton from '@/components/apple/ApButton.vue'
import ApSkeleton from '@/components/apple/ApSkeleton.vue'
import ApInput from '@/components/apple/ApInput.vue'
import ApSelect from '@/components/apple/ApSelect.vue'
import { filterSelectGroupStyle } from '@/styles/layout/page-styles';
import ApFormItem from '@/components/apple/ApFormItem.vue'
import ApPagination from '@/components/apple/ApPagination.vue'
import ApTable from '@/components/apple/ApTable.vue'
import ApDescriptionList from '@/components/apple/ApDescriptionList.vue'
import type { User } from '@/types/models';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import { formatTime } from '@/utils/format/format-time';
import { validators, getValidationMessage } from '@/utils/core/validators';
import { getRoleName, ROLE } from '@/utils/core/constants';
import { getUserStatus, getUserStatusLabel, getUserStatusBadge } from '@/types/user-helpers';

const auth = useAuthStore();
const ui = useUIStore();
const { isMobile } = useBreakpoint();

const hoverId = shallowRef<number | null>(null);
const roleFilter = ref('0');

const {
  loading, loadError, retryLoad, items, total, page, pageSize, keyword,
  showCreateModal, showEditModal, showDeleteModal, editingItem,
  formErrors, viewMode,
  fetchList, handleSearch, changePage, openCreate, openEdit, openDelete, closeModals,
  handleDelete, handleSubmit,
} = useAdminCrud<User>({
  fetchFn: async (p) => {
    const res = await getUsers({
      page: p.page,
      page_size: p.pageSize,
      role_id: roleFilter.value !== '0' ? Number(roleFilter.value) : undefined,
      keyword: p.keyword || undefined });
    return { data: res.data, total: res.total, page: res.page, page_size: res.page_size };
  },
  createFn: (d) => createUser({ username: d.username!, password: (d as Record<string, unknown>).password as string, given_name: d.given_name, email: d.email, mobile: d.mobile, phone: d.phone, department: d.department, position: d.position, location: d.location, im: d.im, role_id: d.role_id! }),
  updateFn: async (d) => {
    await updateUser(d.id!, { given_name: d.given_name, email: d.email, mobile: d.mobile, phone: d.phone, department: d.department, position: d.position, location: d.location, im: d.im, role_id: d.role_id! });
    // P1-15: 编辑弹窗“重置密码（可选）”此前从不进入任何请求，超管填了新密码却提示“已更新”而密码未变；
    // 后端 PUT /users/:id 无 password 字段，这里改走 changePassword（超管免旧密码）
    const pwd = (d as Record<string, unknown>).password as string | undefined;
    if (pwd) {
      await changePassword(d.id!, undefined, pwd);
    }
  },
  deleteFn: (id) => deleteUser(id),
  itemName: t('uiText.k3'),
  resetForm: () => { form.value = { username: '', password: '', given_name: '', email: '', mobile: '', phone: '', department: '', position: '', location: '', im: '', role_id: 3 }; },
});

const isEdit = computed(() => !!editingItem.value);
const submitting = shallowRef(false);
const exporting = shallowRef(false);

const form = ref({
  username: '', password: '', given_name: '', email: '',
  mobile: '', phone: '', department: '', position: '', location: '', im: '', role_id: 3 });

const pwdForm = ref({ new_password: '' });
const showPwdModal = shallowRef(false);
const pwdTarget = ref<User | null>(null);

const deleteMessage = shallowRef('');

const roleOptions = [
  { label: t('adminSys.k370'), value: '0' },
  { label: t('adminSys.k327'), value: '1' },
  { label: t('adminSys.k371'), value: '2' },
  { label: t('adminSys.k372'), value: '3' },
];

const roleSelectOptions = [
  { label: t('adminSys.k371'), value: 2 },
  { label: t('adminSys.k372'), value: 3 },
];

const ONLINE_THRESHOLD_MINUTES = 30;

const onlineUserIds = computed(() => {
  const threshold = Date.now() / 1000 - ONLINE_THRESHOLD_MINUTES * 60;
  return items.value
    .filter(u => u.is_delete !== 1 && u.is_forbidden !== 1 && u.last_time && u.last_time >= threshold)
    .map(u => u.id);
});

function isUserOnline(userId: number): boolean {
  return onlineUserIds.value.includes(userId);
}

function openCreateModal() {
  form.value = { username: '', password: '', given_name: '', email: '', mobile: '', phone: '', department: '', position: '', location: '', im: '', role_id: 3 };
  openCreate();
}

function openEditModal(user: User) {
  form.value = {
    username: user.username, password: '',
    given_name: user.given_name || '', email: user.email || '',
    mobile: user.mobile || '', phone: user.phone || '',
    department: user.department || '', position: user.position || '',
    location: user.location || '', im: user.im || '', role_id: user.role_id };
  openEdit(user);
}

function openDeleteModal(user: User) {
  if (user.role_id === ROLE.SUPER_ADMIN) {
    ui.showToast(t('adminSys.k378'), 'warning');
    return;
  }
  deleteMessage.value = t('sentUi.k30', { p1: user.given_name || user.username });
  openDelete(user);
}

function openPasswordModal(user: User) {
  pwdTarget.value = user;
  pwdForm.value = { new_password: '' };
  showPwdModal.value = true;
}

async function onSave() {
  if (submitting.value) return;
  submitting.value = true;
  try {
    await handleSubmit(
      { username: form.value.username, password: form.value.password, given_name: form.value.given_name, email: form.value.email, mobile: form.value.mobile, phone: form.value.phone, department: form.value.department, position: form.value.position, location: form.value.location, im: form.value.im, role_id: form.value.role_id } as Partial<User> & Record<string, unknown>,
      (data) => {
        const errors: Record<string, string> = {};
        if (!isEdit.value) {
          if (!data.username?.trim()) errors.username = t('uiText.k4');
          else if (!validators.username(data.username)) errors.username = t('uiText.k5');
          if (!validators.password((data as Record<string, unknown>).password as string || '')) errors.password = getValidationMessage('password', (data as Record<string, unknown>).password as string || '');
        }
        if (data.email && !validators.email(data.email)) errors.email = t('uiText.k6');
        if (data.mobile && !validators.phone(data.mobile)) errors.mobile = t('uiText.k7');
        // P1-15: 编辑态填了重置密码也要校验强度，避免半路 400
        if (isEdit.value && (data as Record<string, unknown>).password && !validators.password((data as Record<string, unknown>).password as string)) {
          errors.password = getValidationMessage('password', (data as Record<string, unknown>).password as string);
        }
        return Object.keys(errors).length > 0 ? errors : null;
      }
    );
  } finally {
    submitting.value = false;
  }
}

async function handleChangePwd() {
  if (!pwdTarget.value || !pwdForm.value.new_password || submitting.value) return;
  if (!validators.password(pwdForm.value.new_password)) {
    ui.showToast(getValidationMessage('password', pwdForm.value.new_password), 'error');
    return;
  }
  submitting.value = true;
  try {
    await changePassword(pwdTarget.value.id, undefined, pwdForm.value.new_password);
    ui.showToast(t('adminSys.k379'), 'success');
    showPwdModal.value = false;
  } catch (e: unknown) {
    ui.showToast((e as Error).message || t('fallback.k64'), 'error');
  } finally {
    submitting.value = false;
  }
}

const showToggleModal = shallowRef(false);
const toggleUserId = shallowRef(0);
const toggleUserName = shallowRef('');
const toggleForbidding = shallowRef(true);
const toggleAction = computed(() => (toggleForbidding.value ? t('uiText.k8') : t('uiText.k9')));
const toggleUserRole = shallowRef(0);

const toggleConfirmMessage = computed(() => {
  if (toggleForbidding.value) {
    const roleText = toggleUserRole.value === 1 ? t('uiText.k10') : toggleUserRole.value === 2 ? t('uiText.k11') : t('uiText.k12');
    return t('uiText.k13', { p1: toggleUserName.value, p2: roleText });
  }
  return t('uiText.k14', { p1: toggleUserName.value });
});

async function handleToggle(user: User) {
  if (user.role_id === ROLE.SUPER_ADMIN) return;
  toggleUserId.value = user.id;
  toggleUserName.value = user.username;
  toggleForbidding.value = user.is_forbidden !== 1;
  toggleUserRole.value = user.role_id;
  showToggleModal.value = true;
}

const togglingForbidden = shallowRef(false);
async function confirmToggle() {
  // P1-72: /forbidden 是“读状态取反”的非幂等接口，弹窗退出动画期间连点会翻转两次回到原状；
  // 用并发守卫保证只发一次请求
  if (togglingForbidden.value) return;
  togglingForbidden.value = true;
  try {
    await toggleUserForbidden(toggleUserId.value);
    ui.showToast(t('sentUi.k31', { p1: toggleAction.value }), 'success');
    await fetchList();
  } catch (e: unknown) {
    ui.showToast((e as Error).message || t('fallback.k64'), 'error');
  } finally {
    showToggleModal.value = false;
    togglingForbidden.value = false;
  }
}



async function handleExportCSV() {
  if (exporting.value) return;
  exporting.value = true;
  try {
    const blob = await exportUsersCSV();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `users_${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
    ui.showToast(t('adminSys.k380'), 'success');
  } catch (e: unknown) {
    ui.showToast((e as Error).message || t('fallback.k49'), 'error');
  } finally {
    exporting.value = false;
  }
}

onMounted(() => { fetchList(); });

const showDetailModal = shallowRef(false);
const detailUser = ref<User | null>(null);

const detailItems = computed(() => {
  const u = detailUser.value;
  if (!u) return [];
  return [
    { label: t('adminSys.k73'), value: u.username },
    { label: t('adminSys.k342'), value: u.given_name },
    { label: t('adminSys.k350'), value: getRoleName(u.role_id) },
    { label: t('adminSys.k343'), value: u.email },
    { label: t('adminSys.k373'), value: u.mobile },
    { label: t('adminSys.k347'), value: u.phone },
    { label: t('adminSys.k345'), value: u.department },
    { label: t('adminSys.k346'), value: u.position },
    { label: t('adminSys.k374'), value: u.location },
    { label: 'IM', value: u.im },
    { label: t('adminSys.status'), value: getUserStatusLabel(u) },
    { label: t('adminSys.k375'), value: u.last_time ? formatTime(u.last_time) : undefined },
    { label: t('adminSys.k376'), value: u.last_ip },
    { label: t('adminSys.k28'), value: u.create_time ? formatTime(u.create_time) : undefined },
    { label: t('adminSys.k377'), value: u.update_time ? formatTime(u.update_time) : undefined },
  ];
});

function openDetailModal(user: User) {
  detailUser.value = user;
  showDetailModal.value = true;
}

const userColumns: TableColumn<User>[] = [
  { key: 'username', title: t('adminSys.k73') },
  { key: 'given_name', title: t('adminSys.k342') },
  { key: 'email', title: t('adminSys.k343') },
  { key: 'department', title: t('adminSys.k345') },
  { key: 'role_id', title: t('adminSys.k350'), render: (row: User) => getRoleName(row.role_id) },
  { key: 'is_forbidden', title: t('adminSys.status'), render: (row: User) => getUserStatusLabel(row) },
  { key: 'create_time', title: t('adminSys.k28'), render: (row: User) => formatTime(row.create_time) },
];

function roleBadgeStyle(roleId: number) {
  const isSuperAdmin = roleId === 1;
  return {
    background: isSuperAdmin ? cv('red-light') : roleId === 2 ? cv('blue-light') : cv('bg-tertiary'),
    color: isSuperAdmin ? cv('red') : roleId === 2 ? cv('blue') : cv('text-tertiary'),
  };
}

function userListStyle() {
  return {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: AppleSpace.xs,
  };
}

function userRowStyle(disabled: boolean) {
  return {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: `${AppleSpace.sm} ${AppleSpace.md}`,
    background: cv('surface-panel'),
    borderRadius: AppleRadius.card,
    border: `1px solid ${cv('border-light')}`,
    boxShadow: cv('shadow-card'),
    transition: AppleTransition.smooth,
    opacity: disabled ? AppleInteractionState.disabledOpacity : '1',
    filter: disabled ? 'grayscale(0.5)' : 'none',
  };
}

const userRowLeftStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  flex: '1',
  minWidth: '0',
};

const userRowInfoStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
  minWidth: '0',
};

const userRowNameStyle = {
  display: 'flex',
  alignItems: 'baseline',
  gap: AppleSpace.xs,
};

const userRowUsernameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
};

const userRowGivenNameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
};

const userRowMetaStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const userRowRightStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  flexShrink: '0',
};
</script>
