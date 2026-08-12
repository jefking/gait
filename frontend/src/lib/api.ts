export type SyncState =
  | 'idle'
  | 'discovering'
  | 'syncing'
  | 'waiting_rate_limit'
  | 'complete'
  | 'complete_with_warnings'
  | 'failed'

export interface SyncStatus {
  id?: string
  state: SyncState
  started_at?: string
  finished_at?: string
  rate_limit_reset_at?: string
  total_repositories: number
  completed_repositories: number
  failed_repositories: number
  current_repositories?: string[]
  current_workflows?: RepositoryWorkflow[]
  message?: string
  warnings?: string[]
}

export interface RepositoryWorkflow {
  repository_id: number
  full_name: string
  stage: 'updating_git' | 'analyzing' | 'pull_requests' | 'publishing'
  message: string
}

export interface DashboardEvent {
  type: 'sync' | 'snapshot' | 'dashboard' | 'insights'
  revision: number
  repository?: RepositoryEventMetadata
}

export interface RepositoryEventMetadata {
  id: number
  full_name: string
  sync_status: string
  liveness: RepositoryLiveness
}

export interface Viewer {
  login: string
  name: string
  avatar_url: string
  html_url: string
}

export interface OwnerIdentity {
  id: number
  login: string
  type: string
  avatar_url: string
  html_url: string
}

export interface DashboardTotals {
  owners: number
  repositories: number
  contributors: number
  commits: number
  files_changed: number
  lines_added: number
  lines_deleted: number
  pull_requests_opened: number
  pull_requests_open: number
  pull_requests_closed: number
  pull_requests_merged: number
  repositories_without_pr_access: number
  dead_repositories?: number
}

export interface OwnerSummary {
  owner: OwnerIdentity
  repositories: number
  contributors: number
  commits: number
  lines_added: number
  lines_deleted: number
  pull_requests_opened: number
  dead_repositories?: number
}

export type RepositoryLivenessState = 'active' | 'dead' | 'unknown'

export interface RepositoryLiveness {
  state: RepositoryLivenessState
  is_dead: boolean
  basis: 'default_branch_commits'
  reason?: string
  scale?: 'day' | 'week' | 'month' | 'year'
  threshold_value?: number
  threshold_days?: number
  active_span_days?: number
  inactive_days?: number
  first_change_at?: string
  last_change_at?: string
  repository_created_at?: string
  evaluated_at: string
}

export interface ContributorSummary {
  key: string
  login?: string
  name: string
  avatar_url?: string
  type?: string
  commits: number
  pull_requests: number
  repositories: number
  owners: number
  lines_added: number
  lines_deleted: number
  last_activity_at?: string
}

export interface PullRequestTotals {
  opened: number
  open: number
  closed: number
  merged: number
}

export interface RepositorySummary {
  id: number
  name: string
  full_name: string
  html_url: string
  description?: string
  default_branch: string
  private: boolean
  archived: boolean
  fork: boolean
  created_at?: string
  owner: OwnerIdentity
  commits: number
  contributors: number
  files_changed: number
  lines_added: number
  lines_deleted: number
  pull_requests: PullRequestTotals | null
  last_activity_at?: string
  sync_status: string
  sync_message?: string
  liveness?: RepositoryLiveness
}

export interface Snapshot {
  generated_at: string
  viewer: Viewer
  totals: DashboardTotals
  owners: OwnerSummary[]
  contributors: ContributorSummary[]
  repositories: RepositorySummary[]
}

export interface DashboardResponse {
  snapshot: Snapshot | null
  sync: SyncStatus
}

export type ActivityGroup = 'owner' | 'contributor'
export type ActivityMetric = 'commits' | 'pull_requests'
export type ActivityGranularity = 'day' | 'week' | 'month'

export interface ActivityPoint {
  date?: string
  month?: string
  value: number
}

export interface ActivitySeries {
  key: string
  label: string
  avatar_url?: string
  total: number
  points: ActivityPoint[]
}

export interface ActivityResponse {
  group_by: ActivityGroup
  metric: ActivityMetric
  granularity?: ActivityGranularity
  available_from?: string
  available_to?: string
  from?: string
  to?: string
  series: ActivitySeries[]
}

export interface ActivityOptions {
  groupBy: ActivityGroup
  metric: ActivityMetric
  ownerId?: number
  repositoryId?: number
  excludeDead?: boolean
  from?: string
  to?: string
}

