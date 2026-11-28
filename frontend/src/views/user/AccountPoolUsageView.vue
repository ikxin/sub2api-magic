<template>
  <AppLayout>
    <h1 class="mb-4 text-lg font-semibold text-gray-900 dark:text-white lg:hidden">
      {{ t('nav.accountPoolUsage') }}
    </h1>
    <TablePageLayout>
      <template #table>
        <div v-if="loadFailed" role="alert" class="px-6 py-12 text-center text-sm text-gray-600 dark:text-gray-300">
          {{ t('accountPoolUsage.loadFailed') }}
        </div>
        <DataTable v-else :columns="columns" :data="items" :loading="loading" row-key="id" :estimate-row-height="104">
          <template #cell-name="{ row }">
            <div class="flex min-w-0 flex-col items-end gap-1 md:items-start">
              <span class="max-w-xs font-medium leading-5 text-gray-900 [overflow-wrap:anywhere] dark:text-white">{{ row.name }}</span>
              <time
                v-if="row.expires_at"
                :datetime="row.expires_at"
                class="text-xs leading-4 text-gray-500 dark:text-gray-400"
              >
                {{ t('admin.accounts.subscriptionExpires') }} {{ formatAccountExpiry(row.expires_at) }}
              </time>
            </div>
          </template>
          <template #cell-platform_type="{ row }">
            <PlatformTypeBadge
              :platform="row.platform"
              :type="row.type"
              :plan-type="row.plan_type"
              :privacy-mode="row.privacy_mode"
            />
          </template>
          <template #cell-capacity="{ row }">
            <AccountConcurrencyBadge :current="row.current_concurrency" :max="row.concurrency" />
          </template>
          <template #cell-status="{ row }">
            <div class="inline-flex flex-col items-start gap-1 text-left">
              <span :class="['badge', statusClasses[row.status as AccountPoolStatus]]">
                {{ t(`admin.accounts.status.${row.status}`) }}<span v-if="row.status_code">| {{ row.status_code }}</span>
              </span>
              <time
                v-if="row.recovers_at"
                :datetime="row.recovers_at"
                :title="formatDateTimeToMinute(row.recovers_at)"
                class="text-[11px] leading-4 text-gray-500 dark:text-gray-400"
              >
                {{ statusRecoveryText(row) }}
              </time>
            </div>
          </template>
          <template #cell-windows="{ row }">
            <div class="pool-usage-window flex min-w-0 flex-col items-start gap-1 whitespace-nowrap text-left">
              <template v-if="row.windows?.length">
                <div v-for="window in row.windows" :key="window.kind" class="min-w-0 max-w-full">
                  <div v-if="window.cost_limit" class="mb-0.5 text-[10px] leading-4 tabular-nums text-gray-500 dark:text-gray-400">
                    ${{ window.cost_limit.used.toFixed(2) }} / ${{ window.cost_limit.limit.toFixed(2) }}
                  </div>
                  <UsageProgressBar
                    :label="t(`accountPoolUsage.windows.${window.kind}`)"
                    :utilization="window.utilization"
                    :utilization-unknown="window.utilization_unknown"
                    :resets-at="window.resets_at"
                    :window-stats="window.window_stats"
                    :estimated-total-cost="
                      !window.cost_limit && (row.platform === 'openai' || row.platform === 'anthropic')
                        ? estimateWindowTotalCost(window)
                        : null
                    "
                    :show-now-when-idle="row.platform === 'openai'"
                    :color="windowColors[window.kind as AccountPoolUsageWindowKind]"
                    class="leading-5"
                    label-width="auto"
                    window-stats-position="inline"
                  />
                </div>
              </template>
              <span v-else class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('accountPoolUsage.noQuotaSample') }}
              </span>
            </div>
          </template>
          <template #cell-reset_credits="{ value }">
            <div class="flex flex-col items-end gap-1 md:items-start">
              <time
                v-for="(expiry, index) in value?.expirations"
                :key="`${expiry}-${index}`"
                :datetime="expiry"
                :title="formatDateTimeToMinute(expiry)"
                class="whitespace-nowrap rounded bg-gray-100 px-1.5 py-0.5 text-[10px] leading-4 tabular-nums text-gray-600 dark:bg-dark-800 dark:text-gray-300"
              >
                {{ t('admin.accounts.openaiQuotaReset.expiresAt', { time: formatDateTime(expiry, resetCreditDateOptions) }) }}
              </time>
            </div>
          </template>
          <template #cell-priority="{ value }">
            <span class="font-mono tabular-nums">{{ value }}</span>
          </template>
          <template #empty>
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('accountPoolUsage.empty') }}</p>
          </template>
        </DataTable>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { useNow } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import {
  listAccountPoolUsage,
  type AccountPoolStatus,
  type AccountPoolUsage,
  type AccountPoolUsageWindowKind
} from '@/api/accountPoolUsage'
import AccountConcurrencyBadge from '@/components/account/AccountConcurrencyBadge.vue'
import UsageProgressBar from '@/components/account/UsageProgressBar.vue'
import DataTable from '@/components/common/DataTable.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import { estimateWindowTotalCost } from '@/utils/accountUsage'
import { formatCountdown, formatDateOnly, formatDateTime, formatDateTimeToMinute } from '@/utils/format'

