<template>
  <div :style="aiSubPageStyle">
      <!-- 统计卡片 -->
      <div :style="crudStatsRowStyle('compact')">
        <ApStatCard :label="$t('aiView.k125')" :value="sources.length" :icon="'Database'" :color="cv('blue')" size="compact" />
        <ApStatCard :label="$t('aiView.k126')" :value="activeCount" :icon="'CircleCheck'" :color="cv('green')" size="compact" />
        <ApStatCard label="MySQL" :value="mysqlCount" :icon="'Server'" :color="cv('purple')" size="compact" />
        <ApStatCard label="PostgreSQL" :value="postgresCount" :icon="'Server'" :color="cv('orange')" size="compact" />
      </div>

      <!-- 操作栏 -->
      <div :style="aiSubSectionHeaderStyle">
      <span :style="sectionTitleStyle">{{ $t('aiView.k119') }}</span>
      <ApSelect v-model="selectedSpaceId" :options="spaceOptions" :placeholder="$t('aiView.k135')" searchable :style="{ width: '220px', flexShrink: '0' }" @update:model-value="fetchSources" />
        <ApButton variant="primary" size="small" @click="openCreate">
          <ApIcon name="Plus" :size="16" /> {{ $t('aiView.k120') }}
        </ApButton>
      </div>

      <!-- 加载中 -->
      <ApSkeleton v-if="loading" variant="grid" :count="4" />

      <!-- 空状态 -->
      <ApEmpty
        v-else-if="sources.length === 0"
        :title="$t('aiView.k138')"
        :description="$t('vAiUi.k19')"
        icon="Database"
        :action-text="$t('lastAttrsUi.k3')"
        @action="openCreate"
      />

      <!-- 数据源列表 -->
      <div v-else :style="gridStyle">
        <div v-for="ds in sources" :key="ds.id" :style="cardStyle">
          <div :style="cardHeaderStyle">
            <div :style="cardTitleRowStyle">
              <span :style="cardNameStyle">{{ ds.name }}</span>
              <span :style="dbTypeBadgeStyle(ds.db_type)">{{ ds.db_type }}</span>
            </div>
            <div :style="statusDotContainerStyle">
              <span :style="statusDotStyle(ds.is_active)" />
            </div>
          </div>
          <div :style="cardInfoStyle">
            <ApIcon name="Server" :size="14" :color="cv('text-tertiary')" />
            <span>{{ ds.host }}:{{ ds.port }}</span>
          </div>
          <div :style="cardInfoStyle">
            <ApIcon name="Database" :size="14" :color="cv('text-tertiary')" />
            <span>{{ ds.db_name }}</span>
          </div>
          <div v-if="ds.last_sync_at" :style="cardInfoStyle">
            <ApIcon name="RefreshCw" :size="14" :color="cv('text-tertiary')" />
            <span>上次同步: {{ formatTime(ds.last_sync_at) }}</span>
          </div>
          <div :style="cardActionsStyle">
            <ApButton variant="ghost" size="small" :loading="testingId === ds.id" @click="handleTestConnection(ds)">
              <ApIcon name="Zap" :size="16" /> {{ $t('aiView.test') }}
            </ApButton>
            <ApButton variant="ghost" size="small" :loading="syncingId === ds.id" @click="handleSync(ds)">
              <ApIcon name="RefreshCw" :size="16" /> {{ $t('aiView.k122') }}
            </ApButton>
            <ApButton variant="ghost" size="small" @click="openEdit(ds)">
              <ApIcon name="Pencil" :size="16" /> {{ $t('aiView.edit') }}
            </ApButton>
            <ApButton variant="danger" size="mini" icon-only :aria-label="$t('aiView.k139')" @click="openDelete(ds)">
              <ApIcon name="Trash2" :size="16" />
            </ApButton>
          </div>
        </div>
      </div>

      <!-- 新建/编辑弹窗 -->
      <ApModal v-model="showFormModal" size="large" :title="editingId ? '编辑数据源' : '新建数据源'" density="compact">
        <div :style="formGridStyle">
          <ApInput v-model="form.name" :label="$t('aiView.name')" :placeholder="$t('aiView.k136')" />
          <!-- P1-4/P1-16: 后端 space_id 为 binding:"required" 且列表按 space_id 精确过滤，
               自由输入 0 会被 400 拒绝，改为从我的空间中选择 -->
          <ApSelect v-model="form.space_id" :options="spaceOptions" :label="$t('aiView.k128')" :placeholder="$t('aiView.k17')" searchable />
          <ApSelect v-model="form.db_type" :options="dbTypeOptions" :label="$t('aiView.k129')" />
          <ApInput v-model="form.host" :label="$t('aiView.k130')" placeholder="localhost" />
          <ApInput v-model="form.port" type="number" :label="$t('aiView.k131')" placeholder="3306" />
          <ApInput v-model="form.db_name" :label="$t('aiView.k132')" placeholder="db_name" />
          <ApInput v-model="form.username" :label="$t('aiView.k133')" placeholder="root" />
          <ApInput v-model="form.password" type="password" :label="editingId ? '密码（编辑需重输）' : '密码'" :placeholder="editingId ? '出于安全考虑不回显，请重新输入' : '••••••'" />
        </div>
        <template #footer>
          <ApButton variant="ghost" size="small" @click="showFormModal = false">{{ $t('aiView.cancel') }}</ApButton>
          <ApButton size="small" :loading="saving" @click="handleSave">
            {{ editingId ? $t('vAiUi.k20') : $t('vAiUi.k21') }}
          </ApButton>
        </template>
      </ApModal>

      <!-- P1-76: 后端测试接口要求明文密码且列表接口抹掉密码，测试已有数据源需临时补录密码 -->
      <ApModal v-model="showTestPwdModal" size="small" :title="$t('aiView.k77')" density="compact">
        <ApInput v-model="testPassword" type="password" :label="$t('aiView.k134')" :placeholder="$t('aiView.k137')" />
        <template #footer>
          <ApButton variant="ghost" size="small" @click="showTestPwdModal = false">{{ $t('aiView.cancel') }}</ApButton>
          <ApButton size="small" :loading="testingId !== null" @click="confirmTestConnection">{{ $t('aiView.test') }}</ApButton>
        </template>
      </ApModal>

      <!-- 删除确认弹窗 -->
      <ApConfirmModal
        v-model="showDeleteModal"
        :title="$t('aiView.k139')"
        :message="`确定删除数据源「${deletingDs?.name}」吗？此操作不可撤销。`"
        type="danger"
        :confirm-text="$t('aiView.delete')"
        @confirm="handleDelete"
      />
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted } from 'vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';







