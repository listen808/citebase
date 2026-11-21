/**
 * 共享工具函数
 * 包含项目中重复定义的通用函数
 */

/**
 * 验证重定向URL是否安全
 * 检查URL协议和来源是否合法，防止开放重定向攻击
 * @param url 要验证的URL
 * @returns 是否为安全的重定向URL
 */
export function isSafeRedirect(url: string): boolean {
  try {
    const parsed = new URL(url, window.location.origin);
    return ['http:', 'https:'].includes(parsed.protocol);
  } catch {
    return false;
  }
}

/**
 * HTML转义函数
 * 用于防止XSS攻击，将HTML特殊字符转义为实体
 * @param text 需要转义的文本
 * @returns 转义后的文本
 */
export function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

/**
 * 防抖函数
 * @param func 需要防抖的函数
 * @param wait 等待时间（毫秒）
 * @param immediate 是否立即执行
 * @returns 防抖后的函数
 */
export function debounce<T extends (...args: unknown[]) => unknown>(
  func: T,
  wait: number,
  immediate = false
): ((...args: Parameters<T>) => void) & { cancel: () => void } {
  let timeout: ReturnType<typeof setTimeout> | null = null;

  const debounced = function(this: unknown, ...args: Parameters<T>) {
    const later = () => {
      timeout = null;
      if (!immediate) {
        func.apply(this, args);
      }
    };
    
    const callNow = immediate && !timeout;
    
    if (timeout) {
      clearTimeout(timeout);
    }
    timeout = setTimeout(later, wait);
    
    if (callNow) {
      func.apply(this, args);
    }
  } as ((...args: Parameters<T>) => void) & { cancel: () => void };

  // 取消尚未执行的防抖调用（组件卸载清理用，替代各处手写 clearTimeout）
  debounced.cancel = () => {
    if (timeout) {
      clearTimeout(timeout);
      timeout = null;
    }
  };

  return debounced;
}

/**
 * 延迟执行
 * @param ms 延迟时间（毫秒）
 * @returns Promise
 */
export function delay(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * 将颜色转换为带透明度的等价颜色
 * @param hex 十六进制颜色值（如 '#0071e3'），也接受 CSS 变量/函数色值
 * @param alpha 透明度（0-1）
 * @returns rgba 或 color-mix 颜色字符串
 */
export function hexToRgba(hex: string, alpha: number): string {
  const raw = hex.trim();
  // 非十六进制输入（如 var(--apple-blue)）无法解析通道值，退化为 color-mix 以保持主题联动
  if (!/^#(?:[0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/i.test(raw)) {
    return `color-mix(in srgb, ${raw} ${Math.round(alpha * 100)}%, transparent)`;
  }
  let h = raw.replace('#', '');
  if (h.length === 3 || h.length === 4) h = h.split('').slice(0, 3).map((c) => c + c).join('');
  if (h.length === 8) h = h.slice(0, 6);
  const r = parseInt(h.substring(0, 2), 16);
  const g = parseInt(h.substring(2, 4), 16);
  const b = parseInt(h.substring(4, 6), 16);
  return `rgba(${r},${g},${b},${alpha})`;
}
/**
 * 给任意十六进制 / rgb() 颜色追加透明度，替代 `${color}80` 这类字符串拼接。
 * 传入非颜色（如 var(--apple-x)）时原样返回，由调用方保证可解析。
 */
export function withAlpha(color: string, alpha: number): string {
  const raw = color.trim();
  const a = Math.max(0, Math.min(1, alpha));
  if (/^rgba?\(/i.test(raw)) {
    const nums = raw.match(/[\d.]+/g) || [];
    return `rgba(${nums[0]},${nums[1]},${nums[2]},${a})`;
  }
  if (!/^#[0-9a-f]{3,8}$/i.test(raw)) return raw;
  let h = raw.replace('#', '');
  if (h.length === 3 || h.length === 4) h = h.split('').slice(0, 3).map((c) => c + c).join('');
  h = h.slice(0, 6);
  const r = parseInt(h.slice(0, 2), 16), g = parseInt(h.slice(2, 4), 16), b = parseInt(h.slice(4, 6), 16);
  return `rgba(${r},${g},${b},${a})`;
}
