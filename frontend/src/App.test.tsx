import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import type { DashboardResponse, DeliveryResponse, IdentitySummary } from './lib/api'

const cachedDashboard: DashboardResponse = {
  sync: { state: 'idle', total_repositories: 0, completed_repositories: 0, failed_repositories: 0 },
  configuration: { selected_target: { id: 1, login: 'octo-org', type: 'Organization', avatar_url: '', html_url: '' }, available_targets: [{ id: 9, login: 'octocat', type: 'User', avatar_url: '', html_url: '' }, { id: 1, login: 'octo-org', type: 'Organization', avatar_url: '', html_url: '' }], token_available: true },
  snapshot: {
    generated_at: '2025-01-02T03:04:05Z',
    viewer: { id: 9, login: 'octocat', name: 'The Octocat', type: 'User', avatar_url: '', html_url: 'https://github.com/octocat' },
    totals: { owners: 1, repositories: 1, contributors: 1, commits: 2, files_changed: 2, lines_added: 4, lines_deleted: 1, pull_requests_opened: 1, pull_requests_open: 1, pull_requests_closed: 0, pull_requests_merged: 1, repositories_without_pr_access: 0, dead_repositories: 1 },
    owners: [{ owner: { id: 1, login: 'octo-org', type: 'Organization', avatar_url: '', html_url: '' }, repositories: 1, contributors: 1, commits: 2, lines_added: 4, lines_deleted: 1, pull_requests_opened: 1, dead_repositories: 1 }],
    contributors: [{ key: 'github:octocat', login: 'octocat', name: 'The Octocat', commits: 2, pull_requests: 1, repositories: 1, owners: 1, lines_added: 4, lines_deleted: 1 }],
    repositories: [{ id: 10, name: 'hello-world', full_name: 'octo-org/hello-world', html_url: 'https://github.com/octo-org/hello-world', default_branch: 'main', private: false, archived: false, fork: false, owner: { id: 1, login: 'octo-org', type: 'Organization', avatar_url: '', html_url: '' }, commits: 2, contributors: 1, files_changed: 2, lines_added: 4, lines_deleted: 1, pull_requests: { opened: 1, open: 0, closed: 1, merged: 1 }, sync_status: 'synced', liveness: { state: 'dead', is_dead: true, basis: 'default_branch_commits', evaluated_at: '2025-01-02T00:00:00Z' } }],
  },
}

