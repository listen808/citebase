<template>
  <div :style="aiSubPageStyle">
    <ApSegmentedControl v-model="activeTab" :options="tabs" />

    <div :style="contentStyle">
      <!-- Tab 1: Schema 空间 -->
      <template v-if="activeTab === 'spaces'">
        <div :style="{ ...crudStatsRowStyle('compact'), marginBottom: 0 }">
          <ApStatCard
            v-for="stat in schemaStats"
            :key="stat.label"
            :icon="stat.icon"
            :label="stat.label"
            :value="stat.value"
            :color="stat.color"
            size="compact"
          />
        </div>

        <div :style="sectionStyle">
          <div :style="aiSubSectionHeaderStyle">
            <span :style="sectionTitleStyle">{{ $t('aiVec.k64') }}</span>
            <ApButton variant="primary" size="small" @click="showCreateSpace = true">
              <ApIcon name="Plus" :size="16" /> {{ $t('aiVec.k65') }}
            </ApButton>
          </div>
          <ApSkeleton v-if="spacesLoading" variant="card" :lines="3" />
          <div v-else-if="spaces.length === 0">
            <ApEmpty icon="Database" :title="$t('aiVec.k94')" :description="$t('vAiUi.k62')" />
          </div>
          <div v-else>
            <div v-for="space in spaces" :key="space.id" :style="spaceCardStyle">
              <div :style="spaceHeaderStyle">
                <div :style="spaceInfoStyle">
                  <div :style="spaceNameStyle">{{ space.name }}</div>
                  <div :style="spaceMetaStyle">
                    <span :style="dbTypeBadgeStyle(space.db_type)">{{ space.db_type }}</span>
                    <span :style="spaceMetaItemStyle"><ApIcon name="ExternalLink" :size="12" :color="cv('text-tertiary')" />关联 Space #{{ space.space_id }}</span>
                    <span :style="spaceMetaItemStyle"><ApIcon name="Database" :size="12" :color="cv('text-tertiary')" />{{ space.table_count || 0 }} {{ $t('vAiUi.k69') }}</span>
                  </div>
                </div>
                <div :style="spaceActionsStyle">
                  <ApButton variant="secondary" size="small" :loading="scanningId === space.id" :disabled="scanningId === space.id" @click="handleScan(space)">
                    <ApIcon v-if="scanningId !== space.id" name="RefreshCw" :size="16" />
                    {{ scanningId === space.id ? '扫描中...' : $t('vAiUi.k68') }}
                  </ApButton>
                  <ApButton variant="danger" size="small" @click="onDeleteSpace(space)"><ApIcon name="Trash2" :size="16" />{{ $t('aiVec.delete') }}</ApButton>
                </div>
              </div>
              <div v-if="space.last_scan_at" :style="spaceScanInfoStyle">
                <ApIcon name="Clock" :size="12" :color="cv('text-tertiary')" />
                <span>上次扫描: {{ formatTime(space.last_scan_at) }}</span>
              </div>
            </div>
          </div>
        </div>

        <ApModal v-model="showCreateSpace" size="medium" density="compact" :title="$t('aiVec.k65')">
          <div :style="formGroupStyle">
            <ApInput
              v-model="newSpace.name"
              :label="$t('aiVec.name')"
              :placeholder="$t('aiVec.k86')"
            />
          </div>
          <div :style="formGroupStyle">
            <ApSelect
              v-model="newSpace.space_id"
              :label="$t('aiVec.k78')"
              :options="workspaceOptions"
              :searchable="true"
            />
          </div>
          <div :style="formGroupStyle">
            <ApSelect
              v-model="newSpace.db_type"
              :label="$t('aiVec.k79')"
              :options="dbTypeOptions"
            />
          </div>
          <template #footer>
            <ApButton variant="ghost" size="small" @click="showCreateSpace = false">{{ $t('aiVec.cancel') }}</ApButton>
            <ApButton variant="primary" size="small" :loading="spaceCreating" @click="handleCreateSpace">{{ $t('aiVec.k68') }}</ApButton>
          </template>
        </ApModal>
      </template>

      <!-- Tab 2: 表结构浏览 -->
      <template v-if="activeTab === 'tables'">
        <div :style="filterRowStyle">
          <ApSelect
            v-model="selectedSpaceForTables"
            :options="spaceOptions"
            :placeholder="$t('aiVec.k87')"
            :searchable="true"
            @update:model-value="loadTables"
          />
          <ApInput
            v-model="tableKeyword"
            :placeholder="$t('aiVec.k88')"
            @update:model-value="handleTableSearch"
          />
        </div>

        <ApSkeleton v-if="tablesLoading" variant="list" :lines="5" />
        <div v-else-if="!selectedSpaceForTables">
          <ApEmpty icon="Table" :title="$t('aiVec.k95')" :description="$t('vAiUi.k63')" />
        </div>
        <div v-else-if="tables.length === 0">
          <ApEmpty icon="Table" :title="$t('aiVec.k96')" :description="$t('vAiUi.k64')" />
        </div>
        <div v-else>
          <div :style="tableCountStyle">
            <span :style="tableCountTextStyle">共 {{ tableTotal }} 张表</span>
            <span v-if="tableKeyword" :style="tableCountSubStyle">（匹配 {{ tables.length }} 张）</span>
          </div>
          <div :style="tableListStyle">
            <div
              v-for="tbl in tables"
              :key="tbl.id"
              :style="tableItemStyle(selectedTable?.table_name === tbl.table_name)"
              @click="handleSelectTable(tbl)"
            >
              <div :style="tableItemNameStyle">{{ tbl.table_name }}</div>
              <div v-if="tbl.table_comment" :style="tableItemCommentStyle">{{ tbl.table_comment }}</div>
            </div>
          </div>

          <div v-if="selectedTable" :style="tableDetailStyle">
            <div :style="detailHeaderStyle">
              <div :style="detailTitleStyle">{{ selectedTable.table_name }}</div>
              <span v-if="selectedTable.table_comment" :style="detailCommentStyle">{{ selectedTable.table_comment }}</span>
            </div>
            <ApSkeleton v-if="tableDetailLoading" variant="list" :lines="5" />
            <div v-else-if="tableColumns.length > 0">
              <ApTable :columns="columnTableColumns" :data="tableColumns" />
            </div>
            <div v-else-if="tableRelations.length > 0" :style="relationsSectionStyle">
              <div :style="relTitleStyle">{{ $t('aiVec.k69') }}</div>
              <div v-for="rel in tableRelations" :key="rel.id" :style="relItemStyle">
                <span>{{ rel.source_table }}.{{ rel.source_column }}</span>
                <ApIcon name="ChevronRight" :size="12" :color="cv('text-tertiary')" />
                <span>{{ rel.target_table }}.{{ rel.target_column }}</span>
                <span :style="relTypeBadgeStyle">{{ rel.relation_type }}</span>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- Tab 3: 业务规则 -->
      <template v-if="activeTab === 'rules'">
        <div :style="filterRowStyle">
          <ApSelect
            v-model="selectedSpaceForRules"
            :options="spaceOptions"
            :placeholder="$t('aiVec.k87')"
            :searchable="true"
            @update:model-value="loadBusinessRules"
          />
          <ApButton v-if="selectedSpaceForRules" variant="secondary" size="small" @click="showCreateRule = true">
            <ApIcon name="Plus" :size="16" /> {{ $t('aiVec.k70') }}
          </ApButton>
        </div>

        <ApSkeleton v-if="rulesLoading" variant="card" :lines="3" />
        <div v-else-if="!selectedSpaceForRules">
          <ApEmpty icon="ScrollText" :title="$t('aiVec.k95')" :description="$t('vAiUi.k65')" />
        </div>
        <div v-else-if="businessRules.length === 0">
          <ApEmpty icon="ScrollText" :title="$t('aiVec.k97')" :description="$t('vAiUi.k66')" />
        </div>
        <div v-else>
          <div v-for="rule in businessRules" :key="rule.id" :style="ruleCardStyle">
            <div :style="ruleHeaderStyle">
              <div :style="ruleNameStyle">{{ rule.name }}</div>
              <div :style="ruleBadgesStyle">
                <span :style="ruleTypeBadgeStyle">{{ rule.type }}</span>
                <ApButton variant="danger" size="mini" icon-only :aria-label="$t('aiVec.k101')" @click="onDeleteRule(rule)"><ApIcon name="Trash2" :size="16" /></ApButton>
              </div>
            </div>
            <div v-if="rule.table_name || rule.field_name" :style="ruleTargetStyle">
              <ApIcon name="Database" :size="12" :color="cv('text-tertiary')" />
              <span>{{ rule.table_name }}{{ rule.field_name ? `.${rule.field_name}` : '' }}</span>
            </div>
            <div :style="ruleContentStyle">{{ rule.content }}</div>
            <div v-if="rule.description" :style="ruleDescStyle">{{ rule.description }}</div>
          </div>
        </div>

        <ApModal v-model="showCreateRule" size="large" density="compact" :title="$t('aiVec.k70')">
          <div :style="formGridStyle">
            <div :style="formGroupStyle">
              <ApInput
                v-model="newRule.name"
                :label="$t('aiVec.k80')"
                :placeholder="$t('aiVec.k80')"
              />
            </div>
            <div :style="formGroupStyle">
              <ApSelect
                v-model="newRule.type"
                :label="$t('aiVec.k81')"
                :options="ruleTypeOptions"
              />
            </div>
            <div :style="formGroupStyle">
              <ApInput
                v-model="newRule.table_name"
                :label="$t('aiVec.k82')"
                :placeholder="$t('aiVec.k89')"
              />
            </div>
            <div :style="formGroupStyle">
              <ApInput
                v-model="newRule.field_name"
                :label="$t('aiVec.k83')"
                :placeholder="$t('aiVec.k90')"
              />
            </div>
          </div>
          <div :style="{ ...formGroupStyle, gridColumn: '1 / -1' }">
            <ApInput
              v-model="newRule.content"
              :label="$t('aiVec.k84')"
              :placeholder="$t('aiVec.k91')"
            />
          </div>
          <div :style="{ ...formGroupStyle, gridColumn: '1 / -1' }">
            <ApInput
              v-model="newRule.description"
              :label="$t('aiVec.k85')"
              :placeholder="$t('aiVec.k92')"
            />
          </div>
          <template #footer>
            <ApButton variant="ghost" size="small" @click="showCreateRule = false">{{ $t('aiVec.cancel') }}</ApButton>
            <ApButton variant="primary" size="small" :loading="ruleCreating" @click="handleCreateRule">{{ $t('aiVec.k68') }}</ApButton>
          </template>
        </ApModal>
      </template>

      <!-- Tab 4: 自然语言查询 -->
      <template v-if="activeTab === 'ask'">
        <div :style="filterRowStyle">
          <ApSelect
            v-model="selectedSpaceForAsk"
            :options="spaceOptions"
            :placeholder="$t('aiVec.k87')"
            :searchable="true"
          />
        </div>

        <div :style="askInputAreaStyle">
          <ApTextarea
            v-model="askQuestion"
            :style="askTextareaStyle"
            :placeholder="$t('aiVec.k93')"
            @keydown.enter.ctrl="handleAsk"
          />
          <div :style="askActionsStyle">
            <span :style="askHintStyle">{{ $t('aiVec.k71') }}</span>
            <ApButton variant="primary" size="small" :loading="askLoading" :disabled="!selectedSpaceForAsk || !askQuestion.trim()" @click="handleAsk">
              <ApIcon name="Send" :size="16" /> {{ $t('aiVec.k72') }}
            </ApButton>
          </div>
        </div>

        <ApSkeleton v-if="askLoading" variant="card" :lines="3" />

        <div v-else-if="askResult" :style="askResultStyle">
          <div :style="resultSectionStyle">
            <div :style="resultLabelStyle"><ApIcon name="Code" :size="14" :color="cv('blue')" /> {{ $t('aiVec.k73') }}</div>
            <pre :style="sqlBlockStyle">{{ askResult.sql }}</pre>
          </div>
          <div :style="resultSectionStyle">
            <div :style="resultLabelStyle"><ApIcon name="FileText" :size="14" :color="cv('green')" /> {{ $t('aiVec.k74') }}</div>
            <div :style="explanationStyle">{{ askResult.explanation }}</div>
          </div>
          <div v-if="askResult.confidence" :style="confidenceBarStyle">
            <span :style="confLabelStyle">{{ $t('aiVec.k75') }}</span>
            <ApProgress :value="askResult.confidence * 100" :label="$t('aiVec.k75')" :tone="toneForScore(askResult.confidence)" :style="{ flex: 1 }" />
            <span :style="confValueStyle">{{ Math.round(askResult.confidence * 100) }}%</span>
          </div>
          <div v-if="askResult.sources?.length" :style="sourcesStyle">
            <span :style="confLabelStyle">{{ $t('aiVec.k76') }}</span>
            <div :style="sourceListStyle">
              <span v-for="src in askResult.sources" :key="src.table_name" :style="sourceTagStyle">
                {{ src.table_name }} <span v-if="src.confidence" :style="sourceConfStyle">{{ Math.round(src.confidence * 100) }}%</span>
              </span>
            </div>
          </div>
          <div v-if="askResult.warnings?.length" :style="warningsStyle">
            <div v-for="(warn, i) in askResult.warnings" :key="i" :style="warningItemStyle">
              <ApIcon name="AlertTriangle" :size="12" :color="cv('orange')" />
              <span>{{ warn }}</span>
            </div>
          </div>
        </div>

        <div v-else-if="!askLoading && !askResult">
          <ApEmpty icon="MessageSquare" :title="$t('aiVec.k98')" :description="$t('vAiUi.k67')" />
        </div>
      </template>

      <!-- Tab 5: 数据源管理 -->
      <template v-if="activeTab === 'datasources'">
        <AIDataSourceView />
      </template>
    </div>

  <ApConfirmModal
    v-model="showDeleteSpaceConfirm"
    :title="$t('aiVec.k99')"
    :message="`确认删除 Schema 空间「${pendingDeleteSpace?.name || ''}」？此操作不可撤销。`"
    :confirm-text="$t('aiVec.delete')"
    type="danger"
    @confirm="confirmDeleteSpace"
  />
  <ApConfirmModal
    v-model="showDeleteRuleConfirm"
    :title="$t('aiVec.k100')"
    :message="`确认删除业务规则「${pendingDeleteRule?.name || ''}」？`"
    :confirm-text="$t('aiVec.delete')"
    type="danger"
    @confirm="confirmDeleteRule"
  />
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted, h } from 'vue';
import { logger } from '@/utils/logger';
import { useUIStore } from '@/stores/app/ui';

