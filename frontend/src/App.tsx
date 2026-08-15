import { Database, LoaderCircle, RefreshCw, TriangleAlert } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { DashboardView } from './components/DashboardView'
import { TokenModal } from './components/TokenModal'
import {
  getDashboard,
  getIdentities,
  getInsightDelivery,
  isSyncActive,
  startSync,
  subscribeToDashboardEvents,
  type DashboardResponse,
  type ActorKind,
  type DeliveryResponse,
  type IdentitySummary,
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
  const [excludeDead, setExcludeDead] = useState(false)
  const [dateRange, setDateRange] = useState<{ from: string; to: string; userSelected: boolean }>()
  const [insightEpoch, setInsightEpoch] = useState(0)
  const [identityResult, setIdentityResult] = useState<KeyedResult<IdentitySummary[]>>()
  const [deliveryResult, setDeliveryResult] = useState<KeyedResult<DeliveryResponse>>()

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
  const selectedFrom = dateRange?.from
  const selectedTo = dateRange?.to
  const insightScopeKey = dashboard?.snapshot
    ? [ownerId ?? '', excludeDead, selectedFrom ?? '', selectedTo ?? ''].join(':')
    : ''
  const insightRequestKey = insightScopeKey && contentGeneration
    ? `${contentGeneration}:${insightEpoch}:${insightScopeKey}`
    : ''
  const identities = identityResult?.scopeKey === insightScopeKey ? identityResult.data : []
  const identitiesLoading = Boolean(snapshotGeneratedAt) && identityResult?.scopeKey !== insightScopeKey

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
    const filters = { organizationId: ownerId, excludeDead, from: selectedFrom, to: selectedTo }
    Promise.all([
      getInsightDelivery(filters, controller.signal),
      getIdentities(filters, controller.signal),
    ])
      .then(([delivery, identityResponse]) => {
        setDeliveryResult({ key: insightRequestKey, scopeKey: insightScopeKey, data: delivery })
        setIdentityResult({ key: insightRequestKey, scopeKey: insightScopeKey, data: identityResponse.identities })
        if (delivery.meta.available_from && delivery.meta.available_to) {
          setDateRange((current) =>
            current?.userSelected
              ? {
                  from: delivery.meta.from ?? current.from,
                  to: delivery.meta.available_to!,
                  userSelected: true,
                }
              : {
                  from: delivery.meta.from ?? delivery.meta.available_from!,
                  to: delivery.meta.available_to!,
                  userSelected: false,
                },
          )
        }
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === 'AbortError') return
        setDeliveryResult((current) => current?.scopeKey === insightScopeKey ? current : undefined)
        setIdentityResult((current) => current?.scopeKey === insightScopeKey ? current : undefined)
        setDashboardError(error instanceof Error ? error.message : 'Could not load team delivery evidence.')
      })
    return () => controller.abort()
  }, [insightRequestKey, insightScopeKey, ownerId, excludeDead, selectedFrom, selectedTo])

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

  const changeDateRange = useCallback((from: string, to: string) => {
    setDateRange({ from, to, userSelected: true })
  }, [])

  const changeOrganization = useCallback((id?: number) => {
    setOwnerId(id)
    setDateRange(undefined)
  }, [])

  const changeIdentity = useCallback((key: string, update: { kind?: ActorKind; display_name?: string; canonical_key?: string; unmerge?: boolean }) => {
    void updateIdentityClassification(key, update)
      .then(() => setInsightEpoch((current) => current + 1))
      .catch((error: unknown) => setDashboardError(error instanceof Error ? error.message : 'Could not update identity.'))
  }, [])

  return (
    <main className="bg-slate-950 text-slate-100">
      <div className="page-glow" aria-hidden="true" />
      {dashboard?.snapshot ? (
        <DashboardView
          snapshot={dashboard.snapshot}
          sync={dashboard.sync}
          delivery={deliveryResult?.scopeKey === insightScopeKey ? deliveryResult.data : null}
          deliveryLoading={deliveryResult?.scopeKey !== insightScopeKey}
          refreshing={Boolean(insightRequestKey) && deliveryResult?.key !== insightRequestKey}
          identities={identities}
          identitiesLoading={identitiesLoading}
          ownerId={ownerId}
          excludeDead={excludeDead}
          dateFrom={dateRange?.from}
          dateTo={dateRange?.to}
          onOwnerChange={changeOrganization}
          onDateRangeChange={changeDateRange}
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