const { t } = useI18n()
const now = useNow({ interval: 60_000 })

const columns = computed(() => [
  { key: 'name', label: t('admin.accounts.columns.name') },
  { key: 'platform_type', label: t('admin.accounts.columns.platformType') },
  { key: 'capacity', label: t('admin.accounts.columns.capacity') },
  { key: 'status', label: t('common.status') },
  { key: 'windows', label: t('admin.accounts.columns.usageWindows') },
  { key: 'reset_credits', label: t('accountPoolUsage.resetCredits') },
  { key: 'priority', label: t('admin.accounts.columns.priority') }
])

const statusClasses = {
  active: 'badge-success',
  inactive: 'badge-gray',
  expired: 'badge-gray',
  rateLimited: 'badge-warning',
  overloaded: 'badge-danger',
  tempUnschedulable: 'badge-warning',
  quotaExceeded: 'badge-warning',
  error: 'badge-danger'
} satisfies Record<AccountPoolStatus, string>

const windowColors = {
  fiveHour: 'indigo',
  sevenDay: 'emerald',
  sevenDayFable: 'purple',
  thirtyDay: 'purple',
  localDaily: 'indigo',
  localWeekly: 'emerald',
  localTotal: 'amber'
} as const

const resetCreditDateOptions: Intl.DateTimeFormatOptions = {
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit'
}

function formatAccountExpiry(value: string) {
  return formatDateOnly(value.length === 10 ? `${value}T00:00:00` : value)
}

function statusRecoveryText(row: AccountPoolUsage) {
  const countdown = formatCountdown(row.recovers_at, now.value)
  if (countdown && row.status === 'rateLimited') {
    return t('admin.accounts.status.rateLimitedAutoResume', { time: countdown })
  }
  if (countdown && row.status === 'overloaded') {
    return t('common.time.countdown.withSuffix', { time: countdown })
  }
  return t('admin.accounts.status.tempUnschedulableUntil', { time: formatDateTimeToMinute(row.recovers_at) })
}

const items = shallowRef<AccountPoolUsage[]>([])
const loading = ref(true)
const loadFailed = ref(false)
const controller = new AbortController()

onMounted(async () => {
  try {
    items.value = await listAccountPoolUsage({ signal: controller.signal })
  } catch {
    if (!controller.signal.aborted) loadFailed.value = true
  } finally {
    loading.value = false
  }
})
onBeforeUnmount(() => controller.abort())
</script>

<style scoped>
@media (max-width: 767px) {
  .pool-usage-window :deep(.usage-window-stats) {
    @apply flex-wrap;
  }
}
</style>
