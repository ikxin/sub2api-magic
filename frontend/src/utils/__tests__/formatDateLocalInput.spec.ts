import { describe, expect, it, vi } from 'vitest'

import { formatDateLocalInput, getLast24HoursRange, getRangeGranularity, isLast24HoursRange, toRangeBoundary } from '../format'

describe('formatDateLocalInput', () => {
  it('formats the calendar date in local time', () => {
    const localDate = new Date('2026-07-12T16:30:00Z')
    vi.spyOn(localDate, 'getFullYear').mockReturnValue(2026)
    vi.spyOn(localDate, 'getMonth').mockReturnValue(6)
    vi.spyOn(localDate, 'getDate').mockReturnValue(13)

    expect(formatDateLocalInput(localDate)).toBe('2026-07-13')
  })

  it('returns an empty string for an invalid date', () => {
    expect(formatDateLocalInput(new Date('invalid'))).toBe('')
  })
})

describe('exact date ranges', () => {
  it.each([
    '2026-11-16T15:42:13.456+08:00',
    '2026-03-08T15:42:13.456-04:00',
    '2026-11-01T15:42:13.456-05:00'
  ])('keeps a 24-hour duration at %s', (end) => {
    const range = getLast24HoursRange(new Date(end))
    expect(Date.parse(range.end) - Date.parse(range.start)).toBe(86_400_000)
    expect(Date.parse(range.end)).toBe(Date.parse(end))
    expect(isLast24HoursRange(range.start, range.end)).toBe(true)
    expect(getRangeGranularity(range.start, range.end)).toBe('hour')
  })

  it('distinguishes calendar dates from rolling ranges', () => {
    expect(isLast24HoursRange('2026-11-15', '2026-11-16')).toBe(false)
    expect(isLast24HoursRange('invalidT', 'invalidT')).toBe(false)
    expect(getRangeGranularity('2026-11-01', '2026-11-16')).toBe('day')
  })

  it('preserves timestamps and expands date-only end bounds to the next midnight', () => {
    expect(toRangeBoundary('2026-11-16T15:42:13.456+08:00', true))
      .toBe('2026-11-16T07:42:13.456Z')
    expect(toRangeBoundary('2026-11-16', true))
      .toBe(new Date(2026, 10, 17).toISOString())
    expect(toRangeBoundary('2026-11-16'))
      .toBe(new Date(2026, 10, 16).toISOString())
    expect(toRangeBoundary(undefined)).toBeUndefined()
  })
})
