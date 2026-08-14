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
  stage: 'updating_git' | 'analyzing' | 'pull_requests' | 'delivery_evidence' | 'publishing'
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

export type ActivityGranularity = 'day' | 'week' | 'month'

export type ActorKind = 'human' | 'agent' | 'unknown'

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

export interface DeliveryCoverage {
  organizations: number
  repositories: number
  index_eligible_repositories: number
  merged_pull_requests: number
  attributed_pull_requests: number
  unattributed_pull_requests: number
  incomplete_authorship_evidence_pull_requests: number
  unknown_authorship_identity_pull_requests: number
  detailed_pull_requests: number
  complete_commit_evidence_pull_requests: number
  truncated_commit_evidence_pull_requests: number
  attribution_rate: number
  actions_runs: number
  actions_covered_pull_requests: number
  actions_permission_denied: boolean
  actions_truncated: boolean
  direct_commit_coverage: boolean
  low_confidence_baseline: boolean
}

export interface DeliveryMeta {
  available_from?: string
  available_to?: string
  from?: string
  to?: string
  granularity?: ActivityGranularity
  scope: {
    organization_id?: number
    organization?: string
    repository_id?: number
    repository?: string
    exclude_dead: boolean
  }
  coverage: DeliveryCoverage
  unavailable?: string[]
}

export interface DeliveryRawMetrics {
  merged_pull_requests: number
  additions: number
  deletions: number
  changed_lines: number
  commits: number
  direct_commits: number
}

export interface DeliveryModeMetrics extends DeliveryRawMetrics { index: number }

export interface DeliveryVelocityPoint {
  date: string
  human: DeliveryModeMetrics
  agent: DeliveryModeMetrics
  collaborative: DeliveryModeMetrics
  authorship_unknown: DeliveryRawMetrics
  total_index: number
  complete: boolean
}

export type DeliveryPerformanceMode = 'human' | 'human_human' | 'human_agent' | 'agent'

export interface DeliveryPerformanceBreakdown {
  human: DeliveryModeMetrics
  human_human: DeliveryModeMetrics
  human_agent: DeliveryModeMetrics
  agent: DeliveryModeMetrics
  authorship_unknown: DeliveryRawMetrics
  total_index: number
  leader: DeliveryPerformanceMode | 'tie' | 'none'
}

export interface DeliveryPerformancePoint extends DeliveryPerformanceBreakdown {
  date: string
}

export interface DeliveryQualityPoint {
  date: string
  actions_failure_incidence?: number
  actions_pull_sample: number
  failed_actions_attempts: number
  total_actions_attempts: number
  revert_rate?: number
  commit_sample: number
  review_coverage?: number
  review_sample: number
  retained_line_rate?: number
  retention_sample: number
  median_merge_hours?: number
  p90_merge_hours?: number
  merge_time_sample: number
}

export interface DeliveryQualitySignal {
  key: string
  label: string
  direction: 'improving' | 'declining' | 'inconclusive' | 'insufficient'
  delta?: number
  interval_low?: number
  interval_high?: number
  sample: number
}

export interface DeliveryFlowPoint {
  date: string
  merged_pull_requests: number
  median_changed_lines?: number
  p90_changed_lines?: number
  median_commits?: number
  p90_commits?: number
  median_additions?: number
  p90_additions?: number
  median_deletions?: number
  p90_deletions?: number
}

export interface DeliveryImpact {
  tier: 'matched_difference_in_differences' | 'paired_pre_post' | 'insufficient_evidence'
  verdict: string
  estimate?: number
  interval_low?: number
  interval_high?: number
  treated_repositories: number
  control_repositories: number
  adoption_coverage: number
  pre_weeks: number
  post_weeks: number
  quality_deltas: Array<{ key: string; delta?: number; interval_low?: number; interval_high?: number; sample: number }>
}

export interface DeliveryResponse {
  meta: DeliveryMeta
  summary: {
    narrative: string
    velocity_vs_baseline?: number
    agent_associated_share: number
    quality_direction: 'improving' | 'declining' | 'mixed' | 'stable/inconclusive' | 'insufficient'
    leader: string
  }
  velocity: DeliveryVelocityPoint[]
  performance: {
    daily: DeliveryPerformancePoint[]
    overall: DeliveryPerformanceBreakdown
  }
  raw: {
    human: DeliveryRawMetrics
    agent: DeliveryRawMetrics
    collaborative: DeliveryRawMetrics
    authorship_unknown: DeliveryRawMetrics
    total: DeliveryRawMetrics
  }
  quality: { direction: string; signals: DeliveryQualitySignal[]; points: DeliveryQualityPoint[] }
  flow: {
    summary: {
      as_of: string
      open_pull_requests: number
      median_open_age_days?: number
      p90_open_age_days?: number
      median_changed_lines?: number
      p90_changed_lines?: number
      median_commits?: number
      p90_commits?: number
      median_additions?: number
      p90_additions?: number
      median_deletions?: number
      p90_deletions?: number
      merged_pull_request_sample: number
    }
    points: DeliveryFlowPoint[]
  }
  impact: DeliveryImpact
}

export interface DeliveryFilters {
  organizationId?: number
  excludeDead?: boolean
  from?: string
  to?: string
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

function deliveryQuery(filters: DeliveryFilters) {
  const query = new URLSearchParams()
  if (filters.organizationId) query.set('organization_id', String(filters.organizationId))
  if (filters.excludeDead) query.set('exclude_dead', 'true')
  if (filters.from) query.set('from', filters.from)
  if (filters.to) query.set('to', filters.to)
  return query
}

export function getInsightDelivery(filters: DeliveryFilters, signal?: AbortSignal): Promise<DeliveryResponse> {
  return requestJSON<DeliveryResponse>(`/api/insights/delivery?${deliveryQuery(filters)}`, { signal })
}

export function getInsightNetwork(filters: DeliveryFilters, signal?: AbortSignal): Promise<NetworkResponse> {
  return requestJSON<NetworkResponse>(`/api/insights/network?${deliveryQuery(filters)}`, { signal })
}

export function getIdentities(filters: DeliveryFilters = {}, signal?: AbortSignal): Promise<{ identities: IdentitySummary[] }> {
  return requestJSON<{ identities: IdentitySummary[] }>(`/api/identities?${deliveryQuery(filters)}`, { signal })
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
  pat?: string,
  signal?: AbortSignal,
): Promise<SyncStatus> {
  const response = await requestJSON<{ sync: SyncStatus }>('/api/sync', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(pat ? { pat } : {}),
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
