import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import type { DashboardResponse, IdentitySummary } from './lib/api'

const cachedDashboard: DashboardResponse = {
  sync: {
    state: 'idle',
    total_repositories: 0,
    completed_repositories: 0,
    failed_repositories: 0,
  },
  snapshot: {
    generated_at: '2025-01-02T03:04:05Z',
    viewer: {
      login: 'octocat',
      name: 'The Octocat',
      avatar_url: '',
      html_url: 'https://github.com/octocat',
    },
    totals: {
      owners: 1,
      repositories: 1,
      contributors: 1,
      commits: 2,
      files_changed: 2,
      lines_added: 4,
      lines_deleted: 1,
      pull_requests_opened: 1,
      pull_requests_open: 1,
      pull_requests_closed: 0,
      pull_requests_merged: 0,
      repositories_without_pr_access: 0,
      dead_repositories: 1,
    },
    owners: [
      {
        owner: { id: 1, login: 'octo-org', type: 'Organization', avatar_url: '', html_url: '' },
        repositories: 1,
        contributors: 1,
        commits: 2,
        lines_added: 4,
        lines_deleted: 1,
        pull_requests_opened: 1,
        dead_repositories: 1,
      },
    ],
    contributors: [
      { key: 'github:octocat', login: 'octocat', name: 'The Octocat', commits: 2, pull_requests: 1, repositories: 1, owners: 1, lines_added: 4, lines_deleted: 1 },
    ],
    repositories: [
      {
        id: 10,
        name: 'hello-world',
        full_name: 'octo-org/hello-world',
        html_url: 'https://github.com/octo-org/hello-world',
        default_branch: 'main',
        private: false,
        archived: false,
        fork: false,
        owner: { id: 1, login: 'octo-org', type: 'Organization', avatar_url: '', html_url: '' },
        commits: 2,
        contributors: 1,
        files_changed: 2,
        lines_added: 4,
        lines_deleted: 1,
        pull_requests: { opened: 1, open: 1, closed: 0, merged: 0 },
        sync_status: 'synced',
        liveness: {
          state: 'dead',
          is_dead: true,
          basis: 'default_branch_commits',
          scale: 'month',
          threshold_value: 3,
          threshold_days: 91,
          active_span_days: 365,
          inactive_days: 400,
          evaluated_at: '2025-01-02T00:00:00Z',
        },
      },
    ],
  },
}

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

class FakeEventSource {
  static instance: FakeEventSource
  listeners = new Map<string, EventListenerOrEventListenerObject>()

  constructor(readonly url: string) {
    FakeEventSource.instance = this
  }

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    this.listeners.set(type, listener)
  }

  close() {}

  emit(event: { type: 'sync' | 'snapshot' | 'dashboard' | 'insights'; revision: number }) {
    const listener = this.listeners.get('dashboard')
    const message = new MessageEvent('dashboard', { data: JSON.stringify(event) })
    if (typeof listener === 'function') listener(message)
    else listener?.handleEvent(message)
  }
}

