<template>
  <div :style="aiSubPageStyle">
    <ApSegmentedControl v-model="activeTab" :options="tabs" />

    <div :style="contentStyle">
      <!-- 质量巡检 -->
      <template v-if="activeTab === 'quality'">
        <div :style="statsRowStyle">
          <ApStatCard
            v-for="stat in qualityStats"
            :key="stat.label"
            :label="stat.label"
            :value="stat.value"
            :icon="stat.icon"
            :color="stat.color"
          />
        </div>

        <div :style="sectionStyle">
          <div :style="aiSubSectionHeaderStyle">
            <span :style="sectionTitleStyle">{{ $t('aiView2.k1') }}</span>
            <span v-if="lastAnalysisTime" :style="sectionMetaStyle">上次巡检: {{ lastAnalysisTime }}</span>
            <ApButton variant="primary" size="small" :loading="qualityLoading" @click="handleRunQuality">
              <ApIcon v-if="!qualityLoading" name="RefreshCw" :size="16" />
              {{ qualityLoading ? $t('vAiUi.k33') : $t('vAiUi.k34') }}
            </ApButton>
          </div>
          <ApSkeleton v-if="qualityLoading" variant="card" :lines="3" />
          <div v-else-if="qualityIssues.length === 0">
            <ApEmpty icon="CheckCircle" :title="$t('aiView2.k5')" :description="$t('vAiUi.k31')" />
          </div>
          <div v-else>
            <div v-for="issue in qualityIssues" :key="issue.id" :style="issueCardStyle">
              <div :style="issueHeaderStyle">
                <ApIcon :name="issueIcon(issue.severity)" :size="14" :color="severityColor(issue.severity)" />
                <span :style="issueTitleStyle">{{ issue.title }}</span>
                <span :style="severityBadgeStyle(issue.severity)">{{ issue.severity }}</span>
              </div>
              <div :style="issueDescStyle">{{ issue.description }}</div>
              <div v-if="issue.affected_docs?.length" :style="issueAffectedStyle">
                <span v-for="doc in issue.affected_docs.slice(0, 5)" :key="doc" :style="docTagStyle">{{ doc }}</span>
                <span v-if="issue.affected_docs.length > 5" :style="moreTagStyle">+{{ issue.affected_docs.length - 5 }} 更多</span>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- 智能推荐 -->
      <template v-if="activeTab === 'recommend'">
        <div :style="sectionStyle">
          <div :style="aiSubSectionHeaderStyle">
            <span :style="sectionTitleStyle">{{ $t('aiView2.k2') }}</span>
            <ApButton variant="secondary" size="small" @click="loadRecommendations">
              <ApIcon name="RefreshCw" :size="16" /> {{ $t('aiView2.refresh') }}
            </ApButton>
          </div>
          <ApSkeleton v-if="recommendationsLoading" variant="card" :lines="3" />
          <div v-else-if="recommendations.length === 0">
            <ApEmpty icon="Lightbulb" :title="$t('aiView2.k6')" :description="$t('vAiUi.k32')" />
          </div>
          <div v-else>
            <div v-for="rec in recommendations" :key="rec.id" :style="recCardStyle" @click="openDocument(rec)">
              <div :style="recContentStyle">
                <div :style="recTitleStyle">{{ rec.title }}</div>
                <div :style="recScoreStyle">
                  <ApProgress :value="(rec.score || 0) * 100" :label="$t('aiView2.k4')" :tone="toneForScore(rec.score || 0)" :style="{ width: AppleSpace['4xl'] }" />
                  <span :style="scoreTextStyle">{{ Math.round((rec.score || 0) * 100) }}% 匹配</span>
                </div>
                <div :style="recReasonStyle">{{ rec.reason }}</div>
              </div>
              <ApIcon name="ChevronRight" :size="16" :color="cv('text-quaternary')" />
            </div>
          </div>
        </div>
      </template>
    </div>
    </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { logger } from '@/utils/logger';








import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import { cv, AppleRadius, AppleSpace, AppleType, AppleFontWeight } from '@/utils/core/apple-tokens'
import { useUIStore } from '@/stores/app/ui'
import { toneForScore } from '@/utils/core/progress-tone'
import { formatTime } from '@/utils/format/format-time';
import { aiSubPageStyle, aiSubSectionStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle, aiSubSectionMetaStyle, crudStatsRowStyle, aiSubDescStyle as issueDescStyle, aiSubTagRowStyle as issueAffectedStyle } from '@/styles/ai/ai-page-styles';
import { runQualityPatrol, getRecommendations } from '@/api/modules/ai/ai-knowledge';
import type { IconName } from '@/utils/icons';

const activeTab = ref('quality');
const router = useRouter();
const ui = useUIStore();
const qualityLoading = ref(false);
const recommendationsLoading = ref(false);
const lastAnalysisTime = ref('');


const tabs = [
  { value: 'quality', label: t('aiView2.k7'), icon: 'Shield' },
  { value: 'recommend', label: t('aiView2.k2'), icon: 'Lightbulb' },
];

const qualityStats = ref<Array<{ label: string; value: string; icon: IconName; color: string }> >([
  { label: t('aiView2.k8'), value: '-', icon: 'FileText', color: cv('blue') },
  { label: t('aiView2.k9'), value: '-', icon: 'AlertTriangle', color: cv('orange') },
  { label: t('aiView2.k10'), value: '-', icon: 'Heart', color: cv('green') },
  { label: t('aiView2.k11'), value: '-', icon: 'AlertCircle', color: cv('red') },
]);

