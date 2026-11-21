import { describe, it, expect } from 'vitest';
import {
  validators,
  validationMessages,
  validateValue,
  getValidationMessage,
  minLengthValidator,
  maxLengthValidator,
} from '@/utils/core/validators';

describe('validators.required', () => {
  it('纯空白视为未填', () => {
    expect(validators.required('')).toBe(false);
    expect(validators.required('   ')).toBe(false);
    expect(validators.required('a')).toBe(true);
  });
});

describe('validators.email / url / phone / json 允许留空', () => {
  it('可选规则对空值放行', () => {
    expect(validators.email('')).toBe(true);
    expect(validators.url('')).toBe(true);
    expect(validators.phone('')).toBe(true);
    expect(validators.json('')).toBe(true);
  });

  it('非空时校验格式', () => {
    expect(validators.email('a@b.co')).toBe(true);
    expect(validators.email('a@b')).toBe(false);
    expect(validators.url('https://x.com')).toBe(true);
    expect(validators.url('not a url')).toBe(false);
    expect(validators.phone('13800138000')).toBe(true);
    expect(validators.phone('01380013800')).toBe(false);
    expect(validators.json('{"a":1}')).toBe(true);
    expect(validators.json('{a:1}')).toBe(false);
  });

  it('url 只要求「可被 URL 解析」，非 http 协议同样放行（调用方需另做白名单）', () => {
    expect(validators.url('ftp://x')).toBe(true);
    expect(validators.url('htp://x')).toBe(true);
  });
});

describe('validators.username', () => {
  it('限定字符集与 3-50 长度', () => {
    expect(validators.username('ab')).toBe(false);
    expect(validators.username('abc')).toBe(true);
    expect(validators.username('a_b-c9')).toBe(true);
    expect(validators.username('中文用户')).toBe(false);
  });
});

describe('validators.password', () => {
  it('不足 8 位直接拒绝', () => {
    expect(validators.password('Ab1!')).toBe(false);
  });

  it('8 位以上且字符类型至少 3 种', () => {
    expect(validators.password('abcdefgh')).toBe(false);
    expect(validators.password('abcdefgH')).toBe(false);
    expect(validators.password('Abcdefg1')).toBe(true);
    expect(validators.password('Abcdef1!')).toBe(true);
  });
});

describe('validators.port', () => {
  it('范围 1-65535 且拒绝非数字', () => {
    expect(validators.port('0')).toBe(false);
    expect(validators.port('1')).toBe(true);
    expect(validators.port('8080')).toBe(true);
    expect(validators.port('65535')).toBe(true);
    expect(validators.port('65536')).toBe(false);
    expect(validators.port('')).toBe(false);
    expect(validators.port('abc')).toBe(false);
  });
});

describe('空间与文档名长度', () => {
  it('边界值', () => {
    expect(validators.spaceName(' ')).toBe(false);
    expect(validators.spaceName('x')).toBe(true);
    expect(validators.spaceName('  '.repeat(51))).toBe(false);
    expect(validators.documentName('')).toBe(false);
    expect(validators.spaceDescription('')).toBe(true);
    expect(validators.spaceDescription('x'.repeat(501))).toBe(false);
  });
});

describe('validateValue', () => {
  it('返回第一条失败消息', () => {
    expect(validateValue('ab', ['required', 'username'])).toBe(validationMessages.username);
  });

  it('全部通过返回空串', () => {
    expect(validateValue('abc123', ['required', 'username'])).toBe('');
  });

  it('支持对象规则覆盖默认消息', () => {
    expect(validateValue('ab', [{ name: 'username', message: '自定义提示' }])).toBe('自定义提示');
  });

  it('支持函数规则', () => {
    expect(validateValue('x', [(v) => v.length > 3])).toBe('输入不合法');
    expect(validateValue('xxxx', [(v) => v.length > 3])).toBe('');
  });
});

describe('getValidationMessage（旧版 API）', () => {
  it('已知字段返回消息，未知字段返回空串', () => {
    expect(getValidationMessage('email', 'bad')).toBe(validationMessages.email);
    expect(getValidationMessage('email', 'a@b.co')).toBe('');
    expect(getValidationMessage('unknownField', 'x')).toBe('');
  });
});

describe('长度工厂', () => {
  it('minLengthValidator / maxLengthValidator', () => {
    expect(minLengthValidator(3).fn('ab')).toBe(false);
    expect(minLengthValidator(3).fn('abc')).toBe(true);
    expect(minLengthValidator(3).message).toBe('至少 3 个字符');
    expect(maxLengthValidator(2).fn('abc')).toBe(false);
    expect(maxLengthValidator(2, '太长了').message).toBe('太长了');
  });
});