describe('App', () => {
  it('loads cached data by default and opens credentials from settings', async () => {
    const fetchMock = mockAPI()
    const print = vi.spyOn(window, 'print').mockImplementation(() => undefined)
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(await screen.findByText('Team constellation')).toBeInTheDocument()
    expect(screen.getByText('Human × Agent intelligence')).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'hello-world' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'GitHub settings' }))
    fireEvent.click(screen.getByRole('tab', { name: 'Projects' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Exclude dead projects' }))
    expect(screen.getByRole('option', { name: 'hello-world' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    expect(screen.queryByRole('option', { name: 'hello-world' })).not.toBeInTheDocument()
    expect(screen.getByText('All repositories are excluded by project settings.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Export PDF/ }))
    expect(print).toHaveBeenCalledOnce()
    fireEvent.click(screen.getByRole('button', { name: 'GitHub settings' }))
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: 'GitHub' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('refreshes repositories with retained server credentials', async () => {
    const fetchMock = mockAPI()
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)

    await screen.findByText('Team constellation')
    fireEvent.click(screen.getByRole('button', { name: 'Refresh repositories' }))

    await waitFor(() => expect(fetchMock.mock.calls.some(([url, init]) =>
      url === '/api/sync' && init?.body === JSON.stringify({}),
    )).toBe(true))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('keeps existing graphs mounted and only refreshes ranking data for ranking selections', async () => {
    let rankingRequests = 0
    let resolveRanking!: (response: Response) => void
    const pendingRanking = new Promise<Response>((resolve) => { resolveRanking = resolve })
    const ranking = (label: string, cohort: 'agents' | 'humans') => ({
      meta: emptyOverview.meta,
      cohort,
      metric: 'commits',
      favorable_direction: 'higher',
      leaderboard: [{ key: label, label, rank: 1, value: 2, eligible: true, metrics: { commits: 2 } }],
      trajectories: [],
    })
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url === '/api/dashboard') return Promise.resolve(jsonResponse(cachedDashboard))
      if (url.startsWith('/api/insights/overview')) return Promise.resolve(jsonResponse(emptyOverview))
      if (url.startsWith('/api/insights/network')) return Promise.resolve(jsonResponse({ meta: emptyOverview.meta, nodes: [], edges: [], total_identities: 0 }))
      if (url.startsWith('/api/insights/ramps')) return Promise.resolve(jsonResponse({ meta: emptyOverview.meta, handoffs: [], adoptions: [] }))
      if (url.startsWith('/api/insights/rankings')) {
        rankingRequests += 1
        return rankingRequests === 1 ? Promise.resolve(jsonResponse(ranking('Existing ranking', 'agents'))) : pendingRanking
      }
      if (url === '/api/identities') return Promise.resolve(jsonResponse({ identities: [] }))
      return Promise.resolve(jsonResponse({ error: 'not found' }, 404))
    })
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)

    expect(await screen.findByText('Existing ranking')).toBeInTheDocument()
    fireEvent.change(screen.getByRole('combobox', { name: 'Ranking cohort' }), { target: { value: 'humans' } })

    expect(screen.getByText('Existing ranking')).toBeInTheDocument()
    expect(screen.getByText('Updating')).toBeInTheDocument()
    await waitFor(() => expect(rankingRequests).toBe(2))
    expect(fetchMock.mock.calls.filter(([url]) => String(url).startsWith('/api/insights/overview'))).toHaveLength(1)
    expect(fetchMock.mock.calls.filter(([url]) => String(url).startsWith('/api/insights/network'))).toHaveLength(1)
    expect(fetchMock.mock.calls.filter(([url]) => String(url).startsWith('/api/insights/ramps'))).toHaveLength(1)

    act(() => resolveRanking(jsonResponse(ranking('Updated ranking', 'humans'))))
    expect(await screen.findByText('Updated ranking')).toBeInTheDocument()
    expect(screen.queryByText('Existing ranking')).not.toBeInTheDocument()
  })

  it('keeps graphs mounted and coalesces incremental snapshot updates during sync', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('EventSource', FakeEventSource)
    let currentDashboard = cachedDashboard
    let dashboardRequests = 0
    let identityRequests = 0
    let insightRequests = 0
    const pendingIdentityRefresh = new Promise<Response>(() => undefined)
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url === '/api/dashboard') {
        dashboardRequests += 1
        return Promise.resolve(jsonResponse(currentDashboard))
      }
      if (url === '/api/identities') {
        identityRequests += 1
        return identityRequests === 1
          ? Promise.resolve(jsonResponse({ identities: [] }))
          : pendingIdentityRefresh
      }
      if (url.startsWith('/api/insights/')) insightRequests += 1
      if (url.startsWith('/api/insights/overview')) return Promise.resolve(jsonResponse(emptyOverview))
      if (url.startsWith('/api/insights/network')) return Promise.resolve(jsonResponse({ meta: emptyOverview.meta, nodes: [], edges: [], total_identities: 0 }))
      if (url.startsWith('/api/insights/ramps')) return Promise.resolve(jsonResponse({ meta: emptyOverview.meta, handoffs: [], adoptions: [] }))
      if (url.startsWith('/api/insights/rankings')) return Promise.resolve(jsonResponse({ meta: emptyOverview.meta, cohort: 'agents', metric: 'commits', favorable_direction: 'higher', leaderboard: [], trajectories: [] }))
      return Promise.resolve(jsonResponse({ error: 'not found' }, 404))
    })
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)

    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    expect(screen.getByText('Team constellation')).toBeInTheDocument()
    expect(insightRequests).toBe(4)

    FakeEventSource.instance.emit({ type: 'sync', revision: 1 })
    await act(async () => { await vi.advanceTimersByTimeAsync(750) })
    expect(dashboardRequests).toBe(2)
    expect(insightRequests).toBe(4)

    currentDashboard = {
      ...cachedDashboard,
      snapshot: cachedDashboard.snapshot && {
        ...cachedDashboard.snapshot,
        generated_at: '2025-01-02T03:04:08Z',
        totals: { ...cachedDashboard.snapshot.totals, commits: 3 },
      },
    }
    FakeEventSource.instance.emit({ type: 'snapshot', revision: 2 })
    FakeEventSource.instance.emit({ type: 'snapshot', revision: 3 })
    FakeEventSource.instance.emit({ type: 'snapshot', revision: 4 })
    await act(async () => { await vi.advanceTimersByTimeAsync(750) })

    expect(dashboardRequests).toBe(3)
    expect(screen.getByText('Team constellation')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Identity registry' })).not.toBeInTheDocument()

    await act(async () => { await vi.advanceTimersByTimeAsync(1_000) })
    expect(identityRequests).toBe(2)
    expect(insightRequests).toBe(8)
    expect(screen.getByText('Team constellation')).toBeInTheDocument()
    expect(screen.queryByLabelText('Loading team constellation')).not.toBeInTheDocument()
  })

  it('blocks first use, submits a PAT, clears the input, and starts asynchronous progress', async () => {
    const fetchMock = mockAPI({
      snapshot: null,
      sync: {
        state: 'idle',
        total_repositories: 0,
        completed_repositories: 0,
        failed_repositories: 0,
      },
    })
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)
    const input = await screen.findByLabelText('GitHub personal access token')
    fireEvent.change(input, { target: { value: 'ghp_browser_secret' } })
    fireEvent.click(screen.getByRole('button', { name: 'Connect and sync' }))

    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url, init]) =>
        url === '/api/sync' && typeof init?.body === 'string' && init.body.includes('ghp_browser_secret'),
      )).toBe(true)
    })
    expect(input).toHaveValue('')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('does not graph stale insights when a dead-project filtered request fails', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url === '/api/dashboard') return jsonResponse(cachedDashboard)
      if (url.includes('exclude_dead=true')) throw new Error('Filtered insights failed')
      if (url.startsWith('/api/insights/overview')) return jsonResponse(emptyOverview)
      if (url.startsWith('/api/insights/network')) return jsonResponse({ meta: emptyOverview.meta, nodes: [], edges: [], total_identities: 0 })
      if (url.startsWith('/api/insights/ramps')) return jsonResponse({ meta: emptyOverview.meta, handoffs: [], adoptions: [] })
      if (url.startsWith('/api/insights/rankings')) return jsonResponse({ meta: emptyOverview.meta, cohort: 'agents', metric: 'commits', favorable_direction: 'higher', leaderboard: [{ key: 'dead', label: 'Dead project activity', rank: 1, value: 2, eligible: true, metrics: { commits: 2 } }], trajectories: [] })
      if (url === '/api/identities') return jsonResponse({ identities: [] })
      return jsonResponse({ error: 'not found' }, 404)
    })
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)

    expect(await screen.findByText('Dead project activity')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'GitHub settings' }))
    fireEvent.click(screen.getByRole('tab', { name: 'Projects' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Exclude dead projects' }))
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Filtered insights failed')
    expect(screen.queryByText('Dead project activity')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Loading rank trajectories')).toBeInTheDocument()
  })

  it('holds insights until every identity is classified', async () => {
    const unresolved: IdentitySummary = {
      key: 'mystery', canonical_key: 'mystery', name: 'Mystery Actor', kind: 'unknown', evidence: 'unverified_git_identity', confidence: 'unknown', commits: 2, pull_requests: 0, reviews: 0,
    }
    const fetchMock = mockAPI(cachedDashboard, [unresolved])
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Identity registry' })).toBeInTheDocument()
    expect(screen.queryByText('Team constellation')).not.toBeInTheDocument()
    expect(fetchMock.mock.calls.some(([url]) => String(url).startsWith('/api/insights/'))).toBe(false)

    fireEvent.click(await screen.findByRole('button', { name: 'Classify Mystery Actor as Human' }))

    expect(await screen.findByText('Team constellation')).toBeInTheDocument()
    await waitFor(() => expect(fetchMock.mock.calls.some(([url]) => String(url).startsWith('/api/insights/'))).toBe(true))
  })
})

