/**
 * 业务模型类型定义
 * 与后端 model 保持一致
 */

export interface Document {
  id: number;
  parent_id: number;
  space_id: number;
  space_name?: string; // 空间名称（用于面包屑）
  name: string;
  content?: string;
  html_content?: string;
  type: number; // 1 page, 2 dir
  path: string;
  path_ids?: number[]; // 完整路径 ID 列表（搜索结果填充）
  sequence: number;
  icon?: string;           // 页面图标 (emoji)
  cover_image?: string;    // 封面图 URL
  // P1 修复：后端 DocumentResponse 不返回 created_by（实际用 create_user_id），改 optional
  created_by?: number;
  create_user_id: number;
  edit_user_id: number;
  editor_id?: number;
  is_delete: number;
  create_time: number;
  update_time: number;
  creator_name?: string;
  editor_name?: string;
  children?: Document[];
  has_children?: boolean;
  version?: number;
  // P0 修复：与后端 DocumentResponse 对齐
  status?: string;       // published / draft
  published_at?: number;
}

export interface DocumentVersion {
  id: number;
  document_id: number;
  version: number;
  content: string;
  change_log: string;
  created_by: number;
  created_at: number;
  creator_name?: string;
}

export interface Space {
  id: number;
  name: string;
  description?: string;
  tags?: string;
  visit_level: string; // 后端返回 string: "0"所有 "1"登录 "2"成员
  is_share?: number;   // 1=允许分享 0=禁止
  is_export?: number;  // 1=允许导出 0=禁止
  status?: number;
  created_by: number;
  is_delete: number;
  sequence?: number;
  create_time: number;
  update_time: number;
  member_count?: number;
  doc_count?: number;
  privilege?: number; // 当前用户在当前空间的权限
  creator_id?: number; // 创建者ID（后端返回 creator_id）
  creator_name?: string;
}

// P2 修复：与后端 admin.Privilege 对齐（补齐 parent_id/type/controller/icon/target 等字段）
export interface Privilege {
  id: number;
  name: string;
  parent_id: number;
  type: string;          // controller / menu
  controller: string;
  action: string;
  icon: string;
  target: string;
  is_display: number;    // 0 不显示 1 显示
  is_menu: number;
  sequence: number;
  list_order: number;
  is_delete: number;
  create_time: number;
  update_time: number;
}

export interface HighlightFragment {
  text: string;
  html: string;
  score: number;
}

export interface SearchResult {
  document_id?: number;
  space_id: number;
  space_name?: string;
  name: string;
  type: number;
  path?: string;
  parent_id?: number;
  path_ids?: number[];
  score: number;
  highlights: Record<string, string>;
  highlight_fragments?: Record<string, HighlightFragment[]>;
  match_count?: number;
  create_time: number;
  updated_at?: number;
  view_count?: number;
  _normScore?: number;
  score_detail?: {
    fulltext_score: number;
    vector_score: number;
    fusion_method: string;
  };
}

export interface BacklinkResult {
  source_doc_id: number;
  source_doc_name: string;
  space_id: number;
  space_name: string;
  link_text: string;
}

export interface ForwardLinkResult {
  target_doc_id: number;
  target_doc_name: string;
  space_id: number;
  space_name: string;
  link_text: string;
}

export interface Collection {
  id: number;
  user_id: number;
  document_id: number;
  document_name?: string;
  space_name?: string;
  create_time: number;
}

// P2 修复：与后端 social.Follow 对齐
// 旧版字段（user_id/target_id/target_type）已废弃，请使用 FollowTarget
export interface Follow {
  id: number;
  user_id: number;
  target_id: number;
  target_type: string; // 'user' | 'document'
  create_time: number;
}

export interface LogEntry {
  id: number;
  user_id: number;
  username?: string;
  action: string;
  target_type: string;
  target_id: number;
  detail?: string;
  create_time: number;
}

export interface HotDocument {
  document_id: number;
  name: string;
  space_name: string;
  count: number;
}

export interface DashboardStats {
  total_users: number;
  active_users: number;
  forbidden_users?: number;
  total_spaces: number;
  total_documents: number;
  today_documents?: number;
  indexed_documents?: number;
  index_status?: string;
  top_creator?: { user_id: number; username: string; given_name?: string; count: number };
  top_editor?: { user_id: number; username: string; given_name?: string; count: number };
  top_collector?: { user_id: number; username: string; given_name?: string; count: number };
  top_follower?: { user_id: number; username: string; given_name?: string; count: number };
  active_contributors?: Array<{ user_id: number; username: string; given_name?: string; count: number; last_time?: number; is_forbidden?: number; is_delete?: number }>;
  space_rank?: Array<{ space_id: number; space_name: string; total: number }>;
  document_trend?: Array<{ date: string; count: number }>;
  hot_documents?: HotDocument[];
}

