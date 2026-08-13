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
]

describe('OwnerSelect', () => {
  it('shows GitHub avatars in the list and selected control', () => {
    const change = vi.fn()
    const { rerender } = render(<OwnerSelect owners={owners} onChange={change} />)
    fireEvent.click(screen.getByRole('button', { name: 'Organization All organizations' }))
    const option = screen.getByRole('option', { name: 'example-org' })
    expect(option.querySelector('img')).toHaveAttribute('src', owners[0].owner.avatar_url)
    fireEvent.click(option)
    expect(change).toHaveBeenCalledWith(7)

    rerender(<OwnerSelect owners={owners} value={7} onChange={change} />)
    const selected = screen.getByRole('button', { name: 'Organization example-org' })
    expect(selected.querySelector('img')).toHaveAttribute('src', owners[0].owner.avatar_url)
  })
})
