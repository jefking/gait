import {
  Activity,
  Archive,
  ArrowDown,
  ArrowUp,
  Building2,
  GitCommitHorizontal,
  GitFork,
  GitPullRequest,
  History,
  FileDown,
  LockKeyhole,
  RefreshCw,
  Search,
  Settings,
  Skull,
  Users,
  X,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import type {
  ActivityGroup,
  ActivityMetric,
  ActivityResponse,
  OwnerSummary,
  RepositorySummary,
  Snapshot,
  SyncStatus,
} from '../lib/api'
import { isSyncActive } from '../lib/api'
import { ActivityChart } from './ActivityChart'
import { Avatar } from './Avatar'
import { DateRangeSlider } from './DateRangeSlider'
import { OwnerSelect } from './OwnerSelect'

interface DashboardViewProps {
  snapshot: Snapshot
  sync: SyncStatus
  activity: ActivityResponse | null
  activityLoading: boolean
  groupBy: ActivityGroup
  metric: ActivityMetric
  ownerId?: number
  repositoryId?: number
  excludeDead: boolean
  dateFrom?: string
  dateTo?: string
  onGroupByChange: (group: ActivityGroup) => void
  onMetricChange: (metric: ActivityMetric) => void
  onOwnerChange: (id?: number) => void
  onRepositoryChange: (id?: number) => void
  onDateRangeChange: (from: string, to: string) => void
  onRefresh: () => void
  onSettings: () => void
}

const numbers = new Intl.NumberFormat()
const compact = new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 })

