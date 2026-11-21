<template>
  <div :style="aiSubPageStyle">
    <ApSkeleton v-if="loading" :lines="5" />
    <template v-else>
    <!-- 配置引导（仅未完成初始化时显示） -->
    <div v-if="!configReady" :style="guideCardStyle">
      <div :style="guideHeaderStyle">
        <ApIcon name="Zap" :size="16" :color="cv('blue')" />
        <span :style="guideTitleStyle">{{ $t('aiView2.k83') }}</span>
      </div>
      <div :style="guideStepsStyle">
        <div :style="guideStepStyle(!!providerName)">
          <div :style="stepIconStyle(!!providerName)"><ApIcon v-if="!!providerName" name="Check" :size="14" /><span v-else>1</span></div>
          <div>
            <div :style="stepTitleStyle">{{ $t('aiView2.k84') }}</div>
            <div :style="stepDescStyle">{{ providerName ? $t('vAiUi.k45') + providerName : $t('vAiUi.k46') }}</div>
          </div>
          <ApButton v-if="!providerName" variant="secondary" size="small" @click="$emit('navigate', 'model')">{{ $t('aiView2.k85') }}</ApButton>
        </div>
        <div :style="guideStepStyle(vectorStats.embedder_ready)">
          <div :style="stepIconStyle(vectorStats.embedder_ready)"><ApIcon v-if="vectorStats.embedder_ready" name="Check" :size="14" /><span v-else>2</span></div>
          <div>
            <div :style="stepTitleStyle">{{ $t('aiView2.k86') }}</div>
            <div :style="stepDescStyle">{{ vectorStats.embedder_ready ? $t('vAiUi.k47') : $t('vAiUi.k48') }}</div>
          </div>
          <ApButton v-if="!vectorStats.embedder_ready" variant="secondary" size="small" @click="$emit('navigate', 'vector')">{{ $t('aiView2.k85') }}</ApButton>
        </div>
        <div :style="guideStepStyle(vectorStats.doc_count > 0)">
          <div :style="stepIconStyle(vectorStats.doc_count > 0)"><ApIcon v-if="vectorStats.doc_count > 0" name="Check" :size="14" /><span v-else>3</span></div>
          <div>
            <div :style="stepTitleStyle">{{ $t('aiView2.k87') }}</div>
            <div :style="stepDescStyle">{{ vectorStats.doc_count > 0 ? $t('aiView2.k134', { n: vectorStats.doc_count }) : $t('aiView2.k135') }}</div>
          </div>
          <ApButton v-if="vectorStats.doc_count === 0 && vectorStats.embedder_ready" variant="secondary" size="small" @click="$emit('navigate', 'vector')">{{ $t('aiView2.k88') }}</ApButton>
        </div>
      </div>
    </div>

    <!-- 状态总览 -->
    <div :style="statusBannerStyle">
      <div :style="statusLeftStyle">
        <div :style="statusDotStyle(statusEnabled)"></div>
        <div>
          <div :style="statusTitleStyle">{{ $t('aiView2.k136') }} {{ statusEnabled ? $t('vAiUi.k49') : $t('vAiUi.k50') }}</div>
          <div :style="statusSubStyle">{{ providerName }} · {{ modelName }}</div>
        </div>
      </div>
      <ApButton v-if="!statusEnabled" variant="primary" size="small" @click="$emit('navigate', 'model')">{{ $t('aiView2.k89') }}</ApButton>
    </div>

    <!-- 核心指标 -->
    <div :style="statsGridStyle">
      <ApStatCard v-for="stat in coreStats" :key="stat.label" :label="stat.label" :value="stat.value" :icon="stat.icon" :color="stat.color" size="compact" />
    </div>

    <!-- 两栏布局 -->
    <div :style="twoColStyle">
      <!-- 向量索引状态 -->
      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="Database" :size="16" :color="cv('purple')" />
            <span :style="sectionTitleStyle">{{ $t('aiView2.k90') }}</span>
          </div>
        </template>
        <div :style="kvListStyle">
          <div :style="kvRowStyle">
            <span :style="kvLabelStyle">{{ $t('aiView2.k91') }}</span>
            <span :style="kvValueStyle">{{ vectorStats.doc_count }}</span>
          </div>
          <div :style="kvRowStyle">
            <span :style="kvLabelStyle">Embedder</span>
            <span :style="kvBadgeStyle(vectorStats.embedder_ready)">{{ vectorStats.embedder_ready ? $t('vAiUi.k51') : $t('vAiUi.k52') }}</span>
          </div>
          <div :style="kvRowStyle">
            <span :style="kvLabelStyle">Syncer</span>
            <span :style="kvBadgeStyle(vectorStats.syncer_ready)">{{ vectorStats.syncer_ready ? $t('vAiUi.k51') : $t('vAiUi.k52') }}</span>
          </div>
          <div :style="kvRowStyle">
            <span :style="kvLabelStyle">{{ $t('aiView2.k92') }}</span>
            <span :style="kvValueStyle">{{ vectorStats.tasks?.pending ?? 0 }}</span>
          </div>
          <div :style="kvRowStyle">
            <span :style="kvLabelStyle">{{ $t('aiView2.k93') }}</span>
            <span :style="kvValueStyle">{{ vectorStats.tasks?.failed ?? 0 }}</span>
          </div>
        </div>
      </ApCard>

      <!-- 用户反馈 -->
      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="ThumbsUp" :size="16" :color="cv('green')" />
            <span :style="sectionTitleStyle">{{ $t('aiView2.k94') }}</span>
          </div>
        </template>
        <div v-if="feedbackTotal > 0" :style="kvListStyle">
          <div :style="kvRowStyle">
            <span :style="kvLabelStyle">{{ $t('aiView2.k95') }}</span>
            <span :style="kvValueStyle">{{ feedbackTotal }}</span>
          </div>
          <div :style="kvRowStyle">
            <span :style="kvLabelStyle">{{ $t('aiView2.k96') }}</span>
            <span :style="{ ...kvValueStyle, color: cv('green') }">{{ feedbackPositive }}</span>
          </div>
          <div :style="kvRowStyle">
            <span :style="kvLabelStyle">{{ $t('aiView2.k97') }}</span>
            <span :style="{ ...kvValueStyle, color: cv('red') }">{{ feedbackNegative }}</span>
          </div>
          <div :style="kvRowStyle">
            <span :style="kvLabelStyle">{{ $t('aiView2.k98') }}</span>
            <span :style="kvValueStyle">{{ feedbackRate }}%</span>
          </div>
          <div v-if="feedbackReasons.length" :style="{ marginTop: AppleSpace.sm }">
            <div :style="kvLabelStyle">{{ $t('aiView2.k99') }}</div>
            <div v-for="(r, i) in feedbackReasons" :key="i" :style="reasonItemStyle">{{ r }}</div>
          </div>
        </div>
        <ApEmpty v-else icon="ThumbsUp" :title="$t('aiView2.k101')" :description="$t('vAiUi.k43')" />
      </ApCard>
    </div>

    <!-- 最近审计日志 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="ScrollText" :size="16" :color="cv('blue')" />
          <span :style="sectionTitleStyle">{{ $t('aiView2.k100') }}</span>
          <span :style="logCountStyle">{{ recentLogs.length }} {{ $t('vAiUi.k53') }}</span>
        </div>
      </template>
      <div v-if="recentLogs.length > 0" :style="logListStyle">
        <div
          v-for="log in recentLogs"
          :key="log.id"
          :style="{ ...logRowStyle, backgroundColor: hoveredLogId === log.id ? cv('bg-hover') : 'transparent' }"
          @mouseenter="hoveredLogId = log.id"
          @mouseleave="hoveredLogId = null"
          @touchstart="hoveredLogId = log.id"
        >
          <span :style="logActionStyle">{{ actionLabel(log.action) }}</span>
          <span :style="logDetailStyle">{{ log.detail || '-' }}</span>
          <span :style="logTimeStyle">{{ formatTime(log.created_at) }}</span>
        </div>
      </div>
      <ApEmpty v-else icon="ScrollText" :title="$t('aiView2.k102')" :description="$t('vAiUi.k44')" />
    </ApCard>
    </template>
