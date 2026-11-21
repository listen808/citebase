import { get, post, put, del } from '../../client';

/**
 * 文档模板 API（G015）
 * 对应后端 internal/api/workspace/template.go（路由 internal/api/workspace/routes.go:97-101）
 * 前缀 /api/v1 已在 client baseURL 中；{code,message,data} 信封由 api/client.ts 自动拆包。
 */

/** 模板（后端 TemplateResponse，与 DocumentTemplate 字段一致） */
export interface DocumentTemplate {
  id: number;
  /** 0 表示全局模板（仅创建者与超管可改删） */
  space_id: number;
  /** 最长 100，与 space_id 组成唯一索引 uk_tpl_space_name */
  name: string;
  /** 最长 255 */
  description: string;
  /** Markdown 正文 */
  content: string;
  create_user_id: number;
  create_time: number;
  update_time: number;
}

export interface CreateTemplateRequest {
  /** 0 表示全局模板；>0 时后端校验调用者对该空间有编辑权 */
  space_id?: number;
  name: string;
  description?: string;
  content?: string;
}

/**
 * 更新为增量语义：name / description / content 传空字符串时后端会跳过该字段，
 * 因此无法通过本接口清空描述或正文（只能删除后重建）。
 */
export interface UpdateTemplateRequest {
  name?: string;
  description?: string;
  content?: string;
}

/** response.Paginated 拆包后的分页结构 */
export interface TemplateListData {
  data: DocumentTemplate[];
  total: number;
  page: number;
  page_size: number;
}

/**
 * 模板列表（后端 SQL 为 `space_id = ? OR space_id = 0`）：
 * space_id=0 → 仅全局模板；space_id>0 → 该空间 + 全局模板。没有「全部空间」查询。
 */
export function listTemplates(params: { space_id?: number; page?: number; page_size?: number } = {}) {
  return get<TemplateListData>('/templates', { params });
}

/** 模板详情 */
export function getTemplate(id: number) {
  return get<DocumentTemplate>(`/templates/${id}`);
}

/** 创建模板 */
export function createTemplate(data: CreateTemplateRequest) {
  return post<DocumentTemplate>('/templates', data);
}

/** 更新模板（仅创建者 / 空间管理员 / 超管） */
export function updateTemplate(id: number, data: UpdateTemplateRequest) {
  return put<{ id: number }>(`/templates/${id}`, data);
}

/** 删除模板（仅创建者 / 空间管理员 / 超管） */
export function deleteTemplate(id: number) {
  return del<{ id: number }>(`/templates/${id}`);
}
