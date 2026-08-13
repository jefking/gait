import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { OwnerSummary } from '../lib/api'
import { OwnerSelect } from './OwnerSelect'

const owners: OwnerSummary[] = [
  {
    owner: {
      id: 7,
      login: 'example-org',
      type: 'Organization',
      avatar_url: 'https://avatars.example.test/example-org.png',
      html_url: 'https://github.com/example-org',
    },
    repositories: 3,
    contributors: 4,
    commits: 10,
    lines_added: 20,
    lines_deleted: 5,
    pull_requests_opened: 2,
  },
  {
    owner: {
      id: 8,
      login: 'second-org',
      type: 'Organization',
      avatar_url: 'https://avatars.example.test/second-org.png',
      html_url: 'https://github.com/second-org',
    },
    repositories: 2,
    contributors: 3,
    commits: 8,
    lines_added: 12,
    lines_deleted: 3,
    pull_requests_opened: 1,
  },
]

describe('OwnerSelect', () => {
  it('shows every organization and selects all or exactly one', () => {
    const change = vi.fn()
    const { rerender } = render(<OwnerSelect owners={owners} onChange={change} />)
    expect(screen.getByRole('radio', { name: 'All organizations' })).toHaveAttribute('aria-checked', 'true')
    const example = screen.getByRole('radio', { name: 'example-org' })
    const second = screen.getByRole('radio', { name: 'second-org' })
    expect(example.querySelector('img')).toHaveAttribute('src', owners[0].owner.avatar_url)
    expect(second).toBeVisible()
    fireEvent.click(example)
    expect(change).toHaveBeenLastCalledWith(7)

    rerender(<OwnerSelect owners={owners} value={7} onChange={change} />)
    expect(screen.getByRole('radio', { name: 'example-org' })).toHaveAttribute('aria-checked', 'true')
    fireEvent.click(screen.getByRole('radio', { name: 'second-org' }))
    expect(change).toHaveBeenLastCalledWith(8)
    fireEvent.click(screen.getByRole('radio', { name: 'All organizations' }))
    expect(change).toHaveBeenLastCalledWith()
  })
})
