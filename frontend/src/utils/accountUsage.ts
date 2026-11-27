import type { UsageProgress, WindowStats } from '@/types'

export function hasWindowStats(stats?: WindowStats | null): boolean {
  return !!stats && (stats.requests > 0 || stats.tokens > 0 || stats.cost > 0)
}

export function estimateWindowTotalCost(
  window?: Pick<UsageProgress, 'utilization' | 'utilization_unknown' | 'window_stats'> | null
): number | null {
  const utilization = window?.utilization
  const currentCost = window?.window_stats?.cost
  if (
    window?.utilization_unknown ||
    typeof utilization !== 'number' ||
    typeof currentCost !== 'number' ||
    !Number.isFinite(utilization) ||
    !Number.isFinite(currentCost) ||
    utilization <= 0 ||
    currentCost <= 0
  ) {
    return null
  }

  const estimate = (currentCost * 100) / utilization
  return Number.isFinite(estimate) && estimate > 0 ? estimate : null
}