export function DashboardView({
  snapshot,
  sync,
  activity,
  activityLoading,
  groupBy,
  metric,
  ownerId,
  repositoryId,
  excludeDead,
  dateFrom,
  dateTo,
  onGroupByChange,
  onMetricChange,
  onOwnerChange,
  onRepositoryChange,
  onDateRangeChange,
  onRefresh,
  onSettings,
}: DashboardViewProps) {
  const [search, setSearch] = useState('')
  const repositoriesForOwner = ownerId
    ? snapshot.repositories.filter((repository) => repository.owner.id === ownerId && (!excludeDead || !repository.liveness?.is_dead))
    : snapshot.repositories.filter((repository) => !excludeDead || !repository.liveness?.is_dead)
  const selectedOwner = snapshot.owners.find((owner) => owner.owner.id === ownerId)
  const selectedRepository = snapshot.repositories.find((repository) => repository.id === repositoryId)

  const filteredRepositories = useMemo(() => {
    const query = search.trim().toLowerCase()
    return snapshot.repositories.filter((repository) => {
      if (excludeDead && repository.liveness?.is_dead) return false
      return !query || `${repository.full_name} ${repository.description ?? ''}`.toLowerCase().includes(query)
    })
  }, [excludeDead, search, snapshot.repositories])

  const deadRepositories = snapshot.totals.dead_repositories
    ?? snapshot.repositories.filter((repository) => repository.liveness?.is_dead).length

  const summaryCards = [
    { label: 'Repositories', value: snapshot.totals.repositories, icon: GitFork, detail: `${snapshot.totals.owners} owners · ${deadRepositories} dead` },
    { label: 'Commits', value: snapshot.totals.commits, icon: GitCommitHorizontal, detail: `${snapshot.totals.contributors} contributors` },
    { label: 'Pull requests', value: snapshot.totals.pull_requests_opened, icon: GitPullRequest, detail: `${snapshot.totals.pull_requests_merged} merged` },
    { label: 'Lines changed', value: snapshot.totals.lines_added + snapshot.totals.lines_deleted, icon: Activity, detail: `${compact.format(snapshot.totals.files_changed)} files touched` },
  ]

  return (
    <div className="dashboard-shell mx-auto w-full max-w-[1480px] px-4 pb-16 pt-5 sm:px-6 lg:px-8">
      <header className="report-header flex flex-col justify-between gap-5 border-b border-white/8 pb-6 sm:flex-row sm:items-center">
        <div className="flex items-center gap-4">
          <div className="grid size-11 place-items-center rounded-2xl bg-cyan-300 text-slate-950 shadow-lg shadow-cyan-400/10">
            <Activity aria-hidden="true" className="size-6" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-xl font-semibold tracking-tight text-white">Gait</h1>
              <span className="rounded-full border border-white/8 bg-white/[0.04] px-2 py-0.5 text-[10px] font-semibold uppercase tracking-widest text-slate-500">
                Git intelligence
              </span>
            </div>
            <p className="mt-1 text-xs text-slate-500">
              Updated {formatDate(snapshot.generated_at)}
            </p>
          </div>
        </div>
        <div className="flex items-center justify-between gap-3 sm:justify-end">
          <a
            href={snapshot.viewer.html_url}
            target="_blank"
            rel="noreferrer"
            className="flex items-center gap-3 rounded-xl px-2 py-1.5 transition hover:bg-white/5"
          >
            <Avatar src={snapshot.viewer.avatar_url} name={snapshot.viewer.name || snapshot.viewer.login} size="sm" />
            <span className="hidden text-left sm:block">
              <span className="block text-sm font-medium text-slate-200">{snapshot.viewer.name || snapshot.viewer.login}</span>
              <span className="block text-xs text-slate-500">@{snapshot.viewer.login}</span>
            </span>
          </a>
          <div className="no-print flex items-center gap-2">
            <button
              type="button"
              onClick={() => window.print()}
              aria-label="Export PDF"
              title="Export PDF"
              className="grid size-10 place-items-center rounded-xl border border-white/10 bg-white/5 text-slate-200 transition hover:border-cyan-300/30 hover:bg-cyan-300/5"
            >
              <FileDown aria-hidden="true" className="size-4" />
            </button>
            <button
              type="button"
              onClick={onRefresh}
              disabled={isSyncActive(sync)}
              aria-label="Refresh repositories"
              title="Refresh repositories"
              className="grid size-10 place-items-center rounded-xl border border-white/10 bg-white/5 text-slate-200 transition hover:border-cyan-300/30 hover:bg-cyan-300/5 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <RefreshCw aria-hidden="true" className={`size-4 ${isSyncActive(sync) ? 'animate-spin' : ''}`} />
            </button>
            <button
              type="button"
              onClick={onSettings}
              disabled={isSyncActive(sync)}
              aria-label="GitHub settings"
              title="GitHub settings"
              className="grid size-10 place-items-center rounded-xl border border-white/10 bg-white/5 text-slate-200 transition hover:border-cyan-300/30 hover:bg-cyan-300/5 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <Settings aria-hidden="true" className="size-4" />
            </button>
          </div>
        </div>
      </header>

      <SyncNotification sync={sync} />

      <section aria-label="Repository totals" className="report-summary mt-7 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {summaryCards.map(({ label, value, icon: Icon, detail }) => (
          <article key={label} className="report-metric metric-card rounded-2xl border border-white/8 bg-slate-900/60 p-5">
            <div className="flex items-start justify-between">
              <div>
                <p className="text-xs font-medium uppercase tracking-wider text-slate-500">{label}</p>
                <p className="mt-2 text-3xl font-semibold tracking-tight text-white">{compact.format(value)}</p>
                <p className="mt-1 text-xs text-slate-500">{detail}</p>
              </div>
              <span className="rounded-xl bg-white/[0.04] p-2.5 text-cyan-300 ring-1 ring-white/8">
                <Icon aria-hidden="true" className="size-5" />
              </span>
            </div>
          </article>
        ))}
      </section>

      <section className="report-activity mt-7 rounded-3xl border border-white/8 bg-slate-900/55 p-4 shadow-2xl shadow-black/10 sm:p-6">
        <div className="flex flex-col gap-5 xl:flex-row xl:items-end xl:justify-between">
          <div>
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-widest text-cyan-300">
              <History aria-hidden="true" className="size-4" />
              All-time activity
            </div>
            <h2 className="mt-2 text-xl font-semibold text-white">Momentum over time</h2>
            <p className="mt-1 text-sm text-slate-500">Activity on each repository’s default branch.</p>
            <p className="print-only report-scope">
              {metric === 'commits' ? 'Commits' : 'Pull requests opened'} by {groupBy}
              {' · '}{selectedOwner?.owner.login ?? 'All owners'}
              {' · '}{selectedRepository?.full_name ?? 'All repositories'}
              {dateFrom && dateTo ? ` · ${dateFrom} to ${dateTo}` : ''}
              {activity?.granularity ? ` · ${activity.granularity}` : ''}
            </p>
          </div>
          <div className="no-print grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
            <SegmentedControl
              label="Metric"
              value={metric}
              options={[['commits', 'Commits'], ['pull_requests', 'PRs opened']]}
              onChange={(value) => onMetricChange(value as ActivityMetric)}
            />
            <SegmentedControl
              label="Group by"
              value={groupBy}
              options={[['owner', 'Owner'], ['contributor', 'Contributor']]}
              onChange={(value) => onGroupByChange(value as ActivityGroup)}
            />
            <OwnerSelect
              owners={snapshot.owners}
              value={ownerId}
              onChange={(next) => {
                onOwnerChange(next)
                if (repositoryId && !snapshot.repositories.some((repo) => repo.id === repositoryId && (!next || repo.owner.id === next))) {
                  onRepositoryChange(undefined)
                }
              }}
            />
            <label className="block text-xs font-medium text-slate-500">
              Repository
              <select
                value={repositoryId ?? ''}
                onChange={(event) => onRepositoryChange(event.target.value ? Number(event.target.value) : undefined)}
                className="mt-1.5 w-full rounded-lg border border-white/10 bg-slate-950 px-3 py-2 text-sm text-slate-200 outline-none focus:border-cyan-400/50"
              >
                <option value="">All repositories</option>
                {repositoriesForOwner.map((repository) => <option key={repository.id} value={repository.id}>{repository.name}</option>)}
              </select>
            </label>
          </div>
        </div>
        {activity?.available_from && activity.available_to && dateFrom && dateTo && (
          <div className="no-print">
            <DateRangeSlider
              availableFrom={activity.available_from}
              availableTo={activity.available_to}
              from={dateFrom}
              to={dateTo}
              granularity={activity.granularity}
              onChange={onDateRangeChange}
            />
          </div>
        )}
        <div className="report-chart mt-7">
          <ActivityChart activity={activity} loading={activityLoading} />
        </div>
      </section>

      <div className="report-detail-grid mt-7 grid items-start gap-7 xl:grid-cols-[minmax(0,1fr)_360px]">
        <section className="report-portfolio min-w-0 rounded-3xl border border-white/8 bg-slate-900/55 p-4 sm:p-6">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-widest text-cyan-300">Repository portfolio</p>
              <h2 className="mt-2 text-xl font-semibold text-white">Owners and repositories</h2>
            </div>
            <div className="no-print flex flex-col gap-2 sm:flex-row sm:items-center">
              <label className="relative block sm:w-72">
                <span className="sr-only">Search repositories</span>
                <Search aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-slate-500" />
                <input
                  type="search"
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="Search repositories"
                  className="w-full rounded-xl border border-white/10 bg-slate-950 py-2.5 pl-10 pr-3 text-sm text-slate-200 outline-none placeholder:text-slate-600 focus:border-cyan-400/50"
                />
              </label>
            </div>
          </div>

          <div className="report-owner-list mt-6 space-y-3">
            {snapshot.owners.map((owner) => (
              <OwnerGroup
                key={owner.owner.id}
                owner={owner}
                repositories={filteredRepositories.filter((repository) => repository.owner.id === owner.owner.id)}
              />
            ))}
            {filteredRepositories.length === 0 && (
              <p className="rounded-2xl border border-dashed border-white/10 py-12 text-center text-sm text-slate-500">
                {excludeDead && !search ? 'All repositories are excluded by project settings.' : `No repositories match “${search}”.`}
              </p>
            )}
          </div>
        </section>

        <section className="report-contributors rounded-3xl border border-white/8 bg-slate-900/55 p-5 sm:p-6">
          <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-widest text-cyan-300">
            <Users aria-hidden="true" className="size-4" />
            Contributors
          </div>
          <h2 className="mt-2 text-xl font-semibold text-white">Most active people</h2>
          <div className="mt-5 divide-y divide-white/6">
            {snapshot.contributors.slice(0, 20).map((contributor, index) => (
              <article key={contributor.key} className="report-contributor-row flex items-center gap-3 py-3 first:pt-0">
                <span className="w-5 text-right text-xs tabular-nums text-slate-600">{index + 1}</span>
                <Avatar src={contributor.avatar_url} name={contributor.name} size="sm" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-slate-200">{contributor.name}</p>
                  <p className="truncate text-xs text-slate-500">
                    {numbers.format(contributor.commits)} commits · {numbers.format(contributor.pull_requests)} PRs
                  </p>
                </div>
                <span className="text-xs font-medium tabular-nums text-cyan-300">
                  {compact.format(contributor.commits + contributor.pull_requests)}
                </span>
              </article>
            ))}
          </div>
        </section>
      </div>
    </div>
  )
}