import ApSelect from '@/components/apple/ApSelect.vue';
import ApTable from '@/components/apple/ApTable.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';

import type { TableColumn } from '@/components/apple/ApTable.vue';
import { icons } from '@/utils/icons';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import AIDataSourceView from './AIDataSourceView.vue';
import { cv } from '@/utils/core/apple-tokens';
import { toneForScore } from '@/utils/core/progress-tone';
import { useBreakpoint } from '@/utils/responsive';
import { aiSubPageStyle, crudStatsRowStyle, aiSubSectionStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle, aiFormGridStyle, formGroupStyle } from '@/styles/ai/ai-page-styles';
import { formatTime } from '@/utils/format/format-time';
import { schemaContentStyle, schemaSpaceCardStyle, schemaSpaceHeaderStyle, schemaSpaceInfoStyle, schemaSpaceNameStyle, schemaSpaceMetaStyle, schemaSpaceMetaItemStyle, schemaSpaceScanInfoStyle, schemaSpaceActionsStyle, schemaDbTypeBadgeStyle, schemaFilterRowStyle, schemaTableListStyle, schemaTableItemStyle, schemaTableItemNameStyle, schemaTableItemCommentStyle, schemaTableDetailStyle, schemaDetailHeaderStyle, schemaDetailTitleStyle, schemaDetailCommentStyle, schemaTypeBadgeStyle, schemaPkBadgeStyle, schemaFkBadgeStyle, schemaIdxBadgeStyle, schemaTableCountStyle, schemaTableCountTextStyle, schemaTableCountSubStyle, schemaRelationsSectionStyle, schemaRelTitleStyle, schemaRelItemStyle, schemaRelTypeBadgeStyle, schemaRuleCardStyle, schemaRuleHeaderStyle, schemaRuleNameStyle, schemaRuleBadgesStyle, schemaRuleTypeBadgeStyle, schemaRuleTargetStyle, schemaRuleContentStyle, schemaRuleDescStyle, schemaAskInputAreaStyle, schemaAskTextareaStyle, schemaAskActionsStyle, schemaAskHintStyle, schemaAskResultStyle, schemaResultSectionStyle, schemaResultLabelStyle, schemaSqlBlockStyle, schemaExplanationStyle, schemaConfidenceBarStyle, schemaConfLabelStyle, schemaConfValueStyle, schemaSourcesStyle, schemaSourceListStyle, schemaSourceTagStyle, schemaSourceConfStyle, schemaWarningsStyle, schemaWarningItemStyle } from '@/styles/ai/schema-styles';
import { schemaApi, type SchemaSpace, type SchemaTable, type SchemaColumn, type SchemaRelation, type BusinessRule, type SQLResult } from '@/api/modules/schema-sql/schema-sql';
import { getAllSpaces } from '@/api/modules/space/space';
import type { Space } from '@/types/models';

