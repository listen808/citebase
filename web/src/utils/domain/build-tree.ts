import type { Document } from '@/types/models';

export function buildTree(flatList?: Document[] | null): Document[] {
  if (!Array.isArray(flatList)) return [];
  const map = new Map<number, Document>();
  const roots: Document[] = [];

  flatList.forEach((doc) => {
    map.set(doc.id, { ...doc, children: [] });
  });

  flatList.forEach((doc) => {
    const node = map.get(doc.id)!;
    if (doc.parent_id === 0) {
      roots.push(node);
    } else {
      const parent = map.get(doc.parent_id);
      if (parent) {
        parent.children = parent.children || [];
        parent.children.push(node);
      }
    }
  });

  return roots;
}