export type ActorKind = 'human' | 'agent' | 'unknown'
export type WorkKind = 'human_only' | 'agent_only' | 'mixed' | 'unknown'
export type RankCohort = 'agents' | 'humans' | 'human_agent' | 'human_human'
export type RankMetric = 'commits' | 'pull_requests' | 'revert_rate' | 'retained_line_rate' | 'interaction_days' | 'handoffs' | 'review_interactions'

export interface InsightCoverage {
  total_commits: number
  classified_commits: number
  unknown_commits: number
  classification_rate: number
  mature_commits: number
  eligible_pull_requests: number
  reviewed_pull_requests: number
}

export interface InsightMeta {
  available_from?: string
  available_to?: string
  from?: string
  to?: string
  granularity?: ActivityGranularity
  session_hours: number
  adoption_days: number
  survival_days: number
  coverage: InsightCoverage
  unavailable?: string[]
  truncated?: boolean
  total_results?: number
}

export interface IdentitySummary {
  key: string
  canonical_key: string
  name: string
  login?: string
  avatar_url?: string
  kind: ActorKind
  evidence: string
  confidence: 'confirmed' | 'suggested' | 'unknown'
  aliases?: string[]
  commits: number
  pull_requests: number
  reviews: number
}

export interface InsightSummary {
  agent_participation: number
  handoff_lift?: number
  handoff_episodes: number
  quality_direction?: number
  strongest_pair?: string
  strongest_pair_days: number
}

export interface TimelinePoint {
  date: string
  human_only: number
  agent_only: number
  mixed: number
  unknown: number
  pull_requests: number
}

export interface QualityPoint {
  date: string
  revert_rate?: number
  merge_rate?: number
  median_merge_hours?: number
  review_coverage?: number
  retained_line_rate?: number
  commit_sample: number
  pull_request_sample: number
  retention_sample?: number
}

export interface PulsePoint {
  date: string
  human_only: number
  agent_only: number
  mixed: number
  unknown: number
}

export interface RepositoryPulse {
  repository_id: number
  name: string
  total: number
  points: PulsePoint[]
}

export interface OverviewResponse {
  meta: InsightMeta
  summary: InsightSummary
  timeline: TimelinePoint[]
  quality: QualityPoint[]
  repositories: RepositoryPulse[]
}

export interface NetworkNode extends IdentitySummary {
  activity: number
}

export interface NetworkEdge {
  source: string
  target: string
  pair_type: 'human_agent' | 'human_human' | 'agent_agent' | 'unknown'
  interaction_days: number
  coauthorships: number
  review_interactions: number
  handoffs: number
  human_to_agent: number
  repositories: string[]
  periods?: string[]
}

export interface NetworkResponse {
  meta: InsightMeta
  nodes: NetworkNode[]
  edges: NetworkEdge[]
  total_identities: number
}

export interface RampPoint {
  key: string
  human: IdentitySummary
  agent: IdentitySummary
  episodes: number
  completed_episodes: number
  interaction_days: number
  baseline: number
  after: number
  absolute_change: number
  observed_lift?: number
  quality_delta?: number
  mature: boolean
  rank_eligible: boolean
}

export interface AdoptionPoint {
  repository_id: number
  repository: string
  adopted_at: string
  baseline: number
  after: number
  absolute_change: number
  observed_lift?: number
  quality_delta?: number
  mature: boolean
}

export interface RampResponse {
  meta: InsightMeta
  handoffs: RampPoint[]
  adoptions: AdoptionPoint[]
}

export interface RankEntry {
  key: string
  label: string
  kind?: ActorKind
  rank: number
  value: number
  eligible: boolean
  metrics: Record<string, number>
}

export interface RankSeries {
  key: string
  label: string
  points: Array<{ date: string; rank: number; value: number }>
}

export interface RankingResponse {
  meta: InsightMeta
  cohort: RankCohort
  metric: RankMetric
  favorable_direction: 'higher' | 'lower'
  leaderboard: RankEntry[]
  trajectories: RankSeries[]
}

export interface InsightFilters {
  ownerId?: number
  repositoryId?: number
  actorKind?: ActorKind
  excludeDead?: boolean
  from?: string
  to?: string
  sessionHours?: number
  adoptionDays?: number
  survivalDays?: number
}

export interface InsightBundle {
  overview: OverviewResponse
  network: NetworkResponse
  ramps: RampResponse
  rankings: RankingResponse
}

export class APIError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'APIError'
    this.status = status
  }
}

