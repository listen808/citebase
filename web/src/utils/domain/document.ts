export function parseContent(contentRes: unknown): string {
  if (typeof contentRes === 'string') return contentRes;
  if (contentRes && typeof contentRes === 'object' && 'content' in contentRes) {
    return String((contentRes as Record<string, unknown>).content);
  }
  return String(contentRes || '');
}
