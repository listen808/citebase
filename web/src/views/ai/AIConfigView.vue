<template>
  <div :style="aiSubPageStyle">
    <ApSkeleton v-if="loading" :lines="4" />
    <template v-else>
    <!-- 服务状态 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Cpu" :size="16" :color="cv('blue')" />
          <span :style="sectionTitleStyle">{{ $t('aiView.k71') }}</span>
        </div>
      </template>
      <div :style="statusRowStyle">
        <span :style="aiSubStatusDotStyle(enabled)" />
        <span :style="statusLabelStyle(enabled)">{{ enabled ? $t('vAiUi.k14') : $t('vAiUi.k15') }}</span>
      </div>
      <div :style="hintStyle">{{ $t('aiView.k72') }}</div>
    </ApCard>

    <!-- 模型配置：左选右配 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Zap" :size="16" :color="cv('orange')" />
          <span :style="sectionTitleStyle">{{ $t('aiView.k75') }}</span>
        </div>
      </template>

      <div :style="splitLayoutStyle">
        <!-- 左侧：Provider 分组列表 -->
        <div :style="providerListStyle">
          <div v-for="group in providerGroups" :key="group.title" :style="groupStyle" role="radiogroup" :aria-label="group.title">
            <div :style="groupTitleStyle">{{ group.title }}</div>
            <div
              v-for="p in group.providers"
              :key="p.label"
              :style="providerItemStyle(isPresetActive(p))"
              tabindex="0"
              role="radio"
              :aria-checked="isPresetActive(p)"
              @click="selectProvider(p)"
              @keydown.enter="selectProvider(p)"
              @keydown.space.prevent="selectProvider(p)"
            >
              <div :style="providerIconStyle(isPresetActive(p), p.color)">
                <component :is="p.icon" :size="16" :color="isPresetActive(p) ? cv('on-accent') : p.color" />
              </div>
              <div :style="providerInfoStyle">
                <div :style="providerNameStyle(isPresetActive(p))">{{ p.label }}</div>
                <div :style="providerDescStyle">{{ p.desc }}</div>
              </div>
              <ApIcon v-if="isPresetActive(p)" name="Check" :size="16" :color="cv('blue')" />
            </div>
          </div>
        </div>

        <!-- 右侧：配置表单 -->
        <div :style="configPanelStyle">
          <div :style="panelHeaderStyle">
            <div :style="panelTitleStyle">
              {{ selectedPreset.label }} {{ $t('vAiUi.k18') }}
            </div>
            <div :style="panelHintStyle">{{ selectedPreset.desc }}</div>
          </div>

          <div :style="formGridStyle">
            <ApInput
              v-if="form.provider_type !== 'ollama'"
              v-model="form.base_url"
              label="API Base URL"
              :placeholder="selectedPreset.defaultBaseUrl"
            />
            <ApInput
              v-if="form.provider_type !== 'ollama'"
              v-model="form.api_key"
              type="password"
              label="API Key"
              placeholder="sk-..."
            />

            <ApFormItem :label="$t('aiView.k83')">
              <ApSelect
                v-model="form.model"
                :options="modelOptions"
                :placeholder="$t('aiView.k87')"
                :searchable="true"
              />
            </ApFormItem>

            <ApFormItem v-if="showFastModel" :label="$t('aiView.k84')">
              <ApSelect
                v-model="form.fast_model"
                :options="modelOptions"
                :placeholder="$t('aiView.k87')"
                :searchable="true"
              />
            </ApFormItem>

            <ApFormItem v-if="showReasoningModel" :label="$t('aiView.k85')">
              <ApSelect
                v-model="form.reasoning_model"
                :options="modelOptions"
                :placeholder="$t('aiView.k87')"
                :searchable="true"
              />
            </ApFormItem>

            <ApFormItem label="Temperature">
              <!-- P1-8：数值从标签括号移到控件右端，拖动时视线不用来回跳 -->
              <div :style="sliderRowStyle">
                <ApSlider v-model="form.temperature" :min="0" :max="1" :step="0.1" aria-label="Temperature" />
                <span :style="sliderValueStyle">{{ form.temperature.toFixed(1) }}</span>
              </div>
            </ApFormItem>

            <ApInput
              :model-value="String(form.max_tokens)"
              type="number"
              label="Max Tokens"
              placeholder="4096"
              @update:model-value="(v) => form.max_tokens = Number(v)"
            />
          </div>

          <div :style="actionRowStyle">
            <ApButton variant="primary" size="small" :loading="saving" @click="handleSave">
              <ApIcon name="Save" :size="16" /> {{ $t('aiView.k76') }}
            </ApButton>
            <ApButton variant="secondary" size="small" :loading="testing" @click="handleTest">
              <ApIcon name="Zap" :size="16" /> {{ $t('aiView.k77') }}
            </ApButton>
            <ApButton
              variant="ghost"
              size="small"
              :loading="discovering"
              :disabled="discovering"
              @click="discoverModels"
            >
              <ApIcon name="Radio" :size="16" /> {{ $t('aiView.k121') }}
            </ApButton>
            <div v-if="saveResult" :style="aiSubResultStyle(saveResult.success)">
              <ApIcon v-if="saveResult.success" name="Check" :size="14" :color="cv('green')" />
              <ApIcon v-else name="AlertTriangle" :size="14" :color="cv('red')" />
              {{ saveResult.message }}
            </div>
            <div v-if="testResult" :style="aiSubResultStyle(testResult.success)">
              <ApIcon v-if="testResult.success" name="Check" :size="14" :color="cv('green')" />
              <ApIcon v-else name="AlertTriangle" :size="14" :color="cv('red')" />
              {{ testResult.message }}
            </div>
            <div v-if="discoverResult" :style="aiSubResultStyle(discoverResult.success)">
              <ApIcon v-if="discoverResult.success" name="Check" :size="14" :color="cv('green')" />
              <ApIcon v-else name="AlertTriangle" :size="14" :color="cv('red')" />
              {{ discoverResult.message }}
            </div>
          </div>
        </div>
      </div>
    </ApCard>

    <!-- 分析器限额与自动调度：后端早有 GET/POST，此前只能在 config.yaml 里改，
         页面上既看不见当前值也改不了 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Brain" :size="16" :color="cv('purple')" />
          <span :style="sectionTitleStyle">{{ $t('aiView.k78') }}</span>
        </div>
      </template>

      <div v-if="analyzerUnavailable" :style="hintStyle">
        {{ $t('aiView.k79') }}
      </div>
      <template v-else>
        <div :style="analyzerGridStyle">
          <ApFormItem
            v-for="key in analyzerKeys"
            :key="key"
            :label="ANALYZER_META[key].label"
            :hint="$t('aiSet.k299', { min: ANALYZER_META[key].min, max: ANALYZER_META[key].max })"
            :error="analyzerErrors[key]"
          >
            <ApInput v-model="analyzerForm[key]" type="number" :aria-label="ANALYZER_META[key].label" />
          </ApFormItem>
        </div>
        <div :style="aiSubActionRowStyle">
          <ApButton variant="primary" size="small" :loading="savingAnalyzer" @click="saveAnalyzer">
            <ApIcon name="Save" :size="16" /> {{ $t('aiView.k80') }}
          </ApButton>
          <ApButton variant="ghost" size="small" @click="loadAnalyzer">{{ $t('aiView.k81') }}</ApButton>
        </div>

        <div :style="scheduleRowStyle">
          <ApFormItem :label="$t('aiView.k86')" :hint="$t('vAiUi.k13')">
            <ApInput v-model="scheduleForm" :aria-label="$t('aiView.k91')" placeholder="0 3 * * *" />
          </ApFormItem>
          <div :style="statusRowStyle">
            <span :style="aiSubStatusDotStyle(scheduleRunning)" />
            <span :style="statusLabelStyle(scheduleRunning)">{{ scheduleRunning ? $t('vAiUi.k16') : $t('vAiUi.k17') }}</span>
          </div>
        </div>
        <div :style="aiSubActionRowStyle">
          <ApButton variant="secondary" size="small" :loading="savingSchedule" @click="saveSchedule">
            {{ $t('aiView.k82') }}
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
import type { Component } from 'vue';
import { logger } from '@/utils/logger';
import { ref, computed, onMounted } from 'vue';
import { icons } from '@/utils/icons';
import ApIcon from '@/components/apple/ApIcon.vue';
import { appleFixedColumns, cv, AppleFont, AppleFontWeight, AppleGridFloor, AppleRadius, AppleSpace, AppleType, AppleTransition, AppleLineHeight } from '@/utils/core/apple-tokens';
import { useBreakpoint } from '@/utils/responsive';
import { aiSubPageStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle, aiSubActionRowStyle, aiSubResultStyle, aiSubStatusDotStyle, statusRowStyle as baseStatusRowStyle } from '@/styles/ai/ai-page-styles';
import { getAIConfig, updateAIConfig, testAIConnection, discoverAIModels, getAnalyzerConfig, updateAnalyzerConfig, getAISchedule, updateAISchedule } from '@/api/modules/ai/ai-config';
import type { AnalyzerConfig } from '@/types/ai';
import { useUIStore } from '@/stores/app/ui';

