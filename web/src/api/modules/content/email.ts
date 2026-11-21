import { get, post, put, del } from '../../client';
import type { PaginatedResponse, PaginatedParams } from '../common';
import type { EmailServer } from '@/types/models';

export interface CreateEmailRequest {
  name: string;
  sender_address: string;
  sender_name?: string;
  sender_title_prefix?: string;
  host: string;
  port: string;
  username: string;
  password: string;
  is_ssl: number;
}

export interface UpdateEmailRequest {
  name: string;
  sender_address: string;
  sender_name?: string;
  sender_title_prefix?: string;
  host: string;
  port: string;
  username: string;
  password?: string;
  is_ssl: number;
}

export interface TestEmailRequest extends CreateEmailRequest {
  to_emails: string;
}

export function getEmails(params?: PaginatedParams) {
  return get<PaginatedResponse<EmailServer>>('/emails', { params });
}

export function createEmail(data: CreateEmailRequest) {
  return post<EmailServer>('/emails', data);
}

export function updateEmail(id: number, data: UpdateEmailRequest) {
  return put<EmailServer>(`/emails/${id}`, data);
}

export function deleteEmail(id: number) {
  return del<void>(`/emails/${id}`);
}

export function setDefaultEmail(id: number) {
  return put<void>(`/emails/${id}/default`);
}

export function testEmail(data: TestEmailRequest) {
  return post<void>('/emails/test', data);
}
