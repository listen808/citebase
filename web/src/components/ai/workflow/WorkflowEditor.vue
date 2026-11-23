<template>
  <div :style="editorContainerStyle">
    <!-- Top toolbar -->
    <div :style="toolbarStyle">
      <div :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.sm }">
        <ApButton variant="ghost" size="small" @click="$emit('back')">
          <ApIcon name="ArrowLeft" :size="AppleIconSize.control" /> {{ $t('wfUi.k1') }}
        </ApButton>
        <span :style="dividerStyle" />
        <span :style="editorTitleStyle">{{ workflowName || $t('fallback.k67') }}</span>
      </div>
      <div :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.sm }">
        <ApButton variant="secondary" size="small" :loading="saving" @click="handleSave">
          <ApIcon name="Save" :size="AppleIconSize.control" /> {{ $t('wfUi.save') }}
        </ApButton>
        <ApButton variant="primary" size="small" :loading="executing" @click="$emit('execute')">
          <ApIcon name="Play" :size="AppleIconSize.control" /> {{ $t('wfUi.k3') }}
        </ApButton>
      </div>
    </div>

    <div :style="mainAreaStyle">
      <!-- Left: Node palette -->
      <div :style="paletteStyle">
        <div :style="paletteTitleStyle">{{ $t('wfUi.k4') }}</div>
        <div
          v-for="nt in nodeTypes"
          :key="nt.type"
          :style="paletteItemStyle"
          draggable="true"
          @dragstart="onDragStart($event, nt.type)"
        >
          <div :style="paletteIconStyle(nt.color)">
            <ApIcon :name="nt.icon" :size="14" />
          </div>
          <div>
            <div :style="paletteNameStyle">{{ nt.label }}</div>
            <div :style="paletteDescStyle">{{ nt.desc }}</div>
          </div>
        </div>
      </div>

      <!-- Center: Vue Flow canvas -->
      <div ref="canvasWrapperRef" :style="canvasWrapperStyle">
        <VueFlow
          v-model:nodes="nodes"
          v-model:edges="edges"
          :default-viewport="{ zoom: 1, x: 0, y: 0 }"
          :min-zoom="0.3"
          :max-zoom="2"
          :snap-to-grid="true"
          :snap-grid="[16, 16]"
          fit-view-on-init
          @node-click="onNodeClick"
          @connect="onConnect"
          @drop="onDrop"
          @dragover="onDragover"
        >
          <Background />
          <Controls />
          <MiniMap />

          <!-- Custom node templates -->
          <template #node-start>
            <div :style="customNodeStyle(cv('green'))">
              <div :style="nodeHeaderStyle(cv('green'))">
                <ApIcon name="Play" :size="12" /> {{ $t('wfUi.k5') }}
              </div>
            </div>
          </template>

          <template #node-end>
            <div :style="customNodeStyle(cv('text-secondary'))">
              <div :style="nodeHeaderStyle(cv('text-secondary'))">
                <ApIcon name="Square" :size="12" /> {{ $t('wfUi.k6') }}
              </div>
            </div>
          </template>

          <template #node-llm="nodeProps">
            <div :style="customNodeStyle(cv('blue'))">
              <div :style="nodeHeaderStyle(cv('blue'))">
                <ApIcon name="Brain" :size="12" /> {{ nodeProps.data?.label || $t('fallback.k3') }}
              </div>
              <div :style="nodeBodyStyle">
                {{ nodeProps.data?.config?.prompt ? truncate(nodeProps.data.config.prompt as string, 60) : $t('wfUi.k18') }}
              </div>
            </div>
          </template>

          <template #node-tool="nodeProps">
            <div :style="customNodeStyle(cv('purple'))">
              <div :style="nodeHeaderStyle(cv('purple'))">
                <ApIcon name="Wrench" :size="12" /> {{ nodeProps.data?.label || $t('fallback.k54') }}
              </div>
              <div :style="nodeBodyStyle">
                工具: {{ nodeProps.data?.config?.tool_name || $t('fallback.k83') }}
              </div>
            </div>
          </template>

          <template #node-condition="nodeProps">
            <div :style="customNodeStyle(cv('orange'))">
              <div :style="nodeHeaderStyle(cv('orange'))">
                <ApIcon name="GitBranch" :size="12" /> {{ nodeProps.data?.label || $t('fallback.k84') }}
              </div>
              <div :style="nodeBodyStyle">
                {{ nodeProps.data?.config?.expression || $t('fallback.k89') }}
              </div>
            </div>
          </template>

          <template #node-parallel="nodeProps">
            <div :style="customNodeStyle(cv('teal'))">
              <div :style="nodeHeaderStyle(cv('teal'))">
                <ApIcon name="Layers" :size="12" /> {{ nodeProps.data?.label || $t('fallback.k56') }}
              </div>
              <div :style="nodeBodyStyle">
                {{ $t('wfUi.k7') }}
              </div>
            </div>
          </template>
        </VueFlow>
      </div>

      <!-- Right: Config panel -->
      <div :style="configPanelStyle">
        <template v-if="selectedNode">
          <div :style="configTitleStyle">{{ $t('wfUi.k8') }}</div>
          <div :style="configFieldStyle">
            <label :style="labelStyle">{{ $t('wfUi.k9') }}</label>
            <ApInput v-model="selectedNode.data.label" :placeholder="$t('wfUi.k9')" />
          </div>

          <!-- LLM node config -->
          <template v-if="selectedNode.type === 'llm'">
            <div :style="configFieldStyle">
              <label :style="labelStyle">{{ $t('wfUi.k10') }}</label>
              <ApTextarea
                :model-value="String((selectedNode.data.config as Record<string, unknown>).prompt || '')"
                :style="textareaStyle"
                :placeholder="$t('wfHint.nodePrompt', { ref: '{{nodeID}}' })"
                @update:model-value="(v: string) => ((selectedNode!.data.config as Record<string, unknown>).prompt = v)"
              />
            </div>
            <div :style="configFieldStyle">
              <label :style="labelStyle">{{ $t('wfUi.k11') }}</label>
              <ApSelect
                :model-value="getTierValue(selectedNode)"
                :options="[
                  { label: $t('cExtraUi.k3'), value: 'primary' },
                  { label: $t('cExtraUi.k4'), value: 'fast' },
                  { label: $t('cExtraUi.k5'), value: 'reasoning' },
                ]"
                @update:model-value="(v: string | number | null) => (selectedNode!.data.config.tier = v)"
              />
            </div>
          </template>

          <!-- Tool node config -->
          <template v-if="selectedNode.type === 'tool'">
            <div :style="configFieldStyle">
              <label :style="labelStyle">{{ $t('wfUi.k12') }}</label>
              <ApSelect
                :model-value="getToolNameValue(selectedNode)"
                :options="toolOptions"
                @update:model-value="(v: string | number | null) => (selectedNode!.data.config.tool_name = v)"
              />
            </div>
            <div :style="configFieldStyle">
              <label :style="labelStyle">{{ $t('wfUi.k13') }}</label>
              <ApTextarea
                :model-value="String((selectedNode.data.config as Record<string, unknown>).params || '')"
                :style="textareaStyle"
                placeholder='{"query": "{{search_query}}"}'
                @update:model-value="(v: string) => ((selectedNode!.data.config as Record<string, unknown>).params = v)"
              />
            </div>
          </template>

          <!-- Condition node config -->
          <template v-if="selectedNode.type === 'condition'">
            <div :style="configFieldStyle">
              <label :style="labelStyle">{{ $t('wfUi.k14') }}</label>
              <ApTextarea
                :model-value="String((selectedNode.data.config as Record<string, unknown>).expression || '')"
                :style="textareaStyle"
                placeholder='如: {{score}} > 0.7'
                @update:model-value="(v: string) => ((selectedNode!.data.config as Record<string, unknown>).expression = v)"
              />
            </div>
          </template>

          <div :style="{ marginTop: AppleSpace.md }">
            <ApButton variant="danger" size="small" @click="deleteNode(selectedNode.id)">
              <ApIcon name="Trash2" :size="AppleIconSize.control" /> {{ $t('wfUi.k15') }}
            </ApButton>
          </div>
        </template>
        <template v-else>
          <div :style="emptyConfigStyle">
            <ApIcon name="MousePointer" :size="AppleIconSize.section" :style="{ color: cv('text-quaternary'), marginBottom: AppleSpace.sm }" />
            <div :style="{ fontSize: AppleType.microUIBold.size, color: cv('text-tertiary') }">{{ $t('wfUi.k16') }}</div>
            <div :style="{ fontSize: AppleType.microUI.size, color: cv('text-quaternary'), marginTop: AppleSpace.xs }">{{ $t('wfUi.k17') }}</div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref, computed, watch } from 'vue';
