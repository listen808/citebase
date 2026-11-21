<template>
  <ApModal
    :model-value="modelValue"
    :title="title"
    size="regular"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <div :style="modalBodyStyle">
      <div v-if="loading" :style="loadingStyle">{{ $t('docUi1.k13') }}</div>
      <div v-else>
        <div :style="currentLocationStyle">
          <span :style="labelStyle">{{ $t('docUi1.k82') }}</span>
          <span :style="pathStyle">{{ currentPath }}</span>
        </div>

        <div :style="treeContainerStyle">
          <div
            v-for="node in flatTree"
            :key="node.id"
            :style="getNodeStyle(node)"
            @click="selectTarget(node)"
          >
            <span :style="indentStyle(node)"></span>
            <ApButton
              v-if="node.type === 2 && node.has_children"
              variant="ghost"
              size="mini"
              icon-only
              :aria-label="$t('docUi1.k85')"
              @click.stop="toggleExpand(node)"
            >
              <ApIcon
                name="ChevronRight"
                :size="16"
                :style="{ transform: expandedNodes.has(node.id) ? 'rotate(90deg)' : 'rotate(0deg)', transition: AppleTransition.fast }"
              />
            </ApButton>
            <span v-else :style="placeholderStyle"></span>
            <ApIcon v-if="node.type === 2" name="Folder" :size="16" :style="folderIconStyle" />
            <ApIcon v-else name="FileText" :size="16" :style="fileIconStyle" />
            <span :style="nameStyle(node)">{{ node.name }}</span>
            <ApIcon v-if="selectedTarget?.id === node.id" name="Check" :size="14" :style="checkStyle" />
          </div>

          <div v-if="!tree.length" :style="emptyStyle">
            {{ $t('docUi1.k83') }}
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <ApButton variant="secondary" size="small" @click="$emit('update:modelValue', false)">
        {{ $t('docUi1.cancel') }}
      </ApButton>
      <ApButton
        variant="primary"
        :disabled="!selectedTarget"
        :loading="confirming"
        @click="handleConfirm"
      >
        {{ $t('docUi1.confirm2') }}
      </ApButton>
    </template>
  </ApModal>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, watch } from 'vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import { cv, AppleFont, AppleFontWeight, AppleSpace, AppleRadius, AppleTransition, AppleType } from '@/utils/core/apple-tokens';
import { loadingStyle, emptyStateStyle as emptyStyle } from '@/styles/layout/page-styles';
import type { Document } from '@/types/models';
import { getRootDocuments, getDocumentChildren } from '@/api/modules/document/document';
import { useUIStore } from '@/stores/app/ui';

const props = defineProps<{
  modelValue: boolean;
  title: string;
  spaceId: number;
  currentDocId?: number;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void;
  (e: 'confirm', targetId: number): void;
}>();
const { t } = useI18n();

const ui = useUIStore();
const loading = shallowRef(false);
const confirming = shallowRef(false);
const tree = ref<Document[]>([]);
const selectedTarget = ref<Document | null>(null);
const expandedNodes = ref<Set<number>>(new Set());
const loadedNodes = ref<Set<number>>(new Set());

interface TreeNode extends Document {
  level: number;
  has_children?: boolean;
}

async function loadRoot() {
  loading.value = true;
  try {
    const data = await getRootDocuments(props.spaceId);
    tree.value = (data ?? [])
      .filter(doc => doc.type === 2 && doc.id !== props.currentDocId)
      .map(doc => ({
        ...doc,
        has_children: doc.has_children ?? false,
        children: [],
      }));
    loadedNodes.value = new Set([0]);
  } catch {
    tree.value = [];
    ui.showToast(t('docUi1.k86'), 'error');
  } finally {
    loading.value = false;
  }
}

async function loadChildren(node: Document) {
  if (loadedNodes.value.has(node.id)) return;

  try {
    const data = await getDocumentChildren(node.id);
    const children = (data ?? [])
      .filter(doc => doc.type === 2 && doc.id !== props.currentDocId)
      .map(doc => ({
        ...doc,
        has_children: doc.has_children ?? false,
        children: [],
      }));

    function updateChildren(nodes: Document[]): boolean {
      for (const n of nodes) {
        if (n.id === node.id) {
          n.children = children;
          return true;
        }
        if (n.children && updateChildren(n.children)) {
          return true;
        }
      }
      return false;
    }
    updateChildren(tree.value);
    loadedNodes.value = new Set(loadedNodes.value);
    loadedNodes.value.add(node.id);
    loadedNodes.value = new Set(loadedNodes.value);
  } catch {
    ui.showToast(t('docUi1.k87'), 'error');
  }
}

