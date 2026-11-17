import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

import DateRangePicker from '../DateRangePicker.vue'

const messages: Record<string, string> = {
  'dates.today': 'Today',
  'dates.yesterday': 'Yesterday',
  'dates.last24Hours': 'Last 24 Hours',
  'dates.last7Days': 'Last 7 Days',
  'dates.last14Days': 'Last 14 Days',
  'dates.last30Days': 'Last 30 Days',
  'dates.thisMonth': 'This Month',
  'dates.lastMonth': 'Last Month',
  'dates.startDate': 'Start Date',
  'dates.endDate': 'End Date',
  'dates.apply': 'Apply',
  'dates.selectDateRange': 'Select date range'
}

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({
    t: (key: string) => messages[key] ?? key,
    locale: ref('en')
  })
}))

const now = new Date('2026-11-16T07:42:13.456Z')
const start = '2026-11-15T07:42:13.456Z'
let wrapper: ReturnType<typeof mount<typeof DateRangePicker>>

describe('DateRangePicker', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(now)
  })
  afterEach(() => {
    wrapper?.unmount()
    vi.useRealTimers()
  })

  it('recognizes a supplied last 24 hours range', () => {
    wrapper = mount(DateRangePicker, {
      props: {
        startDate: start,
        endDate: now.toISOString()
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('Last 24 Hours')
  })

  it('does not label yesterday and today as last 24 hours', () => {
    wrapper = mount(DateRangePicker, {
      props: { startDate: '2026-11-15', endDate: '2026-11-16' },
      global: { stubs: { Icon: true } }
    })
    expect(wrapper.find('.date-picker-trigger').text()).not.toContain('Last 24 Hours')
  })

  it('emits range updates with last24Hours preset when applied', async () => {
    wrapper = mount(DateRangePicker, {
      props: {
        startDate: '2026-11-16',
        endDate: '2026-11-16'
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    await wrapper.find('.date-picker-trigger').trigger('click')
    const presetButton = wrapper.findAll('.date-picker-preset').find((node) =>
      node.text().includes('Last 24 Hours')
    )
    expect(presetButton).toBeDefined()

    await presetButton!.trigger('click')
    await wrapper.find('.date-picker-apply').trigger('click')

    const expectedStart = start
    const expectedEnd = now.toISOString()

    expect(wrapper.emitted('update:startDate')?.[0]).toEqual([expectedStart])
    expect(wrapper.emitted('update:endDate')?.[0]).toEqual([expectedEnd])
    expect(wrapper.emitted('change')?.[0]).toEqual([
      {
        startDate: expectedStart,
        endDate: expectedEnd,
        preset: 'last24Hours'
      }
    ])
  })

  it('recalculates the rolling window when applying a draft later', async () => {
    wrapper = mount(DateRangePicker, {
      props: { startDate: start, endDate: now.toISOString() },
      global: { stubs: { Icon: true } }
    })
    await wrapper.find('.date-picker-trigger').trigger('click')
    vi.setSystemTime(new Date('2026-11-16T08:00:00.000Z'))
    await wrapper.find('.date-picker-apply').trigger('click')
    expect(wrapper.emitted('change')?.[0]).toEqual([{
      startDate: '2026-11-15T08:00:00.000Z',
      endDate: '2026-11-16T08:00:00.000Z',
      preset: 'last24Hours'
    }])
  })

  it('shows local date inputs and switches to calendar days after an edit', async () => {
    wrapper = mount(DateRangePicker, {
      props: { startDate: start, endDate: now.toISOString() },
      global: { stubs: { Icon: true } }
    })
    await wrapper.find('.date-picker-trigger').trigger('click')
    const inputs = wrapper.findAll<HTMLInputElement>('input[type="date"]')
    const expectedEnd = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
    expect(inputs[1]!.element.value).toBe(expectedEnd)
    await inputs[0]!.setValue('2026-11-02')
    await wrapper.find('.date-picker-apply').trigger('click')
    expect(wrapper.emitted('change')?.[0]).toEqual([{
      startDate: '2026-11-02', endDate: expectedEnd, preset: null
    }])
  })
})
