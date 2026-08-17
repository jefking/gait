import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { TokenModal } from './TokenModal'

const organization = { id: 7, login: 'octo-org', type: 'Organization', avatar_url: '', html_url: '' }

function props(overrides: Partial<React.ComponentProps<typeof TokenModal>> = {}): React.ComponentProps<typeof TokenModal> {
  return {
    open: true,
    hasCachedData: false,
    submitting: false,
    targets: [],
    tokenAvailable: false,
    onDiscover: vi.fn().mockResolvedValue(undefined),
    onSelectTarget: vi.fn().mockResolvedValue(undefined),
    onClose: vi.fn(),
    ...overrides,
  }
}

describe('TokenModal', () => {
  it('uses the GitHub brand mark for GitHub configuration', () => {
    const { container } = render(<TokenModal {...props({ mode: 'settings', hasCachedData: true })} />)
    expect(container.querySelectorAll('img[src="/images/github.svg"]')).toHaveLength(2)
  })

  it('clears the PAT and discovers targets before syncing', async () => {
    const discover = vi.fn().mockResolvedValue(undefined)
    render(<TokenModal {...props({ onDiscover: discover })} />)
    const input = screen.getByLabelText('GitHub personal access token')
    fireEvent.change(input, { target: { value: 'ghp_secret' } })
    fireEvent.click(screen.getByRole('button', { name: 'Continue' }))
    await waitFor(() => expect(discover).toHaveBeenCalledWith('ghp_secret'))
    expect(input).toHaveValue('')
  })

  it('selects exactly one discovered owner', async () => {
    const select = vi.fn().mockResolvedValue(undefined)
    render(<TokenModal {...props({ targets: [{ id: 9, login: 'octocat', type: 'User', avatar_url: '', html_url: '' }, organization], onSelectTarget: select })} />)
    fireEvent.click(screen.getByRole('radio', { name: /octo-org/i }))
    fireEvent.click(screen.getByRole('button', { name: 'Select and sync' }))
    await waitFor(() => expect(select).toHaveBeenCalledWith(7))
    expect(screen.getByRole('radio', { name: /octo-org/i })).toHaveAttribute('aria-checked', 'true')
  })

  it('allows cached data to close credential onboarding', () => {
    const close = vi.fn()
    render(<TokenModal {...props({ hasCachedData: true, onClose: close })} />)
    fireEvent.click(screen.getByRole('button', { name: 'View cached data' }))
    expect(close).toHaveBeenCalledOnce()
  })

  it('presents the selected target in settings', () => {
    render(<TokenModal {...props({ mode: 'settings', hasCachedData: true, targets: [organization], selectedTarget: organization })} />)
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'GitHub' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('radio', { name: /octo-org/i })).toHaveAttribute('aria-checked', 'true')
  })

  it('applies dead-project exclusion only when Done is clicked', () => {
    const onExcludeDeadChange = vi.fn()
    render(<TokenModal {...props({ mode: 'settings', hasCachedData: true, targets: [organization], onExcludeDeadChange })} />)
    fireEvent.click(screen.getByRole('tab', { name: 'Projects' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Exclude dead projects' }))
    expect(onExcludeDeadChange).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    expect(onExcludeDeadChange).toHaveBeenCalledWith(true)
  })

  it('can rediscover with a retained environment token', async () => {
    const discover = vi.fn().mockResolvedValue(undefined)
    render(<TokenModal {...props({ tokenAvailable: true, onDiscover: discover })} />)
    fireEvent.click(screen.getByRole('button', { name: 'Use retained token' }))
    await waitFor(() => expect(discover).toHaveBeenCalledWith())
  })
})