</div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { logger } from '@/utils/logger';
import { ref, computed, onMounted } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';






import { cv, AppleRadius, AppleSpace, AppleType, AppleFontWeight, AppleTransition, AppleGridFloor, appleCardGrid } from '@/utils/core/apple-tokens';
import type { IconName } from '@/utils/icons';
import { useBreakpoint } from '@/utils/responsive';
import { aiSubPageStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle } from '@/styles/ai/ai-page-styles';
import { getAIDashboard, getAIAuditLogs } from '@/api/modules/ai/ai-dashboard';
import { getVectorStats } from '@/api/modules/ai/ai-config';
import { useUIStore } from '@/stores/app/ui';
import { formatNumberCompact, formatTime } from '@/utils/format/format-time';

defineEmits<{ navigate: [section: string] }>();

const ui = useUIStore();
const { isMobile } = useBreakpoint();
const loading = ref(true);
const statusEnabled = ref(false);
const providerName = ref('-');

// P1-2: 配置引导完成度
const configReady = computed(() => statusEnabled.value && !!providerName.value && providerName.value !== '-' && vectorStats.value.embedder_ready && vectorStats.value.doc_count > 0);
const modelName = ref('-');
const tokenUsage = ref(0);
const cacheSize = ref(0);
const analysisTaskCount = ref(0);
const feedbackPositive = ref(0);
const feedbackNegative = ref(0);
const feedbackTotal = ref(0);
const feedbackReasons = ref<string[]>([]);
const vectorStats = ref<{ doc_count: number; embedder_ready: boolean; syncer_ready: boolean; tasks: { pending: number; processing: number; done: number; failed: number; dead: number } }>({ doc_count: 0, embedder_ready: false, syncer_ready: false, tasks: { pending: 0, processing: 0, done: 0, failed: 0, dead: 0 } });
const recentLogs = ref<Array<{ id: number; action: string; detail?: string; result?: string; created_at: string }>>([]);
const hoveredLogId = ref<number | null>(null);