function mockAPI(dashboard: DashboardResponse = cachedDashboard, identities: IdentitySummary[] = []) {
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    if (url === '/api/dashboard') return jsonResponse(dashboard)
    if (url.startsWith('/api/insights/overview')) return jsonResponse(emptyOverview)
    if (url.startsWith('/api/insights/network')) return jsonResponse({ meta: emptyOverview.meta, nodes: [], edges: [], total_identities: 0 })
    if (url.startsWith('/api/insights/ramps')) return jsonResponse({ meta: emptyOverview.meta, handoffs: [], adoptions: [] })
    if (url.startsWith('/api/insights/rankings')) return jsonResponse({ meta: emptyOverview.meta, cohort: 'agents', metric: 'commits', favorable_direction: 'higher', leaderboard: [], trajectories: [] })
    if (url === '/api/identities') return jsonResponse({ identities })
    if (url.startsWith('/api/identities/') && init?.method === 'PATCH') return jsonResponse({ identities: identities.map((identity) => ({ ...identity, kind: 'human', evidence: 'manual_override', confidence: 'confirmed' })) })
    if (url === '/api/sync' && init?.method === 'POST') {
      return jsonResponse({
        sync: {
          id: 'sync-1',
          state: 'discovering',
          total_repositories: 0,
          completed_repositories: 0,
          failed_repositories: 0,
          message: 'Connecting to GitHub',
        },
      }, 202)
    }
    return jsonResponse({ error: 'not found' }, 404)
  })
}

const emptyOverview = {
  meta: {
    available_from: '2024-01-01', available_to: '2025-01-01', from: '2024-01-01', to: '2025-01-01', granularity: 'week', session_hours: 72, adoption_days: 30, survival_days: 30,
    coverage: { total_commits: 0, classified_commits: 0, unknown_commits: 0, classification_rate: 0, mature_commits: 0, eligible_pull_requests: 0, reviewed_pull_requests: 0 },
  },
  summary: { agent_participation: 0, handoff_episodes: 0, strongest_pair_days: 0 },
  timeline: [], quality: [], repositories: [],
}

function jsonResponse(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}