import { logger } from '@/utils/logger';
import { cv, AppleFont, AppleRadius, AppleSpace, AppleType, AppleTransition } from '@/utils/core/apple-tokens';
import { aiSubPageStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle, aiSubFormGridStyle, aiSubCardStyle, aiSubCardHeaderStyle, aiSubCardNameRowStyle, aiSubCardNameStyle, aiSubStatusDotStyle, crudStatsRowStyle, gridWide } from '@/styles/ai/ai-page-styles';
import { useUIStore } from '@/stores/app/ui';
import { useSpaceStore } from '@/stores/workspace/space';
import type { Space } from '@/types/models';
import { formatTime } from '@/utils/format/format-time';
import { listDataSources, createDataSource, updateDataSource, deleteDataSource, testConnection, syncDataSource, type DataSource } from '@/api/modules/ai/ai-datasource';

const ui = useUIStore();
const spaceStore = useSpaceStore();

const loading = ref(true);
const saving = ref(false);
const testingId = ref<number | null>(null);
const syncingId = ref<number | null>(null);

const sources = ref<DataSource[]>([]);

// P1-4: 后端列表接口按 space_id 精确过滤（不传即 0），需要空间选择器驱动列表
const spaces = ref<Space[]>([]);
const selectedSpaceId = ref<number | null>(null);
const spaceOptions = computed(() =>
  spaces.value.map((s) => ({ label: s.name, value: s.id }))
);

