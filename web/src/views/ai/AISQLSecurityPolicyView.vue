<template>
  <div :style="aiSubPageStyle">
    <!-- 顶层 Tab：安全策略 / 错误看板 / 语义指标 -->
    <ApSegmentedControl v-model="topTab" :options="topTabs" />

    <!-- 错误看板 Tab -->
    <AIErrorDashboardView v-if="topTab === 'errors'" />

    <!-- 语义指标 Tab -->
    <AISemanticMetricView v-else-if="topTab === 'metrics'" />

    <!-- 安全策略 Tab -->
    <template v-else>
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Shield" :size="16" :color="cv('blue')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('aiVec.k44') }}</span>
        </div>
      </template>

      <ApSkeleton v-if="policyLoading" :lines="4" />
      <ApEmpty v-else-if="spaces.length === 0" icon="Shield" :title="$t('aiVec.noData')" />
      <template v-else>
      <div :style="formStyle">
        <ApFormItem :label="$t('aiVec.k46')">
          <ApSelect v-model="selectedSpaceId" :options="spaceOptions" :placeholder="$t('aiVec.k27')" :searchable="true" @update:model-value="loadPolicy" />
        </ApFormItem>

        <ApFormItem :label="$t('aiVec.k47')">
          <ApInput v-model="form.max_rows" type="number" placeholder="200" />
        </ApFormItem>

        <ApFormItem :label="$t('aiVec.k48')">
          <ApInput v-model="form.timeout_sec" type="number" placeholder="15" />
        </ApFormItem>

        <ApFormItem :label="$t('aiVec.k49')">
          <ApInput v-model="form.max_cost_tokens" type="number" placeholder="0" />
        </ApFormItem>

        <ApFormItem :label="$t('aiVec.k50')">
          <ApTextarea v-model="form.allow_patterns" :rows="3" :placeholder="$t('aiVec.k54')" />
        </ApFormItem>

        <ApFormItem :label="$t('aiVec.k51')">
          <ApTextarea v-model="form.deny_patterns" :rows="3" :placeholder="$t('aiVec.k55')" />
        </ApFormItem>

        <div :style="switchRowStyle">
          <ApSwitch v-model="form.allow_dml" :label="$t('aiVec.k52')" />
        </div>

        <div :style="switchRowStyle">
          <ApSwitch v-model="form.allow_ddl" :label="$t('aiVec.k53')" />
        </div>
      </div>

      <div :style="aiSubActionRowStyle">
        <ApButton variant="primary" size="small" :loading="saving" @click="handleSave">
          <ApIcon name="Save" :size="16" /> {{ $t('aiVec.k45') }}
        </ApButton>
      </div>
      </template>
    </ApCard>
    </template>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, onMounted, computed } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import AIErrorDashboardView from './settings/AIErrorDashboardView.vue';
import AISemanticMetricView from './settings/AISemanticMetricView.vue';

// 顶层 Tab：安全策略 / 错误看板 / 语义指标
const topTab = ref('security');
const topTabs = [
  { value: 'security', label: t('aiVec.k57'), icon: 'Shield' },
  { value: 'errors', label: t('aiVec.k58'), icon: 'Activity' },
  { value: 'metrics', label: t('aiVec.k59'), icon: 'Database' },
];


import { logger } from '@/utils/logger';
import { cv, AppleSpace } from '@/utils/core/apple-tokens';
import { aiSubPageStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle, aiSubActionRowStyle } from '@/styles/ai/ai-page-styles';
import { getSQLSecurityPolicy, updateSQLSecurityPolicy, type SQLSecurityPolicy } from '@/api/modules/ai/ai-data';
import { useSpaceStore } from '@/stores/workspace/space';
import type { Space } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();
const spaceStore = useSpaceStore();
const spaces = ref<Space[]>([]);
const selectedSpaceId = ref<number | null>(null);
const saving = ref(false);
const policyLoading = ref(false);

interface PolicyForm {
  space_id: number;
  max_rows: string;
  timeout_sec: string;
  max_cost_tokens: string;
  allow_patterns: string;
  deny_patterns: string;
  allow_dml: boolean;
  allow_ddl: boolean;
}

const form = ref<PolicyForm>({
  space_id: 0,
  max_rows: '200',
  timeout_sec: '15',
  max_cost_tokens: '0',
  allow_patterns: '',
  deny_patterns: '',
  allow_dml: false,
  allow_ddl: false,
});

const spaceOptions = computed(() =>
  spaces.value.map((s) => ({ label: s.name, value: s.id }))
);

onMounted(async () => {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
    const firstSpace = spaces.value[0];
    if (firstSpace) {
      selectedSpaceId.value = firstSpace.id;
      await loadPolicy();
    }
  } catch (e) {
    logger.error('Failed to load spaces:', e);
    ui.showToast(t('aiVec.k60'), 'error');
  }
});

function policyToForm(policy: SQLSecurityPolicy): PolicyForm {
  return {
    space_id: policy.space_id,
    max_rows: String(policy.max_rows ?? 200),
    timeout_sec: String(policy.timeout_sec ?? 15),
    max_cost_tokens: String(policy.max_cost_tokens ?? 0),
    allow_patterns: policy.allow_patterns || '',
    deny_patterns: policy.deny_patterns || '',
    allow_dml: policy.allow_dml || false,
    allow_ddl: policy.allow_ddl || false,
  };
}

function formToPolicy(formValue: PolicyForm): SQLSecurityPolicy {
  return {
    space_id: formValue.space_id,
    max_rows: Number(formValue.max_rows) || 0,
    timeout_sec: Number(formValue.timeout_sec) || 0,
    max_cost_tokens: Number(formValue.max_cost_tokens) || 0,
    allow_patterns: formValue.allow_patterns,
    deny_patterns: formValue.deny_patterns,
    allow_dml: formValue.allow_dml,
    allow_ddl: formValue.allow_ddl,
  };
}

async function loadPolicy() {
  if (!selectedSpaceId.value) return;
  policyLoading.value = true;
  try {
    // P1-3: 接口返回的是策略对象本身而非 { data } 信封，直接用 res，
    // 否则 policy 恒为 undefined，保存时会以默认值静默覆盖线上策略
    const policy = await getSQLSecurityPolicy(selectedSpaceId.value);
    if (policy) {
      form.value = policyToForm(policy);
    }
  } catch (e) {
    logger.error('Failed to load security policy:', e);
    ui.showToast(t('aiVec.k61'), 'error');
  } finally {
    policyLoading.value = false;
  }
}

async function handleSave() {
  if (!selectedSpaceId.value) return;
  saving.value = true;
  try {
    form.value.space_id = selectedSpaceId.value;
    await updateSQLSecurityPolicy(formToPolicy(form.value));
    ui.showToast(t('aiVec.k62'), 'success');
  } catch (e) {
    logger.error('Failed to save security policy:', e);
    ui.showToast(t('aiVec.k63'), 'error');
  } finally {
    saving.value = false;
  }
}

const formStyle = {
  display: 'grid', gap: AppleSpace.md,
};

const switchRowStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.sm,
};


</script>