const loading = ref(true);
const saving = ref(false);
const testing = ref(false);
const discovering = ref(false);
const enabled = ref(false);
const saveResult = ref<{ success: boolean; message: string } | null>(null);
const testResult = ref<{ success: boolean; message: string } | null>(null);
const discoverResult = ref<{ success: boolean; message: string } | null>(null);
const ui = useUIStore();
const { isMobile } = useBreakpoint();

const form = ref({
  provider_type: 'openai',
  base_url: '',
  api_key: '',
  model: '',
  fast_model: '',
  reasoning_model: '',
  temperature: 0.7,
  max_tokens: 4096,
});

// Provider 定义
interface ProviderDef {
  label: string;
  value: string;
  desc: string;
  color: string;
  icon: Component;
  defaultBaseUrl: string;
  models: string[];
  showFastModel?: boolean;
  showReasoningModel?: boolean;
}

// 第三方厂商的官方品牌色（OpenAI 绿 / Anthropic 橙 / Google 蓝 …），刻意**不走 cv()**：
// 它们标识的是外部品牌而非本产品，跟主题切换会丢掉辨识度（把 OpenAI 卡画成 Citebase 蓝
// 是错的）。这是「有理由的硬编码」豁免，与品牌 token 系统不冲突——产品自身的强调色仍只有一套。
const PROVIDER_BRAND_COLORS = {
  openai: '#10A37F',
  anthropic: '#D97757',
  gemini: '#4285F4',
  deepseek: '#4D6BFE',
  qwen: '#615CED',
  zhipu: '#3859FF',
  ernie: '#2932E1',
  ollama: '#222',
  moonshot: '#1D6FE0',
  siliconflow: '#FF6B35',
  yi: '#00B2FF',
  minimax: '#FF4D4F',
} as const;