const showFormModal = ref(false);
const showDeleteModal = ref(false);
const editingId = ref<number | null>(null);
const deletingDs = ref<DataSource | null>(null);

// P1-76: 测试已有数据源需临时补录密码（后端抹掉密码下发，无按 id 测试端点）
const showTestPwdModal = ref(false);
const testPassword = ref('');
const pendingTestDs = ref<DataSource | null>(null);

const form = ref({
  name: '',
  db_type: 'mysql',
  host: '',
  port: '3306',
  db_name: '',
  username: '',
  password: '',
  space_id: null as string | number | null,
});

const dbTypeOptions = [
  { label: 'MySQL', value: 'mysql' },
  { label: 'PostgreSQL', value: 'postgres' },
  { label: 'Oracle', value: 'oracle' },
];

const activeCount = computed(() => sources.value.filter(s => s.is_active).length);
const mysqlCount = computed(() => sources.value.filter(s => s.db_type === 'mysql').length);
const postgresCount = computed(() => sources.value.filter(s => s.db_type === 'postgres').length);

async function fetchSources() {
  if (selectedSpaceId.value == null) {
    sources.value = [];
    return;
  }
  loading.value = true;
  try {
    // P1-4: 后端直接返回裸数组，此前 res.datasources 恒为 undefined 导致列表永远为空
    const res = await listDataSources(Number(selectedSpaceId.value));
    sources.value = Array.isArray(res) ? res : [];
  } catch (e) {
    logger.error('Failed to load data sources:', e);
    ui.showToast(t('aiView.k142'), 'error');
  } finally {
    loading.value = false;
  }
}

function resetForm() {
  form.value = {
    name: '',
    db_type: 'mysql',
    host: '',
    port: '3306',
    db_name: '',
    username: '',
    password: '',
    space_id: selectedSpaceId.value,
  };
}

function openCreate() {
  editingId.value = null;
  resetForm();
  showFormModal.value = true;
}

function openEdit(ds: DataSource) {
  editingId.value = ds.id;
  // P1-16: 回填 db_type/space_id（后端 Update 复用 Create 请求体，这些字段必填）；
  // 密码不回显但必须重输，否则会被后端 required 校验 400 拒绝
  form.value = {
    name: ds.name,
    db_type: ds.db_type,
    host: ds.host,
    port: String(ds.port),
    db_name: ds.db_name,
    username: ds.username,
    password: '',
    space_id: ds.space_id,
  };
  showFormModal.value = true;
}

function openDelete(ds: DataSource) {
  deletingDs.value = ds;
  showDeleteModal.value = true;
}

async function handleSave() {
  if (form.value.space_id == null || Number(form.value.space_id) <= 0) {
    ui.showToast(t('aiView.k143'), 'error');
    return;
  }
  // P1-16: 后端 Update 复用 Create 契约（password binding:"required" 且无条件覆盖存储值），
  // 编辑时必须重输密码，否则请求 400 或已存密码被清空
  if (editingId.value && !form.value.password) {
    ui.showToast(t('aiView.k144'), 'error');
    return;
  }
  saving.value = true;
  try {
    const { name, db_type, host, port, db_name, username, password, space_id } = form.value;
    const payload = { name, db_type, host, port: Number(port), db_name, username, password, space_id: Number(space_id) };
    if (editingId.value) {
      await updateDataSource(editingId.value, payload);
      ui.showToast(t('aiView.k145'), 'success');
    } else {
      await createDataSource(payload);
      ui.showToast(t('aiView.k146'), 'success');
    }
    showFormModal.value = false;
    await fetchSources();
  } catch (e: unknown) {
    ui.showToast((e as { message?: string }).message || t('fallback.k64'), 'error');
  } finally {
    saving.value = false;
  }
}

