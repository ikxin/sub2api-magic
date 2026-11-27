<template>
  <CapacityBadge :color-class="colorClass" :current="current ?? '—'" :max="max">
    <Icon name="grid" size="xs" :stroke-width="2" aria-hidden="true" />
  </CapacityBadge>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import CapacityBadge from '@/components/account/CapacityBadge.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  current?: number
  max: number
}>()

const colorClass = computed(() => {
  if (props.current != null && props.current >= props.max) {
    return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
  }
  if ((props.current ?? 0) > 0) {
    return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  }
  return 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
})
</script>