const allProviders: ProviderDef[] = [
  {
    label: 'OpenAI',
    value: 'openai',
    desc: t('aiView.k92'),
    color: PROVIDER_BRAND_COLORS.openai,
    icon: icons.Zap,
    defaultBaseUrl: 'https://api.openai.com/v1',
    models: ['gpt-6-astra', 'gpt-6-sol', 'gpt-6-luna', 'gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-5.6-luna', 'gpt-4.1', 'gpt-4.1-mini', 'gpt-4.1-nano', 'gpt-4o', 'gpt-4o-mini', 'o4-mini', 'o3', 'o3-mini', 'o1', 'o1-pro'],
    showFastModel: true,
    showReasoningModel: true,
  },
  {
    label: 'Anthropic Claude',
    value: 'anthropic',
    desc: 'Claude Fable 5.1 / Opus 5.5 / Sonnet 5',
    color: PROVIDER_BRAND_COLORS.anthropic,
    icon: icons.Brain,
    defaultBaseUrl: 'https://api.anthropic.com',
    models: ['claude-fable-5-1', 'claude-opus-5-5', 'claude-sonnet-5', 'claude-haiku-4-5', 'claude-sonnet-4-5-20250929', 'claude-opus-4-1-20250805', 'claude-sonnet-4-20250514'],
    showFastModel: true,
    showReasoningModel: true,
  },
  {
    label: 'Google Gemini',
    value: 'gemini',
    desc: 'Gemini 4 Argon / 3.8 Flash / 3 Pro',
    color: PROVIDER_BRAND_COLORS.gemini,
    icon: icons.Globe,
    defaultBaseUrl: 'https://generativelanguage.googleapis.com/v1beta',
    models: ['gemini-4-argon', 'gemini-3.8-flash', 'gemini-3.6-flash', 'gemini-3-pro-preview', 'gemini-3-flash-preview', 'gemini-2.5-pro', 'gemini-2.5-flash', 'gemini-2.0-flash', 'gemini-2.0-flash-lite'],
    showFastModel: true,
  },
  {
    label: 'DeepSeek',
    value: 'deepseek',
    desc: t('aiView.k93'),
    color: PROVIDER_BRAND_COLORS.deepseek,
    icon: icons.Bot,
    defaultBaseUrl: 'https://api.deepseek.com/v1',
    models: ['deepseek-v4-flash', 'deepseek-v4-pro'],
    showFastModel: true,
    showReasoningModel: true,
  },
  {
    label: t('aiView.k94'),
    value: 'qwen',
    desc: 'Qwen3-Max / Plus / Turbo',
    color: PROVIDER_BRAND_COLORS.qwen,
    icon: icons.Radio,
    defaultBaseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    models: ['qwen3.8-max', 'qwen3.5-max', 'qwen3.5-plus', 'qwen3-max', 'qwen3-plus', 'qwen3-turbo', 'qwen-plus', 'qwen-turbo', 'qwen-max', 'qwen-long'],
    showFastModel: true,
  },
  {
    label: t('aiView.k95'),
    value: 'zhipu',
    desc: 'GLM-5 / GLM-4-Plus / GLM-Z1',
    color: PROVIDER_BRAND_COLORS.zhipu,
    icon: icons.Zap,
    defaultBaseUrl: 'https://open.bigmodel.cn/api/paas/v4',
    models: ['glm-5', 'glm-4-plus', 'glm-4-air', 'glm-4-flash', 'glm-4-long', 'glm-z1-air', 'glm-z1-flash'],
  },
  {
    label: t('aiView.k96'),
    value: 'ernie',
    desc: 'ERNIE 4.5 / 4.0 / Speed',
    color: PROVIDER_BRAND_COLORS.ernie,
    icon: icons.Bot,
    defaultBaseUrl: 'https://qianfan.baidubce.com/v2',
    models: ['ernie-4.5', 'ernie-4.0-8k', 'ernie-speed', 'ernie-lite'],
  },
  {
    label: t('aiView.k97'),
    value: 'ollama',
    desc: 'Llama 4 / Qwen3 / DeepSeek-R1',
    color: PROVIDER_BRAND_COLORS.ollama,
    icon: icons.Server,
    defaultBaseUrl: 'http://localhost:11434/v1',
    models: ['llama4:8b', 'qwen3:8b', 'deepseek-r1:8b', 'gemma3:4b', 'phi4:mini'],
    showFastModel: true,
    showReasoningModel: true,
  },
];

