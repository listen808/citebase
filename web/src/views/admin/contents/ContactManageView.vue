<template>
  <ApPageContainer
    :title="$t('admin.k15')"
    :subtitle="$t('admin.k19')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <AdminCrudPage
      v-model:keyword="keyword"
      :error-text="loadError"
      :stats="contactStats"
      :filters="{ search: { placeholder: $t('vAdminUi.k5') } }"
      :loading="loading"
      :total="total"
      :page="page"
      :page-size="pageSize"
      density="compact"
      @retry="retryLoad"
      @search="handleSearch"
      @page-change="changePage"
    >
      <template #actions>
        <ApButton variant="secondary" size="small" @click="handleImport">
          <ApIcon name="Download" :size="16" />
          {{ $t('admin.k1') }}
        </ApButton>
        <ApButton variant="primary" size="small" @click="openCreateModal"><ApIcon name="Plus" :size="16" />{{ $t('admin.k2') }}</ApButton>
      </template>

      <template #content>
        <div v-if="loading && !items.length" :style="cardGridStyle">
          <div v-for="i in 3" :key="i" :style="skeletonCardStyle">
            <div :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.md, padding: AppleSpace.lg }">
              <ApSkeleton variant="circular" :style="{ width: AppleSpace['2xl'], height: AppleSpace['2xl'] }" />
              <div :style="{ flex: '1' }">
                <ApSkeleton :style="{ width: AppleSpace.xl, height: AppleSpace.md, marginBottom: AppleSpace.sm }" />
                <ApSkeleton :style="{ width: '150px', height: AppleSpace['12px'] }" />
              </div>
            </div>
          </div>
        </div>

        <ApEmpty v-else-if="items.length === 0" :icon="'Contact'" :title="$t('admin.k16')" />

        <div v-else :style="contactViewWrapperStyle">
          <div :style="cardGridStyle">
            <ApEntityCard
              v-for="contact in items"
              :key="contact.id"
              :data-letter="getNameLetter(contact.name)"
              :letter="getNameLetter(contact.name)"
              :title="contact.name"
              :subtitle="contact.position || contact.department || ''"
              @mouseenter="activeContactId = contact.id ?? 0"
              @mouseleave="activeContactId = null"
              @focusin="activeContactId = contact.id ?? 0"
              @focusout="activeContactId = null"
            >
              <div :style="cardActionsStyle(isCardActive(contact.id))">
                <ApButton variant="ghost" size="mini" @click="openEditModal(contact)"><ApIcon name="Pencil" :size="16" />{{ $t('admin.edit') }}</ApButton>
                <ApButton variant="danger" size="mini" @click="openDeleteModal(contact)"><ApIcon name="Trash2" :size="16" />{{ $t('admin.delete') }}</ApButton>
              </div>
              <div :style="entityMetaRowStyle()">
                <span v-if="contact.email" :style="entityMetaItemStyle()">
                  <ApIcon name="Mail" :size="12" />
                  {{ contact.email }}
                </span>
                <span v-if="contact.mobile" :style="entityMetaItemStyle()">
                  <ApIcon name="Phone" :size="12" />
                  {{ contact.mobile }}
                </span>
              </div>
            </ApEntityCard>
          </div>

          <div v-if="alphabetLetters.length > 1" :style="alphabetIndexStyle()">
            <div
              v-for="letter in alphabetLetters"
              :key="letter"
              :style="alphabetIndexItemStyle(activeLetter === letter)"
              @click="scrollToLetter(letter)"
            >
              {{ letter }}
            </div>
          </div>
        </div>
      </template>

      <template #modals>
        <ApModal :model-value="showCreateModal || showEditModal" :title="showEditModal ? $t('adminForm.k7') : $t('adminForm.k8')" size="medium" density="compact" @update:model-value="(v: boolean) => { if (!v) closeModals(); }">
          <ApFormItem :label="$t('admin.k8')" required :error="formErrors.name">
            <ApInput v-model="form.name" :placeholder="$t('admin.k8')" />
          </ApFormItem>
          <ApFormItem :label="$t('admin.k9')" required :error="formErrors.email">
            <ApInput v-model="form.email" :placeholder="$t('admin.k9')" type="email" />
          </ApFormItem>
          <ApFormItem :label="$t('admin.k10')" :error="formErrors.mobile">
            <ApInput v-model="form.mobile" :placeholder="$t('admin.k13')" />
          </ApFormItem>
          <ApFormItem :label="$t('admin.k11')">
            <ApInput v-model="form.position" :placeholder="$t('admin.k11')" />
          </ApFormItem>
          <ApFormItem :label="$t('admin.k12')">
            <ApInput v-model="form.department" :placeholder="$t('admin.k12')" />
          </ApFormItem>
          <template #footer>
            <ApButton variant="ghost" size="small" @click="closeModals()">{{ $t('admin.cancel') }}</ApButton>
            <ApButton variant="primary" size="small" :disabled="submitting" @click="onSave">{{ submitting ? $t('vAdminUi.k1') : $t('vAdminUi.save') }}</ApButton>
          </template>
        </ApModal>

        <ApConfirmModal
          v-model="showDeleteModal"
          :title="$t('admin.k17')"
          :message="deleteMessage"
          :confirm-text="$t('admin.delete')"
          :cancel-text="$t('admin.cancel')"
          type="danger"
          @confirm="handleDelete"
        />

        <ApModal v-model="showImportModal" :title="$t('admin.k1')" size="medium" density="compact">
          <div :style="importSearchStyle">
            <ApInput v-model="importKeyword" :placeholder="$t('admin.k14')" :style="{ flex: 1 }" />
            <ApButton variant="secondary" size="small" @click="searchUsers">{{ $t('admin.search') }}</ApButton>
          </div>
          <div v-if="importLoading" :style="{ padding: `${AppleSpace.md} 0` }"><ApSkeleton variant="list" :lines="3" /></div>
          <ApEmpty v-else-if="importUserList.length === 0 && importSearched" :title="$t('admin.k18')" />
          <div v-else :style="importListStyle">
            <div v-for="u in importUserList" :key="u.id" :style="importItemStyle">
              <div :style="importUserInfoStyle">
                <span :style="importUserNameStyle">{{ u.given_name || u.username }}</span>
                <span :style="importUserMetaStyle">{{ u.email || u.username }}</span>
              </div>
              <ApButton variant="ghost" size="small" :disabled="importingIds.has(u.id)" @click="handleImportUser(u)">
                {{ importingIds.has(u.id) ? $t('vAdminUi.k3') : $t('vAdminUi.import') }}
              </ApButton>
            </div>
          </div>
          <template #footer>
            <ApButton variant="ghost" size="small" @click="showImportModal = false">{{ $t('admin.close') }}</ApButton>
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
import { AppleZIndex, cv, AppleLayout, AppleSpace, AppleType, AppleFontWeight, AppleFont, AppleTransition, appleCardGrid } from '@/utils/core/apple-tokens';
import { alphabetIndexStyle, alphabetIndexItemStyle, entityMetaRowStyle, entityMetaItemStyle } from '@/styles/layout/page-styles';
import { getContacts, saveContact, updateContact, deleteContact } from '@/api/modules/user/contact';
import { getUsers } from '@/api/modules/user/user';
import type { Contact } from '@/api/modules/user/contact';
import type { User } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';
import { validators } from '@/utils/core/validators';
import { useAdminCrud } from '@/composables/shared/useAdminCrud';
import AdminCrudPage from '@/components/admin/AdminCrudPage.vue';
import type { StatConfig } from '@/components/admin/AdminCrudPage.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApEntityCard from '@/components/apple/ApEntityCard.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';

