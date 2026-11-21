const CHARS_PER_MINUTE_ZH = 500;

export function estimateReadingTime(content: string): number {
  if (!content) return 0;
  const chineseChars = (content.match(/[\u4e00-\u9fa5]/g) || []).length;
  const englishWords = (content.match(/[a-zA-Z]+/g) || []).length;
  const minutes = chineseChars / CHARS_PER_MINUTE_ZH + englishWords / 200;
  return Math.max(1, Math.ceil(minutes));
}

export function countWords(content: string): number {
  if (!content) return 0;
  const chineseChars = (content.match(/[\u4e00-\u9fa5]/g) || []).length;
  const englishWords = (content.match(/[a-zA-Z]+/g) || []).length;
  return chineseChars + englishWords;
}
