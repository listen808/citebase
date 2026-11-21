import { type Ref, type ComputedRef } from 'vue';
import type { Document } from '@/types/models';

export function useTreeKeyboard(options: {
  tree: Ref<Document[]> | ComputedRef<Document[]>;
  expandedNodes: Ref<Set<number>>;
  focusedId: Ref<number | null>;
  selectedId: Ref<number | null>;
  onToggle: (id: number) => void;
  onSelect: (doc: Document) => void;
  onCreate?: (parentId: number) => void;
  onDelete?: (id: number) => void;
  onRename?: (id: number) => void;
}) {
  function getVisibleNodes(): Document[] {
    const result: Document[] = [];
    const traverse = (nodes: Document[]) => {
      for (const node of nodes) {
        result.push(node);
        if (options.expandedNodes.value.has(node.id) && node.children) {
          traverse(node.children);
        }
      }
    };
    traverse(options.tree.value);
    return result;
  }

  function handleKeydown(e: KeyboardEvent) {
    const visible = getVisibleNodes();
    if (visible.length === 0) return;

    const currentIdx = visible.findIndex(n => n.id === options.focusedId.value);
    const current = currentIdx >= 0 ? visible[currentIdx] : null;

    switch (e.key) {
      case 'ArrowUp':
        e.preventDefault();
        if (currentIdx > 0) {
          options.focusedId.value = visible[currentIdx - 1]!.id;
        } else if (currentIdx < 0 && visible.length > 0) {
          options.focusedId.value = visible[0]!.id;
        }
        break;
      case 'ArrowDown':
        e.preventDefault();
        if (currentIdx < visible.length - 1) {
          options.focusedId.value = visible[currentIdx + 1]!.id;
        } else if (currentIdx < 0 && visible.length > 0) {
          options.focusedId.value = visible[0]!.id;
        }
        break;
      case 'ArrowLeft':
        e.preventDefault();
        if (current && !options.expandedNodes.value.has(current.id) && current.parent_id) {
          options.focusedId.value = current.parent_id;
        }
        break;
      case 'ArrowRight':
        e.preventDefault();
        if (current && options.expandedNodes.value.has(current.id) && current.children?.length) {
          options.focusedId.value = current.children[0]!.id;
        }
        break;
      case 'Enter':
        e.preventDefault();
        if (current) options.onSelect(current);
        break;
      case 'Home':
        e.preventDefault();
        if (visible.length) options.focusedId.value = visible[0]!.id;
        break;
      case 'End':
        e.preventDefault();
        if (visible.length) options.focusedId.value = visible[visible.length - 1]!.id;
        break;
      case ' ':
        e.preventDefault();
        if (current) options.onToggle(current.id);
        break;
      case 'Delete':
        if (current && options.onDelete) {
          e.preventDefault();
          options.onDelete(current.id);
        }
        break;
      case 'F2':
        e.preventDefault();
        if (current && options.onRename) options.onRename(current.id);
        break;
    }
  }

  return { handleKeydown, getVisibleNodes };
}