import { VueFlow, useVueFlow, type Node, type Edge } from '@vue-flow/core';
import { Background } from '@vue-flow/background';
import { Controls } from '@vue-flow/controls';
import { MiniMap } from '@vue-flow/minimap';
import '@vue-flow/core/dist/style.css';
import '@vue-flow/core/dist/theme-default.css';
import '@vue-flow/controls/dist/style.css';
import '@vue-flow/minimap/dist/style.css';
import ApButton from '@/components/apple/ApButton.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import { AppleIconSize, cv, AppleSpace, AppleRadius, AppleType, AppleFontWeight, AppleLineHeight } from '@/utils/core/apple-tokens';
import { scrollSingleAxis } from '@/styles/layout/page-styles';
import { useUIStore } from '@/stores/app/ui';
import type { IconName } from '@/utils/icons';
import type { WorkflowNode, WorkflowEdge } from '@/types/ai';

interface Props {
  workflowName?: string;
  initialNodes?: WorkflowNode[];
  initialEdges?: WorkflowEdge[];
  saving?: boolean;
  executing?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  workflowName: '',
  initialNodes: () => [],
  initialEdges: () => [],
  saving: false,
  executing: false,
});

const emit = defineEmits<{
  'back': [];
  'save': [nodes: WorkflowNode[], edges: WorkflowEdge[]];
  'execute': [];
}>();
const { t } = useI18n();