const emptyDelivery: DeliveryResponse = {
  meta: { available_from: '2024-01-01', available_to: '2025-01-01', from: '2024-01-01', to: '2025-01-01', granularity: 'week', scope: { owner_id: 1, owner: 'octo-org', owner_type: 'Organization', exclude_dead: false }, coverage: { owners: 1, repositories: 1, index_eligible_repositories: 1, merged_pull_requests: 1, attributed_pull_requests: 1, unattributed_pull_requests: 0, incomplete_authorship_evidence_pull_requests: 0, unknown_authorship_identity_pull_requests: 0, detailed_pull_requests: 1, complete_commit_evidence_pull_requests: 1, truncated_commit_evidence_pull_requests: 0, attribution_rate: 1, actions_runs: 0, actions_covered_pull_requests: 0, actions_permission_denied: false, actions_truncated: false, direct_commit_coverage: true, low_confidence_baseline: false } },
  summary: { narrative: 'Team velocity is 18% above its opening pace. Quality is stable or inconclusive.', velocity_vs_baseline: .18, agent_associated_share: .42, quality_direction: 'stable/inconclusive', leader: 'Collaborative' },
  velocity: [{ date: '2024-01-01', complete: true, total_index: 100, human: { index: 50, merged_pull_requests: 1, additions: 8, deletions: 2, changed_lines: 10, commits: 1, direct_commits: 0 }, agent: { index: 20, merged_pull_requests: 0, additions: 0, deletions: 0, changed_lines: 0, commits: 0, direct_commits: 0 }, collaborative: { index: 30, merged_pull_requests: 0, additions: 0, deletions: 0, changed_lines: 0, commits: 0, direct_commits: 0 }, authorship_unknown: { merged_pull_requests: 0, additions: 0, deletions: 0, changed_lines: 0, commits: 0, direct_commits: 0 } }],
  performance: {
    daily: [{ date: '2024-01-01', leader: 'human_agent', total_index: 100, human: { index: 10, merged_pull_requests: 1, additions: 1, deletions: 0, changed_lines: 1, commits: 1, direct_commits: 0 }, human_human: { index: 20, merged_pull_requests: 1, additions: 2, deletions: 0, changed_lines: 2, commits: 1, direct_commits: 0 }, human_agent: { index: 50, merged_pull_requests: 2, additions: 5, deletions: 0, changed_lines: 5, commits: 2, direct_commits: 0 }, agent: { index: 20, merged_pull_requests: 1, additions: 2, deletions: 0, changed_lines: 2, commits: 1, direct_commits: 0 }, authorship_unknown: { merged_pull_requests: 0, additions: 0, deletions: 0, changed_lines: 0, commits: 0, direct_commits: 0 } }],
    overall: { leader: 'human_agent', total_index: 100, human: { index: 10, merged_pull_requests: 1, additions: 1, deletions: 0, changed_lines: 1, commits: 1, direct_commits: 0 }, human_human: { index: 20, merged_pull_requests: 1, additions: 2, deletions: 0, changed_lines: 2, commits: 1, direct_commits: 0 }, human_agent: { index: 50, merged_pull_requests: 2, additions: 5, deletions: 0, changed_lines: 5, commits: 2, direct_commits: 0 }, agent: { index: 20, merged_pull_requests: 1, additions: 2, deletions: 0, changed_lines: 2, commits: 1, direct_commits: 0 }, authorship_unknown: { merged_pull_requests: 0, additions: 0, deletions: 0, changed_lines: 0, commits: 0, direct_commits: 0 } },
  },
  raw: { human: { merged_pull_requests: 1, additions: 8, deletions: 2, changed_lines: 10, commits: 1, direct_commits: 0 }, agent: { merged_pull_requests: 0, additions: 0, deletions: 0, changed_lines: 0, commits: 0, direct_commits: 0 }, collaborative: { merged_pull_requests: 0, additions: 0, deletions: 0, changed_lines: 0, commits: 0, direct_commits: 0 }, authorship_unknown: { merged_pull_requests: 0, additions: 0, deletions: 0, changed_lines: 0, commits: 0, direct_commits: 0 }, total: { merged_pull_requests: 1, additions: 8, deletions: 2, changed_lines: 10, commits: 1, direct_commits: 0 } },
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
    expect(screen.queryByText('Is quality going up or down?')).not.toBeInTheDocument()
    const performanceHeading = screen.getByText('Daily and overall leaders')
    const primaryMeasureHeading = screen.getByText('Shipped velocity by work mode')
    expect(performanceHeading.compareDocumentPosition(primaryMeasureHeading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect((await screen.findAllByText('Human + Human')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('Human + Agent').length).toBeGreaterThan(0)
    expect(screen.queryByText('Collaboration network')).not.toBeInTheDocument()
    expect(screen.queryByRole('radio', { name: 'octo-org' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Repository')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Dead-project settings' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Visit Molten.bot' })).toHaveAttribute('href', 'https://molten.bot')
    expect(screen.getByRole('link', { name: 'Visit Molten.bot' })).toHaveAttribute('target', '_blank')
    expect(screen.queryByText(/Who leads this period/)).not.toBeInTheDocument()
    expect(performanceHeading.closest('main')).not.toHaveClass('min-h-screen')
    expect(document.querySelector('.dashboard-shell')).not.toHaveClass('pb-20')
    fireEvent.click(screen.getByRole('button', { name: 'GitHub settings' }))
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /octo-org/i })).toBeVisible()
  })

  it('refreshes repositories with retained server credentials', async () => {
    const fetchMock = mockAPI(); vi.stubGlobal('fetch', fetchMock); render(<App />)
    await screen.findByText('Shipped velocity by work mode')
    fireEvent.click(screen.getByRole('button', { name: 'Refresh repositories' }))
    await waitFor(() => expect(fetchMock.mock.calls.some(([url, init]) => url === '/api/sync' && init?.body === JSON.stringify({}))).toBe(true))
  })

  it('keeps cached data after restart and requests a PAT only when refresh needs one', async () => {
    const dashboard = structuredClone(cachedDashboard)
    dashboard.configuration.token_available = false
    dashboard.configuration.available_targets = []
    vi.stubGlobal('fetch', mockAPI(dashboard, [], false, emptyDelivery, true))
    render(<App />)
    await screen.findByText('Shipped velocity by work mode')
    fireEvent.click(screen.getByRole('button', { name: 'Refresh repositories' }))
    expect(await screen.findByRole('heading', { name: 'Settings' })).toBeInTheDocument()
    expect(screen.getByLabelText('GitHub personal access token')).toBeVisible()
    expect(screen.getByRole('alert')).toHaveTextContent('Enter a GitHub PAT to refresh this cached target.')
  })

  it('sends the same global scope to delivery and identities without loading the obsolete network', async () => {
    const fetchMock = mockAPI(); vi.stubGlobal('fetch', fetchMock); render(<App />)
    await screen.findByText('Shipped velocity by work mode')
    await waitFor(() => {
      const scoped = fetchMock.mock.calls.map(([url]) => String(url))
      expect(scoped.some((url) => url.startsWith('/api/insights/delivery') && !url.includes('organization_id'))).toBe(true)
      expect(scoped.some((url) => url.startsWith('/api/identities') && !url.includes('organization_id'))).toBe(true)
      expect(fetchMock.mock.calls.some(([url]) => String(url).startsWith('/api/insights/network'))).toBe(false)
    })
  })

  it('changes the configured owner from settings', async () => {
    const dashboard = structuredClone(cachedDashboard)
    dashboard.configuration.available_targets.push({ id: 2, login: 'second-org', type: 'Organization', avatar_url: '', html_url: '' })
    const fetchMock = mockAPI(dashboard)
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)
    await screen.findByText('Shipped velocity by work mode')
    fireEvent.click(screen.getByRole('button', { name: 'GitHub settings' }))
    const second = screen.getByRole('radio', { name: /second-org/i })
    fireEvent.click(second)
    fireEvent.click(screen.getByRole('button', { name: 'Select and sync' }))
    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url, init]) => url === '/api/configuration/github-target' && init?.body === JSON.stringify({ target_id: 2 }))).toBe(true)
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
    const fetchMock = mockAPI({ snapshot: null, sync: { state: 'idle', total_repositories: 0, completed_repositories: 0, failed_repositories: 0 }, configuration: { available_targets: [], token_available: false } }); vi.stubGlobal('fetch', fetchMock); render(<App />)
    const input = await screen.findByLabelText('GitHub personal access token'); fireEvent.change(input, { target: { value: 'ghp_browser_secret' } }); fireEvent.click(screen.getByRole('button', { name: 'Continue' }))
    await screen.findByRole('heading', { name: 'Choose one GitHub owner' })
    await waitFor(() => expect(fetchMock.mock.calls.some(([url, init]) => url === '/api/github/targets' && String(init?.body).includes('ghp_browser_secret'))).toBe(true))
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

  it('collapses and expands the global scope controls', async () => {
    vi.stubGlobal('fetch', mockAPI()); render(<App />)
    const collapse = await screen.findByRole('button', { name: 'Collapse global scope' })
    expect(collapse).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByLabelText('Global scope')).toHaveTextContent('octo-org')

    fireEvent.click(collapse)
    expect(screen.getByRole('button', { name: 'Expand global scope' })).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getByLabelText('Global scope')).not.toHaveTextContent('octo-org')

    fireEvent.click(screen.getByRole('button', { name: 'Expand global scope' }))
    expect(screen.getByLabelText('Global scope')).toHaveTextContent('octo-org')
  })

  it('never requests obsolete overview, ramp, or ranking routes', async () => {
    const fetchMock=mockAPI();vi.stubGlobal('fetch',fetchMock);render(<App/>);await screen.findByText('Shipped velocity by work mode')
    const urls=fetchMock.mock.calls.map(([url])=>String(url));expect(urls.some((url)=>/overview|ramps|rankings/.test(url))).toBe(false)
  })

  it('shows authorship-unknown PRs separately from performance leaders', async () => {
    const delivery = structuredClone(emptyDelivery)
    delivery.meta.coverage.merged_pull_requests = 4
    delivery.meta.coverage.unattributed_pull_requests = 3
    delivery.meta.coverage.incomplete_authorship_evidence_pull_requests = 2
    delivery.meta.coverage.unknown_authorship_identity_pull_requests = 1
    delivery.performance.overall.authorship_unknown.merged_pull_requests = 3
    delivery.performance.daily[0].authorship_unknown.merged_pull_requests = 3
    delivery.raw.authorship_unknown.merged_pull_requests = 3
    delivery.raw.total.merged_pull_requests = 4
    vi.stubGlobal('fetch', mockAPI(undefined, [], false, delivery))
    render(<App />)
    expect(await screen.findByText('3 merged PRs have unknown code authorship')).toBeInTheDocument()
    expect(screen.getByText(/2 with incomplete commit evidence and 1 with unresolved commit authors/)).toBeInTheDocument()
    expect(screen.getAllByText('Authorship unknown').length).toBeGreaterThan(0)
  })

  it('renders sparse owner evidence when legacy collections are null', async () => {
    const sparseDelivery = {
      ...emptyDelivery,
      meta: { ...emptyDelivery.meta, available_from: undefined, available_to: undefined, from: undefined, to: undefined },
      velocity: null,
      quality: { direction: 'insufficient', signals: null, points: null },
      flow: { ...emptyDelivery.flow, points: null },
      impact: { ...emptyDelivery.impact, quality_deltas: null },
    } as unknown as DeliveryResponse
    vi.stubGlobal('fetch', mockAPI(undefined, [], false, sparseDelivery))
    render(<App />)
    expect(await screen.findByText('Velocity requires attributed merged PRs and a non-zero opening baseline.')).toBeInTheDocument()
    expect(screen.queryByText('No quality samples are available.')).not.toBeInTheDocument()
    expect(screen.queryByText('Impact evidence is unavailable.')).not.toBeInTheDocument()
  })
})

