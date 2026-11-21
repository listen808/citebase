/**
 * 时间格式化工具函数
 * 提供统一的时间格式化API，支持多种输入类型和输出格式
 */

import i18n, { appIntlLocale } from '@/i18n';

// 这些是纯工具函数，不在 setup 里，用不了 useI18n()，只能取全局 composer 的 t。
// '3 分钟前' 与 "Oct 11" 都是**运行时算出来的**，源码里没有中文字面量 ——
// 所以 CJK 门禁数不到它们：界面切成英文、日期却还是"2026年10月11日星期日"，
// 是这种漏口的原样。语言开关必须同时管到消息与 Intl。
const { t } = i18n.global;

/**
 * 将时间戳转换为毫秒
 * @param ts 时间戳（秒或毫秒）
 * @returns 毫秒时间戳
 */
function toMs(ts: number): number {
  return ts > 1e12 ? ts : ts * 1000;
}

/**
 * 将输入转换为Date对象
 * @param input 时间戳、Date对象或日期字符串
 * @returns Date对象或null
 */
function toDate(input: Date | number | string | undefined | null): Date | null {
  if (!input) return null;

  if (input instanceof Date) {
    return isNaN(input.getTime()) ? null : input;
  }

  if (typeof input === 'number') {
    const date = new Date(toMs(input));
    return isRealDate(date) ? date : null;
  }

  if (typeof input === 'string') {
    // Go 的 time.Time 零值（0001-01-01T00:00:00Z）按「无」处理，别渲染出公元 1 年
    if (input.startsWith('0001-')) return null;
    const date = new Date(input);
    return isRealDate(date) ? date : null;
  }

  return null;
}

function isRealDate(date: Date): boolean {
  return !isNaN(date.getTime()) && date.getFullYear() > 1;
}

/**
 * 格式化日期和时间
 * @param input 时间戳、Date对象或日期字符串
 * @returns 格式化后的日期时间字符串
 */
export function formatTime(input: Date | number | string | undefined | null): string {
  const date = toDate(input);
  if (!date) return '-';
  return date.toLocaleString(appIntlLocale());
}

/**
 * 格式化日期（支持多种格式）
 * @param input 时间戳、Date对象或日期字符串
 * @param format 输出格式：'locale'(跟随界面语言，默认)、'yyyy-mm-dd'、'yyyy/mm/dd'
 * @returns 格式化后的日期字符串
 */
export function formatDate(
  input: Date | number | string | undefined | null,
  format: 'locale' | 'yyyy-mm-dd' | 'yyyy/mm/dd' = 'locale'
): string {
  const date = toDate(input);
  if (!date) return '-';
  
  switch (format) {
    case 'yyyy-mm-dd': {
      const year = date.getFullYear();
      const month = String(date.getMonth() + 1).padStart(2, '0');
      const day = String(date.getDate()).padStart(2, '0');
      return `${year}-${month}-${day}`;
    }
    case 'yyyy/mm/dd': {
      const year = date.getFullYear();
      const month = String(date.getMonth() + 1).padStart(2, '0');
      const day = String(date.getDate()).padStart(2, '0');
      return `${year}/${month}/${day}`;
    }
    case 'locale':
    default:
      return date.toLocaleDateString(appIntlLocale());
  }
}

/**
 * 格式化时间为相对时间（多久前）
 * @param input 时间戳、Date对象或日期字符串
 * @returns 相对时间字符串
 */
export function formatTimeAgo(input: Date | number | string | undefined | null): string {
  const date = toDate(input);
  if (!date) return t('time.unknown');

  const now = Date.now();
  const diff = now - date.getTime();
  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  // 英文的单/复数交给消息里的 `|` 形式，且**两档时第 0 档是单数**（vue-i18n 按 CLDR 的
  // one | other 取位，写反了就会出现 "3 hour ago"）；不在代码里拼字符串 —— 拼出来的
  // "1 minutes ago" 是机器味最重的一种错。见 format-time.test.ts。
  if (days > 0) return t('time.daysAgo', { n: days });
  if (hours > 0) return t('time.hoursAgo', { n: hours });
  if (minutes > 0) return t('time.minutesAgo', { n: minutes });
  return t('time.justNow');
}

/**
 * 把秒数说成"运行了多久"（时长、TTL 这一类）。
 *
 * 单位串必须住在 time.* 而不是调用点：`DashboardView` 原先写的是
 * `${days}天 ${hours}小时`，界面切成英文就漏出"9天 8小时" —— 这类串 CJK 静态门禁看得见，
 * 但每修一处就少一处，所以收在这里。分钟档也给了：不足一小时时"0小时 5分"是机器味。
 */
export function formatDuration(seconds: number): string {
  if (!seconds || seconds < 0 || isNaN(seconds)) return '--';
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return t('time.durationDaysHours', { d: days, h: hours });
  if (hours > 0) return t('time.durationHoursMinutes', { h: hours, m: minutes });
  return t('time.durationMinutes', { m: minutes });
}