export function SyncNotification({ sync }: { sync: SyncStatus }) {
  const [dismissedFailure, setDismissedFailure] = useState<string>()
  const active = isSyncActive(sync)
  const failureKey = `${sync.id ?? 'sync'}:${sync.state}`

  // Completed jobs leave the screen clean. Repository-level warnings remain
  // visible on their corresponding repository rows.
  if (!active && sync.state !== 'failed') return null
  if (sync.state === 'failed' && dismissedFailure === failureKey) return null

  const progress = sync.total_repositories > 0
    ? Math.round((sync.completed_repositories / sync.total_repositories) * 100)
    : 0
  const tone = sync.state === 'failed'
    ? 'border-rose-400/25 bg-slate-900/95 text-rose-100 shadow-rose-950/30'
    : 'border-cyan-300/25 bg-slate-900/95 text-cyan-100 shadow-cyan-950/30'
  const workflow = sync.current_workflows?.[0]

  return (
    <aside
      aria-live="polite"
      aria-atomic="true"
      className={`no-print fixed right-4 top-4 z-40 w-[min(26rem,calc(100vw-2rem))] overflow-hidden rounded-2xl border shadow-2xl backdrop-blur-xl sm:right-6 sm:top-6 ${tone}`}
    >
      <div className="flex items-start gap-3 px-4 py-3.5">
        <RefreshCw aria-hidden="true" className={`mt-0.5 size-4 shrink-0 ${active ? 'animate-spin' : ''}`} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-sm font-medium">{sync.message ?? sync.state}</p>
            {active && sync.total_repositories > 0 && (
              <span className="text-xs tabular-nums opacity-75">
                {sync.completed_repositories} / {sync.total_repositories} repositories
              </span>
            )}
          </div>
          {workflow ? (
            <p className="mt-1 truncate text-xs opacity-65">{workflow.message}</p>
          ) : sync.current_repositories && sync.current_repositories.length > 0 && (
            <p className="mt-1 truncate text-xs opacity-65">Updating {sync.current_repositories.join(', ')}</p>
          )}
        </div>
        {sync.state === 'failed' && (
          <button
            type="button"
            onClick={() => setDismissedFailure(failureKey)}
            aria-label="Dismiss sync notification"
            className="-mr-1 -mt-1 rounded-lg p-1.5 opacity-60 transition hover:bg-white/10 hover:opacity-100"
          >
            <X aria-hidden="true" className="size-4" />
          </button>
        )}
      </div>
      {active && sync.total_repositories > 0 && (
        <div className="h-0.5 bg-white/5"><div className="h-full bg-cyan-300 transition-[width]" style={{ width: `${progress}%` }} /></div>
      )}
    </aside>
  )
}

