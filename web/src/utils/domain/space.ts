import type { Space } from '@/types/models';

export function mergeMySpacesToAll(
  freshMySpaces: Space[],
  allSpaces: Space[],
  spacesCache: { value: { data: Space[]; timestamp: number } | null }
): Space[] {
  if (!allSpaces.length) return allSpaces;
  const updated = allSpaces.map((s) => {
    const match = freshMySpaces.find((m) => m.id === s.id);
    return match ? { ...s, ...match } : s;
  });
  if (spacesCache.value) {
    spacesCache.value = { data: updated, timestamp: Date.now() };
  }
  return updated;
}
