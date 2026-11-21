<template>
  <div :style="aiSubPageStyle">
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="PenLine" :size="18" :color="cv('green')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('vAiUi.k185') }}</span>
        </div>
      </template>

      <div :style="formStyle">
        <div :style="fieldStyle">
          <div :style="labelStyle">{{ $t('vAiUi.k186') }}</div>
          <ApInput v-model="topic" :style="inputStyle" :aria-label="$t('vAiUi.k197')" :placeholder="$t('vAiUi.k194')" />
        </div>
        <div :style="fieldStyle">
          <div :style="labelStyle">{{ $t('vAiUi.k187') }}</div>
          <ApInput v-model="docIds" :style="inputStyle" :aria-label="$t('vAiUi.k166')" :placeholder="$t('vAiUi.k195')" />
          <div :style="hintStyle">{{ $t('vAiUi.k188') }}</div>
        </div>
        <div :style="aiSubActionRowStyle">
          <ApButton
            variant="primary"
            size="small"
            :loading="generating"
            :disabled="!topic.trim()"
            @click="handleGenerate"
          >
            <ApIcon name="PenLine" :size="16" /> {{ $t('vAiUi.k189') }}
          </ApButton>
          <ApButton v-if="result" variant="ghost" size="small" @click="clearAll">
            <ApIcon name="X" :size="16" /> {{ $t('vAiUi.k190') }}
          </ApButton>
        </div>
      </div>
    </ApCard>

    <ApSkeleton v-if="generating" variant="card" :lines="5" />

    <template v-else-if="result">
      <!-- 引用校验：问题必须显式暴露，不能静默放过 -->
      <ApCard v-if="result.citation_issues.length > 0" size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="AlertTriangle" :size="18" :color="cv('red')" />
            <span :style="aiSubSectionTitleStyle">引用问题（{{ result.citation_issues.length }}）</span>
          </div>
        </template>
        <div :style="listStyle">
          <div v-for="(it, i) in result.citation_issues" :key="i" :style="issueStyle">
            <strong>{{ it.section }}</strong>
            <span :style="issueKindStyle">{{ it.kind === 'missing' ? $t('vAiUi.k199') : $t('vAiUi.k200') }}</span>
            <div :style="rowSummaryStyle">{{ it.detail }}</div>
          </div>
        </div>
      </ApCard>

      <ApCard v-else size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="CheckCircle" :size="18" :color="cv('green')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('vAiUi.k191') }}</span>
          </div>
        </template>
        <div :style="rowSummaryStyle">{{ $t('vAiUi.k192') }}</div>
      </ApCard>

      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="ListTree" :size="18" :color="cv('indigo')" />
            <span :style="aiSubSectionTitleStyle">大纲（{{ result.sections.length }} 节）</span>
          </div>
        </template>
        <div :style="outlineStyle">
          <div v-for="(s, i) in result.sections" :key="i" :style="outlineItemStyle">
            <div :style="outlineTitleStyle">{{ i + 1 }}. {{ s.title }}</div>
            <ul :style="outlinePointsStyle">
              <li v-for="(p, j) in s.key_points" :key="j">{{ p }}</li>
            </ul>
          </div>
        </div>
      </ApCard>

      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="FileText" :size="18" :color="cv('blue')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('vAiUi.k193') }}</span>
          </div>
        </template>
        <ApMarkdownPreview :content="result.markdown" />
      </ApCard>
    </template>

    <ApEmpty
      v-else
      icon="FileText"
      :title="$t('vAiUi.k196')"
      :description="$t('vAiUi.k198')"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref } from 'vue'
import ApSkeleton from '@/components/apple/ApSkeleton.vue'
import ApMarkdownPreview from '@/components/apple/ApMarkdownPreview.vue'

import { cv, AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleType, AppleTransition } from '@/utils/core/apple-tokens'
import {
  aiSubPageStyle,
  aiSubActionRowStyle,
  aiSubSectionHeaderStyle,
  aiSubSectionTitleStyle,
} from '@/styles/ai/ai-page-styles'
import { useUIStore } from '@/stores/app/ui'
import { generateArticle, type ArticleResult } from '@/api/modules/ai/ai-wiki'

const ui = useUIStore()

const topic = ref('')
const docIds = ref('')
const generating = ref(false)
const result = ref<ArticleResult | null>(null)

const { t } = useI18n();
async function handleGenerate() {
  if (!topic.value.trim()) return
  generating.value = true
  result.value = null
  try {
    const ids = docIds.value
      .split(/[,，\s]+/)
      .map((s) => s.trim())
      .filter(Boolean)
      .map(Number)
      .filter((n) => Number.isFinite(n) && n > 0)
    result.value = await generateArticle({ topic: topic.value.trim(), doc_ids: ids })
    ui.showToast(t('vAiUi.k202'), 'success')
  } catch (e: unknown) {
    ui.showToast(e instanceof Error ? e.message: t('vAiUi.k201'), 'error')
  } finally {
    generating.value = false
  }
}

function clearAll() {
  result.value = null
}

const formStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
}

const fieldStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
}

const labelStyle = {
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-secondary'),
}

const inputStyle = {
  width: '100%',
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.small,
  border: `1px solid ${cv('border-light')}`,
  background: cv('surface-panel'),
  color: cv('text-primary'),
  fontSize: AppleType.bodyPrimary.size,
  fontFamily: AppleFont.text,
  transition: AppleTransition.default,
}

const hintStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
}

const rowSummaryStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
}

const listStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
}

const issueStyle = {
  padding: AppleSpace.sm,
  borderRadius: AppleRadius.small,
  border: `1px solid ${cv('border-light')}`,
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-primary'),
}

const issueKindStyle = {
  marginLeft: AppleSpace.xs,
  padding: `1px ${AppleSpace.xs}`,
  borderRadius: AppleRadius.pill,
  background: cv('red-light'),
  color: cv('red'),
  fontSize: AppleType.microUI.size,
}

const outlineStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
}

const outlineItemStyle = {
  padding: AppleSpace.sm,
  borderRadius: AppleRadius.small,
  border: `1px solid ${cv('border-light')}`,
}

const outlineTitleStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
  marginBottom: AppleSpace.xxs,
}

const outlinePointsStyle = {
  margin: 0,
  paddingLeft: AppleSpace.lg,
  color: cv('text-secondary'),
  fontSize: AppleType.legalMeta.size,
}
</script>
