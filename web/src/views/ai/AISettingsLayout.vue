<template>
  <ApPageContainer
    :title="activeMeta.title"
    :subtitle="activeMeta.subtitle"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    :breadcrumb-parent="{ label: $t('vAiUi.k71'), path: '/ai/settings' }"
    compact
  >
    <template #actions>
      <ApButton variant="secondary" size="small" @click="setSection('overview')">{{ $t('aiSetNav.k9') }}</ApButton>
      <ApButton variant="secondary" size="small" @click="router.push('/ai/graph')">
        {{ $t('aiSetNav.k10') }}
      </ApButton>
      <ApButton variant="primary" size="small" @click="router.push('/ai/workspace')">
        {{ $t('aiSetNav.k11') }}
      </ApButton>
    </template>
    <div :style="layoutStyle">
      <!-- Apple-style sidebar -->
      <nav :style="sidebarStyle">
        <div v-for="group in groups" :key="group.label" :style="groupStyle">
          <div :style="groupLabelStyle">{{ group.label }}</div>
          <ApButton
            v-for="item in group.items"
            :key="item.key"
            :variant="activeSection === item.key ? 'tint' : 'ghost'"
            size="small"
            :aria-current="activeSection === item.key ? 'page' : undefined"
            :style="navItemStyle"
            @click="setSection(item.key as keyof typeof sectionComponents)"
          >
            <component :is="item.icon" :size="16" :aria-hidden="true" />
            <span :style="{ flex: '1', textAlign: 'left' as const }">{{ item.label }}</span>
          </ApButton>
        </div>
      </nav>

      <!-- Content — 懒加载 + KeepAlive 缓存：切换子视图时保留状态 -->
      <div :style="contentStyle">
        <KeepAlive :max="3">
          <Suspense>
            <template #default>
              <component :is="sectionComponents[activeSection]" :key="activeSection" @navigate="handleNavigate" />
            </template>
            <template #fallback>
              <!-- 31 个子页都是懒加载的，原先这里是一行"加载中..."纯文字：
                   全站其他地方占位都用骨架（ApSkeleton 52 个文件在用），这一处是例外 -->
              <div :style="{ padding: AppleSpace.lg }">
                <ApSkeleton variant="text" :lines="5" />
              </div>
            </template>
          </Suspense>
        </KeepAlive>
      </div>
    </div>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { defineAsyncComponent, ref, markRaw, computed, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { icons } from '@/utils/icons';
import ApButton from '@/components/apple/ApButton.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';


import { cv, AppleFontWeight, AppleGlass, AppleRadius, AppleSpace, AppleType, AppleLayout, AppleTransition } from '@/utils/core/apple-tokens';
import { useBreakpoint } from '@/utils/responsive';
import { logger } from '@/utils/logger';

// 设置页子视图懒加载 — 按功能分组，同组组件共享 chunk，减少请求数
const asyncComponentOptions = {
  onError(error: Error, retry: () => void, fail: () => void, attempts: number) {
    logger.error(`[AISettings] Async component load failed (attempt ${attempts})`, error);
    if (attempts <= 2) {
      retry();
    } else {
      fail();
    }
  },
};

const AIOverviewView = defineAsyncComponent({ loader: () => import('./AIOverviewView.vue'), ...asyncComponentOptions });
const AIAnalysisView = defineAsyncComponent({ loader: () => import('./AIAnalysisView.vue'), ...asyncComponentOptions });
const AISearchOverviewView = defineAsyncComponent({ loader: () => import('./AISearchOverviewView.vue'), ...asyncComponentOptions });

const AIConfigView = defineAsyncComponent({ loader: () => import('./AIConfigView.vue'), ...asyncComponentOptions });
const AIRAGVectorView = defineAsyncComponent({ loader: () => import('./AIRAGVectorView.vue'), ...asyncComponentOptions });
const AIPromptTemplateView = defineAsyncComponent({ loader: () => import('./AIPromptTemplateView.vue'), ...asyncComponentOptions });

const AISchemaView = defineAsyncComponent({ loader: () => import('./AISchemaView.vue'), ...asyncComponentOptions });
const AIKnowledgeBaseView = defineAsyncComponent({ loader: () => import('./AIKnowledgeBaseView.vue'), ...asyncComponentOptions });
const AIImportView = defineAsyncComponent({ loader: () => import('./AIImportView.vue'), ...asyncComponentOptions });
const AIAutoTagView = defineAsyncComponent({ loader: () => import('./AIAutoTagView.vue'), ...asyncComponentOptions });

const AIMCPView = defineAsyncComponent({ loader: () => import('./AIMCPView.vue'), ...asyncComponentOptions });
const AIWorkflowView = defineAsyncComponent({ loader: () => import('./AIWorkflowView.vue'), ...asyncComponentOptions });
const AIWidgetView = defineAsyncComponent({ loader: () => import('./AIWidgetView.vue'), ...asyncComponentOptions });
const AIWriteView = defineAsyncComponent({ loader: () => import('./AIWriteView.vue'), ...asyncComponentOptions });

const AITokenView = defineAsyncComponent({ loader: () => import('./AITokenView.vue'), ...asyncComponentOptions });
const AIOperationsView = defineAsyncComponent({ loader: () => import('./AIOperationsView.vue'), ...asyncComponentOptions });
const AIObservabilityView = defineAsyncComponent({ loader: () => import('./AIObservabilityView.vue'), ...asyncComponentOptions });

const AISQLSecurityPolicyView = defineAsyncComponent({ loader: () => import('./AISQLSecurityPolicyView.vue'), ...asyncComponentOptions });

// LLM Wiki：知识编译与词条生成
const CitebaseCompileView = defineAsyncComponent({ loader: () => import('./CitebaseCompileView.vue'), ...asyncComponentOptions });
const CitebaseWriteView = defineAsyncComponent({ loader: () => import('./CitebaseWriteView.vue'), ...asyncComponentOptions });
const AIABExperimentView = defineAsyncComponent({ loader: () => import('./settings/AIABExperimentView.vue'), ...asyncComponentOptions });

// Text2SQL 运维：执行历史 / 审计追踪 / 少样本示例
const AISQLHistoryView = defineAsyncComponent({ loader: () => import('./settings/AISQLHistoryView.vue'), ...asyncComponentOptions });
const AISQLAuditTraceView = defineAsyncComponent({ loader: () => import('./settings/AISQLAuditTraceView.vue'), ...asyncComponentOptions });
const AISQLExampleView = defineAsyncComponent({ loader: () => import('./settings/AISQLExampleView.vue'), ...asyncComponentOptions });

// RAG 评估 / 数据敏感性 / 高级问答试跑
const AIRagasEvalView = defineAsyncComponent({ loader: () => import('./settings/AIRagasEvalView.vue'), ...asyncComponentOptions });
const AIDataSensitivityView = defineAsyncComponent({ loader: () => import('./settings/AIDataSensitivityView.vue'), ...asyncComponentOptions });
const AIAdvancedAskView = defineAsyncComponent({ loader: () => import('./settings/AIAdvancedAskView.vue'), ...asyncComponentOptions });

// 评估控制台 / 能力成熟度
const AIEvalConsoleView = defineAsyncComponent({ loader: () => import('./settings/AIEvalConsoleView.vue'), ...asyncComponentOptions });
const AICapabilityHealthView = defineAsyncComponent({ loader: () => import('./settings/AICapabilityHealthView.vue'), ...asyncComponentOptions });

// 记忆系统：后端 8 条 /ai/memory* 路由此前没有任何 UI 入口
const AIMemoryView = defineAsyncComponent({ loader: () => import('./settings/AIMemoryView.vue'), ...asyncComponentOptions });

// Wiki 治理：引脚/待审/体检/链接四组后端路由早已注册，此前只在编译页能碰到边角
const CitebaseGovernanceView = defineAsyncComponent({ loader: () => import('./settings/CitebaseGovernanceView.vue'), ...asyncComponentOptions });

const { isMobile } = useBreakpoint();

// section → 组件 的映射（配合 <component :is="..." /> 做条件渲染）
const sectionComponents = {
  overview: markRaw(AIOverviewView),
  analysis: markRaw(AIAnalysisView),
  'search-overview': markRaw(AISearchOverviewView),
  model: markRaw(AIConfigView),
  rag: markRaw(AIRAGVectorView),
  prompts: markRaw(AIPromptTemplateView),
  schema: markRaw(AISchemaView),
  'sql-security': markRaw(AISQLSecurityPolicyView),
  'sql-history': markRaw(AISQLHistoryView),
  'sql-audit-trace': markRaw(AISQLAuditTraceView),
  'sql-example': markRaw(AISQLExampleView),
  'ragas-eval': markRaw(AIRagasEvalView),
  'data-sensitivity': markRaw(AIDataSensitivityView),
  'advanced-ask': markRaw(AIAdvancedAskView),
  'eval-console': markRaw(AIEvalConsoleView),
  'capability-health': markRaw(AICapabilityHealthView),
  memory: markRaw(AIMemoryView),
  'wiki-governance': markRaw(CitebaseGovernanceView),
  'ab-experiment': markRaw(AIABExperimentView),
  'knowledge-base': markRaw(AIKnowledgeBaseView),
  'auto-tag': markRaw(AIAutoTagView),
  import: markRaw(AIImportView),
  mcp: markRaw(AIMCPView),
  workflows: markRaw(AIWorkflowView),
  widget: markRaw(AIWidgetView),
  write: markRaw(AIWriteView),
  token: markRaw(AITokenView),
  operations: markRaw(AIOperationsView),
  observability: markRaw(AIObservabilityView),
  'wiki-compile': markRaw(CitebaseCompileView),
  'wiki-write': markRaw(CitebaseWriteView),
} as const;

const validSectionKeys = Object.keys(sectionComponents) as Array<keyof typeof sectionComponents>;

const route = useRoute();
const router = useRouter();

function resolveSectionFromQuery(raw: string | null | undefined): keyof typeof sectionComponents {
  if (raw && validSectionKeys.includes(raw as keyof typeof sectionComponents)) {
    return raw as keyof typeof sectionComponents;
  }
  return 'overview';
}

const activeSection = ref<keyof typeof sectionComponents>(resolveSectionFromQuery(route.query.section as string | undefined));


watch(activeSection, (val) => {
  if (val !== (route.query.section as string | undefined)) {
    router.replace({ query: { ...route.query, section: val } });
  }
});

watch(() => route.query.section, (val) => {
  const resolved = resolveSectionFromQuery(val as string | undefined);
  if (resolved !== activeSection.value) {
    activeSection.value = resolved;
  }
});

const setSection = (key: keyof typeof sectionComponents) => {

  activeSection.value = key;
};

function handleNavigate(section: string) {
  if (validSectionKeys.includes(section as keyof typeof sectionComponents)) {
    setSection(section as keyof typeof sectionComponents);
  }
}

// 子页面 title/subtitle 元信息 — 由 ApPageContainer 统一渲染外层 H1
// 命名规则：title 与 nav label 保持一致，subtitle 用 · 分隔关键词
const sectionMeta: Record<keyof typeof sectionComponents, { title: string; subtitle: string }> = {
  overview: { title: t('aiSetNav.k12'), subtitle: t('aiSetNav.k13') },
  analysis: { title: t('aiSetNav.k14'), subtitle: t('aiSetNav.k15') },
  'search-overview': { title: t('aiSetNav.k16'), subtitle: t('aiSetNav.k17') },
  model: { title: t('aiSetNav.k18'), subtitle: t('aiSetNav.k19') },
  rag: { title: t('aiSetNav.k20'), subtitle: t('aiSetNav.k21') },
  prompts: { title: t('aiSetNav.k22'), subtitle: t('aiSetNav.k23') },
  schema: { title: t('aiSetNav.k24'), subtitle: t('aiSetNav.k25') },
  'knowledge-base': { title: t('aiSetNav.k26'), subtitle: t('aiSetNav.k27') },
  'auto-tag': { title: t('aiSetNav.k28'), subtitle: t('aiSetNav.k29') },
  import: { title: t('aiSetNav.k30'), subtitle: t('aiSetNav.k31') },
  mcp: { title: t('aiSetNav.k32'), subtitle: t('aiSetNav.k33') },
  workflows: { title: t('aiSetNav.k34'), subtitle: t('aiSetNav.k35') },
  widget: { title: t('aiSetNav.k36'), subtitle: t('aiSetNav.k37') },
  write: { title: t('aiSetNav.k38'), subtitle: t('aiSetNav.k39') },
  token: { title: t('aiSetNav.k40'), subtitle: t('aiSetNav.k41') },
  operations: { title: t('aiSetNav.k42'), subtitle: t('aiSetNav.k43') },
  observability: { title: t('aiSetNav.k44'), subtitle: t('aiSetNav.k45') },
  'ab-experiment': { title: t('aiSetNav.k46'), subtitle: t('aiSetNav.k47') },
  'sql-security': { title: t('aiSetNav.k48'), subtitle: t('aiSetNav.k49') },
  'sql-history': { title: t('aiSetNav.k50'), subtitle: t('aiSetNav.k51') },
  'sql-audit-trace': { title: t('aiSetNav.k52'), subtitle: t('aiSetNav.k53') },
  'sql-example': { title: t('aiSetNav.k54'), subtitle: t('aiSetNav.k55') },
  'ragas-eval': { title: t('aiSetNav.k56'), subtitle: t('aiSetNav.k57') },
  'data-sensitivity': { title: t('aiSetNav.k58'), subtitle: t('aiSetNav.k59') },
  'advanced-ask': { title: t('aiSetNav.k60'), subtitle: t('aiSetNav.k61') },
  'eval-console': { title: t('aiSetNav.k62'), subtitle: t('aiSetNav.k63') },
  'capability-health': { title: t('aiSetNav.k64'), subtitle: t('aiSetNav.k65') },
  memory: { title: t('aiSetNav.k66'), subtitle: t('aiSetNav.k67') },
  'wiki-governance': { title: t('aiSetNav.k68'), subtitle: t('aiSetNav.k69') },
  'wiki-compile': { title: t('aiSetNav.k70'), subtitle: t('aiSetNav.k71') },
  'wiki-write': { title: t('aiSetNav.k72'), subtitle: t('aiSetNav.k73') },
};

const activeMeta = computed(() => sectionMeta[activeSection.value]);

const groups = [
  {
    label: t('aiSetNav.k74'),
    items: [
      { key: 'overview', label: t('aiSetNav.k12'), icon: icons.LayoutDashboard },
      { key: 'analysis', label: t('aiSetNav.k14'), icon: icons.PieChart },
      { key: 'search-overview', label: t('aiSetNav.k16'), icon: icons.Search },
    ],
  },
  {
    label: t('aiSetNav.k75'),
    items: [
      { key: 'model', label: t('aiSetNav.k18'), icon: icons.Cpu },
      { key: 'rag', label: t('aiSetNav.k20'), icon: icons.FileSearch },
      { key: 'prompts', label: t('aiSetNav.k22'), icon: icons.ClipboardList },
    ],
  },
  {
    label: t('aiSetNav.k76'),
    items: [
      { key: 'schema', label: t('aiSetNav.k24'), icon: icons.Database },
      { key: 'knowledge-base', label: t('aiSetNav.k26'), icon: icons.Library },
      { key: 'import', label: t('aiSetNav.k30'), icon: icons.FileUp },
      { key: 'auto-tag', label: t('aiSetNav.k28'), icon: icons.Tag },
      { key: 'sql-history', label: t('aiSetNav.k50'), icon: icons.History },
      { key: 'sql-audit-trace', label: t('aiSetNav.k52'), icon: icons.ScanSearch },
      { key: 'sql-example', label: t('aiSetNav.k54'), icon: icons.Layers },
      { key: 'advanced-ask', label: t('aiSetNav.k60'), icon: icons.FlaskConical },
    ],
  },
  {
    label: t('aiSetNav.k77'),
    items: [
      { key: 'mcp', label: t('aiSetNav.k32'), icon: icons.Wrench },
      { key: 'workflows', label: t('aiSetNav.k34'), icon: icons.Workflow },
      { key: 'widget', label: t('aiSetNav.k36'), icon: icons.Blocks },
      { key: 'write', label: t('aiSetNav.k38'), icon: icons.SquarePen },
      { key: 'wiki-compile', label: t('aiSetNav.k70'), icon: icons.BookMarked },
      { key: 'wiki-write', label: t('aiSetNav.k72'), icon: icons.PenLine },
      { key: 'wiki-governance', label: t('aiSetNav.k68'), icon: icons.ScrollText },
    ],
  },
  {
    label: t('aiSetNav.k78'),
    items: [
      { key: 'observability', label: t('aiSetNav.k44'), icon: icons.Radar },
      { key: 'token', label: t('aiSetNav.k40'), icon: icons.Coins },
      { key: 'operations', label: t('aiSetNav.k42'), icon: icons.ServerCog },
      { key: 'sql-security', label: t('aiSetNav.k48'), icon: icons.Shield },
      { key: 'ragas-eval', label: t('aiSetNav.k56'), icon: icons.Target },
      { key: 'data-sensitivity', label: t('aiSetNav.k58'), icon: icons.Lock },
      { key: 'eval-console', label: t('aiSetNav.k62'), icon: icons.Gauge },
      { key: 'capability-health', label: t('aiSetNav.k64'), icon: icons.HeartPulse },
      { key: 'memory', label: t('aiSetNav.k66'), icon: icons.Brain },
    ],
  },
  {
    label: t('aiSetNav.k79'),
    items: [
      { key: 'ab-experiment', label: t('aiSetNav.k46'), icon: icons.Split },
    ],
  },
];

const layoutStyle = computed(() => ({
  display: 'grid', gridTemplateColumns: isMobile.value ? '1fr' : '200px 1fr', gap: AppleSpace.lg, alignItems: 'start',
}));

const sidebarStyle = {
  position: 'sticky' as const, top: AppleSpace.md,
  // 21 项导航累计高于视口时，没有 maxHeight 的 overflowY 永不生效，尾部两组会永久滚不到
  maxHeight: `calc(100dvh - ${AppleLayout.shellBarHeight} - ${AppleSpace.md} * 2)`,
  // 液态玻璃语言（上缘高光斜面 + 浮起阴影）从 AppleGlass.standard 展开；
  // 底色/模糊/描边保持本列自己的档位（blur-panel 比 glass-blur 薄一档，1px 描边是既有设计）。
  // 本列是滚动容器（overflowY auto），不能走 .apple-glass-layer 的 ::before（绝对定位伪元素会随内容滚走）
  ...AppleGlass.standard,
  backgroundColor: cv('surface-panel'), borderRadius: AppleRadius.medium,
  // 这一列是与正文**并排**的壳面（grid 两列，正文在它右侧滚），最坏背景是页面自己的
  // canvas —— 豁免理由同 ApSidebar，登记在 check-glass 的 SHELL_EXEMPT。
  // 之前它铺 surface-panel 却不铺模糊：整页唯一一处"半透明白板压着内容"的假玻璃。
  backdropFilter: cv('blur-panel'), WebkitBackdropFilter: cv('blur-panel'),
  // 描边走玻璃亮边令牌（原先写成 1px border-light 把 AppleGlass.standard 的 rim 顶掉了：
  // 同为壳面，侧栏有 rim、这一列没有，材质语言在这里断了一档）
  border: `0.5px solid ${cv('glass-rim')}`, padding: `${AppleSpace.xs} 0`,
  overflowY: 'auto' as const,
};

const groupStyle = { marginBottom: AppleSpace.xxs };

const groupLabelStyle = {
  padding: `${AppleSpace.xxs} ${AppleSpace.md} ${AppleSpace.xxs}`,
  fontSize: AppleType.legalMeta.size, color: cv('text-tertiary'),
  fontWeight: String(AppleFontWeight.semibold), letterSpacing: AppleType.legalMeta.letterSpacing,
};

const navItemStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.xs,
  width: '100%', padding: `${AppleSpace.xxs} ${AppleSpace.sm}`, margin: `1px ${AppleSpace.xs}`,
  borderRadius: AppleRadius.micro, justifyContent: 'flex-start',
  transition: AppleTransition.default,
  height: '28px',
};

const contentStyle = { minWidth: 0 };
</script>