export interface ServerStatus {
  local_ip: string;
  hostname: string;
  os: string;
  platform: string;
  platform_family: string;
  uptime: number;
  cpu_percent: number;
  memory_percent: number;
  memory_used: number;
  memory_total: number;
  disk_percent: number;
  disk_used: number;
  disk_total: number;
  network_percent: number;
  error_log_count: number;
}

export interface User {
  id: number;
  username: string;
  given_name: string;
  mobile: string;
  phone: string;
  email: string;
  department: string;
  position: string;
  location: string;
  im: string;
  avatar?: string;
  last_ip: string;
  last_time: number;
  role_id: number;
  role_name: string;
  is_forbidden: number;
  is_delete: number;
  create_time: number;
  update_time: number;
  following?: number;
  followers?: number;
  is_following?: boolean;
  permissions?: string[];
  /**
   * 账号级个性化偏好。缺省/null = **这个账号从没同步过**，前端要按这个区分"没配过"
   * 与"配的就是浅色实心" —— 后端把空值合成成默认档的话，每次登录都会盖掉本地选择。
   * 形状与取值判据见 utils/core/preference-sync.ts 与后端 model/user/preferences.go。
   */
  preferences?: { theme: string; material: string; glass_clear: boolean; updated_at: number } | null;
}

export interface Role {
  role_id: number;
  name: string;
  type: number;
  description: string;
  list_order: number;
  is_delete: number;
  create_time: number;
  update_time: number;
}

export interface SpaceMember {
  user_id: number;
  username: string;
  given_name?: string;
  privilege: number;
}

export interface SSOConfig {
  id: number;
  name: string;
  url: string;
  username_prefix: string;
  ext_data: string;
  is_used: number;
  create_time: number;
  update_time: number;
}

export interface EmailServer {
  id: number;
  name: string;
  sender_address: string;
  sender_name: string;
  sender_title_prefix: string;
  host: string;
  port: string;
  username: string;
  is_ssl: number;
  is_used: number;
  create_time: number;
  update_time: number;
}

export interface Link {
  id: number;
  name: string;
  url: string;
  sort: number;
  click_count?: number;
  create_time?: number;
}

// P2 修复：与后端 workspace.AttachmentResponse 对齐（补齐 download_url、source、media_type）
export interface Attachment {
  id: number;
  document_id: number;
  user_id: number;
  name: string;
  path: string;
  source: number;
  size: number;
  url?: string;
  download_url?: string;
  create_time?: number;
}

export interface UploadResult {
  id: number;
  document_id: number;
  name: string;
  url: string;
  path: string;
  size: number;
  create_time: number;
}

// 与后端 admin.PluginResponse 对齐：仅返回 id/plugin_key/title/description/cover_url/conf_value/is_used/created_at/updated_at。
export interface Plugin {
  id: number;
  plugin_key: string;
  title: string;
  description: string;
  cover_url: string;
  conf_value: string;
  is_used: number;
  created_at: string;
  updated_at: string;
}

export interface FollowTarget {
  id: number;
  user_id: number;
  type: number;
  object_id: number;
  create_time: number;
  target_user?: {
    user_id: number;
    username: string;
    given_name: string;
    email: string;
    avatar?: string;
  };
  target_doc?: {
    document_id: number;
    name: string;
    space_id: number;
    space_name?: string;
  };
}

export interface CollectionItem {
  id: number;
  type: number;
  resource_id: number;
  name?: string;
  space_id?: number;
  space_name?: string;
  create_time: number;
}

export interface CommentItem {
  id: number
  document_id: number
  parent_id: number
  resolved: number
  content: string
  selection_text?: string
  selection_range?: string
  creator_id: number
  creator_name: string
  created_at: number
  updated_at: number
  replies?: CommentItem[]
}

// P2 修复：补齐前端缺失类型定义

// 与后端 admin.ApiKey 对齐（明文 key 仅创建时返回一次，库内加密存储）
export interface ApiKey {
  id: number;
  app_name: string;
  owner_id: number;
  status: string;        // active / revoked
  rate_limit: number;     // 每分钟请求上限
  expires_at?: string;    // ISO 时间，空表示不过期
  last_used_at?: string;
  created_at: string;
  updated_at: string;
}

// 与后端 workspace.SavedSearchResponse 对齐
export interface SavedSearch {
  id: number;
  user_id: number;
  name: string;
  query: string;
  type: string;          // all / title / content
  space_id: number;
  tag_filter: string;
  sort_by: string;       // relevance / time_desc / time_asc
  is_public: number;
  use_count: number;
  create_time: number;
  update_time: number;
}

// 与后端 workspace.WebhookConfigResponse 对齐
export interface WebhookConfig {
  id: number;
  user_id: number;
  space_id: number;
  name: string;
  url: string;
  secret?: string;
  events: string;        // 逗号分隔：doc.created,doc.updated,doc.deleted,comment.created
  is_active: number;
  create_time: number;
  update_time: number;
}

export interface WebhookDelivery {
  id: number;
  webhook_id: number;
  event: string;
  status_code: number;
  response: string;
  success: number;
  retry_count: number;
  create_time: number;
}
