import { Database, LoaderCircle, RefreshCw, TriangleAlert } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { DashboardView } from './components/DashboardView'
import { TokenModal } from './components/TokenModal'
import {
  getDashboard,
  getIdentities,
  getInsightNetwork,
  getInsightOverview,
  getInsightRamps,
  getInsightRankings,
  isSyncActive,
  startSync,
  subscribeToDashboardEvents,
  type DashboardResponse,
  type ActorKind,
  type IdentitySummary,
  type NetworkResponse,
  type OverviewResponse,
  type RankCohort,
  type RankMetric,
  type RankingResponse,
  type RampResponse,
  updateIdentity as updateIdentityClassification,
} from './lib/api'

interface KeyedResult<T> {
  key: string
  scopeKey: string
  data: T
}

const dashboardEventBatchMilliseconds = 750
const contentRefreshIntervalMilliseconds = 2_500

function App() {
  const [dashboard, setDashboard] = useState<DashboardResponse | null>(null)
  const [loadingDashboard, setLoadingDashboard] = useState(true)
  const [dashboardError, setDashboardError] = useState<string>()
  const [modalOpen, setModalOpen] = useState(false)
  const [modalMode, setModalMode] = useState<'connect' | 'settings'>('connect')
  const [modalError, setModalError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const [ownerId, setOwnerId] = useState<number>()
  const [repositoryId, setRepositoryId] = useState<number>()
  const [excludeDead, setExcludeDead] = useState(false)
  const [actorKind, setActorKind] = useState<ActorKind>()
  const [dateRange, setDateRange] = useState<{ from: string; to: string; userSelected: boolean }>()
  const [windows, setWindows] = useState({ sessionHours: 72, adoptionDays: 30, survivalDays: 30 })
  const [rankCohort, setRankCohort] = useState<RankCohort>('agents')
  const [rankMetric, setRankMetric] = useState<RankMetric>('commits')
  const [insightEpoch, setInsightEpoch] = useState(0)
  const [identityResult, setIdentityResult] = useState<{ key: string; data: IdentitySummary[] }>()
  const [overviewResult, setOverviewResult] = useState<KeyedResult<OverviewResponse>>()
  const [networkResult, setNetworkResult] = useState<KeyedResult<NetworkResponse>>()
  const [rampResult, setRampResult] = useState<KeyedResult<RampResponse>>()
  const [rankingResult, setRankingResult] = useState<KeyedResult<RankingResponse>>()

  const contentGenerationRef = useRef('')
  const pendingContentGenerationRef = useRef('')
  const contentRefreshTimerRef = useRef<number | undefined>(undefined)
  const lastContentRefreshRef = useRef(0)
  const [contentGeneration, setContentGeneration] = useState('')

  const commitContentGeneration = useCallback(() => {
    contentRefreshTimerRef.current = undefined
    const generation = pendingContentGenerationRef.current
    if (!generation || generation === contentGenerationRef.current) return
    contentGenerationRef.current = generation
    lastContentRefreshRef.current = Date.now()
    setContentGeneration(generation)
  }, [])

  const queueContentGeneration = useCallback((generation?: string) => {
    if (!generation || generation === contentGenerationRef.current) return
    pendingContentGenerationRef.current = generation
    if (!contentGenerationRef.current) {
      window.clearTimeout(contentRefreshTimerRef.current)
      commitContentGeneration()
      return
    }
    if (contentRefreshTimerRef.current !== undefined) return
    const elapsed = Date.now() - lastContentRefreshRef.current
    contentRefreshTimerRef.current = window.setTimeout(
      commitContentGeneration,
      Math.max(0, contentRefreshIntervalMilliseconds - elapsed),
    )
  }, [commitContentGeneration])

  const applyDashboard = useCallback((response: DashboardResponse) => {
    setDashboard((current) => ({
      snapshot: current?.snapshot && current.snapshot.generated_at === response.snapshot?.generated_at
        ? current.snapshot
        : response.snapshot,
      sync: response.sync,
    }))
    queueContentGeneration(response.snapshot?.generated_at)
    setDashboardError(undefined)
    if (response.sync.state === 'failed') {
      setModalMode('connect')
      setModalError(response.sync.message || 'The sync failed.')
      setModalOpen(true)
    } else if (!response.snapshot && !isSyncActive(response.sync)) {
      setModalMode('connect')
      setModalOpen(true)
    }
  }, [queueContentGeneration])

  useEffect(() => () => window.clearTimeout(contentRefreshTimerRef.current), [])

  useEffect(() => {
    const controller = new AbortController()
    getDashboard(controller.signal)
      .then(applyDashboard)
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === 'AbortError') return
        setDashboardError(error instanceof Error ? error.message : 'Could not load the dashboard.')
        setModalMode('connect')
        setModalOpen(true)
      })
      .finally(() => setLoadingDashboard(false))
    return () => controller.abort()
  }, [applyDashboard])

  const syncActive = dashboard ? isSyncActive(dashboard.sync) : false
  const snapshotGeneratedAt = dashboard?.snapshot?.generated_at
  const identities = identityResult?.data ?? []
  const identitiesLoading = Boolean(snapshotGeneratedAt) && !identityResult
  const selectedFrom = dateRange?.userSelected ? dateRange.from : undefined
  const selectedTo = dateRange?.userSelected ? dateRange.to : undefined
  const identitiesClassified = !identitiesLoading && identities.every((identity) => identity.kind !== 'unknown')
  const insightScopeKey = dashboard?.snapshot && identitiesClassified
    ? [ownerId ?? '', repositoryId ?? '', actorKind ?? '', excludeDead, selectedFrom ?? '', selectedTo ?? '', windows.sessionHours, windows.adoptionDays, windows.survivalDays].join(':')
    : ''
  const insightRequestKey = insightScopeKey && contentGeneration
    ? `${contentGeneration}:${insightEpoch}:${insightScopeKey}`
    : ''
  const rankingScopeKey = insightScopeKey ? `${insightScopeKey}:${rankCohort}:${rankMetric}` : ''
  const rankingRequestKey = rankingScopeKey && contentGeneration
    ? `${contentGeneration}:${insightEpoch}:${rankingScopeKey}`
    : ''

  useEffect(() => {
    let cancelled = false
    let refreshTimer: number | undefined
    let controller: AbortController | undefined
    let refreshInFlight = false
    let refreshQueued = false
    const refreshDashboard = () => {
      if (refreshInFlight) {
        refreshQueued = true
        return
      }
      refreshInFlight = true
      controller = new AbortController()
      void getDashboard(controller.signal)
        .then((response) => {
          if (!cancelled) applyDashboard(response)
        })
        .catch((error: unknown) => {
          if (cancelled || (error instanceof DOMException && error.name === 'AbortError')) return
          setDashboardError(error instanceof Error ? error.message : 'Could not refresh live dashboard data.')
        })
        .finally(() => {
          refreshInFlight = false
          if (refreshQueued && !cancelled) {
            refreshQueued = false
            scheduleRefresh()
          }
        })
    }
    const scheduleRefresh = () => {
      if (refreshTimer !== undefined) return
      refreshTimer = window.setTimeout(() => {
        refreshTimer = undefined
        controller?.abort()
        refreshDashboard()
      }, dashboardEventBatchMilliseconds)
    }
    const unsubscribe = subscribeToDashboardEvents(scheduleRefresh)
    return () => {
      cancelled = true
      window.clearTimeout(refreshTimer)
      controller?.abort()
      unsubscribe()
    }
  }, [applyDashboard])

  useEffect(() => {
    if (!syncActive) return
    let cancelled = false
    const poll = () => {
      void getDashboard()
        .then((response) => {
          if (cancelled) return
          applyDashboard(response)
        })
        .catch((error: unknown) => {
          if (!cancelled) setDashboardError(error instanceof Error ? error.message : 'Could not refresh sync progress.')
        })
    }
    const interval = window.setInterval(poll, 10000)
    return () => {
      cancelled = true
      window.clearInterval(interval)
    }
  }, [applyDashboard, syncActive, dashboard?.sync.id])

  useEffect(() => {
    if (!insightRequestKey) return
    const controller = new AbortController()
    const filters = { ownerId, repositoryId, actorKind, excludeDead, from: selectedFrom, to: selectedTo, ...windows }
    Promise.all([
      getInsightOverview(filters, controller.signal),
      getInsightNetwork(filters, controller.signal),
      getInsightRamps(filters, controller.signal),
    ])
      .then(([overview, network, ramps]) => {
        setOverviewResult({ key: insightRequestKey, scopeKey: insightScopeKey, data: overview })
        setNetworkResult({ key: insightRequestKey, scopeKey: insightScopeKey, data: network })
        setRampResult({ key: insightRequestKey, scopeKey: insightScopeKey, data: ramps })
        if (overview.meta.available_from && overview.meta.available_to) {
          setDateRange((current) =>
            current?.userSelected
              ? {
                  from: overview.meta.from ?? current.from,
                  to: overview.meta.to ?? current.to,
                  userSelected: true,
                }
              : {
                  from: overview.meta.from ?? overview.meta.available_from!,
                  to: overview.meta.to ?? overview.meta.available_to!,
                  userSelected: false,
                },
          )
        }
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === 'AbortError') return
        setOverviewResult((current) => current?.scopeKey === insightScopeKey ? current : undefined)
        setNetworkResult((current) => current?.scopeKey === insightScopeKey ? current : undefined)
        setRampResult((current) => current?.scopeKey === insightScopeKey ? current : undefined)
        setDashboardError(error instanceof Error ? error.message : 'Could not load Human × Agent insights.')
      })
    return () => controller.abort()
  }, [insightRequestKey, insightScopeKey, ownerId, repositoryId, actorKind, excludeDead, selectedFrom, selectedTo, windows])

  useEffect(() => {
    if (!rankingRequestKey) return
    const controller = new AbortController()
    const filters = { ownerId, repositoryId, actorKind, excludeDead, from: selectedFrom, to: selectedTo, ...windows }
    getInsightRankings(filters, rankCohort, rankMetric, controller.signal)
      .then((rankings) => setRankingResult({ key: rankingRequestKey, scopeKey: rankingScopeKey, data: rankings }))
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === 'AbortError') return
        setRankingResult((current) => current?.scopeKey === rankingScopeKey ? current : undefined)
        setDashboardError(error instanceof Error ? error.message : 'Could not load rankings.')
      })
    return () => controller.abort()
  }, [rankingRequestKey, rankingScopeKey, ownerId, repositoryId, actorKind, excludeDead, selectedFrom, selectedTo, windows, rankCohort, rankMetric])

  useEffect(() => {
    if (!contentGeneration) return
    const controller = new AbortController()
    getIdentities(controller.signal)
      .then((result) => setIdentityResult({ key: contentGeneration, data: result.identities }))
      .catch((error: unknown) => {
        if (!(error instanceof DOMException && error.name === 'AbortError')) setDashboardError(error instanceof Error ? error.message : 'Could not load identities.')
      })
    return () => controller.abort()
  }, [contentGeneration])

  const connect = async (pat: string) => {
    setSubmitting(true)
    setModalError(undefined)
    try {
      const sync = await startSync(pat)
      setDashboard((current) => ({ snapshot: current?.snapshot ?? null, sync }))
      setDashboardError(undefined)
      setModalOpen(false)
    } catch (error) {
      setModalError(error instanceof Error ? error.message : 'Could not start the sync.')
    } finally {
      setSubmitting(false)
    }
  }

  const refresh = async () => {
    setDashboardError(undefined)
    try {
      const sync = await startSync()
      setDashboard((current) => ({ snapshot: current?.snapshot ?? null, sync }))
    } catch (error) {
      setDashboardError(error instanceof Error ? error.message : 'Could not refresh repositories.')
    }
  }

  const changeActorKind = useCallback((kind?: ActorKind) => {
    setActorKind(kind)
    if (kind === 'human') { setRankCohort('humans'); setRankMetric('commits') }
    if (kind === 'agent') { setRankCohort('agents'); setRankMetric('commits') }
  }, [])

  const changeDateRange = useCallback((from: string, to: string) => {
    setDateRange({ from, to, userSelected: true })
  }, [])

  const changeRanking = useCallback((cohort: RankCohort, metric: RankMetric) => {
    setRankCohort(cohort)
    setRankMetric(metric)
  }, [])

  const changeIdentity = useCallback((key: string, update: { kind?: ActorKind; display_name?: string; canonical_key?: string; unmerge?: boolean }) => {
    void updateIdentityClassification(key, update)
      .then((result) => {
        setIdentityResult({ key: contentGeneration, data: result.identities })
        setInsightEpoch((current) => current + 1)
      })
      .catch((error: unknown) => setDashboardError(error instanceof Error ? error.message : 'Could not update identity.'))
  }, [contentGeneration])

  return (
    <main className="min-h-screen bg-slate-950 text-slate-100">
      <div className="page-glow" aria-hidden="true" />
      {dashboard?.snapshot ? (
        <DashboardView
          snapshot={dashboard.snapshot}
          sync={dashboard.sync}
          insights={{
            overview: overviewResult?.data ?? null,
            network: networkResult?.data ?? null,
            ramps: rampResult?.data ?? null,
            rankings: rankingResult?.data ?? null,
          }}
          insightsLoading={{
            overview: !overviewResult,
            network: !networkResult,
            ramps: !rampResult,
            rankings: !rankingResult,
          }}
          insightsRefreshing={{
            overview: Boolean(insightRequestKey) && Boolean(overviewResult) && overviewResult?.key !== insightRequestKey,
            network: Boolean(insightRequestKey) && Boolean(networkResult) && networkResult?.key !== insightRequestKey,
            ramps: Boolean(insightRequestKey) && Boolean(rampResult) && rampResult?.key !== insightRequestKey,
            rankings: Boolean(rankingRequestKey) && Boolean(rankingResult) && rankingResult?.key !== rankingRequestKey,
          }}
          identities={identities}
          identitiesLoading={identitiesLoading}
          ownerId={ownerId}
          repositoryId={repositoryId}
          actorKind={actorKind}
          excludeDead={excludeDead}
          dateFrom={dateRange?.from}
          dateTo={dateRange?.to}
          sessionHours={windows.sessionHours}
          adoptionDays={windows.adoptionDays}
          survivalDays={windows.survivalDays}
          rankCohort={rankCohort}
          rankMetric={rankMetric}
          onOwnerChange={setOwnerId}
          onRepositoryChange={setRepositoryId}
          onActorKindChange={changeActorKind}
          onDateRangeChange={changeDateRange}
          onWindowsChange={setWindows}
          onRankChange={changeRanking}
          onIdentityChange={changeIdentity}
          onRefresh={() => void refresh()}
          onSettings={() => {
            setModalError(undefined)
            setModalMode('settings')
            setModalOpen(true)
          }}
        />
      ) : (
        <EmptyDashboard
          loading={loadingDashboard}
          error={dashboardError}
          active={syncActive}
          message={dashboard?.sync.message}
          completed={dashboard?.sync.completed_repositories ?? 0}
          total={dashboard?.sync.total_repositories ?? 0}
          onTryAgain={() => setModalOpen(true)}
        />
      )}

      {dashboardError && dashboard?.snapshot && (
        <div role="alert" className="fixed bottom-4 left-1/2 z-40 flex -translate-x-1/2 items-center gap-2 rounded-xl border border-rose-400/20 bg-slate-900 px-4 py-3 text-sm text-rose-200 shadow-2xl">
          <TriangleAlert aria-hidden="true" className="size-4" />
          {dashboardError}
        </div>
      )}

      <TokenModal
        open={modalOpen}
        mode={modalMode}
        hasCachedData={Boolean(dashboard?.snapshot)}
        submitting={submitting}
        error={modalError}
        excludeDead={excludeDead}
        onExcludeDeadChange={(next) => {
          setExcludeDead(next)
          if (next && dashboard?.snapshot?.repositories.find((repository) => repository.id === repositoryId)?.liveness?.is_dead) {
            setRepositoryId(undefined)
          }
        }}
        onSubmit={connect}
        onViewCached={() => {
          setModalError(undefined)
          setModalOpen(false)
        }}
      />
    </main>
  )
}