const ui = useUIStore();
const { isMobile } = useBreakpoint();

const activeTab = ref('spaces');
const tabs = [
  { value: 'spaces', label: t('aiVec.k102'), icon: icons.Database },
  { value: 'tables', label: t('aiVec.k103'), icon: icons.Table2 },
  { value: 'rules', label: t('aiVec.k104'), icon: icons.Columns3 },
  { value: 'ask', label: t('aiVec.k98'), icon: icons.Link2 },
  { value: 'datasources', label: t('aiVec.k105'), icon: icons.HardDrive },
];

const spacesLoading = ref(false);
const spaces = ref<SchemaSpace[]>([]);
const scanningId = ref<number | null>(null);
const showCreateSpace = ref(false);
const spaceCreating = ref(false);
const newSpace = ref({ name: '', space_id: '', db_type: 'mysql' });

// 用于下拉选择的工作区列表
const allSpaces = ref<Space[]>([]);
const allSpacesLoading = ref(false);

const schemaStats = ref([
  { label: t('aiVec.k106'), value: '-', icon: icons.Table2, color: cv('blue') },
  { label: t('aiVec.k107'), value: '-', icon: icons.Columns3, color: cv('green') },
  { label: t('aiVec.k108'), value: '-', icon: icons.Link2, color: cv('purple') },
  { label: t('aiVec.k109'), value: '-', icon: icons.RefreshCw, color: cv('orange') },
]);

