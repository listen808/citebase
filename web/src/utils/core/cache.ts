/**
 * 缓存相关类型定义和工具函数
 * 用于统一管理项目中的缓存逻辑
 */

import { CACHE_TTL } from './constants';

/**
 * 缓存条目接口
 */
export interface CacheEntry<T> {
  data: T;
  timestamp: number;
}

/**
 * 检查缓存是否有效
 * @param entry 缓存条目
 * @param ttl 缓存有效期（毫秒），默认使用全局CACHE_TTL
 * @returns 缓存是否有效
 */
export function isCacheValid<T>(
  entry: CacheEntry<T> | null | undefined,
  ttl: number = CACHE_TTL
): entry is CacheEntry<T> {
  return entry != null && Date.now() - entry.timestamp < ttl;
}

/**
 * 创建缓存条目
 * @param data 要缓存的数据
 * @returns 缓存条目
 */
export function createCacheEntry<T>(data: T): CacheEntry<T> {
  return {
    data,
    timestamp: Date.now(),
  };
}