const compatiblePresets: ProviderDef[] = [
  {
    label: 'Moonshot Kimi',
    value: 'openai',
    desc: 'Kimi K2.5（provider=openai）',
    color: PROVIDER_BRAND_COLORS.moonshot,
    icon: icons.Cloud,
    defaultBaseUrl: 'https://api.moonshot.cn/v1',
    models: ['kimi-k2.5', 'kimi-k2-0711-preview', 'moonshot-v1-8k', 'moonshot-v1-32k', 'moonshot-v1-128k'],
  },
  {
    label: t('aiView.k98'),
    value: 'openai',
    desc: t('aiView.k99'),
    color: PROVIDER_BRAND_COLORS.siliconflow,
    icon: icons.Cloud,
    defaultBaseUrl: 'https://api.siliconflow.cn/v1',
    models: ['Qwen/Qwen3-235B-A22B', 'Qwen/Qwen3-32B', 'Qwen/Qwen3-8B', 'deepseek-ai/DeepSeek-R1', 'BAAI/bge-m3'],
  },
  {
    label: t('aiView.k100'),
    value: 'openai',
    desc: 'Yi-Lightning（provider=openai）',
    color: PROVIDER_BRAND_COLORS.yi,
    icon: icons.Cloud,
    defaultBaseUrl: 'https://api.lingyiwanwu.com/v1',
    models: ['yi-lightning', 'yi-large'],
  },
  {
    label: 'MiniMax',
    value: 'openai',
    desc: 'MiniMax-Text-01（provider=openai）',
    color: PROVIDER_BRAND_COLORS.minimax,
    icon: icons.Cloud,
    defaultBaseUrl: 'https://api.minimax.chat/v1',
    models: ['MiniMax-Text-01', 'abab6.5-chat'],
  },
];

