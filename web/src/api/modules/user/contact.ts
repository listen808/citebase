import { get, post, put, del } from '../../client';

export interface Contact {
  id: number;
  name: string;
  mobile: string;
  email: string;
  position: string;
  department?: string;
  create_time?: number;
}

export function getContacts(page = 1, pageSize = 20, keyword = '') {
  return get<{ data: Contact[]; total: number; page: number; page_size: number }>('/contacts', {
    params: { page, page_size: pageSize, keyword },
  });
}

export interface SaveContactRequest {
  name: string;
  email: string;
  mobile?: string;
  position?: string;
  department?: string;
}

export function saveContact(data: SaveContactRequest) {
  return post<Contact>('/contacts', data);
}

export function updateContact(id: number, data: SaveContactRequest) {
  return put<Contact>(`/contacts/${id}`, data);
}

export function deleteContact(id: number) {
  return del<void>(`/contacts/${id}`);
}