const qualityIssues = ref<Array<{ id: string; severity: string; title: string; description: string; affected_docs?: string[] }>>([]);
const recommendations = ref<Array<{ id: number; title: string; score: number; reason: string; space_id: number; document_id?: number }>>([]);

function issueIcon(severity: string) {
  if (severity === 'critical' || severity === 'error' || severity === 'high') return 'AlertCircle';
  if (severity === 'warning' || severity === 'medium') return 'AlertTriangle';
  return 'Info';
}
function severityColor(severity: string) {
  if (severity === 'critical' || severity === 'high' || severity === 'error') return cv('red');
  if (severity === 'warning' || severity === 'medium') return cv('yellow');
  return cv('blue');
}

async function handleRunQuality() {
  qualityLoading.value = true;

  qualityIssues.value = [];
  try {
    const res = await runQualityPatrol(0) as Record<string, unknown>;
    if (res.issues && Array.isArray(res.issues)) {
      qualityIssues.value = (res.issues as Array<Record<string, unknown>>).map((issue, idx) => ({
        id: String(issue.document_id || idx),
        severity: String(issue.severity || 'info'),
        title: String(issue.issue_type || issue.title || t('fallback.k103')),
        description: String(issue.description || ''),
        affected_docs: issue.document_name ? [String(issue.document_name)] : undefined,
      }));
    }
    qualityStats.value = [
      { label: t('aiView2.k8'), value: String(res.total_docs || 0), icon: 'FileText', color: cv('blue') },
      { label: t('aiView2.k9'), value: String(res.issue_count || 0), icon: 'AlertTriangle', color: cv('orange') },
      { label: t('aiView2.k10'), value: `${Math.round((Number(res.health_score || 0)) * 100)}%`, icon: 'Heart', color: cv('green') },
      { label: t('aiView2.k12'), value: String(Array.isArray(res.suggestions) ? (res.suggestions as unknown[]).length : 0), icon: 'AlertCircle', color: cv('red') },
    ];
    lastAnalysisTime.value = formatTime(Date.now() / 1000);
  } catch (e) { logger.error('质量巡检失败:', e); ui.showToast(t('aiView2.k13'), 'error'); }
  qualityLoading.value = false;
}

async function loadRecommendations() {
  recommendationsLoading.value = true;
  recommendations.value = [];

  try {
    const res = await getRecommendations(10);
    recommendations.value = (res.recommendations || []).map((r: Record<string, unknown>) => ({
      id: Number(r.id || r.document_id || 0),
      title: String(r.title || r.document_name || t('fallback.k75')),
      score: Number(r.score || 0),
      reason: String(r.reason || t('fallback.k45')),
      space_id: Number(r.space_id || 0),
      document_id: Number(r.document_id || r.id || 0),
    }));
  } catch (e) { logger.error('加载推荐失败:', e); ui.showToast(t('aiView2.k14'), 'error'); }
  recommendationsLoading.value = false;
}

function openDocument(rec: { space_id?: number; document_id?: number }) {
  if (rec.space_id && rec.document_id) {
    router.push(`/space/${rec.space_id}/doc/${rec.document_id}`);
  }
}

onMounted(() => {
  // 质量巡检和推荐按需加载，无需预加载
});

// ====== Styles ======
const contentStyle = { display: 'flex', flexDirection: 'column' as const };

const statsRowStyle = crudStatsRowStyle('compact');

const sectionStyle = aiSubSectionStyle;
const sectionTitleStyle = aiSubSectionTitleStyle;
const sectionMetaStyle = aiSubSectionMetaStyle;

const issueCardStyle = { padding: AppleSpace.md, borderBottom: `1px solid ${cv('border-light')}` };
const issueHeaderStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.sm, marginBottom: AppleSpace.xs };
const issueTitleStyle = { fontSize: AppleType.legalMeta.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary') };
const severityBadgeStyle = (s: string) => ({
  padding: `1px ${AppleSpace.md}`,
  borderRadius: AppleRadius.pill,
  fontSize: AppleType.legalMeta.size,
  background: (s === 'critical' || s === 'high' || s === 'error') ? cv('red-bg-subtle') : (s === 'warning' || s === 'medium' ? cv('yellow-bg-subtle') : cv('blue-bg-subtle')),
  color: (s === 'critical' || s === 'high' || s === 'error') ? cv('red') : (s === 'warning' || s === 'medium' ? cv('yellow') : cv('blue')),
});
const docTagStyle = { padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.micro, background: cv('bg-secondary'), color: cv('text-secondary'), fontSize: AppleType.legalMeta.size };
const moreTagStyle = { padding: `1px ${AppleSpace.sm}`, borderRadius: AppleRadius.micro, background: 'transparent', color: cv('text-tertiary'), fontSize: AppleType.legalMeta.size };

const recCardStyle = { display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: AppleSpace.md, borderBottom: `1px solid ${cv('border-light')}`, cursor: 'pointer' };
const recContentStyle = { flex: 1 };
const recTitleStyle = { fontSize: AppleType.controlLabel.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary'), marginBottom: AppleSpace.xxs };
const recScoreStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.sm, marginBottom: AppleSpace.xs };
const scoreTextStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-tertiary') };
const recReasonStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), lineHeight: 1.4 };
</script>