const selectedSpaceForTables = ref('');
const tableKeyword = ref('');
const tablesLoading = ref(false);
const tables = ref<SchemaTable[]>([]);
const tableTotal = ref(0);
const selectedTable = ref<SchemaTable | null>(null);
const tableDetailLoading = ref(false);
const tableColumns = ref<SchemaColumn[]>([]);
const tableRelations = ref<SchemaRelation[]>([]);

const columnTableColumns: TableColumn<SchemaColumn>[] = [
  { key: 'ordinal_position', title: '#' },
  { key: 'column_name', title: t('aiVec.k110') },
  { key: 'data_type', title: t('aiVec.type'), render: (row) => h('span', { style: typeBadgeStyle }, row.data_type) },
  { key: 'length', title: t('aiVec.k112'), render: (row) => h('span', {}, row.length ? String(row.length) : '-') },
  { key: 'nullable', title: t('aiVec.k113'), render: (row) => h('span', {}, row.nullable === 1 ? '是' : '否') },
  { key: 'default_value', title: t('aiVec.k114'), render: (row) => h('span', {}, row.default_value || '-') },
  { key: 'is_primary_key', title: t('aiVec.k115'), render: (row) => row.is_primary_key === 1 ? h('span', { style: pkBadgeStyle }, 'PK') : h('span', {}, '') },
  { key: 'is_foreign_key', title: t('aiVec.k116'), render: (row) => row.is_foreign_key === 1 ? h('span', { style: fkBadgeStyle }, 'FK') : h('span', {}, '') },
  { key: 'is_index', title: t('aiVec.k117'), render: (row) => row.is_index === 1 ? h('span', { style: idxBadgeStyle }, 'IDX') : h('span', {}, '') },
  { key: 'column_comment', title: t('aiVec.k118'), render: (row) => h('span', {}, row.column_comment || '-') },
];

