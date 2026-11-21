import { get, post } from '../../client';
import type { PaginatedResponse } from '../common';

/**
 * 文档审批流 API（后端 internal/api/workspace/approval.go）
 * 单线性审批：一个文档同一时刻只有一条 pending 记录。
 */

/** 审批状态：pending / approved / rejected */
export type ApprovalStatus = 'pending' | 'approved' | 'rejected';

/** 审批处理决定：approve / reject（后端 binding oneof 约束） */
export type ApprovalDecision = 'approve' | 'reject';

/** 审批链类型：sequential / parallel（后端模型字段，响应体未透传，仅类型占位） */
export type ApprovalChainType = 'sequential' | 'parallel';

/** 审批记录（对应 workspacemodel.ApprovalResponse） */
export interface ApprovalRecord {
  id: number;
  document_id: number;
  space_id: number;
  /** 提交人用户 ID */
  submitter: number;
  /** 当前审批人用户 ID */
  approver: number;
  status: ApprovalStatus;
  comment: string;
  /** Unix 秒，0 表示未处理 */
  submit_time: number;
  handle_time: number;
}

/** 提交审批请求（对应 workspacemodel.ApprovalRequest，document_id/approver 必填） */
export interface SubmitApprovalRequest {
  document_id: number;
  approver: number;
  comment?: string;
}

/** 处理审批请求（对应 workspacemodel.ApprovalHandleRequest，decision 必填） */
export interface HandleApprovalRequest {
  decision: ApprovalDecision;
  comment?: string;
}

/** 提交审批：POST /documents/approvals */
export function submitApproval(data: SubmitApprovalRequest) {
  return post<ApprovalRecord>('/documents/approvals', data);
}

/** 处理审批（通过/驳回）：POST /documents/approvals/:id/handle */
export function handleApproval(approvalId: number, data: HandleApprovalRequest) {
  return post<ApprovalRecord>(`/documents/approvals/${approvalId}/handle`, data);
}

/** 取文档最新一条审批记录：GET /documents/:id/approval（无记录时后端返回 404） */
export function getDocumentApproval(documentId: number) {
  return get<ApprovalRecord>(`/documents/${documentId}/approval`);
}

/** 待我审批：GET /documents/approvals/pending */
export function listPendingApprovals(params?: { page?: number; page_size?: number }) {
  return get<PaginatedResponse<ApprovalRecord>>('/documents/approvals/pending', { params });
}

/** 我提交的审批：GET /documents/approvals/submitted */
export function listSubmittedApprovals(params?: { page?: number; page_size?: number }) {
  return get<PaginatedResponse<ApprovalRecord>>('/documents/approvals/submitted', { params });
}