const { addEdges, removeNodes, project } = useVueFlow();
const ui = useUIStore();

// Convert backend WorkflowNode[] to vue-flow Node[]
function toVfNodes(wfNodes: WorkflowNode[]): Node[] {
  if (!wfNodes?.length) {
    // Default: start -> end
    return [
      { id: 'start', type: 'start', position: { x: 250, y: 50 }, data: { label: t('wfUi.k5') } },
      { id: 'end', type: 'end', position: { x: 250, y: 350 }, data: { label: t('wfUi.k6') } },
    ];
  }
  return wfNodes.map((n, i) => ({
    id: n.id,
    type: n.type,
    position: { x: 250 + (i % 3) * 200, y: 50 + Math.floor(i / 3) * 150 },
    data: { label: n.name, config: n.config || {} },
  }));
}

function toVfEdges(wfEdges: WorkflowEdge[]): Edge[] {
  return (wfEdges || []).map((e, i) => ({
    id: `e-${e.source}-${e.target}-${i}`,
    source: e.source,
    target: e.target,
    label: e.condition || undefined,
    animated: false,
    type: 'smoothstep',
  }));
}

const nodes = ref<Node[]>(toVfNodes(props.initialNodes));
const edges = ref<Edge[]>(toVfEdges(props.initialEdges));

// Watch for prop changes
watch(() => [props.initialNodes, props.initialEdges], ([n, e]) => {
  nodes.value = toVfNodes(n as WorkflowNode[]);
  edges.value = toVfEdges(e as WorkflowEdge[]);
});

const selectedNode = ref<Node | null>(null);

function onNodeClick({ node }: { node: Node }) {
  selectedNode.value = node;
}

function onConnect(params: { source: string; target: string }) {
  if (params.source === params.target) return;
  const visited = new Set<string>();
  function hasPath(from: string, to: string): boolean {
    if (from === to) return true;
    if (visited.has(from)) return false;
    visited.add(from);
    return edges.value.some(e => e.source === from && hasPath(e.target, to));
  }
  if (hasPath(params.target, params.source)) {
    ui.showToast(t('wfUi.k35'), 'warning');
    return;
  }
  addEdges([{ id: `e-${params.source}-${params.target}`, source: params.source, target: params.target, type: 'smoothstep' }]);
}

// Drag and drop
const draggedType = ref<string | null>(null);

