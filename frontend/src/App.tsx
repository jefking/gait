import { Activity, Database, LoaderCircle, RefreshCw, TriangleAlert } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { DashboardView } from './components/DashboardView'
import { TokenModal } from './components/TokenModal'
import {
  getActivity,
  getDashboard,
  isSyncActive,
  startSync,
  subscribeToDashboardEvents,
  type ActivityGroup,
  type ActivityMetric,
  type ActivityResponse,
  type DashboardResponse,
} from './lib/api'

function App() {
  const [dashboard, setDashboard] = useState<DashboardResponse | null>(null)
  const [loadingDashboard, setLoadingDashboard] = useState(true)
  const [dashboardError, setDashboardError] = useState<string>()
  const [modalOpen, setModalOpen] = useState(false)
  const [modalMode, setModalMode] = useState<'connect' | 'settings'>('connect')
  const [modalError, setModalError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const [groupBy, setGroupBy] = useState<ActivityGroup>('owner')
  const [metric, setMetric] = useState<ActivityMetric>('commits')
  const [ownerId, setOwnerId] = useState<number>()
  const [repositoryId, setRepositoryId] = useState<number>()
  const [excludeDead, setExcludeDead] = useState(false)
  const [dateRange, setDateRange] = useState<{ from: string; to: string; userSelected: boolean }>()
  const [activityResult, setActivityResult] = useState<{
    key: string
    data: ActivityResponse
  }>()

  const applyDashboard = useCallback((response: DashboardResponse) => {
    setDashboard(response)
    setDashboardError(undefined)
    if (response.sync.state === 'failed') {
      setModalMode('connect')
      setModalError(response.sync.message || 'The sync failed.')
      setModalOpen(true)
    } else if (!response.snapshot && !isSyncActive(response.sync)) {
      setModalMode('connect')
      setModalOpen(true)
    }
  }, [])

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
  const selectedFrom = dateRange?.userSelected ? dateRange.from : undefined
  const selectedTo = dateRange?.userSelected ? dateRange.to : undefined
  const activityRequestKey = dashboard?.snapshot
    ? [dashboard.snapshot.generated_at, groupBy, metric, ownerId ?? '', repositoryId ?? '', excludeDead, selectedFrom ?? '', selectedTo ?? ''].join(':')
    : ''
  const currentActivity = activityResult?.key === activityRequestKey ? activityResult.data : null

  useEffect(() => {
    let cancelled = false
    let refreshTimer: number | undefined
    let controller: AbortController | undefined
    const unsubscribe = subscribeToDashboardEvents(() => {
      window.clearTimeout(refreshTimer)
      refreshTimer = window.setTimeout(() => {
        controller?.abort()
        controller = new AbortController()
        void getDashboard(controller.signal)
          .then((response) => {
            if (!cancelled) applyDashboard(response)
          })
          .catch((error: unknown) => {
            if (cancelled || (error instanceof DOMException && error.name === 'AbortError')) return
            setDashboardError(error instanceof Error ? error.message : 'Could not refresh live dashboard data.')
          })
      }, 75)
    })
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
    if (!activityRequestKey) return
    const controller = new AbortController()
    getActivity(
      { groupBy, metric, ownerId, repositoryId, excludeDead, from: selectedFrom, to: selectedTo },
      controller.signal,
    )
      .then((data) => {
        setActivityResult({ key: activityRequestKey, data })
        if (data.available_from && data.available_to) {
          setDateRange((current) =>
            current?.userSelected
              ? {
                  from: data.from ?? current.from,
                  to: data.to ?? current.to,
                  userSelected: true,
                }
              : {
                  from: data.available_from!,
                  to: data.available_to!,
                  userSelected: false,
                },
          )
        }
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === 'AbortError') return
        setDashboardError(error instanceof Error ? error.message : 'Could not load activity history.')
      })
    return () => controller.abort()
  }, [activityRequestKey, groupBy, metric, ownerId, repositoryId, excludeDead, selectedFrom, selectedTo])

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

  return (
    <main className="min-h-screen bg-slate-950 text-slate-100">
      <div className="page-glow" aria-hidden="true" />
      {dashboard?.snapshot ? (
        <DashboardView
          snapshot={dashboard.snapshot}
          sync={dashboard.sync}
          activity={currentActivity}
          activityLoading={!currentActivity}
          groupBy={groupBy}
          metric={metric}
          ownerId={ownerId}
          repositoryId={repositoryId}
          excludeDead={excludeDead}
          dateFrom={dateRange?.from}
          dateTo={dateRange?.to}
          onGroupByChange={setGroupBy}
          onMetricChange={setMetric}
          onOwnerChange={setOwnerId}
          onRepositoryChange={setRepositoryId}
          onDateRangeChange={(from, to) => setDateRange({ from, to, userSelected: true })}
          onRefresh={() => {
            setModalError(undefined)
            setModalMode('settings')
            setModalOpen(true)
          }}
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
          {active || loading ? <LoaderCircle aria-hidden="true" className="size-8 animate-spin" /> : <Activity aria-hidden="true" className="size-8" />}
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
