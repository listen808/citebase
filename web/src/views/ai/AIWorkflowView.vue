<template>
  <div :style="aiSubPageStyle">
    <!-- Visual Editor View -->
    <WorkflowEditor
      v-if="editingWorkflow"
      :workflow-name="editingWorkflow.name"
      :initial-nodes="editingWorkflow.nodes"
      :initial-edges="editingWorkflow.edges"
      :saving="saving"
      :executing="!!executeLoading[editingWorkflow.id]"
      @back="editingWorkflow = null"
      @save="handleEditorSave"
      @execute="handleExecute(editingWorkflow.id)"
    />

    <!-- List View -->
    <div v-else :style="{ display: 'flex', flexDirection: 'column', minWidth: 0 }">
    <div :style="crudStatsRowStyle('compact')">
      <ApStatCard
        v-for="stat in stats"
        :key="stat.label"
        size="compact"
        :label="stat.label"
        :value="stat.value"
        :icon="stat.icon"
        :color="stat.color"
      />
    </div>

    <div :style="sectionStyle">
      <div :style="aiSubSectionHeaderStyle">
        <span :style="sectionTitleStyle">{{ $t('vAiUi.k82') }}</span>
        <div :style="actionsRowStyle">
          <ApButton variant="secondary" size="small" :loading="loading" @click="loadWorkflows">
            <ApIcon v-if="!loading" name="RefreshCw" :size="16" /> {{ $t('vAiUi.refresh') }}
          </ApButton>
          <ApButton variant="primary" size="small" @click="showCreateModal = true">
            <ApIcon name="Plus" :size="16" /> {{ $t('vAiUi.k84') }}
          </ApButton>
          <ApButton variant="secondary" size="small" @click="openGenerateModal">
            <ApIcon name="Wand" :size="16" /> AI 生成
          </ApButton>
        </div>
      </div>

      <ApSkeleton v-if="loading" variant="card" :lines="3" />
      <div v-else-if="workflows.length === 0">
        <ApEmpty icon="Workflow" :title="$t('vAiUi.k97')" :description="$t('vAiUi.k99')" />
      </div>
      <div v-else>
        <div v-for="wf in workflows" :key="wf.id" :style="rowStyle">
          <div :style="cardHeaderStyle">
            <div :style="cardInfoStyle">
              <div :style="cardNameStyle">{{ wf.name }}</div>
              <div :style="cardDescStyle">{{ wf.description || $t('fallback.k111') }}</div>
            </div>
            <div :style="badgeRowStyle">
              <span :style="typeBadgeStyle(wf.type)">{{ typeLabel(wf.type) }}</span>
              <span :style="nodeBadgeStyle">{{ (wf.nodes?.length || 0) }} {{ $t('vAiUi.k102') }}</span>
              <span :style="statusBadgeStyle(wf.active)">{{ wf.active !== false ? '运行中' : $t('vAiUi.k100') }}</span>
            </div>
          </div>
          <div :style="metaStyle">
            <span :style="metaItemStyle">
              <ApIcon name="Clock" :size="12" :color="cv('text-tertiary')" />
              {{ wf.created_at ? formatTime(wf.created_at) : $t('vAiUi.k101') }}
            </span>
            <span v-if="wf.cron_expr" :style="metaItemStyle">
              <ApIcon name="Clock" :size="12" :color="cv('text-tertiary')" />
              {{ wf.cron_expr }}
            </span>
          </div>
          <div :style="actionsStyle">
            <ApButton variant="primary" size="small" @click="openEditor(wf)">
              <ApIcon name="Pencil" :size="16" /> {{ $t('vAiUi.edit') }}
            </ApButton>
            <ApButton variant="secondary" size="small" :loading="executeLoading[wf.id]" :disabled="toggleLoading[wf.id]" @click="handleExecute(wf.id)">
              <ApIcon v-if="!executeLoading[wf.id]" name="Play" :size="16" /> {{ $t('vAiUi.k86') }}
            </ApButton>
            <ApButton variant="secondary" size="small" :loading="toggleLoading[wf.id]" :disabled="executeLoading[wf.id]" @click="handleToggle(wf.id, wf.active === false)">
              <ApIcon v-if="!toggleLoading[wf.id]" :name="wf.active !== false ? 'Pause' : 'Play'" :size="16" />
              {{ wf.active !== false ? '暂停' : $t('vAiUi.enable') }}
            </ApButton>
            <!-- 列表行删除走 ghost+红图标（与权限树同档）：实心 danger 成排出现是噪声（审查 §四.6），确认弹窗里再上实心红 -->
            <ApButton variant="ghost" size="small" @click="confirmDelete(wf)">
              <ApIcon name="Trash2" :size="16" :color="cv('red')" /> {{ $t('vAiUi.delete') }}
            </ApButton>
          </div>
        </div>
      </div>
    </div>

    <ApModal v-model="showCreateModal" :title="$t('vAiUi.k84')" size="medium">
      <div :style="formGroupStyle">
        <ApInput
          v-model="form.name"
          :label="$t('vAiUi.name')"
          :placeholder="$t('vAiUi.k94')"
        />
      </div>
      <div :style="formGroupStyle">
        <ApInput
          v-model="form.description"
          :label="$t('vAiUi.desc')"
          :placeholder="$t('vAiUi.k95')"
        />
      </div>
      <div :style="formGroupStyle">
        <ApSelect
          v-model="form.type"
          :label="$t('vAiUi.type')"
          :options="workflowTypeOptions"
        />
      </div>
      <div :style="formGroupStyle">
        <ApInput
          v-model="form.cron_expr"
          :label="$t('vAiUi.k93')"
          :placeholder="$t('vAiUi.k96')"
        />
      </div>
      <template #footer>
        <ApButton variant="secondary" size="small" @click="showCreateModal = false">{{ $t('vAiUi.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="creating" @click="handleCreate">{{ $t('vAiUi.k89') }}</ApButton>
      </template>
    </ApModal>

    <ApModal v-model="showDeleteModal" :title="$t('vAiUi.k98')" size="small">
      <div :style="deleteMsgStyle">{{ $t('vAiUi.k236', { p1: deletingWF?.name }) }}</div>
      <template #footer>
        <ApButton variant="secondary" size="small" @click="showDeleteModal = false">{{ $t('vAiUi.cancel') }}</ApButton>
        <ApButton variant="danger" :loading="deleting" size="small" @click="handleDelete">
          <ApIcon name="Trash2" :size="16" /> {{ $t('vAiUi.delete') }}
        </ApButton>
      </template>
    </ApModal>

    <!-- W6：AI 生成工作流（草稿 → 预览 → 启用/删除） -->
    <ApModal v-model="showGenerateModal" :title="'AI 生成工作流'" size="medium">
      <!-- 生成前：输入描述 -->
      <div v-if="!generatedWF">
        <div :style="formGroupStyle">
          <ApTextarea
            v-model="genDescription"
            :rows="3"
            label="描述工作流"
            placeholder="例如：先检索知识库中的产品文档，再让 LLM 总结要点并翻译成英文，最后输出。"
          />
        </div>
        <div :style="{ ...metaStyle, marginBottom: 0 }">
          <ApIcon name="Info" :size="14" :color="cv('text-tertiary')" />
          <span :style="{ fontSize: AppleType.legalMeta.size, color: cv('text-tertiary') }">
            生成结果为草稿（未启用），请在下方预览确认无误后手动启用。
          </span>
        </div>
      </div>
      <!-- 生成后：预览草稿节点 -->
      <div v-else>
        <div :style="formGroupStyle">
          <ApInput :model-value="generatedWF.name" :label="$t('vAiUi.name')" :read-only="true" />
        </div>
        <div v-if="generatedWF.description" :style="{ ...formGroupStyle, marginBottom: 0 }">
          <div :style="{ fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), marginBottom: AppleSpace.xxs }">{{ $t('vAiUi.desc') }}</div>
          <div :style="{ fontSize: AppleType.caption.size, color: cv('text-primary'), lineHeight: 1.5 }">{{ generatedWF.description }}</div>
        </div>
        <div :style="formGroupStyle">
          <div :style="{ fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), marginBottom: AppleSpace.xs }">
            节点（{{ (generatedWF.nodes || []).length }}）
          </div>
          <div v-for="(node, idx) in (generatedWF.nodes || [])" :key="node.id" :style="genNodeRowStyle">
            <span :style="genNodeIndexStyle">{{ idx + 1 }}</span>
            <span :style="genNodeTypeStyle(node.type)">{{ genNodeTypeLabel(node.type) }}</span>
            <span :style="genNodeNameStyle">{{ node.name || node.id }}</span>
            <span v-if="nodeConfigSummary(node)" :style="genNodeConfigStyle">{{ nodeConfigSummary(node) }}</span>
          </div>
          <div v-if="(generatedWF.nodes || []).length === 0" :style="{ fontSize: AppleType.legalMeta.size, color: cv('text-tertiary') }">无节点</div>
        </div>
        <div :style="{ ...metaStyle, marginBottom: 0 }">
          <ApIcon name="Info" :size="14" :color="cv('orange')" />
          <span :style="{ fontSize: AppleType.legalMeta.size, color: cv('orange') }">
            该工作流当前为草稿（未启用）。启用后将按上方节点顺序执行。
          </span>
        </div>
      </div>
      <template #footer>
        <template v-if="!generatedWF">
          <ApButton variant="secondary" size="small" @click="showGenerateModal = false">{{ $t('vAiUi.cancel') }}</ApButton>
          <ApButton variant="primary" size="small" :loading="generating" :disabled="!genDescription.trim()" @click="handleGenerate">
            <ApIcon v-if="!generating" name="Wand" :size="16" /> 生成
          </ApButton>
        </template>
        <template v-else>
          <ApButton variant="ghost" size="small" @click="showGenerateModal = false">关闭</ApButton>
          <ApButton variant="secondary" size="small" :loading="genDeleteLoading" @click="handleGenerateDelete">
            <ApIcon name="Trash2" :size="16" :color="cv('red')" /> 删除
          </ApButton>
          <ApButton variant="primary" size="small" :loading="genEnableLoading" @click="handleGenerateEnable">
            <ApIcon v-if="!genEnableLoading" name="Play" :size="16" /> 启用
          </ApButton>
        </template>
      </template>
    </ApModal>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted } from 'vue';
// Workflow 与二级导航「工作流管理」同符号：总数卡与空态由 GitBranch 统一到 Workflow。
// 模板内图标走 <ApIcon name="X">，统计卡的 icon 传注册表字符串名（ApStatCard 接受 IconName）。
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import { cv, AppleRadius, AppleSpace, AppleType, AppleFontWeight } from '@/utils/core/apple-tokens';
import { useBreakpoint } from '@/utils/responsive';
import { formatTime } from '@/utils/format/format-time';
import { aiSubSectionStyle as sectionStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle as sectionTitleStyle, crudStatsRowStyle, formGroupStyle, aiSubPageStyle } from '@/styles/ai/ai-page-styles';
import { useUIStore } from '@/stores/app/ui';
import { listWorkflows, createWorkflow, updateWorkflow, executeWorkflow, toggleWorkflow, deleteWorkflow, generateWorkflow, type WorkflowDefinition, type CreateWorkflowRequest } from '@/api/modules/ai/ai-data';
import WorkflowEditor from '@/components/ai/workflow/WorkflowEditor.vue';
import type { WorkflowNode, WorkflowEdge } from '@/types/ai';

const uiStore = useUIStore();
const { isMobile } = useBreakpoint();
const toast = uiStore.showToast;

const loading = ref(true);
const creating = ref(false);
const deleting = ref(false);
const saving = ref(false);
const executeLoading = ref<Record<string, boolean>>({});
const toggleLoading = ref<Record<string, boolean>>({});

const workflows = ref<WorkflowDefinition[]>([]);
const showCreateModal = ref(false);
const showDeleteModal = ref(false);
const deletingWF = ref<WorkflowDefinition | null>(null);
const editingWorkflow = ref<WorkflowDefinition | null>(null);

// W6：AI 生成工作流
const showGenerateModal = ref(false);
const genDescription = ref('');
const generatedWF = ref<WorkflowDefinition | null>(null);
const generating = ref(false);
const genEnableLoading = ref(false);
const genDeleteLoading = ref(false);

const form = ref({
  name: '',
  description: '',
  type: 'quality',
  cron_expr: '',
});

const { t } = useI18n();
const workflowTypeOptions = [
  { label: t('vAiUi.k103'), value: 'quality' },
  { label: t('vAiUi.k104'), value: 'analysis' },
  { label: t('vAiUi.k105'), value: 'summary' },
];

const activeCount = computed(() => workflows.value.filter(w => w.active !== false).length);
const pausedCount = computed(() => workflows.value.filter(w => w.active === false).length);

const stats = computed(() => [
  { label: t('vAiUi.k106'), value: String(workflows.value.length), icon: 'Workflow' as const, color: cv('blue') },
  { label: t('vAiUi.k49'), value: String(activeCount.value), icon: 'CheckCircle' as const, color: cv('green') },
  { label: t('vAiUi.k100'), value: String(pausedCount.value), icon: 'Pause' as const, color: cv('orange') },
  { label: t('vAiUi.k107'), value: String(workflows.value.reduce((sum, w) => sum + (w.nodes?.length || 0), 0)), icon: 'AlertCircle' as const, color: cv('purple') },
]);

function typeLabel(type?: string) {
  const map: Record<string, string> = { quality: '质量巡检', analysis: '知识分析', summary: '摘要生成' };
  return map[type || ''] || type || t('fallback.k97');
}

async function loadWorkflows() {
  loading.value = true;
  try {
    const res = await listWorkflows();
    workflows.value = (res.workflows || []).map((w) => ({
      ...w,
      id: String(w.id || ''),
      active: w.active !== undefined ? Boolean(w.active) : true,
    }));
  } catch (e: unknown) {
    toast((e as { message?: string }).message || t('fallback.k31'), 'error');
  }
  loading.value = false;
}

function openEditor(wf: WorkflowDefinition) {
  editingWorkflow.value = wf;
}

async function handleEditorSave(nodes: WorkflowNode[], edges: WorkflowEdge[]) {
  if (!editingWorkflow.value) return;
  saving.value = true;
  try {
    const res = await updateWorkflow(editingWorkflow.value.id, {
      name: editingWorkflow.value.name,
      description: editingWorkflow.value.description,
      nodes,
      edges,
    });
    editingWorkflow.value.nodes = nodes;
    editingWorkflow.value.edges = edges;
    const updated = res?.workflow;
    if (updated) {
      const target = workflows.value.find((w) => w.id === editingWorkflow.value!.id);
      if (target) {
        target.nodes = updated.nodes;
        target.edges = updated.edges;
      }
    }
    toast(res?.message || t('fallback.k52'), 'success');
  } catch (e: unknown) {
    toast((e as { message?: string }).message || t('fallback.k8'), 'error');
  }
  saving.value = false;
}

async function handleCreate() {
  if (!form.value.name.trim()) {
    toast('请输入工作流名称', 'warning');
    return;
  }
  creating.value = true;
  try {
    const payload: CreateWorkflowRequest = {
      name: form.value.name,
      description: form.value.description || undefined,
      type: form.value.type || undefined,
      cron_expr: form.value.cron_expr || undefined,
    };
    await createWorkflow(payload);
    showCreateModal.value = false;
    form.value = { name: '', description: '', type: 'quality', cron_expr: '' };
    await loadWorkflows();
    toast('工作流创建成功', 'success');
  } catch (e: unknown) {
    toast((e as { message?: string }).message || t('fallback.k15'), 'error');
  }
  creating.value = false;
}

async function handleExecute(id: string) {
  executeLoading.value = { ...executeLoading.value, [id]: true };
  try {
    const res = await executeWorkflow(id);
    toast(res?.message || t('fallback.k53'), 'success');
  } catch (e: unknown) {
    toast((e as { message?: string }).message || t('fallback.k60'), 'error');
  }
  executeLoading.value = { ...executeLoading.value, [id]: false };
}

async function handleToggle(id: string, active: boolean) {
  toggleLoading.value = { ...toggleLoading.value, [id]: true };
  try {
    await toggleWorkflow(id, active);
    const wf = workflows.value.find(w => w.id === id);
    if (wf) wf.active = active;
    toast(active ? '工作流已启用' : '工作流已暂停', 'success');
  } catch (e: unknown) {
    toast((e as { message?: string }).message || t('fallback.k64'), 'error');
  }
  toggleLoading.value = { ...toggleLoading.value, [id]: false };
}

function confirmDelete(wf: WorkflowDefinition) {
  deletingWF.value = wf;
  showDeleteModal.value = true;
}

async function handleDelete() {
  if (!deletingWF.value) return;
  deleting.value = true;
  try {
    await deleteWorkflow(deletingWF.value.id);
    showDeleteModal.value = false;
    deletingWF.value = null;
    await loadWorkflows();
    toast('工作流已删除', 'success');
  } catch (e: unknown) {
    toast((e as { message?: string }).message || t('fallback.k20'), 'error');
  }
  deleting.value = false;
}

// ====== W6：AI 生成工作流 ======

function openGenerateModal() {
  genDescription.value = '';
  generatedWF.value = null;
  showGenerateModal.value = true;
}

async function handleGenerate() {
  if (!genDescription.value.trim()) {
    toast('请输入工作流描述', 'warning');
    return;
  }
  generating.value = true;
  try {
    const res = await generateWorkflow(genDescription.value.trim());
    const wf = res?.workflow;
    if (wf) {
      generatedWF.value = {
        ...wf,
        id: String(wf.id || ''),
        active: wf.active !== undefined ? Boolean(wf.active) : false,
      };
      await loadWorkflows();
      toast(res?.message || t('fallback.k118'), 'success');
    }
  } catch (e: unknown) {
    toast((e as { message?: string }).message || t('fallback.k119'), 'error');
  }
  generating.value = false;
}

async function handleGenerateEnable() {
  if (!generatedWF.value) return;
  genEnableLoading.value = true;
  try {
    await toggleWorkflow(generatedWF.value.id, true);
    generatedWF.value.active = true;
    await loadWorkflows();
    showGenerateModal.value = false;
    toast('工作流已启用', 'success');
  } catch (e: unknown) {
    toast((e as { message?: string }).message || t('fallback.k64'), 'error');
  }
  genEnableLoading.value = false;
}

async function handleGenerateDelete() {
  if (!generatedWF.value) return;
  genDeleteLoading.value = true;
  try {
    await deleteWorkflow(generatedWF.value.id);
    showGenerateModal.value = false;
    generatedWF.value = null;
    await loadWorkflows();
    toast('草稿已删除', 'success');
  } catch (e: unknown) {
    toast((e as { message?: string }).message || t('fallback.k20'), 'error');
  }
  genDeleteLoading.value = false;
}

function genNodeTypeLabel(type: string): string {
  const map: Record<string, string> = {
    start: '开始',
    end: '结束',
    llm: 'LLM',
    retrieval: '检索',
    tool: '工具',
    condition: '条件',
    parallel: '并行',
  };
  return map[type] || type;
}

function nodeConfigSummary(node: WorkflowNode): string {
  const cfg = node.config;
  if (!cfg) return '';
  if (node.type === 'llm' && typeof cfg.prompt === 'string') {
    const p = (cfg.prompt as string).trim();
    return p.length > 40 ? p.slice(0, 40) + '…' : p;
  }
  if (node.type === 'retrieval' && typeof cfg.query === 'string') {
    const q = (cfg.query as string).trim();
    return q.length > 40 ? q.slice(0, 40) + '…' : q;
  }
  return '';
}

onMounted(() => { loadWorkflows(); });

// ====== Styles ======

// 列表行分隔线，不是卡片：与 page-styles 的 cardStyle 同名会被误当成卡片样式
const rowStyle = { padding: `${AppleSpace.sm} ${AppleSpace.md}`, borderBottom: `1px solid ${cv('border-light')}` };
const cardHeaderStyle = { display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: AppleSpace.xs };
const cardInfoStyle = { flex: 1 };
const cardNameStyle = { fontSize: AppleType.bodyPrimary.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary'), marginBottom: AppleSpace.xxs };
const cardDescStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-secondary') };

const badgeRowStyle = { display: 'flex', gap: AppleSpace.xs, flexShrink: 0 };
const typeBadgeStyle = (_type?: string) => ({
  padding: `2px ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, fontSize: AppleType.legalMeta.size,
  background: cv('blue-bg-subtle'), color: cv('blue'), fontWeight: String(AppleFontWeight.medium),
});
const nodeBadgeStyle = {
  padding: `2px ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, fontSize: AppleType.legalMeta.size,
  background: cv('gray-bg-subtle'), color: cv('text-secondary'), fontWeight: String(AppleFontWeight.medium),
};
const statusBadgeStyle = (active?: boolean) => ({
  padding: `2px ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, fontSize: AppleType.legalMeta.size,
  background: (active !== false) ? cv('green-bg-subtle') : cv('orange-bg-subtle'),
  color: (active !== false) ? cv('green') : cv('orange'),
  fontWeight: String(AppleFontWeight.medium),
});

const metaStyle = { display: 'flex', gap: AppleSpace.sm, marginBottom: AppleSpace.xs };
const metaItemStyle = { display: 'inline-flex', alignItems: 'center', gap: AppleSpace.xxs, fontSize: AppleType.legalMeta.size, color: cv('text-tertiary') };

const actionsStyle = computed(() => ({ display: 'flex', gap: AppleSpace.xs, flexWrap: isMobile.value ? 'wrap' as const : 'nowrap' as const }));
const actionsRowStyle = { display: 'flex', gap: AppleSpace.xs };

const deleteMsgStyle = { fontSize: AppleType.bodyPrimary.size, color: cv('text-secondary'), lineHeight: 1.5 };

// W6：AI 生成预览
const genNodeRowStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  padding: `${AppleSpace.xs} 0`,
  borderBottom: `${AppleSpace.xxs} solid ${cv('border-light')}`,
};
const genNodeIndexStyle = {
  width: 20,
  height: 20,
  borderRadius: AppleRadius.pill,
  background: cv('gray-bg-subtle'),
  color: cv('text-secondary'),
  fontSize: AppleType.legalMeta.size,
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  flexShrink: 0,
};
const genNodeTypeStyle = (type: string) => {
  const colors: Record<string, { bg: string; fg: string }> = {
    start: { bg: cv('green-bg-subtle'), fg: cv('green') },
    end: { bg: cv('red-bg-subtle'), fg: cv('red') },
    llm: { bg: cv('purple-bg-subtle'), fg: cv('purple') },
    retrieval: { bg: cv('blue-bg-subtle'), fg: cv('blue') },
  };
  const c = colors[type] || { bg: cv('gray-bg-subtle'), fg: cv('text-secondary') };
  return {
    padding: `${AppleSpace.xs} ${AppleSpace.xs}`,
    borderRadius: AppleRadius.pill,
    fontSize: AppleType.legalMeta.size,
    background: c.bg,
    color: c.fg,
    fontWeight: String(AppleFontWeight.medium),
    flexShrink: 0,
  };
};
const genNodeNameStyle = {
  fontSize: AppleType.caption.size,
  color: cv('text-primary'),
  fontWeight: String(AppleFontWeight.medium),
  overflow: 'hidden' as const,
  textOverflow: 'ellipsis' as const,
  whiteSpace: 'nowrap' as const,
  flexShrink: 1,
};
const genNodeConfigStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
  overflow: 'hidden' as const,
  textOverflow: 'ellipsis' as const,
  whiteSpace: 'nowrap' as const,
  flexShrink: 0,
  maxWidth: '40%',
};
</script>
