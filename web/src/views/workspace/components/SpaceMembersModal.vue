<template>
  <ApModal v-model="showModal" :title="$t('ws.k10')" size="medium">
    <div v-if="loading" :style="{ display: 'flex', justifyContent: 'center', padding: AppleSpace.lg }">
      <ApSkeleton variant="text" :lines="3" />
    </div>
    <div v-else>
      <div :style="memberAddSectionStyle">
        <div :style="{ flex: '1' }">
          <ApSelect
            :model-value="selectedUserId"
            :options="userSelectOptions"
            :placeholder="$t('ws.k9')"
            @update:model-value="(v: unknown) => selectedUserId = v as number"
          />
        </div>
        <div :style="{ width: '140px' }">
          <ApSelect
            :model-value="selectedPrivilege"
            :options="privilegeOptions"
            @update:model-value="(v: unknown) => selectedPrivilege = v as number"
          />
        </div>
        <ApButton variant="primary" size="small" :disabled="!selectedUserId || adding" @click="handleAdd">
          <ApIcon name="Plus" :size="16" />{{ $t('ws.add') }}
        </ApButton>
      </div>

      <div :style="{ marginTop: AppleSpace.lg }">
        <ApEmpty v-if="!members.length" icon="User" :title="$t('ws.k11')" />
        <div v-for="member in members" :key="member.user_id" :style="memberRowStyle">
          <ApAvatar :text="member.given_name || member.username" size="small" />
          <span :style="memberNameStyle">{{ member.given_name || member.username }}</span>
          <div :style="memberActionsStyle">
            <div :style="{ width: '120px' }">
              <ApSelect
                :model-value="member.privilege"
                :options="privilegeOptions"
                @update:model-value="(v: unknown) => handlePrivilegeChange(member, v as number)"
              />
            </div>
            <ApButton variant="danger" size="small" @click="onRemove(member)">
              <ApIcon name="Trash2" :size="16" />{{ $t('ws.k8') }}
            </ApButton>
          </div>
        </div>
      </div>
    </div>
    <template #footer>
      <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('ws.close') }}</ApButton>
    </template>
  </ApModal>

  <ApConfirmModal
    v-model="showRemoveConfirm"
    :title="$t('ws.k12')"
    :message="`确定移除成员「${pendingRemoveMember?.given_name || pendingRemoveMember?.username || ''}」吗？`"
    :confirm-text="$t('ws.k8')"
    type="danger"
    @confirm="confirmRemove"
  />
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed, ref, shallowRef, watch } from 'vue';
import { logger } from '@/utils/logger';
import { AppleFont, AppleFontWeight, AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';
import { getSpaceMembers, addSpaceMember, removeSpaceMember, updateMemberPrivilege } from '@/api/modules/space/space';
import { getUsers } from '@/api/modules/user/user';
import ApModal from '@/components/apple/ApModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApAvatar from '@/components/apple/ApAvatar.vue';
import type { SpaceMember } from '@/types/models';

interface Props {
  modelValue?: boolean;
  spaceId?: number | null;
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: false,
  spaceId: null,
});

const emit = defineEmits<{
  'update:modelValue': [value: boolean];
  'show-toast': [message: string, type?: 'success' | 'error' | 'info'];
}>();
const { t } = useI18n();

const showModal = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
});

const loading = shallowRef(false);
const members = ref<SpaceMember[]>([]);
const selectedUserId = shallowRef<number | null>(null);
const selectedPrivilege = shallowRef(0);
const adding = shallowRef(false);
const userSelectOptions = ref<{ label: string; value: number }[]>([]);

const privilegeOptions = [
  { label: t('ws.k13'), value: 0 },
  { label: t('ws.k14'), value: 1 },
  { label: t('ws.k15'), value: 2 },
];

async function loadUserOptions() {
  try {
    const res = await getUsers({ keyword: '', page: 1, page_size: 200 });
    userSelectOptions.value = (res.data || []).map(u => ({ label: `${u.given_name || u.username} (${u.username})`, value: u.id }));
  } catch {
    userSelectOptions.value = [];
  }
}

async function loadMembers() {
  if (!props.spaceId) return;
  loading.value = true;
  try {
    members.value = await getSpaceMembers(props.spaceId);
  } catch {
    members.value = [];
  } finally {
    loading.value = false;
  }
}

watch(showModal, async (isOpen) => {
  if (isOpen) {
    selectedUserId.value = null;
    selectedPrivilege.value = 0;
    const results = await Promise.allSettled([loadUserOptions(), loadMembers()]);
    results.forEach((result, index) => {
      if (result.status === 'rejected') {
        logger.error(`Load operation ${index === 0 ? 'user options' : 'members'} failed:`, result.reason);
      }
    });
  }
});

async function handleAdd() {
  if (!selectedUserId.value || !props.spaceId || adding.value) return;
  adding.value = true;
  try {
    await addSpaceMember(props.spaceId, selectedUserId.value, selectedPrivilege.value);
    emit('show-toast', '成员已添加', 'success');
    selectedUserId.value = null;
    await loadMembers();
  } catch (e: unknown) {
    emit('show-toast', (e as Error).message || t('fallback.k88'), 'error');
  } finally {
    adding.value = false;
  }
}

async function handlePrivilegeChange(member: SpaceMember, privilege: number) {
  if (!props.spaceId) return;
  try {
    await updateMemberPrivilege(props.spaceId, member.user_id, privilege);
    emit('show-toast', '权限已更新', 'success');
    member.privilege = privilege;
  } catch (e: unknown) {
    emit('show-toast', (e as Error).message || t('fallback.k71'), 'error');
  }
}

const showRemoveConfirm = ref(false);
const pendingRemoveMember = ref<SpaceMember | null>(null);

function onRemove(member: SpaceMember) {
  if (!props.spaceId) return;
  pendingRemoveMember.value = member;
  showRemoveConfirm.value = true;
}

async function confirmRemove() {
  const member = pendingRemoveMember.value;
  if (!props.spaceId || !member) return;
  try {
    await removeSpaceMember(props.spaceId, member.user_id);
    emit('show-toast', '成员已移除', 'success');
    await loadMembers();
  } catch (e: unknown) {
    emit('show-toast', (e as Error).message || t('fallback.k93'), 'error');
  }
}

const memberAddSectionStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
};

const memberRowStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `${AppleSpace.sm} 0`,
  borderBottom: `1px solid ${cv('border-light')}`,
  gap: AppleSpace.sm,
};

const memberNameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
  flex: '1',
};

const memberActionsStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
};
</script>