function SegmentedControl({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: [string, string][]
  onChange: (value: string) => void
}) {
  return (
    <fieldset>
      <legend className="text-xs font-medium text-slate-500">{label}</legend>
      <div className="mt-1.5 flex rounded-lg border border-white/10 bg-slate-950 p-0.5">
        {options.map(([option, text]) => (
          <button
            key={option}
            type="button"
            onClick={() => onChange(option)}
            className={`flex-1 whitespace-nowrap rounded-md px-2.5 py-1.5 text-xs font-medium transition ${value === option ? 'bg-white/10 text-white' : 'text-slate-500 hover:text-slate-300'}`}
          >
            {text}
          </button>
        ))}
      </div>
    </fieldset>
  )
}

function OwnerGroup({ owner, repositories }: { owner: OwnerSummary; repositories: RepositorySummary[] }) {
  if (repositories.length === 0) return null
  return (
    <details className="report-owner-group group overflow-hidden rounded-2xl border border-white/8 bg-slate-950/35" open={repositories.length <= 6}>
      <summary className="report-owner-summary flex cursor-pointer list-none items-center gap-3 px-4 py-4 transition hover:bg-white/[0.025] [&::-webkit-details-marker]:hidden">
        <Avatar src={owner.owner.avatar_url} name={owner.owner.login} size="md" />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h3 className="truncate font-semibold text-slate-100">{owner.owner.login}</h3>
            {owner.owner.type === 'Organization' && <Building2 aria-label="Organization" className="size-3.5 text-slate-500" />}
          </div>
          <p className="mt-1 text-xs text-slate-500">
            {owner.repositories} repos · {compact.format(owner.commits)} commits · {compact.format(owner.pull_requests_opened)} PRs
            {Boolean(owner.dead_repositories) && ` · ${owner.dead_repositories} dead`}
          </p>
        </div>
        <span className="no-print text-xs text-slate-600 transition group-open:rotate-180">⌄</span>
      </summary>
      <div className="border-t border-white/6">
        {repositories.map((repository) => <RepositoryRow key={repository.id} repository={repository} />)}
      </div>
    </details>
  )
}

