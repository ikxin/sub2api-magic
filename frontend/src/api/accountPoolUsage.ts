import { apiClient } from './client'
import type { AccountPlatform, AccountType, FetchOptions, WindowStats } from '@/types'

export type AccountPoolStatus =
  | 'active'
  | 'inactive'
  | 'expired'
  | 'rateLimited'
  | 'overloaded'
  | 'tempUnschedulable'
  | 'quotaExceeded'
  | 'error'
export type AccountPoolUsageWindowKind =
  | 'fiveHour'
  | 'sevenDay'
  | 'sevenDayFable'
  | 'thirtyDay'
  | 'localDaily'
  | 'localWeekly'
  | 'localTotal'

export interface AccountPoolUsage {
  id: number
  name: string
  platform: AccountPlatform
  type: AccountType
  plan_type?: string
  privacy_mode?: string
  expires_at?: string
  concurrency: number
  current_concurrency?: number
  status: AccountPoolStatus
  status_code?: number
  recovers_at?: string
  priority: number
  reset_credits?: {
    available_count: number
    expirations?: string[]
  }
  windows?: AccountPoolUsageWindow[]
}

export interface AccountPoolUsageWindow {
  kind: AccountPoolUsageWindowKind
  utilization: number
  utilization_unknown?: boolean
  resets_at?: string
  window_stats?: WindowStats
  cost_limit?: {
    used: number
    limit: number
  }
}

export async function listAccountPoolUsage(options?: FetchOptions): Promise<AccountPoolUsage[]> {
  const { data } = await apiClient.get<AccountPoolUsage[]>('/usage/account-pool', {
    signal: options?.signal
  })
  return data
}
