<template>
  <div :style="aiSubPageStyle">
      <!-- 输入区域 -->
      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="Tag" :size="18" :color="cv('blue')" />
            <span :style="sectionTitleStyle">{{ $t('aiView.k56') }}</span>
          </div>
        </template>
        <div :style="formStyle">
          <ApTextarea
            v-model="content"
            :aria-label="$t('aiView.k68')"
            :placeholder="$t('aiView.k66')"
            :rows="8"
            @keydown.ctrl.enter="handleAnalyze"
          />
          <div :style="hintStyle">
            {{ $t('aiView.k57') }} <kbd :style="kbdStyle">Ctrl</kbd> + <kbd :style="kbdStyle">Enter</kbd> {{ $t('aiView.k58') }}
          </div>
          <div :style="aiSubActionRowStyle">
            <ApButton variant="primary" size="small" :loading="analyzing" :disabled="!content.trim()" @click="handleAnalyze">
              <ApIcon name="Zap" :size="16" /> {{ $t('aiView.k59') }}
            </ApButton>
            <ApButton v-if="content" variant="ghost" size="small" @click="clearAll">
              <ApIcon name="X" :size="16" /> {{ $t('aiView.k60') }}
            </ApButton>
          </div>
        </div>
      </ApCard>

      <!-- 结果区域 -->
      <ApCard v-if="result || analyzing" size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="CheckCircle" :size="18" :color="cv('green')" />
            <span :style="sectionTitleStyle">{{ $t('aiView.k11') }}</span>
          </div>
        </template>

        <!-- 加载状态 -->
        <ApSkeleton v-if="analyzing" variant="card" :lines="3" />

        <!-- 结果内容 -->
          <div v-else-if="result" :style="PageLayout.sectionGap">
          <!-- 标签列表 -->
          <div :style="resultSectionStyle">
            <div :style="resultLabelStyle">{{ $t('aiView.k61') }}</div>
            <div :style="tagListStyle">
              <span v-for="(tag, index) in result.tags" :key="index" :style="tagStyle">
                {{ tag }}
              </span>
              <span v-if="result.tags.length === 0" :style="emptyTextStyle">{{ $t('aiView.k62') }}</span>
            </div>
          </div>

          <!-- 分类 -->
          <div v-if="result.category" :style="resultSectionStyle">
            <div :style="resultLabelStyle">{{ $t('aiView.k63') }}</div>
            <div :style="categoryStyle">{{ result.category }}</div>
          </div>

          <!-- 置信度 -->
          <div v-if="result.confidence != null" :style="resultSectionStyle">
            <div :style="resultLabelStyle">{{ $t('aiView.k64') }}</div>
            <div :style="aiSubActionRowStyle">
              <ApProgress
                :value="result.confidence * 100"
                :label="$t('aiView.k65')"
                :tone="toneForScore(result.confidence)"
                size="bar"
                :style="{ flex: 1 }"
              />
              <span :style="confidenceTextStyle">{{ Math.round(result.confidence * 100) }}%</span>
            </div>
          </div>
        </div>
      </ApCard>

      <!-- 空状态 -->
      <ApEmpty
        v-if="!result && !analyzing"
        icon="Tag"
        :title="$t('aiView.k67')"
        :description="$t('vAiUi.k6')"
      />
    </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref } from 'vue'
import ApSkeleton from '@/components/apple/ApSkeleton.vue'
import ApIcon from '@/components/apple/ApIcon.vue';




import { cv, AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleType } from '@/utils/core/apple-tokens'
import { toneForScore } from '@/utils/core/progress-tone'
import { aiSubPageStyle, aiSubActionRowStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle, PageLayout } from '@/styles/ai/ai-page-styles'
import { useUIStore } from '@/stores/app/ui'
import { autoTag, type AutoTagResult } from '@/api/modules/ai/ai-auto-tag'

const ui = useUIStore()

const content = ref('')
const analyzing = ref(false)
// P1-22: 响应嵌套一层 { tags: {...} }，取内层对象后模板结构保持原样
const result = ref<AutoTagResult | null>(null)

async function handleAnalyze() {
  if (!content.value.trim()) return

  analyzing.value = true
  result.value = null

  try {
    const res = await autoTag({ content: content.value.trim() })
    result.value = res.tags || null
    ui.showToast(t('aiView.k70'), 'success')
  } catch (e: unknown) {
    const message = e instanceof Error ? e.message: t('aiView.k69')
    ui.showToast(message, 'error')
  } finally {
    analyzing.value = false
  }
}

function clearAll() {
  content.value = ''
  result.value = null
}

// 样式定义
const sectionTitleStyle = aiSubSectionTitleStyle;

const formStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
}

const hintStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
}

const kbdStyle = {
  padding: `${AppleSpace.xxs} ${AppleSpace.xs}`,
  borderRadius: AppleRadius.micro,
  background: cv('bg-tertiary'),
  border: `1px solid ${cv('border-light')}`,
  fontSize: AppleType.microUI.size,
  fontFamily: AppleFont.mono,
}

const resultSectionStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
}

const resultLabelStyle = {
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-secondary'),
  letterSpacing: AppleType.legalMeta.letterSpacing,
}

const tagListStyle = {
  display: 'flex',
  flexWrap: 'wrap' as const,
  gap: AppleSpace.sm,
}

const tagStyle = {
  display: 'inline-flex',
  alignItems: 'center',
  padding: `${AppleSpace.xxs} ${AppleSpace.md}`,
  borderRadius: AppleRadius.pill,
  background: cv('blue-light'),
  color: cv('blue'),
  // 标签文字走小字档：原先是正文 17px，一颗药丸里压着正文级字号，与全站 12px 标签同排就是两个尺度
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
}

const emptyTextStyle = {
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-tertiary'),
}

const categoryStyle = {
  display: 'inline-flex',
  alignItems: 'center',
  padding: `${AppleSpace.xs} ${AppleSpace.md}`,
  borderRadius: AppleRadius.small,
  background: cv('bg-tertiary'),
  color: cv('text-primary'),
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.medium),
  alignSelf: 'flex-start',
}

const confidenceTextStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  minWidth: '50px',
}
</script>
