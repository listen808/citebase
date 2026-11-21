<template>
  <div :style="aiSubPageStyle">
  <!-- 写作辅助 -->
  <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderGapStyle">
          <ApIcon name="PenTool" :size="16" :color="cv('blue')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('vAiUi.k108') }}</span>
        </div>
      </template>

      <div :style="compactFormStyle">
        <ApSelect
          v-model="action"
          :label="$t('vAiUi.k112')"
          :options="actionOptions"
        />
        <ApInput
          v-model="instruction"
          :label="$t('vAiUi.k113')"
          :placeholder="$t('vAiUi.k118')"
        />
        <ApFormItem :label="$t('vAiUi.k114')">
          <ApTextarea v-model="text" :aria-label="$t('vAiUi.k121')" :placeholder="$t('vAiUi.k119')" :rows="4" />
        </ApFormItem>
        <ApButton variant="primary" size="small" :loading="processing" @click="handleWrite">
          <ApIcon name="Wand" :size="16" /> {{ $t('vAiUi.k86') }}
        </ApButton>
      </div>

      <div v-if="result" :style="compactResultCardStyle">
        <div :style="resultHeaderStyle">
          <div :style="resultTitleStyle">{{ $t('vAiUi.k109') }}</div>
          <ApButton variant="ghost" size="small" @click="copyResult">{{ $t('vAiUi.copy') }}</ApButton>
        </div>
        <div :style="resultTextStyle">{{ result }}</div>
      </div>
    </ApCard>

    <!-- 翻译工具 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderGapStyle">
          <ApIcon name="Languages" :size="16" :color="cv('green')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('vAiUi.k110') }}</span>
        </div>
      </template>
      <div :style="compactFormStyle">
        <div :style="compactFormRowStyle">
          <ApSelect
            v-model="targetLang"
            :label="$t('vAiUi.k115')"
            :options="langOptions"
            :style="{ flex: 1 }"
          />
          <ApSwitch v-model="preserveFormat" :label="$t('vAiUi.k116')" />
        </div>
        <ApFormItem :label="$t('vAiUi.k117')">
          <ApTextarea v-model="translateTextValue" :aria-label="$t('vAiUi.k117')" :placeholder="$t('vAiUi.k120')" :rows="3" />
        </ApFormItem>
        <ApButton variant="primary" size="small" :loading="translating" @click="handleTranslate">
          <ApIcon name="Languages" :size="16" /> {{ $t('vAiUi.k111') }}
        </ApButton>
      </div>
      <div v-if="translateResult" :style="compactResultCardStyle">
        <div :style="resultTitleStyle">翻译结果 ({{ translateResult.target_language }})</div>
        <div :style="resultTextStyle">{{ translateResult.translated_text }}</div>
      </div>
    </ApCard>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { logger } from '@/utils/logger';
import { copyText } from '@/utils/clipboard';
import { ref, computed } from 'vue';

import { cv, AppleFont, AppleFontWeight, AppleLineHeight, AppleSpace, AppleType } from '@/utils/core/apple-tokens';
import { useBreakpoint } from '@/utils/responsive';
import { aiSubPageStyle, aiSubSectionHeaderGapStyle, aiSubSectionTitleStyle, aiResultCardStyle as resultCardStyle, formStyle } from '@/styles/ai/ai-page-styles';
import { writeAssist, translateText } from '@/api/modules/ai/ai-write';
import type { AIWriteAction } from '@/types/ai';
import { useUIStore } from '@/stores/app/ui';

const action = ref<AIWriteAction>('rewrite');
const instruction = ref('');
const text = ref('');
const processing = ref(false);
const result = ref('');
const ui = useUIStore();
const { isMobile } = useBreakpoint();

const targetLang = ref('en');
const preserveFormat = ref(true);
const translateTextValue = ref('');
const translating = ref(false);
const translateResult = ref<{ translated_text: string; source_language: string; target_language: string } | null>(null);

const { t } = useI18n();
const actionOptions = [
  { label: t('vAiUi.k122'), value: 'rewrite' },
  { label: t('vAiUi.k123'), value: 'continue' },
  { label: t('vAiUi.k124'), value: 'summarize' },
  { label: t('vAiUi.k125'), value: 'explain' },
  { label: t('vAiUi.k126'), value: 'shorten' },
  { label: t('vAiUi.k127'), value: 'expand' },
  { label: t('vAiUi.k128'), value: 'formalize' },
  { label: t('vAiUi.k129'), value: 'simplify' },
  { label: t('vAiUi.k130'), value: 'bulletize' },
  { label: t('vAiUi.k131'), value: 'check_quality' },
  { label: t('vAiUi.k132'), value: 'add_comments' },
  { label: t('vAiUi.k133'), value: 'fix_grammar' },
];

const langOptions = [
  { label: t('vAiUi.k134'), value: 'en' },
  { label: t('vAiUi.k135'), value: 'zh' },
  { label: t('vAiUi.k136'), value: 'ja' },
  { label: t('vAiUi.k137'), value: 'ko' },
  { label: t('vAiUi.k138'), value: 'fr' },
  { label: t('vAiUi.k139'), value: 'de' },
];

async function handleWrite() {
  if (!text.value.trim()) return;
  processing.value = true;
  result.value = '';
  try {
    const res = await writeAssist({
      text: text.value,
      action: action.value,
      instruction: instruction.value || undefined,
    });
    result.value = res.result;
  } catch (e) {
    logger.error('Failed to write assist:', e);
    ui.showToast(t('vAiUi.k140'), 'error');
  }
  processing.value = false;
}

async function handleTranslate() {
  if (!translateTextValue.value.trim()) return;
  translating.value = true;
  translateResult.value = null;
  try {
    const res = await translateText({
      text: translateTextValue.value,
      target_language: targetLang.value,
      preserve_format: preserveFormat.value,
    });
    translateResult.value = res;
  } catch (e) {
    logger.error('Failed to translate:', e);
    ui.showToast(t('vAiUi.k141'), 'error');
  }
  translating.value = false;
}

async function copyResult() {
  await copyText(result.value);
}

// ===== 样式 =====
const formRowStyle = computed(() => ({ display: 'flex', gap: AppleSpace.md, alignItems: 'flex-end' as const, flexWrap: isMobile.value ? 'wrap' as const : 'nowrap' as const }));

const compactFormStyle = { ...formStyle, gap: AppleSpace.sm };
const compactFormRowStyle = computed(() => ({ ...formRowStyle.value, gap: AppleSpace.sm }));
const compactResultCardStyle = { ...resultCardStyle, padding: AppleSpace.sm, marginTop: AppleSpace.sm };

const resultHeaderStyle = {
  display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: AppleSpace.sm,
};

const resultTitleStyle = {
  fontFamily: AppleFont.text, fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary'),
};

const resultTextStyle = {
  fontFamily: AppleFont.text, fontSize: AppleType.bodyPrimary.size,
  color: cv('text-primary'), lineHeight: AppleLineHeight.spacious, whiteSpace: 'pre-wrap' as const,
};
</script>
