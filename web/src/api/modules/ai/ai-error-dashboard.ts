import client from '@/api/client'

export const ErrorDashboardAPI = {
  summary(params: { space_id: number }) {
    return client.get('/ai/data-query/error-summary', { params })
  },
  recentFailures(params: { space_id: number; limit?: number }) {
    return client.get('/ai/data-query/recent-failures', { params })
  }
}
