import axios, { AxiosError, type AxiosInstance, type AxiosRequestConfig } from 'axios';
import { storageGet, storageSet, storageRemove, StorageKeys } from '@/utils/core/storage';
import i18n from '@/i18n';

// 根据当前页面协议/主机自动推断后端地址
// 开发时前端在 :3000，后端在 :3001，需要代理或显式配置
// 生产环境通常同域部署，使用相对路径即可
function getBaseURL(): string {
  // 如果显式配置了环境变量，优先使用
  if (import.meta.env.VITE_API_BASE_URL) {
    return import.meta.env.VITE_API_BASE_URL;
  }
  // 开发环境：使用相对路径，通过 Vite 代理转发到后端
  // 生产环境：直接使用相对路径
  return '/api/v1';
}

const PUBLIC_PATHS = new Set(['/install/state', '/health', '/site', '/author/login', '/author/config']);
// P1-1: 认证类端点不参与 401 自动刷新分支，避免 refresh 请求自身 401 时等待自己（refreshPromise 自死锁）
const AUTH_NO_RETRY_PATHS = new Set(['/auth/login', '/auth/refresh', '/auth/logout']);

// P1-2: 统一错误类型：携带 HTTP status 与业务 code，调用方可按状态码分支（如 409 冲突、403/404 区分）
export class ApiError extends Error {
  status?: number;
  code?: number;
  /** 限流时服务端 Retry-After 给的秒数；取不到就留空，由界面按默认值退避 */
  retryAfter?: number;
  constructor(message: string, opts?: { status?: number; code?: number; retryAfter?: number }) {
    super(message);
    this.name = 'ApiError';
    this.status = opts?.status;
    this.code = opts?.code;
    this.retryAfter = opts?.retryAfter;
  }
}

// 兼容旧调用点：从错误对象提取 HTTP 状态码
export function getApiErrorStatus(err: unknown): number | undefined {
  if (err instanceof ApiError) return err.status;
  const e = err as { status?: number; response?: { status?: number } };
  return e?.response?.status ?? e?.status;
}
/** 从错误对象里取服务端给的 Retry-After（秒）；取不到返回 undefined。 */
export function getApiRetryAfter(err: unknown): number | undefined {
  return err instanceof ApiError ? err.retryAfter : undefined;
}

/**
 * 「等一下再试」不等于「页面坏了」。429 与 5xx 都是服务暂时不可用 ——
 * 把整页崩成「渲染内容时发生了未知错误」既吓人也无用，
 * 所以 ErrorBoundary 拿到这类错误时走温和的那一支（带倒计时重试）。
 */
export function isTransientApiError(err: unknown): boolean {
  return isTransientAuthFailure(getApiErrorStatus(err));
}

