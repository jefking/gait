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
})
