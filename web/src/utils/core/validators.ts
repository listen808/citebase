// 预设验证规则的 key —— 方便与 UI 组件和 composable 绑定使用
export type ValidationRuleName =
  | 'required'
  | 'email'
  | 'username'
  | 'password'
  | 'url'
  | 'phone'
  | 'port'
  | 'json'
  | 'usernamePrefix'
  | 'spaceName'
  | 'spaceDescription'
  | 'documentName';

// 单条字段值的验证函数签名
export type ValidatorFn = (value: string) => boolean;

// 预设规则对应的实现
export const validators: Record<ValidationRuleName, ValidatorFn> = {
  required: (value: string) => value.trim().length > 0,

  email: (value: string) => {
    if (!value) return true;
    return /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/.test(value);
  },

  username: (value: string) => /^[a-zA-Z0-9_-]{3,50}$/.test(value),

  password: (value: string) => {
    if (value.length < 8) return false;
    const hasLower = /[a-z]/.test(value);
    const hasUpper = /[A-Z]/.test(value);
    const hasNumber = /[0-9]/.test(value);
    const hasSpecial = /[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?]/.test(value);
    const typesCount = [hasLower, hasUpper, hasNumber, hasSpecial].filter(Boolean).length;
    return typesCount >= 3;
  },

  url: (value: string) => {
    if (!value) return true;
    try {
      new URL(value);
      return true;
    } catch {
      return false;
    }
  },

  phone: (value: string) => {
    if (!value) return true;
    return /^\+?[1-9]\d{6,14}$/.test(value);
  },

  port: (value: string) => {
    const num = parseInt(value, 10);
    return !isNaN(num) && num >= 1 && num <= 65535;
  },

  json: (value: string) => {
    if (!value) return true;
    try {
      JSON.parse(value);
      return true;
    } catch {
      return false;
    }
  },

  usernamePrefix: (value: string) => /^[a-zA-Z0-9_]+$/.test(value),

  spaceName: (value: string) => {
    const trimmed = value.trim();
    return trimmed.length >= 1 && trimmed.length <= 100;
  },

  spaceDescription: (value: string) => {
    if (!value) return true;
    return value.trim().length <= 500;
  },

  documentName: (value: string) => {
    const trimmed = value.trim();
    return trimmed.length >= 1 && trimmed.length <= 200;
  },
};

// 预设规则对应的默认中文错误消息
export const validationMessages: Record<ValidationRuleName, string> = {
  required: '此字段不能为空',
  email: '邮箱格式不正确',
  username: '用户名必须是 3-50 位字母、数字、下划线或连字符',
  password: '密码至少 8 位，且包含大小写字母、数字、特殊字符中的至少 3 种',
  url: 'URL 格式不正确',
  phone: '手机号格式不正确',
  port: '端口必须是 1-65535 的数字',
  json: 'JSON 格式不正确',
  usernamePrefix: '用户名前缀只能包含字母、数字和下划线',
  spaceName: '空间名称长度应为 1-100 字符',
  spaceDescription: '空间描述不能超过 500 字符',
  documentName: '文档标题长度应为 1-200 字符',
};

// 旧版 API —— 保留向后兼容
export function getValidationMessage(field: string, value: string): string {
  switch (field) {
    case 'email':
      if (!validators.email(value)) return validationMessages.email;
      return '';
    case 'username':
      if (!validators.username(value)) return validationMessages.username;
      return '';
    case 'password':
      if (!validators.password(value)) return validationMessages.password;
      return '';
    case 'url':
      if (!validators.url(value)) return validationMessages.url;
      return '';
    case 'phone':
      if (!validators.phone(value)) return validationMessages.phone;
      return '';
    case 'port':
      if (!validators.port(value)) return validationMessages.port;
      return '';
    case 'json':
      if (!validators.json(value)) return validationMessages.json;
      return '';
    case 'usernamePrefix':
      if (!validators.usernamePrefix(value)) return validationMessages.usernamePrefix;
      return '';
    default:
      return '';
  }
}

// 对单个值按照一组规则进行校验，返回第一条错误消息；全部通过则返回空字符串
export function validateValue(
  value: string,
  rules: Array<ValidationRuleName | { name: ValidationRuleName; message?: string } | ValidatorFn>
): string {
  for (const rule of rules) {
    if (typeof rule === 'string') {
      const fn = validators[rule];
      if (fn && !fn(value)) return validationMessages[rule];
    } else if (typeof rule === 'function') {
      if (!rule(value)) return '输入不合法';
    } else {
      const fn = validators[rule.name];
      if (fn && !fn(value)) return rule.message || validationMessages[rule.name];
    }
  }
  return '';
}

// 便捷工厂：自定义最小长度验证
export function minLengthValidator(min: number, message?: string): { fn: ValidatorFn; message: string } {
  return {
    fn: (value) => value.trim().length >= min,
    message: message || `至少 ${min} 个字符`,
  };
}

// 便捷工厂：自定义最大长度验证
export function maxLengthValidator(max: number, message?: string): { fn: ValidatorFn; message: string } {
  return {
    fn: (value) => value.trim().length <= max,
    message: message || `最多 ${max} 个字符`,
  };
}