const ui = useUIStore();

const {
  loading, loadError, retryLoad, submitting, items, total, page, pageSize, keyword,
  showCreateModal, showEditModal, showDeleteModal,
  formErrors,
  fetchList, handleSearch, changePage, openCreate, openEdit, openDelete,
  handleDelete, handleSubmit, closeModals,
} = useAdminCrud<Contact>({
  fetchFn: (p) => getContacts(p.page, p.pageSize, p.keyword),
  createFn: (d) => saveContact(d as unknown as Parameters<typeof saveContact>[0]),
  updateFn: (d) => updateContact(d.id!, d as unknown as Parameters<typeof updateContact>[1]),
  deleteFn: (id) => deleteContact(id),
  itemName: t('adminForm.k18'),
  resetForm: () => { form.value = { name: '', email: '', mobile: '', position: '', department: '' }; },
});

Object.assign(formErrors, { name: '', email: '', mobile: '' });

const form = ref({ name: '', email: '', mobile: '', position: '', department: '' });
const deleteMessage = ref('');

const departmentCount = computed(() => {
  const depts = new Set(items.value.map(c => c.department).filter(Boolean));
  return depts.size;
});

const recentlyAddedCount = computed(() => {
  const sevenDaysAgo = Date.now() / 1000 - 7 * 24 * 60 * 60;
  return items.value.filter(c => (c.create_time ?? 0) > sevenDaysAgo).length;
});

const contactStats = computed<StatConfig[]>(() => [
  { label: t('admin.k20'), value: total.value, icon: 'Contact', color: cv('blue') },
  { label: t('admin.k21'), value: departmentCount.value, icon: 'Building2', color: cv('green') },
  { label: t('admin.k22'), value: recentlyAddedCount.value, icon: 'UserPlus', color: cv('orange') },
]);

const LETTER_ORDER = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ#'.split('');
// 索引只列真实有人的分组：原来 27 个字母恒显，其中绝大多数点了没有任何反应
// （卡片上连 data-letter 锚点都没有），既占掉一整条 492px 高的列，又是假可点。
const alphabetLetters = computed(() => {
  const present = new Set(items.value.map((c) => getNameLetter(c.name)));
  return LETTER_ORDER.filter((letter) => present.has(letter));
});
const activeLetter = shallowRef<string | null>(null);
// 操作位显形有两条路径：指针悬停，或键盘 Tab 进卡片（focusin/focusout）。
// 只挂 hover 的话，键盘用户既看不见「编辑/删除」也不知道那里有东西 —— apple-design §16 Agency。
const activeContactId = shallowRef<number | null>(null);
const isCardActive = (id?: number | null) => activeContactId.value === (id ?? 0);

