import type { Document } from '@/types/models';
import type { SortDocumentItem, MoveDocumentRequest } from '@/api/modules/document/document';

export function useTreeDragReorder() {
  function findSiblings(nodes: Document[], targetId: number): { siblings: Document[]; targetIdx: number } | null {
    for (let i = 0; i < nodes.length; i++) {
      const node = nodes[i];
      if (!node) continue;
      if (node.id === targetId) return { siblings: nodes, targetIdx: i };
      if (node.children) {
        const result = findSiblings(node.children, targetId);
        if (result) return result;
      }
    }
    return null;
  }

  function findNode(nodes: Document[], id: number): Document | null {
    for (const n of nodes) {
      if (n.id === id) return n;
      if (n.children) {
        const found = findNode(n.children, id);
        if (found) return found;
      }
    }
    return null;
  }

  function isDescendantOf(nodes: Document[], ancestorId: number, descendantId: number): boolean {
    const ancestor = findNode(nodes, ancestorId);
    if (!ancestor || !ancestor.children) return false;
    const queue = [...ancestor.children];
    while (queue.length > 0) {
      const child = queue.shift()!;
      if (child.id === descendantId) return true;
      if (child.children) queue.push(...child.children);
    }
    return false;
  }

  async function handleDragReorder(
    tree: Document[],
    dragId: number,
    dropId: number,
    position: 'before' | 'after' | 'inside',
    sortFn: (items: SortDocumentItem[]) => Promise<void>,
    moveFn: (id: number, data: MoveDocumentRequest) => Promise<void>,
  ): Promise<boolean> {
    if (position === 'inside') {
      if (dragId === dropId) return false;
      if (isDescendantOf(tree, dragId, dropId)) return false;
      const sourceNode = findNode(tree, dragId);
      if (!sourceNode) return false;
      const targetNode = findNode(tree, dropId);
      if (!targetNode || targetNode.type !== 2) return false;
      const targetChildren = targetNode.children || [];
      const existingIds = new Set(targetChildren.map(c => c.id));
      if (existingIds.has(dragId)) return false;
      const orders: SortDocumentItem[] = targetChildren.map((c, i) => ({ id: c.id, sequence: i }));
      orders.push({ id: dragId, sequence: targetChildren.length });
      const savedTargetSequences = targetChildren.map((c, i) => ({ id: c.id, sequence: c.sequence || i }));
      const savedParentId = sourceNode.parent_id;
      const savedSequence = sourceNode.sequence;
      const savedChildren = [...targetChildren];
      targetChildren.forEach((c, i) => { c.sequence = i; });
      sourceNode.sequence = targetChildren.length;
      try {
        await moveFn(dragId, { parent_id: dropId, space_id: targetNode.space_id });
        await sortFn(orders);
        sourceNode.parent_id = dropId;
        return true;
      } catch {
        savedTargetSequences.forEach(s => { const n = findNode(tree, s.id); if (n) n.sequence = s.sequence; });
        sourceNode.sequence = savedSequence;
        sourceNode.parent_id = savedParentId;
        targetNode.children = savedChildren;
        return false;
      }
    }

    const result = findSiblings(tree, dropId);
    if (!result) return false;
    const { siblings, targetIdx } = result;
    const sourceIdx = siblings.findIndex(s => s.id === dragId);
    const sourceNode = findNode(tree, dragId);
    if (!sourceNode) return false;

    if (sourceIdx === -1) {
      const dropNode = findNode(tree, dropId);
      if (!dropNode) return false;
      const targetParentId = dropNode.parent_id || 0;
      if (isDescendantOf(tree, dragId, dropId)) return false;
      if (targetParentId > 0 && isDescendantOf(tree, dragId, targetParentId)) return false;
      const savedSourceParentId = sourceNode.parent_id;
      try {
        await moveFn(dragId, { parent_id: targetParentId, space_id: dropNode.space_id });
        sourceNode.parent_id = targetParentId;
      } catch {
        sourceNode.parent_id = savedSourceParentId;
        return false;
      }
      const updatedSiblings = [...siblings, sourceNode];
      const updatedTargetIdx = updatedSiblings.findIndex(s => s.id === dropId);
      const insertIdx = position === 'before' ? updatedTargetIdx : updatedTargetIdx + 1;
      const reordered = updatedSiblings.filter(s => s.id !== dragId);
      const finalInsertIdx = insertIdx;
      reordered.splice(finalInsertIdx, 0, sourceNode);
      const orders: SortDocumentItem[] = reordered.map((n, i) => ({ id: n.id, sequence: i }));
      try {
        await sortFn(orders);
        return true;
      } catch {
        sourceNode.parent_id = savedSourceParentId;
        return false;
      }
    }

    if (sourceIdx === targetIdx) return true;
    if (sourceIdx === targetIdx - 1 && position === 'after') return true;
    if (sourceIdx === targetIdx + 1 && position === 'before') return true;

    const reordered = siblings.filter(s => s.id !== dragId);
    const adjustedTargetIdx = sourceIdx < targetIdx ? targetIdx - 1 : targetIdx;
    const insertIdx = position === 'before' ? adjustedTargetIdx : adjustedTargetIdx + 1;
    reordered.splice(insertIdx, 0, sourceNode);
    const orders: SortDocumentItem[] = reordered.map((n, i) => ({ id: n.id, sequence: i }));
    const savedSequences = siblings.map(s => ({ id: s.id, sequence: s.sequence || 0 }));
    const savedOrder = [...siblings];
    siblings.forEach((s) => { s.sequence = reordered.findIndex(r => r.id === s.id); });
    try {
      await sortFn(orders);
      return true;
    } catch {
      savedSequences.forEach(s => { const n = findNode(tree, s.id); if (n) n.sequence = s.sequence; });
      for (let i = 0; i < siblings.length; i++) {
        siblings[i] = savedOrder[i]!;
      }
      return false;
    }
  }

  return { handleDragReorder, isDescendantOf, findNode };
}
