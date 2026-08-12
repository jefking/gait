import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { TokenModal } from './TokenModal'

describe('TokenModal', () => {
  it('clears the PAT immediately and sends it only to the submit callback', async () => {
    const submit = vi.fn().mockResolvedValue(undefined)
    render(
      <TokenModal
        open
        hasCachedData={false}
        submitting={false}
        onSubmit={submit}
        onViewCached={vi.fn()}
      />,
    )

    const input = screen.getByLabelText('GitHub personal access token')
    fireEvent.change(input, { target: { value: 'ghp_secret' } })
    fireEvent.click(screen.getByRole('button', { name: 'Connect and sync' }))

    await waitFor(() => expect(submit).toHaveBeenCalledWith('ghp_secret'))
    expect(input).toHaveValue('')
    expect(screen.queryByRole('button', { name: 'View cached data' })).not.toBeInTheDocument()
  })

  it('allows an existing snapshot to be viewed without a PAT', () => {
    const viewCached = vi.fn()
    render(
      <TokenModal
        open
        hasCachedData
        submitting={false}
        onSubmit={vi.fn()}
        onViewCached={viewCached}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: 'View cached data' }))
    expect(viewCached).toHaveBeenCalledOnce()
  })

  it('presents cached credentials as an optional settings dialog', () => {
    render(
      <TokenModal
        open
        mode="settings"
        hasCachedData
        submitting={false}
        onSubmit={vi.fn()}
        onViewCached={vi.fn()}
      />,
    )
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'GitHub' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save and sync' })).toBeInTheDocument()
  })

  it('configures dead-project exclusion from the Projects tab', () => {
    const onExcludeDeadChange = vi.fn()
    render(
      <TokenModal
        open
        mode="settings"
        hasCachedData
        submitting={false}
        onSubmit={vi.fn()}
        onViewCached={vi.fn()}
        onExcludeDeadChange={onExcludeDeadChange}
      />,
    )
    fireEvent.click(screen.getByRole('tab', { name: 'Projects' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Exclude dead projects' }))
    expect(onExcludeDeadChange).toHaveBeenCalledWith(true)
    expect(screen.getByText(/remove their activity from graph data/i)).toBeInTheDocument()
  })
})