const client: AxiosInstance = axios.create({
  baseURL: getBaseURL(),
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Token 刷新共享 Promise，替代 localStorage 轮询方案
// 多个并发 401 共享同一个刷新请求，避免死循环
let refreshPromise: Promise<unknown> | null = null;

function extractErrorMessage(data: unknown): string {
  if (!data || typeof data !== 'object') return '请求失败';
  const d = data as Record<string, unknown>;
  if ('message' in d && d.message) {
    return typeof d.message === 'string' ? d.message : String(d.message);
  }
  if ('error' in d && d.error) {
    if (typeof d.error === 'string') return d.error;
    const errObj = d.error as Record<string, unknown>;
    return errObj.message ? String(errObj.message) : String(d.error);
  }
  if ('msg' in d && d.msg) return String(d.msg);
  return '请求失败';
}

/**
 * 后端说给人听的那句话 → 当前界面语言的版本。
 *
 * 契约是两个字段：`message_key` 是**句子**（可翻译，表在 internal/response/messages.go），
 * `message_detail` 是**原因**（哪个字段没填对、冲突的是哪个名字、驱动报了什么，不参与翻译）。
 * 分成两格而不是拼成一句：拼起来英文界面就会读到 "Parameter error: Key: 'name' Error:..."
 * 这种半中半英的串，而且永远翻不干净。
 *
 * 为什么全站只有这一处：全站有两百多个地方显示 err.message（toast、报错页、登录页错误位、
 * ErrorBoundary），逐个调用点改过去一定会漏，而漏掉的恰好是没人手动验证的少见分支
 * —— 限流、并发初始化、越权。
 *
 * 退回中文是刻意的：少一条译文不该让界面出现空白或异常，但也不能没人发现 ——
 * 键集双向核对与"生成物是否过期"由 web/tests/server-messages.test.ts 拦。
 */
export function serverMessage(data: unknown): string {
  const d = (data && typeof data === 'object' ? data : {}) as Record<string, unknown>;
  const key = typeof d.message_key === 'string' ? d.message_key : '';
  const detail = typeof d.message_detail === 'string' ? d.message_detail.trim() : '';
  if (key) {
    const path = `server.${key}`;
    // te() 而不是直接 t()：vue-i18n 对缺失键会返回键名本身（server.xxx）并打一条警告，
    // 那比一句中文更怪。
    if (i18n.global.te(path)) {
      const sentence = String(i18n.global.t(path));
      if (!detail) return sentence;
      // 括号由语言决定（中文全角、英文半角），所以连接方式本身也走语言包，不在这里拼字符串。
      return String(i18n.global.t('chrome.errorWithDetail', { message: sentence, detail }));
    }
  }
  return extractErrorMessage(data);
}

// 认证请求失败里，哪些只是"这会儿问不出结果"、不证明 refresh token 失效，因此不许清登录态：
// 429 来自认证入口的 10 次/分钟限流（多标签页、弱网重试、批量巡检都会踩到），5xx 是服务端抖动。
export function isTransientAuthFailure(status?: number): boolean {
  return status === 429 || (status !== undefined && status >= 500);
}

async function refreshAccessToken(refreshToken: string): Promise<string> {
  return client.post('/auth/refresh', { refresh_token: refreshToken })
    .then(async (res) => {
      const data = res.data as { access_token: string; expires_in?: number };
      const newExpiresAt = Date.now() + (data.expires_in || 3600) * 1000;
      storageSet(StorageKeys.AUTH_TOKEN, data.access_token);
      storageSet(StorageKeys.AUTH_EXPIRES_AT, String(newExpiresAt));
      try {
        const { useAuthStore } = await import('@/stores/auth/auth');
        const auth = useAuthStore();
        auth.token = data.access_token;
        auth.tokenExpiresAt = newExpiresAt;
      } catch { /* store may not be ready during init */ }
      return data.access_token;
    })
    .catch((err: unknown) => {
      // 限流/服务端抖动原样抛回调用方，登录态不动；只有凭证类失败才登出。
      const status = getApiErrorStatus(err);
      if (isTransientAuthFailure(status)) {
        return Promise.reject(err instanceof ApiError
          ? err
          : new ApiError('认证服务暂时不可用，请稍后再试', { status }));
      }
      forceLogout();
      // P1-2: 携带 401 状态，调用方可按状态码分支
      return Promise.reject(new ApiError('登录已过期，请重新登录', { status: 401 }));
    })
    .finally(() => {
      refreshPromise = null;
    });
}

async function forceLogout() {
  try {
    const { useAuthStore } = await import('@/stores/auth/auth');
    const auth = useAuthStore();
    auth.logout();
  } catch {
    storageRemove(StorageKeys.AUTH_TOKEN);
    storageRemove(StorageKeys.AUTH_REFRESH);
    storageRemove(StorageKeys.AUTH_EXPIRES_AT);
  }
}

// 请求拦截器：直接从 localStorage 读取 token（保证完全可靠）
client.interceptors.request.use(
  (config) => {
    const token = storageGet(StorageKeys.AUTH_TOKEN);
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }

    // 如果是 FormData，清除 Content-Type 让浏览器自动设置 multipart/form-data 带 boundary
    if (config.data instanceof FormData) {
      config.headers['Content-Type'] = undefined as unknown as string;
    }

    return config;
  },
  (error) => Promise.reject(error)
);

// 响应拦截器：统一解包 + 错误处理
client.interceptors.response.use(
  (response) => {
    const payload = response.data as Record<string, unknown>;
    if (payload && typeof payload === 'object') {
      // 格式1：后端统一响应 { code: 0, data: ..., message: ... }
      if ('code' in payload) {
        if (payload.code !== 0) {
          // P1-2: 业务错误也携带 status/code，不再压扁成纯 Error
          // 2xx 里夹业务错误码是这套封套的另一半：后端没走 HTTP 状态，message_key 一样要认。
          return Promise.reject(new ApiError(serverMessage(payload) || '请求失败', { status: response.status, code: payload.code as number }));
        }
        response.data = payload.data;
        return response;
      }

      // 格式2：部分后端 API 直接返回 { data: ... }（无 code 字段）
      // 只对纯 data 响应解包；保留分页格式 { data, total, page }
      if ('data' in payload) {
        const keys = Object.keys(payload);
        const extraKeys = keys.filter((k) => !['data', 'message'].includes(k));
        if (extraKeys.length === 0) {
          response.data = payload.data ?? undefined;
        }
      }
    }
    return response;
  },
  (error: AxiosError) => {
    if (error.response) {
      const status = error.response.status;
      // 后端带 message_key 时这里就已是界面语言的文案（见 serverMessage）
      const msg = serverMessage(error.response.data);

      if (status === 401) {
        // P1-1: /auth/refresh、/auth/logout 等认证端点的 401 直接进入拒绝路径，
        // 否则刷新请求自身 401 会去 await 尚未 settle 的 refreshPromise，形成自等待死锁且不登出
        const url = error.config?.url ?? '';
        const isAuthNoRetry = AUTH_NO_RETRY_PATHS.has(url);
        const isPublicRequest = !!error.config?.url && PUBLIC_PATHS.has(error.config.url);
        if (isAuthNoRetry || isPublicRequest) {
          return Promise.reject(new ApiError(msg || '用户名或密码错误', { status }));
        }

        const isInitRequest = error.config?.url === '/auth/me';
        if (!isInitRequest) {
          const refreshToken = storageGet(StorageKeys.AUTH_REFRESH);
          if (refreshToken) {
            if (!refreshPromise) {
              const thisRefresh = refreshAccessToken(refreshToken);
              refreshPromise = thisRefresh;
            }

            return refreshPromise.then((newToken: unknown) => {
              const token = newToken as string;
              if (error.config) {
                error.config.headers.Authorization = `Bearer ${token}`;
                const retryCount = (error.config as unknown as Record<string, unknown>).__retryCount as number || 0;
                if (retryCount >= 1) {
                  return Promise.reject(new ApiError('Token 刷新后仍认证失败，请重新登录', { status }));
                }
                (error.config as unknown as Record<string, unknown>).__retryCount = retryCount + 1;
                return client.request(error.config);
              }
              return Promise.reject(new ApiError('请求配置丢失', { status }));
            }).catch((err: Error) => {
              return Promise.reject(err);
            });
          }

          forceLogout();
        }
        return Promise.reject(new ApiError('登录已过期，请重新登录', { status }));
      }

      if (status === 403) {
        return Promise.reject(new ApiError(`权限不足: ${msg}`, { status }));
      }

      if (status === 429) {
        const retryAfterRaw = Number.parseInt(String(error.response?.headers?.['retry-after'] ?? ''), 10);
        // 后面的话术交回服务端：限流有三档（全局/严格/刷新），它们是不同的句子，
        // 前端硬写一句"操作过于频繁"会把"刷新过于频繁"覆盖掉。
        // 这里**不再往消息里塞 `[rate_limit]` 前缀**：那是给识别用的内部判别串，
        // 而消息会被 ErrorBoundary 当"原因"格子直接印到界面上（实测截图里就是
        // `[rate_limit]请求过于频繁，请稍后再试`）。限流这件事本来就写在 status 里，
        // 消费方（useSpaceInit）改读 status —— 一个事实只留一个载体。
        const serverText = serverMessage(error.response.data);
        const tail = serverText === '请求失败' ? '' : serverText;
        return Promise.reject(new ApiError(tail || '操作过于频繁，请稍后再试', { status, retryAfter: Number.isFinite(retryAfterRaw) && retryAfterRaw > 0 ? retryAfterRaw : undefined }));
      }

      return Promise.reject(new ApiError(`${msg}`, { status }));
    }
    return Promise.reject(error);
  }
);

export default client;

// ensureValidToken 在调用前主动检查 token 是否即将过期，过期则用 refresh token 换新。
// P0-8: 用于 streamAsk 等 fetch 直连场景，避免绕过 axios 拦截器导致 401 后无自动刷新。
// 返回最新可用的 access token；未登录或刷新失败时返回 null（调用方应视为未授权）。
export async function ensureValidToken(): Promise<string | null> {
  const token = storageGet(StorageKeys.AUTH_TOKEN);
  if (!token) return null;

  const expiresAtStr = storageGet(StorageKeys.AUTH_EXPIRES_AT);
  if (!expiresAtStr) return token;

  const expiresAt = Number(expiresAtStr);
  // 提前 60 秒判定为过期，避免边界竞态
  if (Date.now() < expiresAt - 60_000) {
    return token;
  }

  const refreshToken = storageGet(StorageKeys.AUTH_REFRESH);
  if (!refreshToken) {
    await forceLogout();
    return null;
  }

  if (!refreshPromise) {
    refreshPromise = refreshAccessToken(refreshToken);
  }
  try {
    return await (refreshPromise as Promise<string>);
  } catch {
    return null;
  }
}

export function get<T>(url: string, config?: AxiosRequestConfig) {
  return client.get<T>(url, config).then((res) => res.data);
}

export function post<T>(url: string, data?: unknown, config?: AxiosRequestConfig) {
  return client.post<T>(url, data, config).then((res) => res.data);
}

export function put<T>(url: string, data?: unknown, config?: AxiosRequestConfig) {
  return client.put<T>(url, data, config).then((res) => res.data);
}

export function del<T>(url: string, config?: AxiosRequestConfig) {
  return client.delete<T>(url, config).then((res) => res.data);
}

/**
 * 与 axios 实例同一口径的 URL 拼接，给不走 axios 的链路用（浏览器原生下载、XHR 上传）。
 * 这些链路存在的理由就是 30s 全局超时装不下大文件，URL 不能再各抄各的。
 */
export function apiUrl(path: string): string {
  return getBaseURL() + path;
}

/**
 * 带令牌的直连下载 URL：交给浏览器原生下载（流式、可断点续传、不进 JS 内存）。
 * 后端 internal/middleware/auth.go 认 ?token= 查询参数，访问日志对该参数掩码。
 */
export function authorizedApiUrl(path: string, token: string | null): string {
  const url = apiUrl(path);
  if (!token) return url;
  return `${url}${url.includes('?') ? '&' : '?'}token=${encodeURIComponent(token)}`;
}