// 分组
const providerGroups = computed(() => [
  { title: t('aiView.k101'), providers: allProviders.filter(p => ['openai', 'anthropic', 'gemini'].includes(p.value)) },
  { title: t('aiView.k102'), providers: allProviders.filter(p => ['deepseek', 'qwen', 'zhipu', 'ernie'].includes(p.value)) },
  { title: t('aiView.k103'), providers: allProviders.filter(p => p.value === 'ollama') },
  { title: t('aiView.k104'), providers: compatiblePresets },
]);

// 当前选中的预设（包含兼容厂商信息）
const selectedPreset = ref<ProviderDef>(allProviders[0]!);

function selectProvider(preset: ProviderDef) {
  selectedPreset.value = preset;
  form.value.provider_type = preset.value;
  // 只在 base_url 为空或属于已知预设时才覆盖，避免覆盖用户自定义的 URL
  const knownUrls = [...allProviders, ...compatiblePresets].map(p => p.defaultBaseUrl);
  if (!form.value.base_url || knownUrls.includes(form.value.base_url)) {
    form.value.base_url = preset.defaultBaseUrl;
  }
}

// 判断当前是否选中了某个预设（用于左侧高亮）
function isPresetActive(preset: ProviderDef): boolean {
  return selectedPreset.value.label === preset.label
    && form.value.provider_type === preset.value;
}

const showFastModel = computed(() => selectedPreset.value.showFastModel ?? false);
const showReasoningModel = computed(() => selectedPreset.value.showReasoningModel ?? false);

const discoveredModels = ref<string[]>([]);

// 模型选项 = 上游已发现的清单（真实模型，排前）+ 厂商预设（补全，排后）
// + 当前已填但两边都没有的值（保证 ApSelect 不为已配置模型显示空 placeholder）
const modelOptions = computed(() => {
  const seen = new Set<string>();
  const options: Array<{ label: string; value: string }> = [];
  const candidates = [
    ...discoveredModels.value,
    ...(selectedPreset.value.models || []),
    form.value.model,
    form.value.fast_model,
    form.value.reasoning_model,
  ];
  for (const m of candidates) {
    if (m && !seen.has(m)) {
      seen.add(m);
      options.push({ label: m, value: m });
    }
  }
  return options;
});

// 按当前 primary 配置向上游发现模型，并把未填的主/快速/推理模型补上
async function discoverModels() {
  discovering.value = true;
  discoverResult.value = null;
  try {
    const res = await discoverAIModels();
    if (res.success && res.models.length) {
      discoveredModels.value = res.models;
      if (!form.value.model) form.value.model = res.models[0] ?? '';
      if (form.value.fast_model && !res.models.includes(form.value.fast_model)) {
        form.value.fast_model = res.models[0] ?? '';
      }
      if (form.value.reasoning_model && !res.models.includes(form.value.reasoning_model)) {
        form.value.reasoning_model = res.models[0] ?? '';
      }
      discoverResult.value = { success: true, message: res.message };
    } else {
      discoverResult.value = { success: false, message: res.message || t('aiView.k122') };
    }
  } catch (e: unknown) {
    discoverResult.value = { success: false, message: (e as { message?: string }).message || t('aiView.k122') };
  } finally {
    discovering.value = false;
  }
}

async function fetchConfig() {
  loading.value = true;
  try {
    const res = await getAIConfig();
    enabled.value = res.enabled;
    if (res.provider) form.value.provider_type = res.provider;
    if (res.base_url) form.value.base_url = res.base_url;
    if (res.model) form.value.model = res.model;
    if (res.max_tokens) form.value.max_tokens = res.max_tokens;
    if (res.temperature != null) form.value.temperature = res.temperature;
    if (res.api_key && !res.api_key.includes('*')) form.value.api_key = res.api_key;
    if (res.fast_model) form.value.fast_model = res.fast_model;
    if (res.reasoning_model) form.value.reasoning_model = res.reasoning_model;
    const all = [...allProviders, ...compatiblePresets];
    const matched = all.find(
      p => p.value === form.value.provider_type && p.defaultBaseUrl === form.value.base_url
    ) ?? all.find(p => p.value === form.value.provider_type);
    if (matched) selectedPreset.value = matched;
    // 配置加载成功后自动发现一次上游模型，把模型下拉补全
    if (form.value.base_url) void discoverModels();
  } catch (e) {
    logger.error('Failed to fetch AI config:', e);
    ui.showToast(t('aiView.k112'), 'error');
  } finally {
    loading.value = false;
  }
}

