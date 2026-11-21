import { describe, expect, it } from 'vitest';
import i18n from '@/i18n';
import { isTransientAuthFailure, serverMessage } from './client';

// 认证入口挂的是 10 次/分钟/IP 的严格限流。一次限流命中就 forceLogout，
// 等于把"服务端让你慢点"当成"凭证无效"把人踢下线——所以哪些状态码算瞬时，
// 是一条要钉住的边界，不只是 refreshAccessToken 里的一处 if。
describe('isTransientAuthFailure', () => {
  it('限流与服务端抖动不证明凭证失效', () => {
    expect(isTransientAuthFailure(429)).toBe(true);
    expect(isTransientAuthFailure(500)).toBe(true);
    expect(isTransientAuthFailure(502)).toBe(true);
    expect(isTransientAuthFailure(503)).toBe(true);
  });

  it('凭证类失败与未知状态仍按失效处理', () => {
    expect(isTransientAuthFailure(401)).toBe(false);
    expect(isTransientAuthFailure(403)).toBe(false);
    expect(isTransientAuthFailure(499)).toBe(false);
    expect(isTransientAuthFailure(undefined)).toBe(false);
  });
});

// 后端把中文放在 message、把稳定的键放在 message_key（表在 internal/response/messages.go）。
// 这里是全站唯一把这句话换成界面语言的地方 —— 两百多个显示 err.message 的调用点
// 全靠它，所以"认得键就换、认不出就退回原文"这条必须钉住。
describe('serverMessage', () => {
  const withLocale = <T>(locale: 'zh-CN' | 'en-US', run: () => T): T => {
    const before = i18n.global.locale.value;
    i18n.global.locale.value = locale;
    try {
      return run();
    } finally {
      i18n.global.locale.value = before;
    }
  };

  it('英文界面下按 message_key 取英文', () => {
    withLocale('en-US', () => {
      expect(serverMessage({ message: '空间名已存在', message_key: 'space_name_taken' }))
        .toBe('A space with that name already exists');
    });
  });

  it('中文界面下取中文（同一条键，两种界面语言各说各的话）', () => {
    withLocale('zh-CN', () => {
      expect(serverMessage({ message: '空间名已存在', message_key: 'space_name_taken' }))
        .toBe('空间名已存在');
    });
  });

  it('键没登记时退回服务端原文，而不是把键名印到界面上', () => {
    // 这一支是"安静失败"的入口：vue-i18n 对缺失键会返回键名本身（server.xxx），
    // 那比中文更怪，所以走 te() 先探一次。
    withLocale('en-US', () => {
      expect(serverMessage({ message: '某句后端新加的中文', message_key: 'not_registered_yet' }))
        .toBe('某句后端新加的中文');
    });
  });

  it('句子与原因分两格：英文界面拿到英文句子 + 原样原因', () => {
    // 回归的是这批要修的那个形状："参数错误: Key: 'name' Error:..." 拼成一句，
    // 英文界面就永远读到半中半英的串（而且翻不干净）。
    withLocale('en-US', () => {
      expect(serverMessage({ message: '参数错误', message_key: 'parameter_error', message_detail: "name: required" }))
        .toBe('Parameter error (name: required)');
    });
    withLocale('zh-CN', () => {
      expect(serverMessage({ message: '参数错误', message_key: 'parameter_error', message_detail: "name: required" }))
        .toBe('参数错误（name: required）');
    });
  });

  it('没有 message_key 时行为与从前一致', () => {
    expect(serverMessage({ message: 'plain' })).toBe('plain');
    expect(serverMessage({ error: { message: 'nested' } })).toBe('nested');
    expect(serverMessage(null)).toBe('请求失败');
  });
});
