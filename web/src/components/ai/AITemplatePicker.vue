<template>
  <div :style="pickerStyle">
    <div v-if="!selectedTemplate" :style="templateGridStyle">
      <div
        v-for="template in templates"
        :key="template.id"
        :style="templateCardStyle"
        @click="selectTemplate(template)"
      >
        <div :style="templateIconStyle">
          <ApIcon :name="(template.icon || 'FileText') as IconName" :size="24" />
        </div>
        <div :style="templateNameStyle">{{ template.name }}</div>
        <div :style="templateDescStyle">{{ template.description }}</div>
      </div>
    </div>

    <div v-else :style="formStyle">
      <ApButton variant="ghost" size="small" @click="selectedTemplate = null">
        <ApIcon name="ChevronRight" :size="16" />
        {{ $t('aiUi.back') }}
      </ApButton>

      <div :style="formTitleStyle">{{ selectedTemplate.name }}</div>

      <div v-for="varName in selectedTemplate.variables" :key="varName" :style="fieldStyle">
        <label :style="labelStyle">{{ formatVarName(varName) }}</label>
        <ApInput
          :model-value="variableValues[varName] || ''"
          :placeholder="('请输入 ' + formatVarName(varName)) as string"
          @update:model-value="(value) => { variableValues[varName] = value; }"
        />
      </div>

      <div :style="actionsStyle">
        <ApButton variant="secondary" size="small" @click="selectedTemplate = null">
          {{ $t('aiUi.cancel') }}
        </ApButton>
        <ApButton
          variant="primary"
          size="small"
          :loading="loading"
          @click="handleGenerate"
        >
          {{ $t('aiUi.k3') }}
        </ApButton>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue';
import { appleFixedColumns, AppleSpace, AppleRadius, AppleTransition, cv, AppleType, AppleFontWeight } from '@/utils/core/apple-tokens';
import { scrollSingleAxis } from '@/styles/layout/page-styles';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApInput from '@/components/apple/ApInput.vue';
import type { AITemplate } from '@/types/ai-writer';
import type { IconName } from '@/utils/icons';

defineProps<{
  templates: AITemplate[];
}>();

const emit = defineEmits<{
  generate: [templateId: string, variables: Record<string, string>];
}>();

const selectedTemplate = ref<AITemplate | null>(null);
const variableValues = reactive<Record<string, string>>({});
const loading = ref(false);

function selectTemplate(template: AITemplate) {
  selectedTemplate.value = template;
  // 初始化所有变量为空字符串
  Object.keys(variableValues).forEach(key => delete variableValues[key]);
  template.variables.forEach(varName => {
    variableValues[varName] = '';
  });
}

function formatVarName(name: string) {
  return name.replace(/([A-Z])/g, ' $1').replace(/^./, str => str.toUpperCase());
}

async function handleGenerate() {
  if (!selectedTemplate.value) return;

  loading.value = true;
  try {
    emit('generate', selectedTemplate.value.id, { ...variableValues });
    selectedTemplate.value = null;
    Object.keys(variableValues).forEach(key => delete variableValues[key]);
  } finally {
    loading.value = false;
  }
}

const pickerStyle = {
  flex: 1,
  minHeight: 0,
  display: 'flex',
  flexDirection: 'column' as const,
};

const templateGridStyle = {
  display: 'grid',
  gridTemplateColumns: appleFixedColumns(2),
  gap: AppleSpace.md,
  ...scrollSingleAxis,
};

const templateCardStyle = {
  padding: AppleSpace.md,
  border: `1px solid ${cv('border-light')}`,
  borderRadius: AppleRadius.control,
  cursor: 'pointer',
  transition: AppleTransition.smooth,
  background: cv('surface-panel'),
  textAlign: 'center' as const,
};

const templateIconStyle = {
  marginBottom: AppleSpace.sm,
  color: cv('blue'),
};

const templateNameStyle = {
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  marginBottom: AppleSpace.xs,
};

const templateDescStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
};

const formStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
};



const formTitleStyle = {
  fontSize: AppleType.sectionTitle.size,
  letterSpacing: AppleType.sectionTitle.letterSpacing,
  fontWeight: String(AppleFontWeight.semibold),
  marginBottom: AppleSpace.lg,
};

const fieldStyle = {
  marginBottom: AppleSpace.md,
};

const labelStyle = {
  display: 'block',
  marginBottom: AppleSpace.xs,
  fontSize: AppleType.microUIBold.size,
  color: cv('text-secondary'),
};

const actionsStyle = {
  marginTop: 'auto',
  display: 'flex',
  gap: AppleSpace.sm,
  justifyContent: 'flex-end',
};
</script>
