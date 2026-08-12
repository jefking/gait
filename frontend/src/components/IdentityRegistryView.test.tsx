import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ActorKind, IdentitySummary } from '../lib/api'
import { IdentityRegistryView } from './IdentityRegistryView'

const identities: IdentitySummary[] = [
  identity('mystery', 'Mystery Actor', 'unknown', 12),
  identity('alice', 'Alice Human', 'human', 8),
  identity('helper', 'Helper Agent', 'agent', 5),
]

describe('IdentityRegistryView', () => {
  it('defaults to only unresolved actors while keeping saved classifications in filters', () => {
    render(<IdentityRegistryView identities={identities} loading={false} onChange={() => undefined} />)

    expect(screen.getAllByText('1 unknown actor')).toHaveLength(1)
    expect(screen.getByText('Mystery Actor')).toBeInTheDocument()
    expect(screen.queryByText('Alice Human')).not.toBeInTheDocument()
    expect(screen.queryByText('Helper Agent')).not.toBeInTheDocument()
    expect(screen.getByText(/restored after app restarts/i)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Humans\s*1/ }))
    expect(screen.getByText('Alice Human')).toBeInTheDocument()
    expect(screen.queryByText('Mystery Actor')).not.toBeInTheDocument()
  })

  it('classifies an identity through a large drag-and-drop target', () => {
    const onChange = vi.fn()
    render(<IdentityRegistryView identities={identities} loading={false} onChange={onChange} />)
    const dataTransfer = {
      effectAllowed: 'none',
      dropEffect: 'none',
      getData: vi.fn(() => 'mystery'),
      setData: vi.fn(),
    }

    fireEvent.dragStart(screen.getByText('Mystery Actor').closest('article')!, { dataTransfer })
    fireEvent.drop(screen.getByRole('region', { name: 'Drop actor to classify as Agent' }), { dataTransfer })

    expect(onChange).toHaveBeenCalledWith('mystery', { kind: 'agent' })
  })

  it('offers touch- and keyboard-friendly classification buttons', () => {
    const onChange = vi.fn()
    render(<IdentityRegistryView identities={identities} loading={false} onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: 'Classify Mystery Actor as Human' }))

    expect(onChange).toHaveBeenCalledWith('mystery', { kind: 'human' })
  })
})

function identity(key: string, name: string, kind: ActorKind, commits: number): IdentitySummary {
  return {
    key,
    canonical_key: key,
    name,
    kind,
    evidence: kind === 'unknown' ? 'unverified_git_identity' : 'manual_override',
    confidence: kind === 'unknown' ? 'unknown' : 'confirmed',
    commits,
    pull_requests: 0,
    reviews: 0,
  }
}
