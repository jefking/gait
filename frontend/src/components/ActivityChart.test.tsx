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
      avatar_url: 'https://avatars.example.test/example-org.png',
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
    const { container } = render(<ActivityChart activity={activity} loading={false} />)
    expect(screen.getByRole('img', { name: /Repository activity over time/ })).toBeInTheDocument()
    const series = screen.getByRole('button', { name: /example-org/ })
    expect(container.querySelector('img')).toHaveAttribute('src', activity.series[0].avatar_url)
    expect(series).toHaveAttribute('aria-pressed', 'true')
    expect(container.querySelectorAll('path[stroke-width="2.5"]')).toHaveLength(1)
    fireEvent.click(series)
    expect(series).toHaveAttribute('aria-pressed', 'false')
    expect(container.querySelectorAll('path[stroke-width="2.5"]')).toHaveLength(0)
    fireEvent.click(series)
    expect(series).toHaveAttribute('aria-pressed', 'true')
    expect(container.querySelectorAll('path[stroke-width="2.5"]')).toHaveLength(1)
  })

  it('renders an empty state when a filter has no activity', () => {
    render(<ActivityChart activity={{ ...activity, series: [] }} loading={false} />)
    expect(screen.getByText('No activity matches these filters.')).toBeInTheDocument()
  })

  it('keeps the selected owner plotted after changing a multi-owner selection', () => {
    const multiOwnerActivity: ActivityResponse = {
      ...activity,
      series: [
        { key: 'one', label: 'One', total: 4, points: [{ date: '2024-01-01', value: 4 }] },
        { key: 'molten', label: 'Molten-Bot', total: 12, points: [{ date: '2024-01-01', value: 5 }, { date: '2024-02-01', value: 7 }] },
        { key: 'three', label: 'Three', total: 2, points: [{ date: '2024-02-01', value: 2 }] },
      ],
    }
    const { container } = render(<ActivityChart activity={multiOwnerActivity} loading={false} />)

    fireEvent.click(screen.getByRole('button', { name: /One/ }))
    fireEvent.click(screen.getByRole('button', { name: /Three/ }))

    expect(screen.getByRole('button', { name: /Molten-Bot/ })).toHaveAttribute('aria-pressed', 'true')
    expect(container.querySelectorAll('path[stroke-width="2.5"]')).toHaveLength(1)
    expect(container.querySelector('path[data-series-key="molten"]')).toHaveAttribute('d', expect.stringContaining('L'))
    expect(container.querySelector('svg.activity-chart-svg')).toHaveAttribute('data-y-maximum', '7')
  })

  it('plots compatibility points that provide a month instead of a date', () => {
    const compatibilityActivity: ActivityResponse = {
      ...activity,
      series: [{
        key: 'legacy',
        label: 'Legacy owner',
        total: 9,
        points: [{ month: '2024-01', value: 4 }, { month: '2024-02', value: 5 }],
      }],
    }
    const { container } = render(<ActivityChart activity={compatibilityActivity} loading={false} />)
    expect(container.querySelector('path[data-series-key="legacy"]')).toHaveAttribute('d', expect.stringContaining('L'))
  })
})
