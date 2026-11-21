import { ref, computed } from 'vue';
import type { Document } from '@/types/models';

export type SortOrder = 'name-asc' | 'name-desc' | 'date-new' | 'date-old' | 'sequence';

export function useTreeSort(getFilteredTree: () => Document[]) {
  const sortOrder = ref<SortOrder>('sequence');

  const sortedTree = computed(() => {
    const sortFn = (a: Document, b: Document): number => {
      const aName = a.name.toLowerCase();
      const bName = b.name.toLowerCase();
      switch (sortOrder.value) {
        case 'sequence':
          return (a.sequence || 0) - (b.sequence || 0);
        case 'name-asc':
          return aName.localeCompare(bName);
        case 'name-desc':
          return bName.localeCompare(aName);
        case 'date-new':
          return (b.update_time || 0) - (a.update_time || 0);
        case 'date-old':
          return (a.update_time || 0) - (b.update_time || 0);
        default:
          return (a.sequence || 0) - (b.sequence || 0);
      }
    };

    const sortTree = (nodes: Document[]): Document[] => {
      return [...nodes].sort((a, b) => {
        if (a.type !== b.type) {
          return a.type === 2 ? -1 : 1;
        }
        return sortFn(a, b);
      }).map(node => ({
        ...node,
        children: node.children ? sortTree(node.children) : undefined,
      }));
    };

    return sortTree(getFilteredTree());
  });

  function cycleSort() {
    const orders: SortOrder[] = ['sequence', 'name-asc', 'name-desc', 'date-new', 'date-old'];
    const idx = orders.indexOf(sortOrder.value);
    sortOrder.value = orders[(idx + 1) % orders.length] || 'sequence';
  }

  function getSortLabel() {
    switch (sortOrder.value) {
      case 'sequence': return '默认排序';
      case 'name-asc': return '按名称升序';
      case 'name-desc': return '按名称降序';
      case 'date-new': return '按日期最新';
      case 'date-old': return '按日期最早';
      default: return '排序';
    }
  }

  return { sortOrder, sortedTree, cycleSort, getSortLabel };
}
