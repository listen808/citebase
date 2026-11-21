<template>
  <ApModal v-model="showModal" :title="$t('spaceUi.k10')" size="large">
    <div :style="importTypeStyle">
      <ApSegmentedControl
        :model-value="importType"
        :options="importTypeOptions"
        size="small"
        :aria-label="$t('spaceUi.k11')"
        @update:model-value="(v: string | number) => importType = String(v) as ImportType"
      />
    </div>
    <div :style="uploadStyle">
      <input v-if="importType === 'md'" ref="fileInput" type="file" :aria-label="$t('spaceUi.k12')" accept=".md" multiple :style="{ display: 'none' }" @change="onFilesSelected">
      <input v-if="importType === 'confluence'" ref="fileInput" type="file" :aria-label="$t('spaceUi.k13')" accept=".html,.htm" multiple :style="{ display: 'none' }" @change="onFilesSelected">
      <input v-if="importType === 'notion'" ref="fileInput" type="file" :aria-label="$t('spaceUi.k14')" accept=".csv" :style="{ display: 'none' }" @change="onFilesSelected">
      <ApButton variant="secondary" size="small" @click="fileInput?.click()">
        {{ importType === 'md' ? '选择 .md 文件' : importType === 'confluence' ? $t('spaceUi.k15') : $t('spaceUi.k16') }}
      </ApButton>
      <span :style="hintStyle">{{ importType === 'md' ? '支持多选，按文件路径生成目录结构' : importType === 'confluence' ? $t('spaceUi.k17') : $t('spaceUi.k18') }}</span>
    </div>
    <div v-if="files.length > 0" :style="listStyle">
      <div v-for="(f, idx) in files" :key="idx" :style="rowStyle">
        <span :style="pathStyle">{{ f.path }}</span>
        <span v-if="f.error" :style="errorStyle">{{ f.error }}</span>
      </div>
    </div>
    <template #footer>
      <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('spaceUi.cancel') }}</ApButton>
      <ApButton variant="primary" size="small" :loading="importing" @click="onImport">{{ $t('spaceUi.import') }}</ApButton>
    </template>
  </ApModal>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed, ref, watch } from 'vue';
import { AppleFont, AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';
import { importDocuments } from '@/api/modules/document/document';
import { useUIStore } from '@/stores/app/ui';
import ApModal from '@/components/apple/ApModal.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApButton from '@/components/apple/ApButton.vue';

const props = defineProps<{ modelValue: boolean; spaceId: number }>();
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void; (e: 'done'): void }>();

const ui = useUIStore();
const fileInput = ref<HTMLInputElement | null>(null);
const files = ref<{ path: string; content: string; error?: string }[]>([]);
const importing = ref(false);
type ImportType = 'md' | 'confluence' | 'notion';
const importType = ref<ImportType>('md');
const importTypeOptions: { label: string; value: ImportType }[] = [
  { label: 'Markdown (.md)', value: 'md' },
  { label: 'Confluence HTML', value: 'confluence' },
  { label: 'Notion CSV', value: 'notion' },
];

watch(importType, () => {
  reset();
});
const { t } = useI18n();

const showModal = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
});

// ApModal 只 emit update:modelValue，原来挂在 @close 上的 reset 从来没跑过，
// 重开弹窗会留着上一次的待导入清单。
watch(showModal, (open) => {
  if (!open) reset();
});

function reset() {
  files.value = [];
  if (fileInput.value) fileInput.value.value = '';
}

async function onFilesSelected(e: Event) {
  const target = e.target as HTMLInputElement;
  const selected = target.files;
  if (!selected) return;

  const extMap: Record<string, string> = { md: '.md', confluence: '.html', notion: '.csv' };
  const ext = extMap[importType.value] || '';

  const items: { path: string; content: string; error?: string }[] = [];
  for (const file of selected) {
    const name = file.name.toLowerCase();
    const matchExt = importType.value === 'confluence' ? (name.endsWith('.html') || name.endsWith('.htm')) : name.endsWith(ext);
    if (!matchExt) continue;
    try {
      const content = await file.text();
      items.push({
        path: file.webkitRelativePath || file.name,
        content,
      });
    } catch {
      items.push({ path: file.name, content: '', error: '读取失败' });
    }
  }
  files.value = items;
}

async function onImport() {
  const valid = files.value.filter(f => !f.error && f.content);
  if (valid.length === 0) {
    ui.showToast(t('spaceUi.k19'), 'warning');
    return;
  }
  importing.value = true;
  try {
    const result = await importDocuments(props.spaceId, valid.map(v => ({
      path: v.path,
      content: v.content,
      format: importType.value,
    })));
    ui.showToast(t('sentUi.k69', { p1: result.created, p2: result.skipped }), result.errors.length ? 'warning' : 'success');
    emit('done');
    showModal.value = false;
    reset();
  } catch {
    ui.showToast(t('spaceUi.k20'), 'error');
  } finally {
    importing.value = false;
  }
}

const importTypeStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
  marginBottom: AppleSpace.md,
};
const uploadStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.md,
  marginBottom: AppleSpace.lg,
};

const hintStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-secondary'),
};

const listStyle = {
  maxHeight: '50vh',
  overflowY: 'auto' as const,
  borderTop: `1px solid ${cv('border-light')}`,
};

const rowStyle = {
  padding: `${AppleSpace.sm} 0`,
  borderBottom: `1px solid ${cv('border-light')}`,
  fontFamily: AppleFont.text,
  fontSize: AppleType.bodyPrimary.size,
};

const pathStyle = {
  color: cv('text-primary'),
};

const errorStyle = {
  color: cv('red'),
  marginLeft: AppleSpace.sm,
};
</script>