function onDragStart(event: DragEvent, type: string) {
  draggedType.value = type;
  if (event.dataTransfer) {
    event.dataTransfer.setData('application/vueflow', type);
    event.dataTransfer.effectAllowed = 'move';
  }
}

function onDragover(event: DragEvent) {
  event.preventDefault();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'move';
  }
}

function onDrop(event: DragEvent) {
  const type = event.dataTransfer?.getData('application/vueflow') || draggedType.value;
  if (!type) return;

  const { left, top } = (event.currentTarget as HTMLElement).getBoundingClientRect();
  const position = project({
    x: event.clientX - left,
    y: event.clientY - top,
  });

  const id = `${type}-${Date.now()}`;
  const newNode: Node = {
    id,
    type,
    position,
    data: { label: nodeTypeMap[type]?.label || type, config: {} },
  };

  // vue-flow 的 Node 是递归泛型，vue-tsc 在 Ref<Node[]>.push / 展开上会触发 TS2589（编译器限制，非逻辑错误）。
  // newNode 仍以 Node 声明，构造期形状受检；此处仅把数组接收方整体越过限制，不再像旧代码那样撒遍 as any。
  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- 同上：这是 TS2589 的唯一落点，换成窄类型会重新实例化递归泛型
  (nodes.value as Array<any>).push(newNode);
  draggedType.value = null;
}

/* eslint-disable @typescript-eslint/no-explicit-any -- vue-flow 的 Node/Edge 都是递归泛型，方法调用与赋值 RHS 必须落在 any 上，否则 vue-tsc 实例化泛型即 TS2589（见 addNode 上方注释） */
function deleteNode(nodeId: string) {
  removeNodes([nodeId]);
  // 同 push 的 TS2589：vue-flow 的 Node/Edge 都是递归泛型，方法调用要落在 Array<any> 上、
  // 赋值 RHS 也要是 any，否则 vue-tsc 解析目标泛型时会实例化到爆。
  nodes.value = (nodes.value as Array<any>).filter((n: any) => n.id !== nodeId) as any;
  edges.value = (edges.value as Array<any>).filter((e: any) => e.source !== nodeId && e.target !== nodeId) as any;
  selectedNode.value = null;
}

function handleSave() {
  const wfNodes: WorkflowNode[] = (nodes.value as Array<any>).map((n: any) => ({
    id: n.id,
    type: n.type || 'llm',
    name: n.data?.label || n.id,
    config: n.data?.config || {},
  }));
  const wfEdges: WorkflowEdge[] = (edges.value as Array<any>).map((e: any) => ({
    source: e.source,
    target: e.target,
    condition: e.label as string | undefined,
  }));
  emit('save', wfNodes, wfEdges);
}
/* eslint-enable @typescript-eslint/no-explicit-any */

function truncate(s: string, len: number) {
  return s.length > len ? s.slice(0, len) + '...' : s;
}

function getTierValue(node: Node): string | number | null {
  return node.data?.config?.tier ?? null;
}

function getToolNameValue(node: Node): string | number | null {
  return node.data?.config?.tool_name ?? null;
}

// Node type definitions（图标存注册表的名字而不是组件：写错名字编译期就红，
// 而且渲染统一走 ApIcon，描边由尺寸反解）
const nodeTypeMap: Record<string, { label: string; icon: IconName; color: string; desc: string }> = {
  start: { label: t('wfUi.k5'), icon: 'Square', color: cv('green'), desc: t('wfUi.k19') },
  end: { label: t('wfUi.k6'), icon: 'Square', color: cv('text-secondary'), desc: t('wfUi.k20') },
  llm: { label: t('wfUi.k21'), icon: 'Brain', color: cv('blue'), desc: t('wfUi.k22') },
  tool: { label: t('wfUi.k23'), icon: 'Wrench', color: cv('purple'), desc: t('wfUi.k24') },
  condition: { label: t('wfUi.k25'), icon: 'GitBranch', color: cv('orange'), desc: t('wfUi.k26') },
  parallel: { label: t('wfUi.k27'), icon: 'Layers', color: cv('teal'), desc: t('wfUi.k28') },
};

const nodeTypes = computed(() =>
  Object.entries(nodeTypeMap).map(([type, def]) => ({ type, ...def }))
);

