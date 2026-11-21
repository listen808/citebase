import { get } from '../../client';

/**
 * 文档多视图（列表 / 看板 / 画廊）
 * 对应后端 internal/api/workspace/document_view.go
 *
 * 注意：后端 flattenTree 只依据 published_at>0 推导 status（int 0=草稿 1=已发布），
 * 不返回 model 里的 string status；created_by / creator_name / publish_time 三个字段
 * 后端声明了但从未赋值，因此恒为空，UI 不应依赖它们。
 */

/** 视图接口返回的扁平文档项 */
export interface DocumentViewItem {
  id: number;
  name: string;
  /** 1 页面 / 2 目录 */
  type: number;
  space_id: number;
  space_name?: string;
  /** 0 草稿 / 1 已发布 */
  status: number;
  parent_id: number;
  sequence: number;
  update_time: number;
}

/** GET /documents/view/list 为分页信封 {data,total,page,page_size} */
export interface DocumentViewListPage {
  data: DocumentViewItem[];
  total: number;
  page: number;
  page_size: number;
}

/** GET /documents/view/kanban 按发布状态分组 */
export interface DocumentKanbanGroups {
  draft: DocumentViewItem[];
  published: DocumentViewItem[];
}

export interface DocumentViewListParams {
  space_id: number;
  /** -1 全部（默认）/ 0 草稿 / 1 已发布 */
  status?: -1 | 0 | 1;
  page?: number;
  page_size?: number;
}

export function getDocumentListView(params: DocumentViewListParams) {
  return get<DocumentViewListPage>('/documents/view/list', { params });
}

export function getDocumentKanbanView(spaceId: number) {
  return get<DocumentKanbanGroups>('/documents/view/kanban', { params: { space_id: spaceId } });
}

export function getDocumentGalleryView(spaceId: number) {
  return get<DocumentViewItem[]>('/documents/view/gallery', { params: { space_id: spaceId } });
}
