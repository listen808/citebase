<template>
  <div :style="aiSubPageStyle">
    <!-- 高级问答试跑：多跳推理 / Self-RAG -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <ApIcon name="Brain" :size="18" :color="cv('indigo')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k33') }}</span>
          </div>
          <span :style="aiSubDescStyle">{{ $t('aiSet.k34') }}</span>
        </div>
      </template>

      <div :style="formStyle">
        <div :style="formRowStyle">
          <!-- 通道切换以前独占一行（tabBarStyle 自建行），并进这行筛选格的开头；
               套一层 div 是为了让它保持药丸自身的宽度，不被 grid 拉成一列宽 -->
          <div>
            <ApSegmentedControl v-model="channel" :options="channelOptions" size="small" :aria-label="$t('aiSet.k49')" />
          </div>
          <div>
            <div :style="aiSubFilterLabelStyle">{{ $t('aiSet.k35') }}</div>
            <ApSelect
              v-model="selectedSpaceId"
              :options="spaceOptions"
              :placeholder="$t('aiSet.k13')"
              :searchable="true"
            />
          </div>
          <ApInput v-model="limit" type="number" :label="$t('aiSet.k42')" :placeholder="$t('aiSet.k45')" />
        </div>
        <ApTextarea
          v-model="question"
          :label="$t('aiSet.k43')"
          :placeholder="$t('aiSet.k46')"
          :rows="4"
          required
        />
      </div>

      <div :style="formActionsStyle">
        <ApButton variant="primary" size="small" :loading="running" @click="handleAsk">
          <ApIcon name="Play" :size="16" /> {{ channel === 'self-rag' ? $t('vAiUi.k206') : $t('vAiUi.k207') }}
        </ApButton>
        <ApButton variant="ghost" size="small" :disabled="running" @click="clearResult">{{ $t('aiSet.k36') }}</ApButton>
      </div>

      <div v-if="askError" :style="aiSubServerErrorStyle">
        <ApIcon name="AlertTriangle" :size="14" /> {{ askError }}
      </div>

      <template v-if="result">
        <div :style="aiSubSectionStyle">
          <div :style="aiSubSectionHeaderStyle">
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k37') }}</span>
            <span :style="aiSubCountBadgeStyle">{{ sources.length }} 个来源 · {{ elapsedText }}</span>
          </div>
          <div :style="aiSubBodyStyle">{{ result.answer || $t('fallback.k116') }}</div>
        </div>

        <div :style="aiSubSectionStyle">
          <div :style="aiSubSectionHeaderStyle">
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k38') }}</span>
          </div>
          <ApTable
            v-if="sources.length > 0"
            density="compact"
            :columns="sourceColumns"
            :data="sources"
          />
          <ApEmpty v-else icon="Search" :title="$t('aiSet.k48')" :description="$t('vAiUi.k205')" />
        </div>
      </template>
    </ApCard>

    <!-- PII 检测试跑 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <ApIcon name="ScanLine" :size="18" :color="cv('red')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k39') }}</span>
          </div>
          <span :style="aiSubDescStyle">{{ $t('aiSet.k40') }}</span>
        </div>
      </template>

      <ApTextarea
        v-model="piiText"
        :label="$t('aiSet.k44')"
        :placeholder="$t('aiSet.k47')"
        :rows="4"
      />

      <div :style="formActionsStyle">
        <ApButton variant="primary" size="small" :loading="detecting" @click="handleDetectPii">
          <ApIcon name="ScanLine" :size="16" /> {{ $t('aiSet.k41') }}
        </ApButton>
      </div>

      <div v-if="piiError" :style="aiSubServerErrorStyle">
        <ApIcon name="AlertTriangle" :size="14" /> {{ piiError }}
      </div>

      <div v-else-if="piiResult" :style="aiSubSectionStyle">
        <div :style="listStyle(AppleSpace.sm)">
          <div :style="aiSubResultStyle(piiResult.count === 0)">
            {{ piiResult.count === 0 ? '未检出个人信息' : `检出 ${piiResult.count} 处个人信息（已掩码展示）` }}
          </div>
          <div v-if="piiResult.pii_found?.length" :style="aiSubTagRowStyle">
            <span
              v-for="(item, index) in piiResult.pii_found"
              :key="`${item}-${index}`"
              :style="aiSubTagStyle"
            >{{ item }}</span>
          </div>
        </div>
      </div>
    </ApCard>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted } from 'vue';
import { icons } from '@/utils/icons';