async function requestJSON<T>(
  input: RequestInfo | URL,
  init?: RequestInit,
): Promise<T> {
  const response = await fetch(input, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...init?.headers,
    },
  })
  const result: unknown = await response.json().catch(() => null)
  if (!response.ok) {
    const message =
      typeof result === 'object' &&
      result !== null &&
      'error' in result &&
      typeof result.error === 'string'
        ? result.error
        : `Request failed with status ${response.status}`
    throw new APIError(message, response.status)
  }
  return result as T
}

export function getDashboard(signal?: AbortSignal): Promise<DashboardResponse> {
  return requestJSON<DashboardResponse>('/api/dashboard', { signal })
}

export function getActivity(
  options: ActivityOptions,
  signal?: AbortSignal,
): Promise<ActivityResponse> {
  const query = new URLSearchParams({
    group_by: options.groupBy,
    metric: options.metric,
  })
  if (options.ownerId) query.set('owner_id', String(options.ownerId))
  if (options.repositoryId) {
    query.set('repository_id', String(options.repositoryId))
  }
  if (options.excludeDead) query.set('exclude_dead', 'true')
  if (options.from) query.set('from', options.from)
  if (options.to) query.set('to', options.to)
  return requestJSON<ActivityResponse>(`/api/activity?${query}`, { signal })
}

function insightQuery(filters: InsightFilters) {
  const query = new URLSearchParams()
  if (filters.ownerId) query.set('owner_id', String(filters.ownerId))
  if (filters.repositoryId) query.set('repository_id', String(filters.repositoryId))
  if (filters.actorKind) query.set('actor_kind', filters.actorKind)
  if (filters.excludeDead) query.set('exclude_dead', 'true')
  if (filters.from) query.set('from', filters.from)
  if (filters.to) query.set('to', filters.to)
  if (filters.sessionHours) query.set('session_hours', String(filters.sessionHours))
  if (filters.adoptionDays) query.set('adoption_days', String(filters.adoptionDays))
  if (filters.survivalDays) query.set('survival_days', String(filters.survivalDays))
  return query
}

export async function getInsights(
  filters: InsightFilters,
  cohort: RankCohort,
  metric: RankMetric,
  signal?: AbortSignal,
): Promise<InsightBundle> {
  const base = insightQuery(filters)
  const ranking = new URLSearchParams(base)
  ranking.set('cohort', cohort)
  ranking.set('metric', metric)
  const [overview, network, ramps, rankings] = await Promise.all([
    requestJSON<OverviewResponse>(`/api/insights/overview?${base}`, { signal }),
    requestJSON<NetworkResponse>(`/api/insights/network?${base}`, { signal }),
    requestJSON<RampResponse>(`/api/insights/ramps?${base}`, { signal }),
    requestJSON<RankingResponse>(`/api/insights/rankings?${ranking}`, { signal }),
  ])
  return { overview, network, ramps, rankings }
}

export function getIdentities(signal?: AbortSignal): Promise<{ identities: IdentitySummary[] }> {
  return requestJSON<{ identities: IdentitySummary[] }>('/api/identities', { signal })
}

export function updateIdentity(
  key: string,
  update: { kind?: ActorKind; display_name?: string; canonical_key?: string; unmerge?: boolean },
  signal?: AbortSignal,
): Promise<{ identities: IdentitySummary[] }> {
  return requestJSON<{ identities: IdentitySummary[] }>(`/api/identities/${encodeURIComponent(key)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(update),
    signal,
  })
}

export async function startSync(
  pat: string,
  signal?: AbortSignal,
): Promise<SyncStatus> {
  const response = await requestJSON<{ sync: SyncStatus }>('/api/sync', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ pat }),
    signal,
  })
  return response.sync
}

export function isSyncActive(status: SyncStatus): boolean {
  return (
    status.state === 'discovering' ||
    status.state === 'syncing' ||
    status.state === 'waiting_rate_limit'
  )
}

export function subscribeToDashboardEvents(
  onEvent: (event: DashboardEvent) => void,
  onError?: () => void,
): () => void {
  if (typeof EventSource === 'undefined') return () => undefined
  const source = new EventSource('/api/events')
  const handleEvent = (event: Event) => {
    try {
      onEvent(JSON.parse((event as MessageEvent<string>).data) as DashboardEvent)
    } catch {
      // A malformed invalidation can safely be ignored; EventSource reconnects
      // and the polling fallback still reads canonical state.
    }
  }
  source.addEventListener('dashboard', handleEvent)
  if (onError) source.addEventListener('error', onError)
  return () => source.close()
}
