/**
 * 统一日志入口
 * 对 core/error-logger 做薄封装，提供 logger.error / logger.warn / logger.info 对象风格 API
 */

import { logError, logWarn, logInfo } from './core/error-logger';

export const logger = {
  error: (message: string, error?: unknown): void => {
    logError(message, error ?? message);
  },
  warn: (message: string, data?: unknown): void => {
    logWarn(message, formatDetail(data));
  },
  info: (message: string, data?: unknown): void => {
    logInfo(message, formatDetail(data));
  },
};

function formatDetail(data: unknown): string {
  if (data === undefined) return '';
  if (typeof data === 'string') return data;
  try {
    return JSON.stringify(data);
  } catch {
    return String(data);
  }
}

export default logger;
