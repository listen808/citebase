import { get, post } from '../../client';

export interface InstallState {
  db_connected: boolean;
  tables_exist: boolean;
  admin_exists: boolean;
  system_version: string;
}

export interface AdminForm {
  username: string;
  given_name: string;
  email: string;
  password: string;
}

// 健康检查
export function healthCheck() {
  return get<{ status: string }>('/health');
}

export async function isBackendHealthy(): Promise<boolean> {
  try {
    const res = await healthCheck();
    return res.status === 'UP';
  } catch {
    return false;
  }
}

// 获取安装状态
export function getInstallState() {
  return get<InstallState>('/install/state');
}

// 初始化数据库
export function initDatabase() {
  return post<void>('/install/init-db', {});
}

// 初始化系统（创建管理员）
export function initSystem(data: AdminForm) {
  return post<void>('/install/init-system', data);
}
