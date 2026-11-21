/**
 * 统一错误日志和处理工具
 * 用于规范化错误处理、日志记录和用户提示
 */

/**
 * 是否为开发环境
 */
// eslint-disable-next-line @typescript-eslint/no-unused-vars
const isDev = import.meta.env.DEV;

/**
 * 错误上下文信息
 */
export interface ErrorContext {
  /** 错误发生的模块或组件名 */
  module: string;
  /** 错误发生的函数或操作名 */
  operation?: string;
  /** 附加数据 */
  data?: Record<string, unknown>;
}

/**
 * 记录错误日志
 * 开发环境：输出到console.error
 * 生产环境：可以发送到监控平台（如Sentry）
 * @param context 错误上下文
 * @param error 错误对象
 */
export function logError(context: ErrorContext | string, error: unknown): void {
  const contextStr = typeof context === 'string'
    ? context
    : `${context.module}${context.operation ? `.${context.operation}` : ''}`;

  console.error(`[${contextStr}]`, error);
  if (typeof context === 'object' && context.data) {
    console.error('[附加数据]', context.data);
  }
}

/**
 * 记录警告日志
 * @param context 上下文
 * @param message 警告消息
 */
export function logWarn(context: ErrorContext | string, message: string): void {
  const contextStr = typeof context === 'string'
    ? context
    : `${context.module}${context.operation ? `.${context.operation}` : ''}`;

  console.warn(`[${contextStr}]`, message);
}

/**
 * 记录信息日志
 * @param context 上下文
 * @param message 信息消息
 */
export function logInfo(context: ErrorContext | string, message: string): void {
  const contextStr = typeof context === 'string'
    ? context
    : `${context.module}${context.operation ? `.${context.operation}` : ''}`;

  console.warn(`[${contextStr}]`, message);
}

/**
 * 获取用户友好的错误消息
 * @param error 错误对象
 * @returns 错误消息字符串
 */
export function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    // 检查是否为API错误（有message）
    if (error.message) {
      return error.message;
    }
  }
  
  if (typeof error === 'string') {
    return error;
  }
  
  if (error && typeof error === 'object') {
    // 检查是否有message字段
    if ('message' in error && typeof error.message === 'string') {
      return error.message;
    }
    // 检查是否有error字段
    if ('error' in error && typeof error.error === 'string') {
      return error.error;
    }
  }
  
  return '操作失败，请重试';
}

/**
 * 创建统一的错误处理器
 * @param context 错误上下文
 * @param showToast 显示toast的函数
 * @returns 错误处理函数
 */
export function createErrorHandler(
  context: ErrorContext | string,
  showToast: (message: string, type: 'error' | 'warning' | 'info' | 'success') => void
): (error: unknown) => void {
  return (error: unknown) => {
    logError(context, error);
    showToast(getErrorMessage(error), 'error');
  };
}

/**
 * 安全执行函数，自动捕获错误
 * @param fn 要执行的函数
 * @param context 错误上下文
 * @param errorHandler 错误处理器（可选）
 * @returns 执行结果或null
 */
export async function safeExecute<T>(
  fn: () => Promise<T>,
  context: ErrorContext | string,
  errorHandler?: (error: unknown) => void
): Promise<T | null> {
  try {
    return await fn();
  } catch (error) {
    logError(context, error);
    if (errorHandler) {
      errorHandler(error);
    }
    return null;
  }
}

/**
 * 创建带日志的异步函数包装器
 * @param fn 原始函数
 * @param context 上下文
 * @returns 包装后的函数
 */
export function withErrorLogging<T extends (...args: unknown[]) => Promise<unknown>>(
  fn: T,
  context: ErrorContext | string
): T {
  return (async (...args: Parameters<T>) => {
    try {
      return await fn(...args);
    } catch (error) {
      logError(context, error);
      throw error;
    }
  }) as T;
}

/**
 * 错误类型守卫
 * @param error 错误对象
 * @returns 是否为Error实例
 */
export function isError(error: unknown): error is Error {
  return error instanceof Error;
}

/**
 * 检查是否为网络错误
 * @param error 错误对象
 * @returns 是否为网络错误
 */
export function isNetworkError(error: unknown): boolean {
  if (error instanceof Error) {
    return (
      error.message.includes('网络') ||
      error.message.includes('Network') ||
      error.message.includes('timeout') ||
      error.message.includes('Timeout')
    );
  }
  return false;
}

/**
 * 检查是否为权限错误
 * @param error 错误对象
 * @returns 是否为权限错误
 */
export function isPermissionError(error: unknown): boolean {
  if (error instanceof Error) {
    return (
      error.message.includes('权限') ||
      error.message.includes('Permission') ||
      error.message.includes('403')
    );
  }
  return false;
}

/**
 * 根据错误类型获取提示消息
 * @param error 错误对象
 * @returns 提示消息
 */
export function getErrorHint(error: unknown): string {
  if (isNetworkError(error)) {
    return '网络连接失败，请检查网络后重试';
  }
  
  if (isPermissionError(error)) {
    return '您没有权限执行此操作';
  }
  
  return getErrorMessage(error);
}