async function handleDelete() {
  if (!deletingDs.value) return;
  try {
    await deleteDataSource(deletingDs.value.id);
    ui.showToast(t('aiView.k147'), 'success');
    showDeleteModal.value = false;
    deletingDs.value = null;
    await fetchSources();
  } catch (e: unknown) {
    ui.showToast((e as { message?: string }).message || t('fallback.k20'), 'error');
  }
}

// P1-76: 后端抹掉密码下发且 TestConnectionRequest.password 必填，此前固定传空串恒被 400 拒绝；
// 改为点击“测试”先补录密码再发起测试
function handleTestConnection(ds: DataSource) {
  pendingTestDs.value = ds;
  testPassword.value = '';
  showTestPwdModal.value = true;
}

async function confirmTestConnection() {
  const ds = pendingTestDs.value;
  if (!ds) return;
  if (!testPassword.value) {
    ui.showToast(t('aiView.k148'), 'error');
    return;
  }
  testingId.value = ds.id;
  try {
    await testConnection({
      db_type: ds.db_type,
      host: ds.host,
      port: ds.port,
      db_name: ds.db_name,
      username: ds.username,
      password: testPassword.value,
    });
    // 成功时后端返回 { message: t('aiView.k141') }；失败走 400 进入 catch
    ui.showToast(t('aiView.k141'), 'success');
    showTestPwdModal.value = false;
    pendingTestDs.value = null;
  } catch (e: unknown) {
    ui.showToast((e as { message?: string }).message || t('fallback.k87'), 'error');
  } finally {
    testingId.value = null;
  }
}

async function handleSync(ds: DataSource) {
  syncingId.value = ds.id;
  try {
    const res = await syncDataSource(ds.id);
    ui.showToast(res.message || t('fallback.k43'), 'success');
    await fetchSources();
  } catch (e: unknown) {
    ui.showToast((e as { message?: string }).message || t('fallback.k42'), 'error');
  } finally {
    syncingId.value = null;
  }
}

onMounted(async () => {
  try {
    // P1-4: 列表按空间过滤，先取“我的空间”并默认选中第一个
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
    const first = spaces.value[0];
    if (first) {
      selectedSpaceId.value = first.id;
    }
  } catch (e) {
    logger.error('Failed to load spaces:', e);
    ui.showToast(t('aiView.k149'), 'error');
  }
  await fetchSources();
});

// === Styles ===

const sectionTitleStyle = aiSubSectionTitleStyle;

const gridStyle = gridWide;

const cardStyle = {
  ...aiSubCardStyle,
  transition: AppleTransition.default,
};

const cardHeaderStyle = {
  ...aiSubCardHeaderStyle,
  marginBottom: AppleSpace.sm,
};

const cardTitleRowStyle = aiSubCardNameRowStyle;

const cardNameStyle = aiSubCardNameStyle;

const dbTypeBadgeStyle = (dbType: string) => ({
  fontFamily: AppleFont.mono,
  fontSize: AppleType.legalMeta.size,
  padding: `1px ${AppleSpace.sm}`,
  borderRadius: AppleRadius.micro,
  backgroundColor: dbType === 'mysql' ? cv('blue-light') : dbType === 'postgres' ? cv('purple-light') : cv('orange-light'),
  color: dbType === 'mysql' ? cv('blue') : dbType === 'postgres' ? cv('purple') : cv('orange'),
});

const statusDotContainerStyle = {
  display: 'flex',
  alignItems: 'center',
};

const statusDotStyle = aiSubStatusDotStyle;

const cardInfoStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  fontSize: AppleType.legalMeta.size,
  color: cv('text-secondary'),
  marginBottom: AppleSpace.xxs,
};

const cardActionsStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  marginTop: AppleSpace.md,
  paddingTop: AppleSpace.sm,
  borderTop: `1px solid ${cv('border-light')}`,
};

const formGridStyle = aiSubFormGridStyle;
</script>
