<template>
  <ApPageContainer
    :title="$t('social.k73')"
    :subtitle="$t('social.k75')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <template #actions>
      <ApButton variant="primary" size="small" @click="openModal()">
        <ApIcon name="Plus" :size="16" /> {{ $t('social.k55') }}
      </ApButton>
    </template>

    <div :style="listStyle()">
      <ApCard size="small">
        <template #header>
          <div :style="headerRowStyle">
            <div :style="{ display: 'inline-flex', alignItems: 'center', gap: AppleSpace.sm }">
              <ApIcon name="Send" :size="18" :color="cv('blue')" />
              <span :style="sectionTitleStyle">{{ $t('social.k56') }}</span>
              <span :style="countBadgeStyle">{{ webhooks.length }}</span>
            </div>
            <ApButton variant="ghost" size="small" :loading="loading" @click="loadWebhooks">{{ $t('social.refresh') }}</ApButton>
          </div>
        </template>

        <ApTable :columns="webhookColumns" :data="webhooks" :loading="loading" density="compact" hide-header-when-empty>
          <template #empty>
            <ApEmpty :icon="'Send'" :title="$t('social.k93')" :description="$t('social.k94')" />
          </template>
          <template #actions="{ row }">
            <div :style="actionGroupStyle">
              <ApButton variant="ghost" size="small" @click="openDeliveries(row as unknown as WebhookConfig)">
                <ApIcon name="History" :size="16" /> {{ $t('social.k58') }}
              </ApButton>
              <ApButton
                variant="ghost"
                size="small"
                :loading="togglingId === (row as unknown as WebhookConfig).id"
                @click="toggleActive(row as unknown as WebhookConfig)"
              >
                {{ (row as unknown as WebhookConfig).is_active === 1 ? $t('vAdminUi.k27') : $t('vAdminUi.enable') }}
              </ApButton>
              <ApButton variant="ghost" size="small" @click="openModal(row as unknown as WebhookConfig)">{{ $t('social.edit') }}</ApButton>
              <ApButton variant="danger" size="small" @click="askDelete(row as unknown as WebhookConfig)">
                <ApIcon name="Trash2" :size="16" /> {{ $t('social.delete') }}
              </ApButton>
            </div>
          </template>
        </ApTable>
      </ApCard>

      <p :style="hintStyle">
        {{ $t('social.k60') }}
      </p>
    </div>

    <!-- 新建 / 编辑 -->
    <ApModal v-model="showModal" :title="editingId ? $t('adminForm.k13') : $t('adminForm.k14')" size="medium">
      <ApFormItem :label="$t('social.name')" required :error="nameError">
        <ApInput v-model="form.name" :placeholder="$t('social.k69')" :maxlength="100" />
      </ApFormItem>

      <ApFormItem :label="$t('social.k64')" required :error="urlError">
        <ApInput v-model="form.url" placeholder="https://example.com/hooks/citebase" :maxlength="500" />
      </ApFormItem>

      <ApFormItem :label="$t('social.k26')" :hint="editingId ? $t('adminForm.k70') : $t('adminForm.k71')">
        <ApSelect v-model="form.space_id" :options="spaceOptions" :placeholder="$t('social.k70')" searchable :disabled="!!editingId" />
      </ApFormItem>

      <ApFormItem :label="$t('social.k65')" required :error="eventsError">
        <div :style="{ display: 'flex', alignItems: 'flex-end', gap: AppleSpace.sm }">
          <div :style="{ flex: '1' }">
            <ApSelect v-model="eventPick" :options="availableEventOptions" :placeholder="$t('social.k71')" />
          </div>
          <ApButton variant="secondary" size="small" :disabled="!eventPick" @click="addEvent">{{ $t('social.add') }}</ApButton>
        </div>
        <div v-if="form.events.length" :style="tagRowStyle">
          <span
            v-for="ev in form.events"
            :key="ev"
            :style="{ display: 'inline-flex', alignItems: 'center', gap: AppleSpace.xxs }"
          >
            <span :style="tagStyle">{{ ev }}</span>
            <ApButton variant="ghost" size="mini" icon-only :aria-label="$t('adminForm.k72', { p1: ev })" @click="removeEvent(ev)">
              <ApIcon name="X" :size="16" />
            </ApButton>
          </span>
        </div>
      </ApFormItem>

      <ApFormItem :label="$t('social.k66')" :hint="editingId ? $t('adminForm.k73') : $t('adminForm.k74')">
        <ApInput v-model="form.secret" :placeholder="$t('social.k72')" :maxlength="100" />
      </ApFormItem>

      <ApFormItem v-if="editingId" :label="$t('social.k67')">
        <ApSwitch v-model="form.isActive" :label="$t('social.k68')" />
      </ApFormItem>

      <template #footer>
        <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('social.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="saving" @click="handleSave">
          {{ editingId ? $t('vSocialUi.k3') : $t('vSocialUi.create2') }}
        </ApButton>
      </template>
    </ApModal>

    <!-- 投递记录 -->
    <ApModal v-model="showDeliveryModal" :title="$t('adminForm.k75', { p1: deliveryTarget?.name ?? '' })" size="large">
      <ApTable :columns="deliveryColumns" :data="deliveries" :loading="deliveryLoading" density="compact" />
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showDeliveryModal = false">{{ $t('social.close') }}</ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('social.k74')"
      :message="$t('adminForm.k76', { p1: deletingItem?.name ?? '' })"
      type="danger"
      :confirm-text="$t('social.delete')"
      :loading="deleting"
      @confirm="confirmDelete"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted } from 'vue';
