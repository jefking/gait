import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ActivityResponse } from '../lib/api'
import { ActivityChart } from './ActivityChart'

const activity: ActivityResponse = {
  group_by: 'owner',
  metric: 'commits',
  granularity: 'month',
  available_from: '2024-01-01',
  available_to: '2024-02-01',
  from: '2024-01-01',
  to: '2024-02-01',
  series: [
    {
      key: '1',
      label: 'example-org',
      total: 3,
      points: [
        { date: '2024-01-01', value: 1 },
        { date: '2024-02-01', value: 2 },
      ],
    },
  ],
}

describe('ActivityChart', () => {
  it('renders an accessible chart and lets a series be hidden', () => {
    render(<ActivityChart activity={activity} loading={false} />)
    expect(screen.getByRole('img', { name: /Repository activity over time/ })).toBeInTheDocument()
    const series = screen.getByRole('button', { name: /example-org/ })
    expect(series).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(series)
    expect(series).toHaveAttribute('aria-pressed', 'false')
  })

  it('renders an empty state when a filter has no activity', () => {
    render(<ActivityChart activity={{ ...activity, series: [] }} loading={false} />)
    expect(screen.getByText('No activity matches these filters.')).toBeInTheDocument()
  })
})