function getNameLetter(name: string): string {
  const first = name.trim().charAt(0).toUpperCase();
  if (/[A-Z]/.test(first)) return first;
  return '#';
}

function scrollToLetter(letter: string) {
  activeLetter.value = letter;
  const el = document.querySelector(`[data-letter="${letter}"]`);
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }
}

function openCreateModal() {
  form.value = { name: '', email: '', mobile: '', position: '', department: '' };
  openCreate();
}

function openEditModal(contact: Contact) {
  form.value = {
    name: contact.name,
    email: contact.email,
    mobile: contact.mobile || '',
    position: contact.position || '',
    department: contact.department || '',
  };
  openEdit(contact);
}

function openDeleteModal(contact: Contact) {
  deleteMessage.value = t('sentUi.k1', { p1: contact?.name || t('fallback.k77') });
  openDelete(contact);
}

async function onSave() {
  await handleSubmit(
    { name: form.value.name, email: form.value.email, mobile: form.value.mobile, position: form.value.position, department: form.value.department } as Partial<Contact>,
    (data) => {
      const errors: Record<string, string> = {};
      if (!data.name?.trim()) errors.name = t('adminForm.k49');
      if (!validators.email(data.email || '')) errors.email = t('adminForm.k50');
      if (data.mobile && !validators.phone(data.mobile)) errors.mobile = t('adminForm.k51');
      return Object.keys(errors).length > 0 ? errors : null;
    }
  );
}

function handleImport() {
  importKeyword.value = '';
  importUserList.value = [];
  importSearched.value = false;
  showImportModal.value = true;
}

const showImportModal = shallowRef(false);
const importKeyword = ref('');
const importUserList = ref<User[]>([]);
const importSearched = shallowRef(false);
const importLoading = shallowRef(false);
const importingIds = ref<Set<number>>(new Set());

async function searchUsers() {
  importLoading.value = true;
  importSearched.value = true;
  try {
    const res = await getUsers({ keyword: importKeyword.value, page_size: 50 });
    importUserList.value = (res.data || []).filter((u: User) => u.is_delete === 0);
  } catch {
    importUserList.value = [];
    ui.showToast(t('admin.k23'), 'error');
  } finally {
    importLoading.value = false;
  }
}

async function handleImportUser(user: User) {
  if (importingIds.value.has(user.id)) return;
  importingIds.value = new Set(importingIds.value).add(user.id);
  try {
    await saveContact({
      name: user.given_name || user.username,
      email: user.email || '',
      mobile: user.mobile || '',
      position: user.position || '',
      department: user.department || '',
    });
    ui.showToast(t('sentUi.k2', { p1: user.given_name || user.username }), 'success');
    fetchList();
  } catch {
    ui.showToast(t('admin.k24'), 'error');
  } finally {
    const next = new Set(importingIds.value);
    next.delete(user.id);
    importingIds.value = next;
  }
}

onMounted(fetchList);

const cardGridStyle = {
  ...appleCardGrid('entity', AppleSpace.sm),
  flex: '1' as const,
};

const skeletonCardStyle = {
  background: cv('bg-secondary'),
};

const contactViewWrapperStyle = {
  display: 'flex',
  alignItems: 'flex-start' as const,
  // 不拉伸的话，卡片网格会被 27 个字母的索引拉到 492px 高，
  // 再由 grid-auto-rows:1fr 把这份高度摊进每一行 —— 卡中间就出现空洞。
  gap: AppleSpace.md,
  position: 'relative' as const,
};

const importSearchStyle = { display: 'flex', gap: AppleSpace.sm, marginBottom: AppleSpace.sm };
const importListStyle = { maxHeight: '300px', overflowY: 'auto' as const };
const importItemStyle = { display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: `${AppleSpace.xs} ${AppleSpace.sm}`, borderBottom: `1px solid ${cv('border-light')}` };
const importUserInfoStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xxs };
const importUserNameStyle = { fontFamily: AppleFont.text, fontSize: AppleType.controlLabel.size, color: cv('text-primary'), fontWeight: String(AppleFontWeight.medium) };
const importUserMetaStyle = { fontFamily: AppleFont.text, fontSize: AppleType.microUI.size, color: cv('text-tertiary') };

function cardActionsStyle(visible: boolean) {
  return {
    position: 'absolute' as const,
    top: AppleSpace.sm,
    right: AppleSpace.sm,
    display: 'flex',
    gap: AppleSpace.xs,
    opacity: visible ? '1' : '0',
    transition: AppleTransition.smooth,
    pointerEvents: (visible ? 'auto' : 'none') as 'auto' | 'none',
    zIndex: String(AppleZIndex.raised),
  };
}
</script>