const selectedSpaceForRules = ref('');
const rulesLoading = ref(false);
const businessRules = ref<BusinessRule[]>([]);
const showCreateRule = ref(false);
const ruleCreating = ref(false);
const newRule = ref({ name: '', type: 'validation', table_name: '', field_name: '', content: '', description: '' });

const selectedSpaceForAsk = ref('');
const askQuestion = ref('');
const askLoading = ref(false);
const askResult = ref<SQLResult | null>(null);

// Computed options for ApSelect components
const dbTypeOptions = [
  { label: 'MySQL', value: 'mysql' },
  { label: 'PostgreSQL', value: 'postgresql' },
  { label: 'Oracle', value: 'oracle' },
];

const ruleTypeOptions = [
  { label: t('aiVec.k119'), value: 'validation' },
  { label: t('aiVec.k120'), value: 'business' },
  { label: t('aiVec.k121'), value: 'constraint' },
  { label: t('aiVec.k122'), value: 'mapping' },
  { label: t('aiVec.k123'), value: 'custom' },
];

const spaceOptions = computed(() => [
  { label: t('aiVec.k87'), value: '' },
  ...spaces.value.map(s => ({ label: s.name, value: String(s.id) }))
]);

const workspaceOptions = computed(() => [
  { label: t('aiVec.k124'), value: '' },
  ...allSpaces.value.map(s => ({ label: s.name, value: String(s.id) }))
]);

