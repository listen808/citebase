import { get, put } from '../../client';

export interface ConfigItem {
  id: number;
  key: string;
  value: string;
  label: string;
  group_key: string;
  type: string;
  options: string;
  sort: number;
  is_private: number;
  create_time: number;
  update_time: number;
}

export interface ConfigGroup {
  key: string;
  label: string;
  items: ConfigItem[];
}

export function getConfigs(groupKey?: string) {
  return get<ConfigGroup[]>('/configs', { params: groupKey ? { group_key: groupKey } : undefined });
}

export function updateConfig(key: string, value: string) {
  // P1-6: 原写法把请求体放进第三个参数的 data 且位置参数传 null，
  // axios mergeConfig 中位置参数 data 优先，导致 body 被丢弃、系统设置永远 400。
  return put<void>('/configs', { value }, { params: { key } });
}