const toolOptions = [
  { label: t('wfUi.k29'), value: 'search_documents' },
  { label: t('wfUi.k30'), value: 'knowledge_graph_query' },
  { label: t('wfUi.k31'), value: 'quality_check' },
  { label: t('wfUi.k32'), value: 'entity_extract' },
  { label: t('wfUi.k33'), value: 'generate_summary' },
  { label: t('wfUi.k34'), value: 'generate_tags' },
];

// ===== Styles =====
const editorContainerStyle = {
  display: 'flex', flexDirection: 'column' as const,
  flex: 1, minHeight: 0, background: cv('surface-panel'),
};

const toolbarStyle = {
  display: 'flex', justifyContent: 'space-between', alignItems: 'center',
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  borderBottom: `1px solid ${cv('border-light')}`,
  background: cv('surface-panel'),
};

const dividerStyle = {
  width: '1px', height: '16px', background: cv('border-light'),
};

const editorTitleStyle = {
  fontSize: AppleType.bodyPrimary.size, fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
};

const mainAreaStyle = {
  display: 'flex', flex: 1, overflow: 'hidden' as const,
};

const paletteStyle = {
  width: '200px', borderRight: `1px solid ${cv('border-light')}`,
  padding: AppleSpace.md,
  background: cv('bg-secondary'), flexShrink: '0',
  ...scrollSingleAxis,
};

const paletteTitleStyle = {
  fontSize: AppleType.microUI.size, fontWeight: String(AppleFontWeight.medium),
  color: cv('text-tertiary'),
  letterSpacing: AppleType.microUI.letterSpacing, marginBottom: AppleSpace.sm,
};

const paletteItemStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.sm,
  padding: AppleSpace.sm, borderRadius: AppleRadius.small,
  cursor: 'grab', marginBottom: AppleSpace.xs,
  transition: 'background var(--apple-dur-fast)',
};

const paletteIconStyle = (color: string) => ({
  width: '28px', height: '28px', borderRadius: AppleRadius.small,
  background: color + '14', color, display: 'flex',
  alignItems: 'center', justifyContent: 'center', flexShrink: '0',
});

const paletteNameStyle = {
  fontSize: AppleType.microUIBold.size, fontWeight: String(AppleType.microUIBold.weight), color: cv('text-primary'),
};

const paletteDescStyle = {
  fontSize: AppleType.microUI.size, letterSpacing: AppleType.microUI.letterSpacing, color: cv('text-tertiary'),
};

const canvasWrapperStyle = {
  flex: 1, position: 'relative' as const,
};

const configPanelStyle = {
  width: '280px', borderLeft: `1px solid ${cv('border-light')}`,
  padding: AppleSpace.md,
  background: cv('bg-secondary'), flexShrink: '0',
  ...scrollSingleAxis,
};

const configTitleStyle = {
  fontSize: AppleType.bodyPrimary.size, fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'), marginBottom: AppleSpace.md,
};

const configFieldStyle = {
  marginBottom: AppleSpace.md,
};

const labelStyle = {
  display: 'block', fontSize: AppleType.microUI.size, fontWeight: String(AppleType.microUI.weight),
  color: cv('text-secondary'), marginBottom: AppleSpace.xs,
};

const textareaStyle = {
  width: '100%', minHeight: '80px',
};

const emptyConfigStyle = {
  display: 'flex', flexDirection: 'column' as const,
  alignItems: 'center', justifyContent: 'center',
  height: '200px', textAlign: 'center' as const,
};

// Custom node styles
const customNodeStyle = (borderColor: string) => ({
  minWidth: '160px', background: cv('surface-panel'),
  border: `2px solid ${borderColor}`, borderRadius: AppleRadius.medium,
  boxShadow: cv('shadow-subtle'),
  fontSize: AppleType.microUIBold.size, overflow: 'hidden' as const,
});

const nodeHeaderStyle = (color: string) => ({
  display: 'flex', alignItems: 'center', gap: '6px',
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  background: color + '12', color,
  fontWeight: String(AppleFontWeight.medium),
  fontSize: AppleType.microUI.size,
});

const nodeBodyStyle = {
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  fontSize: AppleType.microUI.size, color: cv('text-secondary'),
  lineHeight: AppleLineHeight.default,
};
</script>