async function loadSpaces() {
  spacesLoading.value = true;
  try {
    const res = await schemaApi.listSpaces(1, 50);
    spaces.value = res.data || [];
    await loadAggregatedStats();
  } catch (e) {
    logger.error('Failed to load spaces:', e);
    ui.showToast(t('aiVec.k125'), 'error');
  }
  spacesLoading.value = false;
}

async function loadWorkspaces() {
  if (allSpaces.value.length > 0) return;
  allSpacesLoading.value = true;
  try {
    const res = await getAllSpaces({ page: 1, page_size: 100 });
    allSpaces.value = res.data || [];
  } catch (e) {
    logger.error('Failed to load workspaces:', e);
  }
  allSpacesLoading.value = false;
}

async function loadAggregatedStats() {
  let totalTables = 0;
  let totalColumns = 0;
  let totalRelations = 0;
  let totalScans = 0;
  const results = await Promise.allSettled(spaces.value.map(s => schemaApi.getStats(s.id)));
  for (const r of results) {
    if (r.status === 'fulfilled') {
      totalTables += r.value.table_count || 0;
      totalColumns += r.value.column_count || 0;
      totalRelations += r.value.relation_count || 0;
      totalScans += r.value.scan_task_count || 0;
    }
  }
  schemaStats.value = [
    { label: t('aiVec.k106'), value: String(totalTables), icon: icons.Table2, color: cv('blue') },
    { label: t('aiVec.k107'), value: String(totalColumns), icon: icons.Columns3, color: cv('green') },
    { label: t('aiVec.k108'), value: String(totalRelations), icon: icons.Link2, color: cv('purple') },
    { label: t('aiVec.k109'), value: String(totalScans), icon: icons.RefreshCw, color: cv('orange') },
  ];
}

async function handleCreateSpace() {
  if (!newSpace.value.name.trim()) return;
  if (!newSpace.value.space_id) {
    ui.showToast(t('aiVec.k124'), 'warning');
    return;
  }
  spaceCreating.value = true;
  try {
    await schemaApi.createSpace({
      name: newSpace.value.name,
      space_id: Number(newSpace.value.space_id),
      db_type: newSpace.value.db_type,
    });
    showCreateSpace.value = false;
    newSpace.value = { name: '', space_id: '', db_type: 'mysql' };
    await loadSpaces();
  } catch (e) {
    logger.error('Failed to create space:', e);
    ui.showToast(t('aiVec.k126'), 'error');
  }
  spaceCreating.value = false;
}

const showDeleteSpaceConfirm = ref(false);
const pendingDeleteSpace = ref<SchemaSpace | null>(null);