async function handleSave() {
  saving.value = true;
  saveResult.value = null;
  try {
    await updateAIConfig({
      provider_type: form.value.provider_type,
      base_url: form.value.base_url,
      api_key: form.value.api_key || undefined,
      model: form.value.model,
      fast_model: form.value.fast_model,
      reasoning_model: form.value.reasoning_model,
      temperature: form.value.temperature,
      max_tokens: form.value.max_tokens,
    });
    saveResult.value = { success: true, message: t('aiView.k105') };
    ui.showToast(t('aiView.k123'), 'success');
    await fetchConfig();
  } catch (e: unknown) {
    saveResult.value = { success: false, message: (e as { message?: string }).message || t('fallback.k8') };
  } finally { saving.value = false; }
}

async function handleTest() {
  testing.value = true;
  testResult.value = null;
  try {
    const res = await testAIConnection();
    testResult.value = { success: res.success, message: res.message };
  } catch (e: unknown) {
    testResult.value = { success: false, message: (e as { message?: string }).message || t('fallback.k86') };
  } finally { testing.value = false; }
}

// ---- 知识分析器限额与自动调度 ----
// 上下界照抄后端 updateAnalyzerConfigRequest 的 binding 标签，
// 让超限在本地就报错，而不是发出去吃一个 400 再猜是哪个字段。
const ANALYZER_META = {
  batch_size: { label: t('aiView.k106'), min: 1, max: 50 },
  max_concurrent: { label: t('aiView.k107'), min: 1, max: 10 },
  rate_limit_rpm: { label: t('aiView.k108'), min: 1, max: 1000 },
  doc_max_chars: { label: t('aiView.k109'), min: 1000, max: 100000 },
  context_max_chars: { label: t('aiView.k110'), min: 4000, max: 200000 },
} as const;

type AnalyzerKey = keyof typeof ANALYZER_META;
const analyzerKeys = Object.keys(ANALYZER_META) as AnalyzerKey[];

const analyzerForm = ref<Record<AnalyzerKey, string>>({
  batch_size: '',
  max_concurrent: '',
  rate_limit_rpm: '',
  doc_max_chars: '',
  context_max_chars: '',
});
const analyzerErrors = ref<Partial<Record<AnalyzerKey, string>>>({});
const analyzerUnavailable = ref(false);
const savingAnalyzer = ref(false);

const scheduleForm = ref('');
const scheduleRunning = ref(false);
const savingSchedule = ref(false);

async function loadAnalyzer() {
  analyzerErrors.value = {};
  try {
    const cfg = await getAnalyzerConfig();
    analyzerUnavailable.value = false;
    for (const k of analyzerKeys) analyzerForm.value[k] = String(cfg?.[k] ?? '');
  } catch (e) {
    // AI 未启用时后端回 400「AI服务未启用」，那是状态不是故障，说清即可
    analyzerUnavailable.value = true;
    logger.warn('Analyzer config unavailable:', e);
  }
}

async function saveAnalyzer() {
  const errors: Partial<Record<AnalyzerKey, string>> = {};
  const payload = {} as AnalyzerConfig;
  for (const k of analyzerKeys) {
    const meta = ANALYZER_META[k];
    const raw = analyzerForm.value[k].trim();
    const n = Number(raw);
    if (!raw || !Number.isInteger(n)) errors[k] = '必须是整数';
    else if (n < meta.min || n > meta.max) errors[k] = `应在 ${meta.min}–${meta.max} 之间`;
    else payload[k] = n;
  }
  analyzerErrors.value = errors;
  if (Object.keys(errors).length) {
    ui.showToast(t('aiView.k113'), 'warning');
    return;
  }
  savingAnalyzer.value = true;
  try {
    const saved = await updateAnalyzerConfig(payload);
    for (const k of analyzerKeys) analyzerForm.value[k] = String(saved?.[k] ?? payload[k]);
    ui.showToast(t('aiView.k114'), 'success');
  } catch (e) {
    logger.error('Failed to update analyzer config:', e);
    ui.showToast(t('aiView.k115'), 'error');
  } finally {
    savingAnalyzer.value = false;
  }
}