function RepositoryRow({ repository }: { repository: RepositorySummary }) {
  const dead = Boolean(repository.liveness?.is_dead)
  return (
    <article className={`report-repository-row grid gap-4 border-b border-white/5 px-4 py-4 last:border-b-0 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center ${dead ? 'bg-rose-300/[0.035]' : ''}`}>
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <a href={repository.html_url} target="_blank" rel="noreferrer" className="truncate text-sm font-semibold text-slate-200 transition hover:text-cyan-300">
            {repository.name}
          </a>
          {repository.private && <LockKeyhole aria-label="Private" className="size-3.5 text-amber-300/70" />}
          {repository.archived && <Archive aria-label="Archived" className="size-3.5 text-slate-500" />}
          {repository.fork && <GitFork aria-label="Fork" className="size-3.5 text-slate-500" />}
          {dead && (
            <span
              title={livenessDescription(repository)}
              aria-label={`Dead repository. ${livenessDescription(repository)}`}
              className="inline-grid size-5 place-items-center rounded-full border border-rose-300/25 bg-rose-300/10 text-rose-200"
            >
              <Skull aria-hidden="true" className="size-3" />
            </span>
          )}
          {repository.sync_status === 'warning' && (
            <span title={repository.sync_message} className="rounded-full bg-amber-300/10 px-2 py-0.5 text-[10px] font-medium text-amber-200">cached with warning</span>
          )}
        </div>
        <p className="mt-1 line-clamp-1 text-xs text-slate-500">{repository.description || `Default branch: ${repository.default_branch}`}</p>
      </div>
      <div className="grid grid-cols-2 gap-x-6 gap-y-2 text-xs sm:grid-cols-4 lg:min-w-[430px]">
        <Stat label="Commits" value={repository.commits} />
        <Stat label="Contributors" value={repository.contributors} />
        <div>
          <p className="text-slate-600">Lines</p>
          <p className="mt-0.5 flex items-center gap-2 font-medium tabular-nums text-slate-300">
            <span className="inline-flex items-center text-emerald-300"><ArrowUp className="size-3" />{compact.format(repository.lines_added)}</span>
            <span className="inline-flex items-center text-rose-300"><ArrowDown className="size-3" />{compact.format(repository.lines_deleted)}</span>
          </p>
        </div>
        <div>
          <p className="text-slate-600">Pull requests</p>
          <p className="mt-0.5 font-medium tabular-nums text-slate-300">
            {repository.pull_requests ? `${numbers.format(repository.pull_requests.opened)} · ${numbers.format(repository.pull_requests.merged)} merged` : <span className="text-amber-200/70">Unavailable</span>}
          </p>
        </div>
      </div>
    </article>
  )
}

function Stat({ label, value }: { label: string; value: number }) {
  return <div><p className="text-slate-600">{label}</p><p className="mt-0.5 font-medium tabular-nums text-slate-300">{numbers.format(value)}</p></div>
}

function livenessDescription(repository: RepositorySummary) {
  const liveness = repository.liveness
  if (!liveness) return 'Repository liveness is unavailable.'
  const inactive = `${numbers.format(liveness.inactive_days ?? 0)} days`
  if (liveness.reason === 'no_default_branch_commits') {
    return `No default-branch commits; repository has been inactive for ${inactive}.`
  }
  const value = liveness.threshold_value ?? liveness.threshold_days ?? 0
  const scale = liveness.scale ?? 'day'
  return `No default-branch changes for ${inactive}; dead threshold is ${value} ${scale}${value === 1 ? '' : 's'} (25% of its working lifespan).`
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}