function onDeleteSpace(space: SchemaSpace) {
  pendingDeleteSpace.value = space;
  showDeleteSpaceConfirm.value = true;
}

async function confirmDeleteSpace() {
  const space = pendingDeleteSpace.value;
  if (!space) return;
  try {
    await schemaApi.deleteSpace(space.id);
    showDeleteSpaceConfirm.value = false;
    pendingDeleteSpace.value = null;
    await loadSpaces();
  } catch (e) {
    logger.error('Failed to delete space:', e);
    ui.showToast(t('aiVec.k127'), 'error');
  }
}

async function handleScan(space: SchemaSpace) {
  scanningId.value = space.id;
  try {
    await schemaApi.startScan(space.id);
    await loadSpaces();
  } catch (e) {
    logger.error('Failed to scan space:', e);
    ui.showToast(t('aiVec.k128'), 'error');
  }
  scanningId.value = null;
}

let tableSearchTimer: ReturnType<typeof setTimeout> | null = null;
function handleTableSearch() {
  if (tableSearchTimer) clearTimeout(tableSearchTimer);
  tableSearchTimer = setTimeout(() => loadTables(), 300);
}

async function loadTables() {
  if (!selectedSpaceForTables.value) return;
  tablesLoading.value = true;
  selectedTable.value = null;
  try {
    const res = await schemaApi.listTables(Number(selectedSpaceForTables.value), tableKeyword.value || undefined, 1, 500);
    tables.value = res.data || [];
    tableTotal.value = res.total ?? tables.value.length;
  } catch (e) {
    logger.error('Failed to load tables:', e);
    ui.showToast(t('aiVec.k129'), 'error');
    tableTotal.value = 0;
  }
  tablesLoading.value = false;
}

async function handleSelectTable(table: SchemaTable) {
  selectedTable.value = table;
  tableDetailLoading.value = true;
  tableColumns.value = [];
  tableRelations.value = [];
  try {
    const res = await schemaApi.getTableDetail(Number(selectedSpaceForTables.value), table.table_name);
    tableColumns.value = res.columns || [];
    tableRelations.value = res.relations || [];
  } catch (e) {
    logger.error('Failed to load table detail:', e);
    ui.showToast(t('aiVec.k130'), 'error');
  }
  tableDetailLoading.value = false;
}

async function loadBusinessRules() {
  if (!selectedSpaceForRules.value) return;
  rulesLoading.value = true;
  try {
    const res = await schemaApi.listBusinessRules(Number(selectedSpaceForRules.value));
    businessRules.value = res.data || [];
  } catch (e) {
    logger.error('Failed to load business rules:', e);
    ui.showToast(t('aiVec.k131'), 'error');
  }
  rulesLoading.value = false;
}

async function handleCreateRule() {
  if (!newRule.value.name.trim() || !newRule.value.content.trim()) return;
  ruleCreating.value = true;
  try {
    await schemaApi.createBusinessRule(Number(selectedSpaceForRules.value), newRule.value);
    showCreateRule.value = false;
    newRule.value = { name: '', type: 'validation', table_name: '', field_name: '', content: '', description: '' };
    await loadBusinessRules();
  } catch (e) {
    logger.error('Failed to create business rule:', e);
    ui.showToast(t('aiVec.k132'), 'error');
  }
  ruleCreating.value = false;
}

const showDeleteRuleConfirm = ref(false);
const pendingDeleteRule = ref<BusinessRule | null>(null);

function onDeleteRule(rule: BusinessRule) {
  pendingDeleteRule.value = rule;
  showDeleteRuleConfirm.value = true;
}

async function confirmDeleteRule() {
  const rule = pendingDeleteRule.value;
  if (!rule) return;
  try {
    await schemaApi.deleteBusinessRule(Number(selectedSpaceForRules.value), rule.id);
    await loadBusinessRules();
  } catch (e) {
    logger.error('Failed to delete business rule:', e);
    ui.showToast(t('aiVec.k133'), 'error');
  }
}

