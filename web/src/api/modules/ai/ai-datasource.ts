import { get, post, put, del } from '../../client';

// P1-4: 字段与后端 model/ai/datasource.go 对齐（password 为 json:"-" 永不下发，status 不存在，激活字段是 is_active）
export interface DataSource {
  id: number;
  name: string;
  space_id: number;
  db_type: string;
  host: string;
  port: number;
  db_name: string;
  username: string;
  sync_cron?: string;
  last_sync_at: string | null;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface DataSourceCreateRequest {
  name: string;
  db_type: string;
  host: string;
  port: number;
  db_name: string;
  username: string;
  password: string;
  space_id: number;
  sync_cron?: string;
  is_active?: boolean;
}

// P1-16: 后端 Update 复用 CreateDataSourceRequest，db_type/space_id/password 等字段均为 binding:"required"，
// 此前刻意省略这些字段的编辑请求必被 400 拒绝。前端因此要求编辑时全量回传（密码需重输）。
export type DataSourceUpdateRequest = DataSourceCreateRequest;

export interface TestConnectionRequest {
  db_type: string;
  host: string;
  port: number;
  db_name: string;
  username: string;
  password: string;
}

// P1-4: 后端 List/Get/Create/Update 直接返回裸数组/裸对象（response.Success(c, list/ds)），
// 此前声明的 { datasources }/{ datasource } 信封与返回值不符，导致列表恒为空。
// List 按 space_id 精确过滤，必须显式传参。
export function listDataSources(spaceId: number) {
  return get<DataSource[]>('/datasources', { params: { space_id: spaceId } });
}

export function createDataSource(data: DataSourceCreateRequest) {
  return post<DataSource>('/datasources', data);
}

export function updateDataSource(id: number, data: DataSourceUpdateRequest) {
  return put<DataSource>(`/datasources/${id}`, data);
}

export function deleteDataSource(id: number) {
  return del<void>(`/datasources/${id}`);
}

// P1-76: 成功时后端返回 { message: "连接成功" }（无 success 字段），失败走 400 拒绝；
// 旧代码按 res.success 判断导致成功也弹红色提示。
export function testConnection(data: TestConnectionRequest) {
  return post<{ message: string }>('/datasources/test-connection', data);
}

export function syncDataSource(id: number) {
  return post<{ message: string; table_count: number }>(`/datasources/${id}/sync`);
}
