import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { SyncStatus } from '../lib/api'
import { SyncNotification } from './DashboardView'

const activeSync: SyncStatus = {
  id: 'sync-1',
  state: 'syncing',
  total_repositories: 10,
  completed_repositories: 4,
  failed_repositories: 0,
  current_repositories: ['example/repository'],
  message: 'Updating repositories',
}

describe('SyncNotification', () => {
  it('floats in the top-right while a sync is active', () => {
    render(<SyncNotification sync={activeSync} />)
    const notification = screen.getByText('Updating repositories').closest('aside')
    expect(notification).toHaveClass('fixed', 'right-4', 'top-4')
    expect(screen.getByText('4 / 10 repositories')).toBeInTheDocument()
  })

  it('shows the current repository workflow step', () => {
    render(
      <SyncNotification
        sync={{
          ...activeSync,
          current_workflows: [{
            repository_id: 10,
            full_name: 'example/repository',
            stage: 'analyzing',
            message: 'Analyzing Git history for example/repository',
          }],
        }}
      />,
    )
    expect(screen.getByText('Analyzing Git history for example/repository')).toBeInTheDocument()
  })

  it('disappears when the sync completes', () => {
    const { rerender } = render(<SyncNotification sync={activeSync} />)
    rerender(
      <SyncNotification
        sync={{ ...activeSync, state: 'complete', message: 'Sync complete' }}
      />,
    )
    expect(screen.queryByText('Sync complete')).not.toBeInTheDocument()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('keeps failures actionable until dismissed', () => {
    render(
      <SyncNotification
        sync={{ ...activeSync, state: 'failed', message: 'GitHub rejected the PAT.' }}
      />,
    )
    expect(screen.getByText('GitHub rejected the PAT.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss sync notification' }))
    expect(screen.queryByText('GitHub rejected the PAT.')).not.toBeInTheDocument()
  })
})