function EmptyDashboard({
  loading,
  error,
  active,
  message,
  completed,
  total,
  onTryAgain,
}: {
  loading: boolean
  error?: string
  active: boolean
  message?: string
  completed: number
  total: number
  onTryAgain: () => void
}) {
  return (
    <div className="relative mx-auto grid min-h-screen max-w-2xl place-items-center px-6 py-20 text-center">
      <div>
        <div className="mx-auto grid size-16 place-items-center rounded-3xl bg-cyan-300 text-slate-950 shadow-2xl shadow-cyan-400/15">
          {active || loading
            ? <LoaderCircle aria-hidden="true" className="size-8 animate-spin" />
            : <img src="/images/github.svg" alt="" aria-hidden="true" className="size-9 object-contain" />}
        </div>
        <h1 className="mt-7 text-4xl font-semibold tracking-tight text-white">Your Git history, in motion.</h1>
        <p className="mx-auto mt-4 max-w-lg text-base leading-7 text-slate-400">
          {active
            ? message || 'Discovering repositories and building your first dashboard.'
            : error || 'Connect GitHub to build an all-time view of repositories, contributors, commits, and pull requests.'}
        </p>
        {active && total > 0 && (
          <div className="mx-auto mt-7 max-w-sm">
            <div className="mb-2 flex justify-between text-xs text-slate-500"><span>Repository progress</span><span>{completed} / {total}</span></div>
            <div className="h-1.5 overflow-hidden rounded-full bg-white/5"><div className="h-full rounded-full bg-cyan-300 transition-[width]" style={{ width: `${Math.round((completed / total) * 100)}%` }} /></div>
          </div>
        )}
        {!active && error && (
          <button type="button" onClick={onTryAgain} className="mt-7 inline-flex items-center gap-2 rounded-xl bg-cyan-300 px-5 py-2.5 text-sm font-semibold text-slate-950">
            <RefreshCw aria-hidden="true" className="size-4" /> Try again
          </button>
        )}
        {!active && !error && !loading && (
          <div className="mt-8 inline-flex items-center gap-2 text-xs text-slate-600"><Database aria-hidden="true" className="size-4" /> Persistent, local statistics cache</div>
        )}
      </div>
    </div>
  )
}

export default App
