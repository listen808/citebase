/**
 * 敏感数据脱敏工具函数
 * 用于在前端展示时对敏感字段进行掩码处理
 */

const SENSITIVE_FIELDS = [
  'password',
  'token',
  'secret',
  'key',
  'passwd',
  'pwd',
  'authorization',
  'auth',
  'api_key',
  'apikey',
  'access_key',
  'secret_key',
];

function isSensitiveField(fieldName: string): boolean {
  const lowerName = fieldName.toLowerCase();
  return SENSITIVE_FIELDS.some(sensitive => 
    lowerName === sensitive || lowerName.includes(sensitive)
  );
}

function maskValue(value: string): string {
  if (!value || typeof value !== 'string') return value;
  const len = value.length;
  if (len <= 5) {
    return '*'.repeat(len);
  }
  return value.slice(0, 3) + '***' + value.slice(-2);
}

function maskObject(obj: Record<string, unknown>): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(obj)) {
    if (isSensitiveField(key) && typeof value === 'string') {
      result[key] = maskValue(value);
    } else if (typeof value === 'object' && value !== null) {
      if (Array.isArray(value)) {
        result[key] = value.map(item => 
          typeof item === 'object' && item !== null 
            ? maskObject(item as Record<string, unknown>)
            : item
        );
      } else {
        result[key] = maskObject(value as Record<string, unknown>);
      }
    } else {
      result[key] = value;
    }
  }
  return result;
}

export function maskSensitiveData(params: string | Record<string, unknown> | null | undefined): string {
  if (!params) return '';
  
  let obj: Record<string, unknown>;
  
  if (typeof params === 'string') {
    try {
      obj = JSON.parse(params);
    } catch {
      return params;
    }
  } else if (typeof params === 'object') {
    obj = { ...params };
  } else {
    return String(params);
  }
  
  const masked = maskObject(obj);
  return JSON.stringify(masked, null, 2);
}
