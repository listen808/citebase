import { get } from '../../client';
import type { DashboardStats, ServerStatus } from '@/types/models';

export function getDashboardStats(days: number = 30) {
  return get<DashboardStats>('/stats/dashboard', { params: { days } });
}

export function getServerStatus() {
  return get<ServerStatus>('/stats/server');
}
