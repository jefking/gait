import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import type { DashboardResponse, DeliveryResponse, IdentitySummary } from './lib/api'

const cachedDashboard: DashboardResponse = {
  sync: { state: 'idle', total_repositories: 0, completed_repositories: 0, failed_repositories: 0 },
  snapshot: {
    generated_at: '2025-01-02T03:04:05Z',
    viewer: { login: 'octocat', name: 'The Octocat', avatar_url: '', html_url: 'https://github.com/octocat' },
    totals: { owners: 1, repositories: 1, contributors: 1, commits: 2, files_changed: 2, lines_added: 4, lines_deleted: 1, pull_requests_opened: 1, pull_requests_open: 1, pull_requests_closed: 0, pull_requests_merged: 1, repositories_without_pr_access: 0, dead_repositories: 1 },
    owners: [{ owner: { id: 1, login: 'octo-org', type: 'Organization', avatar_url: '', html_url: '' }, repositories: 1, contributors: 1, commits: 2, lines_added: 4, lines_deleted: 1, pull_requests_opened: 1, dead_repositories: 1 }],
    contributors: [{ key: 'github:octocat', login: 'octocat', name: 'The Octocat', commits: 2, pull_requests: 1, repositories: 1, owners: 1, lines_added: 4, lines_deleted: 1 }],
    repositories: [{ id: 10, name: 'hello-world', full_name: 'octo-org/hello-world', html_url: 'https://github.com/octo-org/hello-world', default_branch: 'main', private: false, archived: false, fork: false, owner: { id: 1, login: 'octo-org', type: 'Organization', avatar_url: '', html_url: '' }, commits: 2, contributors: 1, files_changed: 2, lines_added: 4, lines_deleted: 1, pull_requests: { opened: 1, open: 0, closed: 1, merged: 1 }, sync_status: 'synced', liveness: { state: 'dead', is_dead: true, basis: 'default_branch_commits', evaluated_at: '2025-01-02T00:00:00Z' } }],
  },
}

const emptyDelivery: DeliveryResponse = {
  meta: { available_from: '2024-01-01', available_to: '2025-01-01', from: '2024-01-01', to: '2025-01-01', granularity: 'week', scope: { exclude_dead: false }, coverage: { organizations: 1, repositories: 1, index_eligible_repositories: 1, merged_pull_requests: 1, attributed_pull_requests: 1, unattributed_pull_requests: 0, detailed_pull_requests: 1, complete_commit_evidence_pull_requests: 1, truncated_commit_evidence_pull_requests: 0, attribution_rate: 1, actions_runs: 0, actions_covered_pull_requests: 0, actions_permission_denied: false, actions_truncated: false, direct_commit_coverage: true, low_confidence_baseline: false } },
  summary: { narrative: 'Team velocity is 18% above its opening pace. Quality is stable or inconclusive.', velocity_vs_baseline: .18, agent_associated_share: .42, quality_direction: 'stable/inconclusive', leader: 'Collaborative' },
  velocity: [{ date: '2024-01-01', complete: true, total_index: 100, human: { index: 50, merged_pull_requests: 1, additions: 8, deletions: 2, changed_lines: 10, commits: 1, direct_commits: 0 }, agent: { index: 20, merged_pull_requests: 0, additions: 0, deletions: 0, changed_lines: 0, commits: 0, direct_commits: 0 }, collaborative: { index: 30, merged_pull_requests: 0, additions: 0, deletions: 0, changed_lines: 0, commits: 0, direct_commits: 0 } }],
  raw: { human: { merged_pull_requests: 1, additions: 8, deletions: 2, changed_lines: 10, commits: 1, direct_commits: 0 }, agent: { merged_pull_requests: 0, additions: 0, deletions: 0, changed_lines: 0, commits: 0, direct_commits: 0 }, collaborative: { merged_pull_requests: 0, additions: 0, deletions: 0, changed_lines: 0, commits: 0, direct_commits: 0 }, total: { merged_pull_requests: 1, additions: 8, deletions: 2, changed_lines: 10, commits: 1, direct_commits: 0 } },
  quality: { direction: 'stable/inconclusive', signals: [], points: [] },
  flow: { summary: { as_of: '2025-01-01', open_pull_requests: 0, merged_pull_request_sample: 1 }, points: [] },
  impact: { tier: 'insufficient_evidence', verdict: 'insufficient evidence', treated_repositories: 0, control_repositories: 0, adoption_coverage: 0, pre_weeks: 8, post_weeks: 8, quality_deltas: [] },
}

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks() })

