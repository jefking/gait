import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { DateRangeSlider } from './DateRangeSlider'

describe('DateRangeSlider', () => {
  it('renders oldest and latest handles across the available history', () => {
    render(
      <DateRangeSlider
        availableFrom="2020-01-01"
        availableTo="2025-01-01"
        from="2021-01-01"
        to="2024-01-01"
        granularity="month"
        onChange={vi.fn()}
      />,
    )
    expect(screen.getByLabelText('Oldest activity date')).toHaveAttribute('type', 'range')
    expect(screen.getByLabelText('Latest activity date')).toHaveAttribute('type', 'range')
    expect(screen.getByText('Monthly')).toBeInTheDocument()
  })

  it('commits a constrained UTC date range when a handle is released', () => {
    const change = vi.fn()
    render(
      <DateRangeSlider
        availableFrom="2024-01-01"
        availableTo="2024-01-11"
        from="2024-01-01"
        to="2024-01-11"
        granularity="day"
        onChange={change}
      />,
    )
    const start = screen.getByLabelText('Oldest activity date')
    fireEvent.change(start, { target: { value: '4' } })
    fireEvent.pointerUp(start)
    expect(change).toHaveBeenLastCalledWith('2024-01-05', '2024-01-11')
  })

  it('offers common ranges and applies them relative to the latest activity date', () => {
    const change = vi.fn()
    render(
      <DateRangeSlider
        availableFrom="2023-01-01"
        availableTo="2025-01-10"
        from="2023-01-01"
        to="2025-01-10"
        granularity="month"
        onChange={change}
      />,
    )

    expect(screen.getByRole('button', { name: 'All time' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.queryByRole('button', { name: 'Past 24 hours' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '7 days' }))
    expect(change).toHaveBeenLastCalledWith('2025-01-04', '2025-01-10')
    fireEvent.click(screen.getByRole('button', { name: '31 days' }))
    expect(change).toHaveBeenLastCalledWith('2024-12-11', '2025-01-10')
    fireEvent.click(screen.getByRole('button', { name: '6 months' }))
    expect(change).toHaveBeenLastCalledWith('2024-07-10', '2025-01-10')
    fireEvent.click(screen.getByRole('button', { name: '1 year' }))
    expect(change).toHaveBeenLastCalledWith('2024-01-10', '2025-01-10')
  })

  it('clamps presets to the available history', () => {
    const change = vi.fn()
    render(
      <DateRangeSlider
        availableFrom="2025-01-05"
        availableTo="2025-01-10"
        from="2025-01-05"
        to="2025-01-10"
        granularity="day"
        onChange={change}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: '31 days' }))
    expect(change).toHaveBeenLastCalledWith('2025-01-05', '2025-01-10')
  })

  it('clamps six calendar months to the last valid day', () => {
    const change = vi.fn()
    render(
      <DateRangeSlider
        availableFrom="2024-01-01"
        availableTo="2024-08-31"
        from="2024-01-01"
        to="2024-08-31"
        granularity="week"
        onChange={change}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '6 months' }))
    expect(change).toHaveBeenLastCalledWith('2024-02-29', '2024-08-31')
  })
})