const feedbackRate = computed(() => {
  if (feedbackTotal.value === 0) return 0;
  return Math.round((feedbackPositive.value / feedbackTotal.value) * 100);
});
const { t } = useI18n();

const coreStats = computed<Array<{ label: string; value: string; icon: IconName; color: string }> >(() => [
  { label: t('aiView2.k42'), value: formatNumberCompact(tokenUsage.value), icon: 'Zap', color: cv('blue') },
  { label: t('aiView2.k103'), value: String(vectorStats.value.doc_count), icon: 'Database', color: cv('purple') },
  { label: t('aiView2.k104'), value: String(cacheSize.value), icon: 'Cpu', color: cv('green') },
  { label: t('aiView2.k69'), value: String(analysisTaskCount.value), icon: 'BarChart3', color: cv('orange') },
]);

function actionLabel(action: string): string {
  const map: Record<string, string> = {
    ai_ask: 'AI 问答', ai_agent: 'Agent 调用', ai_config: '配置变更',
    ai_analysis: '分析任务', ai_vector: '向量操作',
  };
  return map[action] || action;
}

async function fetchData() {
  loading.value = true;
  try {
    const [dashRes, vectorRes, logRes] = await Promise.allSettled([
      getAIDashboard(),
      getVectorStats(),
      getAIAuditLogs({ limit: 10 }),
    ]);

    if (dashRes.status === 'fulfilled' && dashRes.value) {
      const d = dashRes.value as Record<string, unknown>;
      statusEnabled.value = Boolean(d.enabled);
      providerName.value = (d.provider as Record<string, unknown>)?.name as string || 'OpenAI';
      tokenUsage.value = Number(d.token_usage || 0);
      cacheSize.value = Number(d.cache_size || 0);
      analysisTaskCount.value = Number(d.analysis_task_count || 0);
      const fb = d.feedback as Record<string, unknown> | undefined;
      if (fb) {
        feedbackPositive.value = Number(fb.positive || 0);
        feedbackNegative.value = Number(fb.negative || 0);
        feedbackTotal.value = Number(fb.total || 0);
        feedbackReasons.value = (fb.recent_reasons as string[]) || [];
      }
    }

    if (vectorRes.status === 'fulfilled' && vectorRes.value) {
      const v = vectorRes.value as Record<string, unknown>;
      vectorStats.value = {
        doc_count: Number(v.doc_count || 0),
        embedder_ready: Boolean(v.embedder_ready),
        syncer_ready: Boolean(v.syncer_ready),
        tasks: (v.tasks as typeof vectorStats.value.tasks) || { pending: 0, processing: 0, done: 0, failed: 0, dead: 0 },
      };
    }

    if (logRes.status === 'fulfilled' && logRes.value) {
      const l = logRes.value as Record<string, unknown>;
      recentLogs.value = (l.logs as typeof recentLogs.value) || [];
    }
  } catch (e) {
    logger.error('Failed to fetch overview data:', e);
    ui.showToast(t('aiView2.k105'), 'warning');
  } finally {
    loading.value = false;
  }
}