import { logger } from '@/utils/logger';
import { cv, AppleLayout, AppleSpace } from '@/utils/core/apple-tokens';
import { formatTime } from '@/utils/format/format-time';
import {
  listStyle,
  actionGroupStyle,
  aiSubSectionHeaderStyle as headerRowStyle,
  aiSubSectionTitleStyle as sectionTitleStyle,
  aiSubCountBadgeStyle as countBadgeStyle,
  aiSubTagRowStyle as tagRowStyle,
  aiSubTagStyle as tagStyle,
  aiSubSectionMetaStyle as hintStyle,
} from '@/styles/ai/ai-page-styles';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import {
  createWebhook,
  deleteWebhook,
  getWebhookDeliveries,
  getWebhookEvents,
  getWebhooks,
  updateWebhook,
  type WebhookConfig,
  type WebhookDelivery,
} from '@/api/modules/social/webhook';
import { useSpaceStore } from '@/stores/workspace/space';
import { useUIStore } from '@/stores/app/ui';
import type { Space } from '@/types/models';

const ui = useUIStore();
const spaceStore = useSpaceStore();

const GLOBAL_SPACE = 0;

const webhooks = ref<WebhookConfig[]>([]);
const spaces = ref<Space[]>([]);
const supportedEvents = ref<string[]>([]);
const loading = ref(false);
const saving = ref(false);
const togglingId = ref<number | null>(null);
const deleting = ref(false);

const showModal = ref(false);
const editingId = ref<number | null>(null);
const eventPick = ref<string>('');

const form = ref({
  name: '',
  url: '',
  secret: '',
  space_id: GLOBAL_SPACE as number | string,
  events: [] as string[],
  isActive: true,
});

const showDeleteModal = ref(false);
const deletingItem = ref<WebhookConfig | null>(null);

const showDeliveryModal = ref(false);
const deliveryTarget = ref<WebhookConfig | null>(null);
const deliveries = ref<WebhookDelivery[]>([]);
const deliveryLoading = ref(false);

const nameError = ref('');
const urlError = ref('');
const eventsError = ref('');

const spaceOptions = computed(() => [
  { label: t('social.k76'), value: GLOBAL_SPACE },
  ...spaces.value.map((s) => ({ label: s.name, value: s.id })),
]);

const availableEventOptions = computed(() =>
  supportedEvents.value
    .filter((ev) => !form.value.events.includes(ev))
    .map((ev) => ({ label: eventLabel(ev), value: ev })),
);

const spaceNames = computed(() => {
  const map = new Map<number, string>();
  spaces.value.forEach((s) => map.set(s.id, s.name));
  return map;
});

function eventLabel(ev: string) {
  const labels: Record<string, string> = {
    'doc.created': t('adminForm.k77'),
    'doc.updated': t('adminForm.k78'),
    'doc.deleted': t('adminForm.k79'),
    'comment.created': t('adminForm.k80'),
    'approval.submitted': t('adminForm.k81'),
    'approval.handled': t('adminForm.k82'),
  };
  return `${labels[ev] ?? ev}（${ev}）`;
}

const webhookColumns = computed<TableColumn<WebhookConfig>[]>(() => [
  { key: 'name', title: t('social.name'), width: '18%', ellipsis: true },
  { key: 'url', title: t('social.k64'), ellipsis: true },
  { key: 'events', title: t('social.k65'), width: '26%', ellipsis: true },
  { key: 'space_id', title: t('social.k17'), width: '14%', render: (row) => spaceNames.value.get(row.space_id) ?? t('fallback.k11') },
  { key: 'is_active', title: t('social.status'), width: '8%', render: (row) => (row.is_active === 1 ? t('vAdminUi.enable') : t('vAdminUi.k27')) },
  { key: 'update_time', title: t('social.k78'), width: '16%', render: (row) => formatTime(row.update_time) },
]);

const deliveryColumns: TableColumn<WebhookDelivery>[] = [
  { key: 'event', title: t('social.k79'), width: '20%' },
  { key: 'status_code', title: t('social.k80'), width: '12%' },
  { key: 'success', title: t('social.k81'), width: '10%', render: (row) => (row.success === 1 ? t('adminForm.k83') : t('adminForm.k84')) },
  { key: 'retry_count', title: t('social.k82'), width: '10%' },
  { key: 'response', title: t('social.k83'), ellipsis: true },
  { key: 'create_time', title: t('social.k84'), width: '18%', render: (row) => formatTime(row.create_time) },
];