describe('App delivery rehaul', () => {
  it('loads the focused delivery story and opens settings', async () => {
    const fetchMock = mockAPI()
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)
    expect(await screen.findByText('Shipped velocity by work mode')).toBeInTheDocument()
    expect(screen.getAllByText('Team delivery evidence').length).toBeGreaterThan(0)
    expect(screen.getByText('Agent-impact evidence')).toBeInTheDocument()
    expect(screen.getByText('Is quality going up or down?')).toBeInTheDocument()
    expect(screen.queryByText(/Who leads this period/)).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'GitHub settings' }))
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument()
  })

  it('refreshes repositories with retained server credentials', async () => {
    const fetchMock = mockAPI(); vi.stubGlobal('fetch', fetchMock); render(<App />)
    await screen.findByText('Shipped velocity by work mode')
    fireEvent.click(screen.getByRole('button', { name: 'Refresh repositories' }))
    await waitFor(() => expect(fetchMock.mock.calls.some(([url, init]) => url === '/api/sync' && init?.body === JSON.stringify({}))).toBe(true))
  })

  it('sends the same global scope to delivery, network, and identities', async () => {
    const fetchMock = mockAPI(); vi.stubGlobal('fetch', fetchMock); render(<App />)
    await screen.findByText('Shipped velocity by work mode')
    fireEvent.click(screen.getByRole('button', { name: 'Organization All organizations' }))
    fireEvent.click(screen.getByRole('option', { name: 'octo-org' }))
    await waitFor(() => {
      const scoped = fetchMock.mock.calls.map(([url]) => String(url)).filter((url) => url.includes('organization_id=1'))
      expect(scoped.some((url) => url.startsWith('/api/insights/delivery'))).toBe(true)
      expect(scoped.some((url) => url.startsWith('/api/insights/network'))).toBe(true)
      expect(scoped.some((url) => url.startsWith('/api/identities'))).toBe(true)
    })
  })

  it('unmounts stale delivery content immediately when scope changes', async () => {
    const fetchMock = mockAPI(undefined, [], true); vi.stubGlobal('fetch', fetchMock); render(<App />)
    expect(await screen.findByText('Team velocity is 18% above its opening pace. Quality is stable or inconclusive.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'GitHub settings' })); fireEvent.click(screen.getByRole('tab', { name: 'Projects' })); fireEvent.click(screen.getByRole('checkbox', { name: 'Exclude dead projects' })); fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    expect(screen.queryByText('Team velocity is 18% above its opening pace. Quality is stable or inconclusive.')).not.toBeInTheDocument()
    expect(await screen.findByRole('alert')).toHaveTextContent('Filtered evidence failed')
  })

  it('blocks first use and submits a PAT', async () => {
    const fetchMock = mockAPI({ snapshot: null, sync: { state: 'idle', total_repositories: 0, completed_repositories: 0, failed_repositories: 0 } }); vi.stubGlobal('fetch', fetchMock); render(<App />)
    const input = await screen.findByLabelText('GitHub personal access token'); fireEvent.change(input, { target: { value: 'ghp_browser_secret' } }); fireEvent.click(screen.getByRole('button', { name: 'Connect and sync' }))
    await waitFor(() => expect(fetchMock.mock.calls.some(([url, init]) => url === '/api/sync' && String(init?.body).includes('ghp_browser_secret'))).toBe(true))
    expect(input).toHaveValue('')
  })

  it('keeps global scope above the scoped identity view', async () => {
    const unresolved: IdentitySummary = { key: 'mystery', canonical_key: 'mystery', name: 'Mystery Actor', kind: 'unknown', evidence: 'unverified_git_identity', confidence: 'unknown', commits: 2, pull_requests: 0, reviews: 0 }
    vi.stubGlobal('fetch', mockAPI(undefined, [unresolved])); render(<App />)
    fireEvent.click(await screen.findByRole('button', { name: /1 unattributed identity/i }))
    expect(screen.getByRole('heading', { name: 'Identity registry' })).toBeInTheDocument()
    expect(screen.getByLabelText('Global scope')).toBeInTheDocument()
    expect(screen.getByText('Mystery Actor')).toBeInTheDocument()
  })

  it('never requests obsolete overview, ramp, or ranking routes', async () => {
    const fetchMock=mockAPI();vi.stubGlobal('fetch',fetchMock);render(<App/>);await screen.findByText('Shipped velocity by work mode')
    const urls=fetchMock.mock.calls.map(([url])=>String(url));expect(urls.some((url)=>/overview|ramps|rankings/.test(url))).toBe(false)
  })
})

function mockAPI(dashboard: DashboardResponse = cachedDashboard, identities: IdentitySummary[] = [], failFiltered=false) {
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url=String(input)
    if(url==='/api/dashboard')return jsonResponse(dashboard)
    if(failFiltered&&url.includes('exclude_dead=true'))throw new Error('Filtered evidence failed')
    if(url.startsWith('/api/insights/delivery'))return jsonResponse(emptyDelivery)
    if(url.startsWith('/api/insights/network'))return jsonResponse({meta:{available_from:'2024-01-01',available_to:'2025-01-01',from:'2024-01-01',to:'2025-01-01',granularity:'week',session_hours:72,adoption_days:30,survival_days:30,coverage:{total_commits:0,classified_commits:0,unknown_commits:0,classification_rate:0,mature_commits:0,eligible_pull_requests:0,reviewed_pull_requests:0}},nodes:[],edges:[],total_identities:0})
    if(url.startsWith('/api/identities?'))return jsonResponse({identities})
    if(url.startsWith('/api/identities/')&&init?.method==='PATCH')return jsonResponse({identities})
    if(url==='/api/sync'&&init?.method==='POST')return jsonResponse({sync:{id:'sync-1',state:'discovering',total_repositories:0,completed_repositories:0,failed_repositories:0,message:'Connecting to GitHub'}},202)
    return jsonResponse({error:'not found'},404)
  })
}

function jsonResponse(value:unknown,status=200){return new Response(JSON.stringify(value),{status,headers:{'Content-Type':'application/json'}})}
