import { ApiError, apiUrl, authorizedApiUrl, del, ensureValidToken, get, post, serverMessage } from '../../client';

// 对应 internal/service/admin/backup.go + internal/api/admin/backup.go
// 路由挂在 authGroup 下，最终前缀为 /api/v1（client.baseURL 已是 /api/v1）。

/** 备份元信息（internal/service/admin.BackupInfo） */
export interface BackupInfo {
  /** 文件名去掉 .zip 的 ID，同时是下载/恢复/删除的路径参数 */
  id: string;
  file_name: string;
  /** 字节数 */
  size: number;
  /** 后端已格式化的可读大小，如 "12.34 MB" */
  size_mb: string;
  /** Unix 秒级时间戳 */
  create_time: number;
  /** 后端已格式化的 "2006-01-02 15:04:05" */
  create_at: string;
}

/**
 * GET /backups 的 data。
 * 后端 ListBackups 在无备份时返回 nil slice，序列化为 null，消费方必须兜底。
 */
export interface BackupListResult {
  backups: BackupInfo[] | null;
  total: number;
}

export function listBackups() {
  return get<BackupListResult>('/backups');
}

/** POST /backups 无请求体（参数全部由后端从配置推导），返回新建备份的元信息 */
// 600MB 级备份打包可能远超 30s 全局超时，这几个写操作必须各自关掉超时
export function createBackup() {
  return post<BackupInfo>('/backups', undefined, { timeout: 0 });
}

/**
 * GET /backups/:id/download
 * 该端点直接回写 zip 二进制（Content-Disposition: attachment），不带响应信封。
 * 不走 axios：blob 会把 600MB 整个吃进 JS 内存且受 30s 全局超时约束，
 * 改为返回带 ?token= 的 URL 交给浏览器原生下载（auth 中间件支持 query 认证）。
 */
export async function backupDownloadUrl(id: string): Promise<string> {
  const token = await ensureValidToken();
  return authorizedApiUrl(`/backups/${id}/download`, token);
}

/**
 * POST /backups/:id/restore
 * 后端要求 query 参数 confirm=1 作为二次确认，缺失时返回 400「请确认恢复操作」。
 * 恢复要回灌整库 + 替换数据目录，同样必须关掉 30s 全局超时。
 */
export function restoreBackup(id: string) {
  return post<null>(`/backups/${id}/restore`, undefined, { params: { confirm: 1 }, timeout: 0 });
}

export function deleteBackup(id: string) {
  return del<null>(`/backups/${id}`);
}

/**
 * POST /backups/import —— multipart 上传外部备份包（zip，须含 database.sql）。
 * 用 XHR 而非 axios：需要上传进度回调，且大包上传不能受全局 30s 超时约束。
 */
export function importBackup(
  file: File,
  onProgress?: (percent: number) => void,
): Promise<BackupInfo> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', apiUrl('/backups/import'));
    // 大文件上传不设超时；错误处理与 client.ts 的信封约定保持一致
    xhr.timeout = 0;
    xhr.responseType = 'json';

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) {
        onProgress(Math.round((e.loaded / e.total) * 100));
      }
    };

    xhr.onload = async () => {
      const body = (xhr.response ?? null) as Record<string, unknown> | null;
      if (xhr.status >= 200 && xhr.status < 300 && body && body.code === 0) {
        resolve(body.data as BackupInfo);
        return;
      }
      // 401 时先试着刷新令牌重传一次，与 axios 拦截器的刷新语义对齐
      if (xhr.status === 401 && (await ensureValidToken())) {
        importBackup(file, onProgress).then(resolve, reject);
        return;
      }
      reject(new ApiError((body && serverMessage(body)) || `上传失败（HTTP ${xhr.status}）`, {
        status: xhr.status,
        code: typeof body?.code === 'number' ? body.code : undefined,
      }));
    };
    xhr.onerror = () => reject(new ApiError('网络错误，上传失败'));
    xhr.ontimeout = () => reject(new ApiError('上传超时'));

    void ensureValidToken().then((token) => {
      if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);
      const form = new FormData();
      form.append('file', file);
      xhr.send(form);
    });
  });
}
