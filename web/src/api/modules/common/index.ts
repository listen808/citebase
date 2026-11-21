export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface PaginatedParams {
  page?: number;
  page_size?: number;
  keyword?: string;
}