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
  message?: string
  warnings?: string[]
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
}

export interface OwnerSummary {
  owner: OwnerIdentity
  repositories: number
  contributors: number
  commits: number
  lines_added: number
  lines_deleted: number
  pull_requests_opened: number
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

export interface ActivityPoint {
  month: string
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
  from?: string
  to?: string
  series: ActivitySeries[]
}

export interface ActivityOptions {
  groupBy: ActivityGroup
  metric: ActivityMetric
  ownerId?: number
  repositoryId?: number
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
  return requestJSON<ActivityResponse>(`/api/activity?${query}`, { signal })
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
