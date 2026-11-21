import { cv } from './apple-tokens';

export type ApTone = 'accent' | 'success' | 'warning' | 'danger';

export const toneColor = (tone: ApTone): string =>
  (tone === 'success' ? cv('green') : tone === 'warning' ? cv('orange') : tone === 'danger' ? cv('red') : cv('blue'));

/** 越高越好：匹配度、置信度、健康分。 */
export const toneForScore = (score: number): ApTone => (score >= 0.8 ? 'success' : score >= 0.5 ? 'warning' : 'danger');

/** 越高越糟：CPU、内存、磁盘、配额占用。阈值由调用方按资源给（磁盘的安全线本来就比 CPU 高）。 */
export const toneForLoad = (percent: number, warn = 70, danger = 85): ApTone =>
  (percent >= danger ? 'danger' : percent >= warn ? 'warning' : 'accent');
