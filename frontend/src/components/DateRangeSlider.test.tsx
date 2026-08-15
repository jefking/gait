import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { DateRangeSlider } from './DateRangeSlider'

describe('DateRangeSlider', () => {
  it('renders a daily start handle, fixed latest date, and four preset snap points', () => {
    const { getByTestId } = render(
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
    expect(screen.getByLabelText('Oldest activity date')).toHaveAttribute('step', '1')
    expect(screen.queryByLabelText('Latest activity date')).not.toBeInTheDocument()
    expect(screen.getAllByText('Jan 1, 2025')).toHaveLength(2)
    expect(screen.getByText('Monthly')).toBeInTheDocument()
    expect(getByTestId('date-range-slider').querySelectorAll('[data-snap-label]')).toHaveLength(4)
    for (const label of ['1 month', '3 months', '6 months', '1 year']) {
      expect(screen.queryByRole('button', { name: label })).not.toBeInTheDocument()
    }
  })

  it('keeps arbitrary days and only snaps dates released near a preset', () => {
    const change = vi.fn()
    render(
      <DateRangeSlider
        availableFrom="2023-01-01"
        availableTo="2025-01-10"
        from="2023-01-01"
        to="2024-12-01"
        granularity="day"
        onChange={change}
      />,
    )
    const start = screen.getByLabelText('Oldest activity date')
    fireEvent.change(start, { target: { value: '689' } })
    fireEvent.pointerUp(start)
    expect(change).toHaveBeenLastCalledWith('2024-11-20', '2025-01-10')

    fireEvent.change(start, { target: { value: '711' } })
    fireEvent.pointerUp(start)
    expect(change).toHaveBeenLastCalledWith('2024-12-10', '2025-01-10')
  })

  it('groups presets that clamp to the same short history', () => {
    const change = vi.fn()
    const { getByTestId } = render(
      <DateRangeSlider
        availableFrom="2025-01-05"
        availableTo="2025-01-10"
        from="2025-01-05"
        to="2025-01-10"
        granularity="day"
        onChange={change}
      />,
    )
    const snapPoints = getByTestId('date-range-slider').querySelectorAll('[data-snap-label]')
    expect(snapPoints).toHaveLength(1)
    expect(snapPoints[0]).toHaveAttribute('data-snap-label', '1 month, 3 months, 6 months, 1 year')
    const start = screen.getByLabelText('Oldest activity date')
    fireEvent.change(start, { target: { value: '4' } })
    fireEvent.pointerUp(start)
    expect(change).toHaveBeenLastCalledWith('2025-01-09', '2025-01-10')
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

    const start = screen.getByLabelText('Oldest activity date')
    fireEvent.change(start, { target: { value: '61' } })
    fireEvent.pointerUp(start)
    expect(change).toHaveBeenLastCalledWith('2024-02-29', '2024-08-31')
  })
})