async function handleAsk() {
  if (!selectedSpaceForAsk.value || !askQuestion.value.trim()) return;
  askLoading.value = true;
  askResult.value = null;
  try {
    const res = await schemaApi.askQuestion(Number(selectedSpaceForAsk.value), askQuestion.value);
    askResult.value = res;
  } catch (e) {
    logger.error('Failed to ask question:', e);
    ui.showToast(t('aiVec.k134'), 'error');
  }
  askLoading.value = false;
}

onMounted(() => {
  loadSpaces();
  loadWorkspaces();
});

// ====== Styles (imported from schema-styles.ts + page-styles.ts) ======
const contentStyle = schemaContentStyle;

const sectionStyle = aiSubSectionStyle;
const sectionTitleStyle = aiSubSectionTitleStyle;
const spaceCardStyle = schemaSpaceCardStyle;
const spaceHeaderStyle = schemaSpaceHeaderStyle;
const spaceInfoStyle = schemaSpaceInfoStyle;
const spaceNameStyle = schemaSpaceNameStyle;
const spaceMetaStyle = schemaSpaceMetaStyle;
const spaceMetaItemStyle = schemaSpaceMetaItemStyle;
const spaceScanInfoStyle = schemaSpaceScanInfoStyle;
const spaceActionsStyle = schemaSpaceActionsStyle;
const dbTypeBadgeStyle = schemaDbTypeBadgeStyle;
const formGridStyle = computed(() => isMobile.value ? { ...aiFormGridStyle, gridTemplateColumns: '1fr' } : aiFormGridStyle);
const filterRowStyle = { ...schemaFilterRowStyle, marginBottom: 0 };
const tableListStyle = schemaTableListStyle;
const tableItemStyle = schemaTableItemStyle;
const tableItemNameStyle = schemaTableItemNameStyle;
const tableItemCommentStyle = schemaTableItemCommentStyle;
const tableDetailStyle = schemaTableDetailStyle;
const detailHeaderStyle = schemaDetailHeaderStyle;
const detailTitleStyle = schemaDetailTitleStyle;
const detailCommentStyle = schemaDetailCommentStyle;
const typeBadgeStyle = schemaTypeBadgeStyle;
const pkBadgeStyle = schemaPkBadgeStyle;
const fkBadgeStyle = schemaFkBadgeStyle;
const idxBadgeStyle = schemaIdxBadgeStyle;
const tableCountStyle = schemaTableCountStyle;
const tableCountTextStyle = schemaTableCountTextStyle;
const tableCountSubStyle = schemaTableCountSubStyle;
const relationsSectionStyle = schemaRelationsSectionStyle;
const relTitleStyle = schemaRelTitleStyle;
const relItemStyle = schemaRelItemStyle;
const relTypeBadgeStyle = schemaRelTypeBadgeStyle;
const ruleCardStyle = schemaRuleCardStyle;
const ruleHeaderStyle = schemaRuleHeaderStyle;
const ruleNameStyle = schemaRuleNameStyle;
const ruleBadgesStyle = schemaRuleBadgesStyle;
const ruleTypeBadgeStyle = schemaRuleTypeBadgeStyle;
const ruleTargetStyle = schemaRuleTargetStyle;
const ruleContentStyle = schemaRuleContentStyle;
const ruleDescStyle = schemaRuleDescStyle;
const askInputAreaStyle = { ...schemaAskInputAreaStyle, marginBottom: 0 };
const askTextareaStyle = schemaAskTextareaStyle;
const askActionsStyle = schemaAskActionsStyle;
const askHintStyle = schemaAskHintStyle;
const askResultStyle = schemaAskResultStyle;
const resultSectionStyle = schemaResultSectionStyle;
const resultLabelStyle = schemaResultLabelStyle;
const sqlBlockStyle = schemaSqlBlockStyle;
const explanationStyle = schemaExplanationStyle;
const confidenceBarStyle = schemaConfidenceBarStyle;
const confLabelStyle = schemaConfLabelStyle;
const confValueStyle = schemaConfValueStyle;
const sourcesStyle = schemaSourcesStyle;
const sourceListStyle = schemaSourceListStyle;
const sourceTagStyle = schemaSourceTagStyle;
const sourceConfStyle = schemaSourceConfStyle;
const warningsStyle = schemaWarningsStyle;
const warningItemStyle = schemaWarningItemStyle;
</script>