async function loadSchedule() {
  try {
    const res = await getAISchedule();
    scheduleForm.value = res?.schedule ?? '';
    scheduleRunning.value = Boolean(res?.running);
  } catch (e) {
    logger.error('Failed to load AI schedule:', e);
    ui.showToast(t('aiView.k116'), 'error');
  }
}

async function saveSchedule() {
  const cron = scheduleForm.value.trim();
  if (!cron) {
    ui.showToast(t('aiView.k117'), 'warning');
    return;
  }
  savingSchedule.value = true;
  try {
    const res = await updateAISchedule(cron);
    scheduleForm.value = res?.schedule ?? cron;
    // 后端只回 cron 不回 running 或改后立即重载，否则界面停留在改之前的状态
    await loadSchedule();
    ui.showToast(t('aiView.k118'), 'success');
  } catch (e) {
    logger.error('Failed to update AI schedule:', e);
    ui.showToast(e instanceof Error && e.message ? e.message: t('aiView.k111'), 'error');
  } finally {
    savingSchedule.value = false;
  }
}

onMounted(() => {
  fetchConfig();
  loadAnalyzer();
  loadSchedule();
});

// 样式
const sectionTitleStyle = aiSubSectionTitleStyle;
const statusRowStyle = { ...baseStatusRowStyle, marginBottom: AppleSpace.xs };

const analyzerGridStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.stat})`,
  gap: AppleSpace.sm,
};

const scheduleRowStyle = {
  display: 'flex',
  alignItems: 'flex-end' as const,
  gap: AppleSpace.md,
  flexWrap: 'wrap' as const,
  marginTop: AppleSpace.sm,
};
const statusLabelStyle = (active: boolean) => ({
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.medium),
  color: active ? cv('green') : cv('text-secondary'),
});
const hintStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-secondary'),
  lineHeight: AppleLineHeight.loose,
};
const actionRowStyle = { ...aiSubActionRowStyle, gap: AppleSpace.sm };

// 滑块与它的当前值同一行：值固定在控件右端
const sliderRowStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.sm };
const sliderValueStyle = {
  fontFamily: AppleFont.mono, fontSize: AppleType.controlLabel.size, color: cv('text-secondary'),
  minWidth: '32px', textAlign: 'right' as const, fontVariantNumeric: 'tabular-nums',
};

// 左选右配布局
const splitLayoutStyle = computed(() => ({ display: 'grid', gridTemplateColumns: isMobile.value ? '1fr' : '260px 1fr', gap: AppleSpace.md }));

const providerListStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
  maxHeight: '560px',
  overflowY: 'auto' as const,
};

const groupStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
};

const groupTitleStyle = {
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-tertiary'),
  letterSpacing: AppleType.legalMeta.letterSpacing,
  padding: `${AppleSpace.xxs} ${AppleSpace.xs}`,
};

const providerItemStyle = (active: boolean) => ({
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.medium,
  border: `1px solid ${active ? cv('blue') : cv('border-light')}`,
  backgroundColor: active ? cv('blue-light') : cv('surface-field'),
  cursor: 'pointer',
  transition: AppleTransition.default,
});

const providerIconStyle = (active: boolean, color: string) => ({
  width: '28px',
  height: '28px',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  borderRadius: AppleRadius.small,
  backgroundColor: active ? cv('blue') : `${color}15`,
  flexShrink: 0,
  transition: AppleTransition.default,
});

const providerInfoStyle = {
  flex: '1',
  minWidth: 0,
};

const providerNameStyle = (active: boolean) => ({
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: active ? cv('blue') : cv('text-primary'),
});

const providerDescStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  marginTop: '1px',
};

// 右侧配置面板
const configPanelStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
  padding: AppleSpace.md,
  borderRadius: AppleRadius.card,
  border: `1px solid ${cv('border-light')}`,
  backgroundColor: cv('bg-secondary'),
};

const panelHeaderStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
  paddingBottom: AppleSpace.sm,
  borderBottom: `1px solid ${cv('border-light')}`,
};

const panelTitleStyle = {
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
};

const panelHintStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
};

const formGridStyle = computed(() => ({ display: 'grid', gridTemplateColumns: isMobile.value ? '1fr' : appleFixedColumns(2), gap: AppleSpace.sm }));
</script>