function toggleExpand(node: Document) {
  const newSet = new Set(expandedNodes.value);
  if (newSet.has(node.id)) {
    newSet.delete(node.id);
  } else {
    newSet.add(node.id);
    if (!loadedNodes.value.has(node.id)) {
      loadChildren(node);
    }
  }
  expandedNodes.value = newSet;
}

function flattenTree(nodes: Document[], level = 0): TreeNode[] {
  const result: TreeNode[] = [];

  for (const node of nodes) {
    result.push({ ...node, level });
    if (node.children && expandedNodes.value.has(node.id)) {
      result.push(...flattenTree(node.children, level + 1));
    }
  }

  return result;
}

const flatTree = computed(() => {
  const treeNodes = flattenTree(tree.value);
  return [
    { id: 0, name: '根目录', type: 2, parent_id: -1, space_id: props.spaceId, level: 0, has_children: false } as TreeNode,
    ...treeNodes
  ];
});

const currentPath = computed(() => {
  if (!selectedTarget.value) return '根目录';
  return selectedTarget.value.name;
});

function isDescendant(node: Document): boolean {
  if (!props.currentDocId) return false;
  if (node.id === props.currentDocId) return true;
  if (!node.path) return false;
  return node.path.split(',').map(Number).includes(props.currentDocId);
}

function selectTarget(node: Document) {
  if (isDescendant(node)) {
    ui.showToast(t('docUi1.k88'), 'warning');
    return;
  }
  selectedTarget.value = node;
  if (node.type === 2 && !expandedNodes.value.has(node.id) && node.has_children) {
    const newSet = new Set(expandedNodes.value);
    newSet.add(node.id);
    expandedNodes.value = newSet;
    if (!loadedNodes.value.has(node.id)) {
      loadChildren(node);
    }
  }
}

async function handleConfirm() {
  if (!selectedTarget.value) return;

  confirming.value = true;
  try {
    emit('confirm', selectedTarget.value.id);
    emit('update:modelValue', false);
    selectedTarget.value = null;
  } finally {
    confirming.value = false;
  }
}

watch(() => props.modelValue, (visible) => {
  if (visible) {
    selectedTarget.value = null;
    expandedNodes.value = new Set();
    loadedNodes.value = new Set();
    loadRoot();
  }
});

// 样式
const modalBodyStyle = {
  padding: AppleSpace.md,
};

const currentLocationStyle = {
  marginBottom: AppleSpace.sm,
  padding: AppleSpace.sm,
  background: cv('gray-50'),
  borderRadius: AppleRadius.card,
};

const labelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

const pathStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-primary'),
  fontWeight: String(AppleFontWeight.medium),
};

const treeContainerStyle = {
  maxHeight: '300px',
  overflow: 'auto',
  border: `1px solid ${cv('border-light')}`,
  borderRadius: AppleRadius.card,
  padding: AppleSpace.xs,
};

function getNodeStyle(node: Document & { level: number }) {
  const isSelected = selectedTarget.value?.id === node.id;
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
    padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
    margin: `${AppleSpace.xxs} 0`,
    borderRadius: AppleRadius.card,
    cursor: 'pointer',
    background: isSelected ? cv('blue-selection-strong') : 'transparent',
    transition: AppleTransition.fast,
  };
}

function indentStyle(node: Document & { level: number }) {
  return {
    width: `${node.level * 20}px`,
    flexShrink: 0,
  };
}

const folderIconStyle = {
  color: cv('folder-icon'),
  flexShrink: 0,
};

const fileIconStyle = {
  color: cv('text-quaternary'),
  flexShrink: 0,
};

function nameStyle(_node: Document & { level: number }) {
  return {
    fontFamily: AppleFont.text,
    fontSize: AppleType.controlLabel.size,
    color: cv('text-primary'),
    flex: 1,
  };
}

const checkStyle = {
  color: cv('blue'),
  flexShrink: 0,
};

const placeholderStyle = {
  width: '14px',
  flexShrink: 0,
};
</script>
