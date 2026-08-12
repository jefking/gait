import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { NetworkResponse, OverviewResponse, RampResponse, RankingResponse } from '../lib/api'
import { MomentumChart } from './MomentumChart'
import { RampChart } from './RampChart'
import { RankChart } from './RankChart'
import { TeamNetwork } from './TeamNetwork'

const meta = {
  available_from: '2025-01-01', available_to: '2025-02-01', from: '2025-01-01', to: '2025-02-01', granularity: 'day' as const, session_hours: 72, adoption_days: 30, survival_days: 30,
  coverage: { total_commits: 7, classified_commits: 6, unknown_commits: 1, classification_rate: 6 / 7, mature_commits: 5, eligible_pull_requests: 2, reviewed_pull_requests: 1 },
}

const overview: OverviewResponse = {
  meta,
  summary: { agent_participation: .4, handoff_lift: .25, handoff_episodes: 3, quality_direction: .05, strongest_pair: 'Alice × Helper', strongest_pair_days: 2 },
  timeline: [
    { date: '2025-01-01', human_only: 2, agent_only: 1, mixed: 1, unknown: 0, pull_requests: 1 },
    { date: '2025-01-02', human_only: 1, agent_only: 1, mixed: 0, unknown: 1, pull_requests: 0 },
  ],
  quality: [
    { date: '2025-01-01', revert_rate: .1, merge_rate: .8, review_coverage: .6, commit_sample: 4, pull_request_sample: 2 },
    { date: '2025-01-02', revert_rate: .05, merge_rate: .9, review_coverage: .8, commit_sample: 3, pull_request_sample: 1 },
  ],
  repositories: [{ repository_id: 1, name: 'org/repo', total: 7, points: [
    { date: '2025-01-01', human_only: 2, agent_only: 1, mixed: 1, unknown: 0 },
    { date: '2025-01-02', human_only: 1, agent_only: 1, mixed: 0, unknown: 1 },
  ] }],
}

describe('Human × Agent charts', () => {
  it('switches the coordinated momentum view to repository pulse', () => {
    render(<MomentumChart overview={overview} loading={false} />)
    expect(screen.getByText('Commit frequency by work mode')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Repository pulse/ }))
    expect(screen.getByText('Repository commit pulse')).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Repository activity heatmap' })).toBeInTheDocument()
  })

  it('labels handoff lift as an association and exposes adoption maturity', () => {
    const ramps: RampResponse = { meta, handoffs: [{
      key: 'alice-helper', human: identity('alice', 'Alice', 'human'), agent: identity('helper', 'Helper', 'agent'),
      episodes: 3, completed_episodes: 3, interaction_days: 4, baseline: 3, after: 6, absolute_change: 3, observed_lift: 1, quality_delta: .1, mature: true, rank_eligible: true,
    }], adoptions: [{ repository_id: 1, repository: 'org/repo', adopted_at: '2025-01-02', baseline: 3, after: 6, absolute_change: 3, observed_lift: 1, mature: true }] }
    render(<RampChart ramps={ramps} loading={false} />)
    expect(screen.getByText(/does not establish that the agent caused/)).toBeInTheDocument()
    expect(screen.getByText(/complete 30-day window/)).toBeInTheDocument()
  })

  it('renders metric-led rank trajectories and leaderboard', () => {
    const rankings: RankingResponse = { meta, cohort: 'agents', metric: 'commits', favorable_direction: 'higher', leaderboard: [{ key: 'helper', label: 'Helper', kind: 'agent', rank: 1, value: 5, eligible: true, metrics: { commits: 5 } }], trajectories: [{ key: 'helper', label: 'Helper', points: [{ date: '2025-01-01', rank: 1, value: 5 }] }] }
    render(<RankChart rankings={rankings} loading={false} />)
    expect(screen.getByText('Rank over time')).toBeInTheDocument()
    expect(screen.getByText('Current leaderboard')).toBeInTheDocument()
    expect(screen.getAllByText('Helper').length).toBeGreaterThan(0)
  })

  it('supports inline identity correction in the constellation detail', () => {
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation(() => null)
    const network: NetworkResponse = { meta, total_identities: 1, edges: [], nodes: [{ ...identity('alice', 'Alice', 'human'), activity: 2 }] }
    const classify = vi.fn()
    render(<TeamNetwork network={network} loading={false} selectedKey="alice" onSelect={() => undefined} onClassify={classify} onRename={() => undefined} onMerge={() => undefined} onUnmerge={() => undefined} />)
    fireEvent.change(screen.getByLabelText('Classification'), { target: { value: 'agent' } })
    expect(classify).toHaveBeenCalledWith('alice', 'agent')
  })

  it('offers paused temporal network playback with a keyboard-operable period control', () => {
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation(() => null)
    const network: NetworkResponse = {
      meta,
      total_identities: 2,
      nodes: [{ ...identity('alice', 'Alice', 'human'), activity: 2 }, { ...identity('helper', 'Helper', 'agent'), activity: 2 }],
      edges: [{ source: 'alice', target: 'helper', pair_type: 'human_agent', interaction_days: 2, coauthorships: 0, review_interactions: 0, handoffs: 2, human_to_agent: 2, repositories: ['org/repo'], periods: ['2025-01-01', '2025-01-02'] }],
    }
    render(<TeamNetwork network={network} loading={false} onSelect={() => undefined} onClassify={() => undefined} onRename={() => undefined} onMerge={() => undefined} onUnmerge={() => undefined} />)
    expect(screen.getByRole('slider', { name: 'Network playback period' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Play' })).toBeInTheDocument()
  })
})

function identity(key: string, name: string, kind: 'human' | 'agent') {
  return { key, canonical_key: key, name, kind, evidence: 'manual_override', confidence: 'confirmed' as const, commits: 2, pull_requests: 0, reviews: 0 }
}
