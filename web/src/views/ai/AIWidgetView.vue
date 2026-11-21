<template>
  <div :style="aiSubPageStyle">
    <!-- Widget 预览 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Monitor" :size="16" :color="cv('blue')" />
          <span :style="sectionTitleStyle">{{ $t('aiSetNav.k110') }}</span>
        </div>
      </template>
      <div :style="previewWrapperStyle">
        <iframe
          v-if="widgetUrl"
          :src="widgetUrl"
          :style="iframeStyle"
          sandbox="allow-scripts"
          title="AI Widget Preview"
        />
        <ApEmpty v-else-if="!embedCode.js_embed && !embedCode.iframe_embed" icon="Code" :title="$t('aiSetNav.k122')" :description="$t('vAiUi.k80')" />
        <ApSkeleton v-else variant="card" :lines="4" />
      </div>
    </ApCard>

    <!-- 嵌入代码 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Code2" :size="16" :color="cv('orange')" />
          <span :style="sectionTitleStyle">{{ $t('aiSetNav.k111') }}</span>
        </div>
      </template>

      <div :style="codeSectionStyle">
        <div :style="codeBlockHeaderStyle">
          <span :style="codeBlockTitleStyle">{{ $t('aiSetNav.k112') }}</span>
          <ApButton variant="ghost" size="small" @click="copyCode(embedCode.js_embed, 'js')">
            <ApIcon name="Copy" :size="16" />
            {{ copiedKey === 'js' ? '已复制' : $t('vAiUi.copy') }}
          </ApButton>
        </div>
        <pre :style="codePreStyle">{{ embedCode.js_embed }}</pre>
      </div>

      <div :style="codeSectionStyle">
        <div :style="codeBlockHeaderStyle">
          <span :style="codeBlockTitleStyle">{{ $t('aiSetNav.k113') }}</span>
          <ApButton variant="ghost" size="small" @click="copyCode(embedCode.iframe_embed, 'iframe')">
            <ApIcon name="Copy" :size="16" />
            {{ copiedKey === 'iframe' ? '已复制' : $t('vAiUi.copy') }}
          </ApButton>
        </div>
        <pre :style="codePreStyle">{{ embedCode.iframe_embed }}</pre>
      </div>
    </ApCard>

    <!-- 使用说明 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Info" :size="16" :color="cv('purple')" />
          <span :style="sectionTitleStyle">{{ $t('aiSetNav.k114') }}</span>
        </div>
      </template>
      <div :style="PageLayout.sectionGap">
        <div :style="instructionItemStyle">
          <div :style="instructionStepStyle">1</div>
          <div>
            <div :style="instructionTitleStyle">{{ $t('aiSetNav.k115') }}</div>
            <div :style="instructionDescStyle">
              {{ $t('aiSetNav.k116') }} <code :style="codeInlineStyle">&lt;head&gt;</code> {{ $t('aiSetNav.k117') }}
            </div>
          </div>
        </div>
        <div :style="instructionItemStyle">
          <div :style="instructionStepStyle">2</div>
          <div>
            <div :style="instructionTitleStyle">{{ $t('aiSetNav.k118') }}</div>
            <div :style="instructionDescStyle">
              {{ $t('aiSetNav.k119') }}
            </div>
          </div>
        </div>
        <div :style="instructionItemStyle">
          <div :style="instructionStepStyle">3</div>
          <div>
            <div :style="instructionTitleStyle">Widget URL</div>
            <div :style="instructionDescStyle">
              {{ $t('aiSetNav.k120') }} <code :style="codeInlineStyle">{{ widgetUrl }}</code> {{ $t('aiSetNav.k121') }}
            </div>
          </div>
        </div>
      </div>
    </ApCard>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, onMounted } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';

import { cv, AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleType, AppleLineHeight } from '@/utils/core/apple-tokens';
import { aiSubPageStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle as sectionTitleStyle, aiSubSectionTitleStyle as codeBlockTitleStyle, PageLayout } from '@/styles/ai/ai-page-styles';
import { logger } from '@/utils/logger';
import { copyText } from '@/utils/clipboard';
import { getEmbedCode, getWidgetUrl, type EmbedCodeResponse } from '@/api/modules/ai/ai-widget';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();

const widgetUrl = ref('');
const embedCode = ref<EmbedCodeResponse>({
  js_embed: '',
  iframe_embed: '',
  base_url: '',
});
const copiedKey = ref('');

onMounted(async () => {
  widgetUrl.value = getWidgetUrl();
  try {
    embedCode.value = await getEmbedCode();
  } catch (e) {
    logger.error('Failed to load embed code:', e);
    ui.showToast(t('aiSetNav.k123'), 'error');
  }
});

async function copyCode(text: string, key: string) {
  const ok = await copyText(text);
  if (!ok) {
    logger.error('Failed to copy to clipboard');
    ui.showToast(t('aiSetNav.k125'), 'error');
    return;
  }
  copiedKey.value = key;
  ui.showToast(t('aiSetNav.k124'), 'success');
  setTimeout(() => { copiedKey.value = ''; }, 2000);
}

const previewWrapperStyle = {
  borderRadius: AppleRadius.medium,
  overflow: 'hidden',
  border: `1px solid ${cv('border-light')}`,
  backgroundColor: cv('surface-panel'),
};

const iframeStyle = {
  width: '100%',
  height: AppleSpace['400px'],
  border: 'none',
  display: 'block',
};

const codeSectionStyle = {
  marginBottom: AppleSpace.md,
};

const codeBlockHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  marginBottom: AppleSpace.sm,
};

const codePreStyle = {
  fontFamily: AppleFont.mono,
  fontSize: AppleType.codeBlock.size,
  lineHeight: AppleLineHeight.loose,
  padding: AppleSpace.md,
  backgroundColor: cv('bg-tertiary'),
  borderRadius: AppleRadius.small,
  color: cv('text-primary'),
  whiteSpace: 'pre-wrap' as const,
  wordBreak: 'break-all' as const,
  overflow: 'auto',
  maxHeight: AppleSpace['200px'],
  margin: 0,
};

const instructionItemStyle = {
  display: 'flex',
  gap: AppleSpace.md,
};

const instructionStepStyle = {
  width: '28px',
  height: '28px',
  borderRadius: AppleRadius.circle,
  backgroundColor: cv('blue'),
  color: cv('on-blue'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.semibold),
  flexShrink: 0,
};

const instructionTitleStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  marginBottom: AppleSpace.xxs,
};

const instructionDescStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-secondary'),
  lineHeight: AppleLineHeight.loose,
};

const codeInlineStyle = {
  fontFamily: AppleFont.mono,
  fontSize: AppleType.legalMeta.size,
  backgroundColor: cv('bg-tertiary'),
  padding: `1px ${AppleSpace.xs}`,
  borderRadius: AppleRadius.micro,
};
</script>