onMounted(fetchData);

const sectionTitleStyle = aiSubSectionTitleStyle;

const statusBannerStyle = {
  display: 'flex', alignItems: 'center', justifyContent: 'space-between',
  padding: `${AppleSpace.sm} ${AppleSpace.md}`, borderRadius: AppleRadius.card,
  border: `1px solid ${cv('border-light')}`, backgroundColor: cv('bg-secondary'),
};
const statusLeftStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.sm };
const statusDotStyle = (on: boolean) => ({
  width: '8px', height: '8px', borderRadius: AppleRadius.circle, flexShrink: 0,
  backgroundColor: on ? cv('green') : cv('text-disabled'),
});
const statusTitleStyle = { fontSize: AppleType.controlLabel.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary') };
const statusSubStyle = { fontSize: AppleType.microUI.size, color: cv('text-secondary'), marginTop: AppleSpace.xxs };

const statsGridStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.stat})`,
  gap: AppleSpace.sm,
};

const twoColStyle = computed(() => isMobile.value ? { display: 'grid', gridTemplateColumns: '1fr', gap: AppleSpace.md } : appleCardGrid('entity'));


const kvListStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xxs };
const kvRowStyle = { display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: `${AppleSpace.xxs} 0` };
const kvLabelStyle = { fontSize: AppleType.microUI.size, color: cv('text-secondary') };
const kvValueStyle = { fontSize: AppleType.legalMeta.size, fontWeight: String(AppleFontWeight.medium), color: cv('text-primary') };
const kvBadgeStyle = (ok: boolean) => ({
  fontSize: AppleType.microUI.size, fontWeight: String(AppleFontWeight.medium),
  color: ok ? cv('green') : cv('red'),
  backgroundColor: ok ? cv('green-light') : cv('red-light'),
  padding: `1px ${AppleSpace.xs}`,
  borderRadius: AppleRadius.pill,
});

const reasonItemStyle = {
  fontSize: AppleType.microUI.size, color: cv('text-secondary'),
  padding: `${AppleSpace.xxs} ${AppleSpace.xs}`, marginTop: AppleSpace.xxs,
  backgroundColor: cv('bg-secondary'), borderRadius: AppleRadius.medium,
};

const logCountStyle = { fontSize: AppleType.microUI.size, color: cv('text-tertiary'), marginLeft: 'auto' };
const logListStyle = { display: 'flex', flexDirection: 'column' as const };
const logRowStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.sm,
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`, margin: `0 -${AppleSpace.sm}`,
  borderBottom: `1px solid ${cv('border-light')}`,
  borderRadius: AppleRadius.small,
  transition: AppleTransition.default,
};
const logActionStyle = {
  fontSize: AppleType.microUI.size, fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'), minWidth: '64px', flexShrink: 0,
};
const logDetailStyle = { fontSize: AppleType.microUI.size, color: cv('text-secondary'), flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const };
const logTimeStyle = { fontSize: AppleType.microUI.size, color: cv('text-tertiary'), flexShrink: 0 };

// P1-2: 配置引导卡片样式
const guideCardStyle = {
  background: cv('blue-selection'),
  border: `1px solid ${cv('blue-light')}`,
  borderRadius: AppleRadius.card,
  padding: AppleSpace.md,
};
const guideHeaderStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.xs, marginBottom: AppleSpace.sm };
const guideTitleStyle = { fontSize: AppleType.controlLabel.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary') };
const guideStepsStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.sm };
const guideStepStyle = (done: boolean) => ({
  display: 'flex', alignItems: 'center', gap: AppleSpace.sm,
  padding: AppleSpace.xs,
  borderRadius: AppleRadius.small,
  background: done ? cv('bg-secondary') : 'transparent',
  opacity: done ? 0.7 : 1,
});
const stepIconStyle = (done: boolean) => ({
  minWidth: '1.125rem',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  color: done ? cv('success') : cv('text-tertiary'),
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
  fontVariantNumeric: 'tabular-nums',
  flexShrink: 0,
});
const stepTitleStyle = { fontSize: AppleType.legalMeta.size, fontWeight: String(AppleFontWeight.medium), color: cv('text-primary') };
const stepDescStyle = { fontSize: AppleType.microUI.size, color: cv('text-secondary') };
</script>