function mockAPI(dashboard: DashboardResponse = cachedDashboard, identities: IdentitySummary[] = [], failFiltered=false, delivery: DeliveryResponse = emptyDelivery, failSync=false) {
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url=String(input)
    if(url==='/api/dashboard')return jsonResponse(dashboard)
    if(failFiltered&&url.includes('exclude_dead=true'))throw new Error('Filtered evidence failed')
    if(url.startsWith('/api/insights/delivery'))return jsonResponse(delivery)
    if(url.startsWith('/api/insights/network'))return jsonResponse({meta:{available_from:'2024-01-01',available_to:'2025-01-01',from:'2024-01-01',to:'2025-01-01',granularity:'week',session_hours:72,adoption_days:30,survival_days:30,coverage:{total_commits:0,classified_commits:0,unknown_commits:0,classification_rate:0,mature_commits:0,eligible_pull_requests:0,reviewed_pull_requests:0}},nodes:[],edges:[],total_identities:0})
    if(url.startsWith('/api/identities?'))return jsonResponse({identities})
    if(url.startsWith('/api/identities/')&&init?.method==='PATCH')return jsonResponse({identities})
    if(url==='/api/github/targets'&&init?.method==='POST')return jsonResponse({viewer:cachedDashboard.snapshot!.viewer,targets:dashboard.configuration.available_targets})
    if(url==='/api/configuration/github-target'&&init?.method==='PUT'){
      const targetId=(JSON.parse(String(init.body)) as {target_id:number}).target_id
      const selected=dashboard.configuration.available_targets.find((target)=>target.id===targetId)
      return jsonResponse({...dashboard,configuration:{...dashboard.configuration,selected_target:selected},sync:{id:'sync-1',state:'discovering',total_repositories:0,completed_repositories:0,failed_repositories:0,message:'Connecting to GitHub'}},202)
    }
    if(url==='/api/sync'&&init?.method==='POST')return failSync
      ? jsonResponse({error:'PAT is required'},400)
      : jsonResponse({sync:{id:'sync-1',state:'discovering',total_repositories:0,completed_repositories:0,failed_repositories:0,message:'Connecting to GitHub'}},202)
    return jsonResponse({error:'not found'},404)
  })
}

function jsonResponse(value:unknown,status=200){return new Response(JSON.stringify(value),{status,headers:{'Content-Type':'application/json'}})}
