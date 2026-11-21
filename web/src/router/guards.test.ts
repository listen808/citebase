// 身份失败的分类：只有 401/403 算「凭据被否」，其余一律算「暂时不知道你是谁」。
// 这条判断决定要不要清掉还有效的 token，出错就会把值守的人踢下线，所以单独钉住。
import { describe, expect, it } from 'vitest';
import { isCredentialRejected } from './guards';

describe('isCredentialRejected', () => {
  it('服务端明确否定凭据的状态码判定为失效', () => {
    expect(isCredentialRejected(401)).toBe(true);
    expect(isCredentialRejected(403)).toBe(true);
  });

  it('后端忙、超时、断网都不该清会话', () => {
    for (const status of [429, 500, 502, 503, 504, 408, 0]) {
      expect(isCredentialRejected(status), `${status} 不是凭据问题`).toBe(false);
    }
  });

  it('拿不到状态码（网络层抛错）按可恢复处理', () => {
    expect(isCredentialRejected(undefined)).toBe(false);
    expect(isCredentialRejected(null)).toBe(false);
  });

  it('业务码 403 之外的 4xx 不连带误伤', () => {
    expect(isCredentialRejected(404)).toBe(false);
    expect(isCredentialRejected(422)).toBe(false);
  });
});