async function loadWebhooks() {
  loading.value = true;
  try {
    const res = await getWebhooks();
    webhooks.value = res ?? [];
  } catch (e) {
    logger.error('Failed to load webhooks:', e);
    ui.showToast((e as Error).message || t('fallback.k23'), 'error');
  } finally {
    loading.value = false;
  }
}

async function loadEvents() {
  try {
    const res = await getWebhookEvents();
    supportedEvents.value = res ?? [];
  } catch (e) {
    logger.error('Failed to load webhook events:', e);
    ui.showToast(t('social.k85'), 'error');
  }
}

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
  } catch (e) {
    logger.error('Failed to load spaces:', e);
  }
}

function resetForm() {
  form.value = {
    name: '',
    url: '',
    secret: '',
    space_id: GLOBAL_SPACE,
    events: [],
    isActive: true,
  };
  nameError.value = '';
  urlError.value = '';
  eventsError.value = '';
  eventPick.value = '';
}

function openModal(item?: WebhookConfig) {
  if (item) {
    editingId.value = item.id;
    form.value = {
      name: item.name,
      url: item.url,
      secret: item.secret ?? '',
      space_id: item.space_id,
      events: item.events ? item.events.split(',').filter(Boolean) : [],
      isActive: item.is_active === 1,
    };
  } else {
    editingId.value = null;
    resetForm();
  }
  showModal.value = true;
}

function addEvent() {
  const picked = eventPick.value;
  if (!picked) return;
  if (!form.value.events.includes(picked)) form.value.events = [...form.value.events, picked];
  eventPick.value = '';
  eventsError.value = '';
}

function removeEvent(ev: string) {
  form.value.events = form.value.events.filter((item) => item !== ev);
}

function validateForm(): boolean {
  nameError.value = form.value.name.trim() ? '' : t('adminForm.k85');
  urlError.value = /^https?:\/\/.+/.test(form.value.url.trim()) ? '' : t('adminForm.k86');
  eventsError.value = form.value.events.length ? '' : t('adminForm.k87');
  return !nameError.value && !urlError.value && !eventsError.value;
}

async function handleSave() {
  if (!validateForm()) return;
  saving.value = true;
  try {
    if (editingId.value) {
      await updateWebhook(editingId.value, {
        name: form.value.name.trim(),
        url: form.value.url.trim(),
        secret: form.value.secret.trim(),
        events: form.value.events.join(','),
        is_active: form.value.isActive ? 1 : 0,
      });
      ui.showToast(t('social.k86'), 'success');
    } else {
      await createWebhook({
        name: form.value.name.trim(),
        url: form.value.url.trim(),
        secret: form.value.secret.trim() || undefined,
        events: form.value.events.join(','),
        space_id: Number(form.value.space_id) || GLOBAL_SPACE,
      });
      ui.showToast(t('social.k87'), 'success');
    }
    showModal.value = false;
    await loadWebhooks();
  } catch (e) {
    logger.error('Failed to save webhook:', e);
    ui.showToast((e as Error).message || t('fallback.k8'), 'error');
  } finally {
    saving.value = false;
  }
}

async function toggleActive(item: WebhookConfig) {
  togglingId.value = item.id;
  try {
    await updateWebhook(item.id, { is_active: item.is_active === 1 ? 0 : 1 });
    ui.showToast(item.is_active === 1 ? t('adminForm.k35') : t('adminForm.k34'), 'success');
    await loadWebhooks();
  } catch (e) {
    logger.error('Failed to toggle webhook:', e);
    ui.showToast((e as Error).message || t('fallback.k64'), 'error');
  } finally {
    togglingId.value = null;
  }
}

async function openDeliveries(item: WebhookConfig) {
  deliveryTarget.value = item;
  showDeliveryModal.value = true;
  deliveryLoading.value = true;
  deliveries.value = [];
  try {
    const res = await getWebhookDeliveries(item.id, 20);
    deliveries.value = res ?? [];
  } catch (e) {
    logger.error('Failed to load deliveries:', e);
    ui.showToast((e as Error).message || t('fallback.k33'), 'error');
  } finally {
    deliveryLoading.value = false;
  }
}

function askDelete(item: WebhookConfig) {
  deletingItem.value = item;
  showDeleteModal.value = true;
}

async function confirmDelete() {
  const target = deletingItem.value;
  if (!target) return;
  deleting.value = true;
  try {
    await deleteWebhook(target.id);
    ui.showToast(t('social.k88'), 'success');
    showDeleteModal.value = false;
    deletingItem.value = null;
    await loadWebhooks();
  } catch (e) {
    logger.error('Failed to delete webhook:', e);
    ui.showToast((e as Error).message || t('fallback.k20'), 'error');
  } finally {
    deleting.value = false;
  }
}

onMounted(() => {
  loadWebhooks();
  loadEvents();
  loadSpaces();
});
</script>