import { logger } from '@/utils/logger';
import { cv, AppleSpace } from '@/utils/core/apple-tokens';
import {
  aiSubPageStyle,
  aiSubSectionStyle,
  aiSubSectionHeaderStyle,
  aiSubSectionTitleStyle,
  aiSubCardNameRowStyle,
  aiSubCountBadgeStyle,
  aiSubDescStyle,
  aiSubBodyStyle,
  aiSubTagRowStyle,
  aiSubTagStyle,
  aiSubFilterLabelStyle,
  aiSubResultStyle,
  aiSubServerErrorStyle,
  formStyle,
  formRowStyle,
  formActionsStyle,
  listStyle,
  AI_TABLE_WIDTH,
} from '@/styles/ai/ai-page-styles';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import { AdvancedAskAPI, type AdvancedAskResult, type AdvancedAskChannel, type AskSource } from '@/api/modules/ai/ai-advanced-ask';
import { DataSensitivityAPI, type DetectPIIResult } from '@/api/modules/ai/ai-data-sensitivity';
import { useSpaceStore } from '@/stores/workspace/space';
import type { Space } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();
const spaceStore = useSpaceStore();

const channel = ref<AdvancedAskChannel>('multi-hop');
const channelOptions = [
  { value: 'multi-hop', label: t('aiSet.k50'), icon: icons.Brain },
  { value: 'self-rag', label: 'Self-RAG', icon: icons.Play },
];

const spaces = ref<Space[]>([]);
const selectedSpaceId = ref<number | null>(null);
const question = ref('');
const limit = ref('5');
const running = ref(false);
const result = ref<AdvancedAskResult | null>(null);
const askError = ref('');
const elapsedMs = ref(0);

const piiText = ref('');
const piiResult = ref<DetectPIIResult | null>(null);
const detecting = ref(false);
const piiError = ref('');

const spaceOptions = computed(() => [
  { label: t('aiSet.k51'), value: 0 },
  ...spaces.value.map((s) => ({ label: s.name, value: s.id })),
]);

const elapsedText = computed(() => `${(elapsedMs.value / 1000).toFixed(1)}s`);

/** rag.AIAnswer.Sources 为 nil 时序列化为 null，统一兜底成空数组 */
const sources = computed<AskSource[]>(() => result.value?.sources ?? []);

const sourceColumns: TableColumn<AskSource>[] = [
  { key: 'document_name', title: t('aiSet.k52'), width: AI_TABLE_WIDTH.xl },
  { key: 'score', title: t('aiSet.k53'), width: AI_TABLE_WIDTH.sm, render: (row) => row.score.toFixed(3) },
  { key: 'position', title: t('aiSet.k54'), width: AI_TABLE_WIDTH.md, render: (row) => `L${row.start_line}-${row.end_line}` },
  { key: 'heading_path', title: t('aiSet.k55'), width: AI_TABLE_WIDTH.lg, ellipsis: true },
  { key: 'snippet', title: t('aiSet.k56'), ellipsis: true },
];

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
    const firstSpace = spaces.value[0];
    if (firstSpace && selectedSpaceId.value === null) {
      selectedSpaceId.value = firstSpace.id;
    }
  } catch (e) {
    logger.error('Failed to load spaces:', e);
    ui.showToast(t('aiSet.k26'), 'error');
  }
}

async function handleAsk() {
  const trimmed = question.value.trim();
  if (!trimmed) {
    ui.showToast(t('aiSet.k59'), 'warning');
    return;
  }
  running.value = true;
  askError.value = '';
  result.value = null;
  const startedAt = Date.now();
  try {
    const res = await AdvancedAskAPI.run(channel.value, {
      question: trimmed,
      space_id: selectedSpaceId.value ?? 0,
      limit: clampLimit(Number(limit.value)),
    });
    result.value = res;
    elapsedMs.value = Date.now() - startedAt;
  } catch (e) {
    logger.error('Failed to run advanced ask:', e);
    askError.value = e instanceof Error ? e.message: t('aiSet.k57');
    elapsedMs.value = Date.now() - startedAt;
  } finally {
    running.value = false;
  }
}

/** 后端缺省 5、上限 20，前端先夹紧避免无效请求 */
function clampLimit(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 5;
  return Math.min(Math.floor(value), 20);
}

function clearResult() {
  result.value = null;
  askError.value = '';
  question.value = '';
}

async function handleDetectPii() {
  const trimmed = piiText.value.trim();
  if (!trimmed) {
    ui.showToast(t('aiSet.k60'), 'warning');
    return;
  }
  detecting.value = true;
  piiError.value = '';
  piiResult.value = null;
  try {
    piiResult.value = await DataSensitivityAPI.detectPII(trimmed);
  } catch (e) {
    logger.error('Failed to detect pii:', e);
    piiError.value = e instanceof Error ? e.message: t('aiSet.k58');
  } finally {
    detecting.value = false;
  }
}

onMounted(() => loadSpaces());
</script>