/**
 * 格式化时间（仅时间部分）
 * @param input 时间戳、Date对象或日期字符串
 * @returns 格式化后的时间字符串 (HH:MM:SS)
 */
export function formatTimeOnly(input: Date | number | string | undefined | null): string {
  const date = toDate(input);
  if (!date) return '-';
  
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const seconds = String(date.getSeconds()).padStart(2, '0');
  return `${hours}:${minutes}:${seconds}`;
}

/**
 * 格式化日期和时间（自定义格式）
 * @param input 时间戳、Date对象或日期字符串
 * @param format 格式字符串，支持: YYYY(年)、MM(月)、DD(日)、HH(时)、mm(分)、ss(秒)
 * @returns 格式化后的日期时间字符串
 */
export function formatDateTime(
  input: Date | number | string | undefined | null,
  format: string = 'YYYY-MM-DD HH:mm:ss'
): string {
  const date = toDate(input);
  if (!date) return '-';
  
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const seconds = String(date.getSeconds()).padStart(2, '0');
  
  return format
    .replace('YYYY', String(year))
    .replace('MM', month)
    .replace('DD', day)
    .replace('HH', hours)
    .replace('mm', minutes)
    .replace('ss', seconds);
}

/**
 * 获取当前时间戳（毫秒）
 * @returns 当前时间戳
 */
export function now(): number {
  return Date.now();
}

/**
 * 获取当前时间戳（秒）
 * @returns 当前时间戳（秒）
 */
export function nowSeconds(): number {
  return Math.floor(Date.now() / 1000);
}

/**
 * 短日期："10/11" 一类（图表轴、token 用量趋势）。
 * 'yyyy-mm-dd' 字符串直接切（避免 new Date 的时区偏移把日子挪一天），其余交给 Date 解析。
 * 此前 AITokenView / DashboardCharts 各有一份且行为不同（一份会因时区差一天），此处收口。
 */
export function formatShortDate(dateStr: string): string {
  if (!dateStr) return '';
  // ISO 串（2026-11-12T00:00:00Z）只取日期段：原先 split('-')[2] 会把 "12T00:00:00Z"
  // 整段当日期返回，Token 页 x 轴因此印出 11/12T00:00:00Z（评审 P1-4）
  const iso = /^(\d{4})-(\d{2})-(\d{2})/.exec(dateStr);
  if (iso) return `${iso[2]}/${iso[3]}`;
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return dateStr;
  return `${d.getMonth() + 1}/${d.getDate()}`;
}

/**
 * 相对时间 + 过期退化为绝对时间的统一口径（搜索结果 / 文档侧栏共用）。
 * @param timestamp 秒级时间戳
 * @param options.recentDays 多少天内算"还算近期"（默认 30）
 * @param options.older 超过近期窗口后的退化形式：'date' = yyyy-mm-dd（搜索结果），
 *                      'full' = 完整本地日期时间（文档侧栏）
 */
export function formatRelativeTime(
  timestamp: number,
  options: { recentDays?: number; older?: 'date' | 'full' } = {}
): string {
  if (!timestamp) return '';
  const { recentDays = 30, older = 'date' } = options;
  const days = Math.floor((Date.now() - timestamp * 1000) / 86400000);
  if (days <= recentDays) return formatTimeAgo(timestamp);
  return older === 'date' ? formatDate(timestamp, 'yyyy-mm-dd') : formatTime(timestamp);
}

/**
 * 数字紧凑显示：1234567 -> "1.2M"、12345 -> "12.3K"（统计卡、观测面板共用）。
 */
export function formatNumberCompact(n: number): string {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K';
  return Math.round(n).toString();
}

/**
 * 毫秒时长的人话口径："850ms" / "1.2s" / "3m 5s"（观测面板、流式任务共用）。
 * 与 formatDuration（秒入参、走 time.* 文案）不同：这是短耗时计时，用符号单位。
 */
export function formatDurationMs(ms: number): string {
  if (!ms || ms < 0 || isNaN(ms)) return '--';
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const mins = Math.floor(ms / 60000);
  const secs = Math.round((ms % 60000) / 1000);
  return `${mins}m ${secs}s`;
}

/**
 * 百分比统一口径。
 * @param input 'fraction'（默认）：入参是 0-1 的比例（RAGAS 指标），null/NaN -> "—"；
 *              'percent'：入参已是百分数值（管理台仪表盘），直接保留一位小数。
 */
export function formatPercent(
  value: number | null | undefined,
  input: 'fraction' | 'percent' = 'fraction'
): string {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  return input === 'fraction' ? `${(value * 100).toFixed(1)}%` : `${value.toFixed(1)}%`;
}

/**
 * 月份短名 + 日："10月11日" / "Oct 11"（会话列表侧栏的时间戳角标）。
 */
export function formatShortMonthDay(input: Date | number | string | undefined | null): string {
  const date = toDate(input);
  if (!date) return '';
  return date.toLocaleDateString(appIntlLocale(), { month: 'short', day: 'numeric' });
}
