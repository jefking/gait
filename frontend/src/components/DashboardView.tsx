import {
  Activity,
  Bot,
  CircleQuestionMark,
  FileDown,
  Fingerprint,
  GitBranch,
  LayoutDashboard,
  Network,
  RefreshCw,
  Settings,
  TrendingDown,
  TrendingUp,
  UsersRound,
  X,
} from 'lucide-react'
import { useCallback, useState } from 'react'
import type {
  ActorKind,
  InsightBundle,
  IdentitySummary,
  RankCohort,
  RankMetric,
  Snapshot,
  SyncStatus,
} from '../lib/api'
import { isSyncActive } from '../lib/api'
import { Avatar } from './Avatar'
import { DateRangeSlider } from './DateRangeSlider'
import { IdentityRegistryView } from './IdentityRegistryView'
import { MomentumChart } from './MomentumChart'
import { OwnerSelect } from './OwnerSelect'
import { RampChart } from './RampChart'
import { RankChart } from './RankChart'
import { TeamNetwork } from './TeamNetwork'

interface DashboardViewProps {
  snapshot: Snapshot
  sync: SyncStatus
  insights: { [Key in keyof InsightBundle]: InsightBundle[Key] | null }
  insightsLoading: Record<keyof InsightBundle, boolean>
  insightsRefreshing: Record<keyof InsightBundle, boolean>
  identities: IdentitySummary[]
  identitiesLoading: boolean
  ownerId?: number
  repositoryId?: number
  excludeDead: boolean
  actorKind?: ActorKind
  dateFrom?: string
  dateTo?: string
  sessionHours: number
  adoptionDays: number
  survivalDays: number
  rankCohort: RankCohort
  rankMetric: RankMetric
  onOwnerChange: (id?: number) => void
  onRepositoryChange: (id?: number) => void
  onActorKindChange: (kind?: ActorKind) => void
  onDateRangeChange: (from: string, to: string) => void
  onWindowsChange: (windows: { sessionHours: number; adoptionDays: number; survivalDays: number }) => void
  onRankChange: (cohort: RankCohort, metric: RankMetric) => void
  onIdentityChange: (key: string, update: { kind?: ActorKind; display_name?: string; canonical_key?: string; unmerge?: boolean }) => void
  onRefresh: () => void
  onSettings: () => void
}

const percent = new Intl.NumberFormat(undefined, { style: 'percent', maximumFractionDigits: 0, signDisplay: 'exceptZero' })

export function DashboardView({
  snapshot, sync, insights, insightsLoading, insightsRefreshing, identities, identitiesLoading, ownerId, repositoryId, actorKind, excludeDead, dateFrom, dateTo,
  sessionHours, adoptionDays, survivalDays, rankCohort, rankMetric,
  onOwnerChange, onRepositoryChange, onActorKindChange, onDateRangeChange, onWindowsChange, onRankChange,
  onIdentityChange, onRefresh, onSettings,
}: DashboardViewProps) {
  const [selectedIdentity, setSelectedIdentity] = useState<string>()
  const [view, setView] = useState<'dashboard' | 'identities'>('dashboard')
  const classifyIdentity = useCallback((key: string, kind: ActorKind) => onIdentityChange(key, { kind }), [onIdentityChange])
  const renameIdentity = useCallback((key: string, display_name: string) => onIdentityChange(key, { display_name }), [onIdentityChange])
  const mergeIdentity = useCallback((key: string, canonical_key: string) => onIdentityChange(key, { canonical_key }), [onIdentityChange])
  const unmergeIdentity = useCallback((key: string) => onIdentityChange(key, { unmerge: true }), [onIdentityChange])
  const unknownIdentities = identities.filter((identity) => identity.kind === 'unknown').length
  const activeView = view
  const repositories = snapshot.repositories.filter((repo) => (!ownerId || repo.owner.id === ownerId) && (!excludeDead || !repo.liveness?.is_dead))
  const meta = insights.overview?.meta
  const summary = insights.overview?.summary
  const coverage = meta?.coverage
  const period = meta?.from && meta.to ? `${meta.from} → ${meta.to}` : 'All available history'
  const attribution = `${coverage?.unknown_commits ?? 0} excluded`
  const cards = [
    { label: 'Agent participation', value: summary ? percent.format(summary.agent_participation) : '—', detail: `${coverage?.classified_commits ?? 0} classified · ${period} · ${attribution}`, icon: Bot, tone: 'text-violet-300' },
    { label: 'Observed handoff lift', value: summary?.handoff_lift === undefined ? '—' : percent.format(summary.handoff_lift), detail: `${summary?.handoff_episodes ?? 0} episodes · ${sessionHours}h windows · ${attribution}`, icon: GitBranch, tone: summary?.handoff_lift !== undefined && summary.handoff_lift >= 0 ? 'text-emerald-300' : 'text-rose-300' },
    { label: 'Quality direction', value: summary?.quality_direction === undefined ? '—' : percent.format(summary.quality_direction), detail: `Latest bucket vs prior · ${coverage?.mature_commits ?? 0} mature · ${attribution}`, icon: summary?.quality_direction !== undefined && summary.quality_direction >= 0 ? TrendingUp : TrendingDown, tone: summary?.quality_direction !== undefined && summary.quality_direction >= 0 ? 'text-emerald-300' : 'text-rose-300' },
    { label: 'Strongest pair', value: summary?.strongest_pair || '—', detail: `${summary?.strongest_pair_days ?? 0} interaction days · ${period} · ${attribution}`, icon: UsersRound, tone: 'text-cyan-300' },
  ]

  return (
    <div className="dashboard-shell mx-auto w-full max-w-[1560px] px-4 pb-20 pt-5 sm:px-6 lg:px-8">
      <header className="report-header flex flex-col justify-between gap-5 border-b border-white/8 pb-6 sm:flex-row sm:items-center">
        <div className="flex items-center gap-4"><img src="/images/gait.svg" alt="" aria-hidden="true" className="h-12 w-auto shrink-0 object-contain drop-shadow-[0_6px_16px_rgba(231,160,52,0.2)]" /><div><div className="flex items-center gap-2"><h1 className="text-xl font-semibold tracking-tight text-white">Gait</h1><span className="rounded-full border border-violet-300/15 bg-violet-300/[0.06] px-2 py-0.5 text-[10px] font-semibold uppercase tracking-widest text-violet-200">Human × Agent intelligence</span></div><p className="mt-1 text-xs text-slate-500">Default-branch engineering relationships · updated {formatDate(snapshot.generated_at)}</p></div></div>
        <div className="flex items-center justify-between gap-3 sm:justify-end"><a href={snapshot.viewer.html_url} target="_blank" rel="noreferrer" className="flex items-center gap-3 rounded-xl px-2 py-1.5 transition hover:bg-white/5"><Avatar src={snapshot.viewer.avatar_url} name={snapshot.viewer.name || snapshot.viewer.login} size="sm" /><span className="hidden text-sm font-medium text-slate-300 sm:block">{snapshot.viewer.name || snapshot.viewer.login}</span></a><div className="no-print flex gap-2"><HeaderButton label="Export PDF" onClick={() => window.print()} icon={<FileDown className="size-4" />} /><HeaderButton label="Refresh repositories" onClick={onRefresh} disabled={isSyncActive(sync)} icon={<RefreshCw className={`size-4 ${isSyncActive(sync) ? 'animate-spin' : ''}`} />} /><HeaderButton label="GitHub settings" onClick={onSettings} disabled={isSyncActive(sync)} icon={<Settings className="size-4" />} /></div></div>
      </header>

      <SyncNotification sync={sync} />

      <nav className="no-print mt-4 flex flex-wrap items-center gap-2" aria-label="Primary views">
        <button type="button" onClick={() => setView('dashboard')} className={`inline-flex min-h-11 items-center gap-2 rounded-xl border px-4 text-sm font-medium transition ${activeView === 'dashboard' ? 'border-cyan-300/30 bg-cyan-300/10 text-cyan-100' : 'border-white/8 bg-slate-900/55 text-slate-400 hover:border-white/15 hover:text-slate-200'}`}><LayoutDashboard aria-hidden="true" className="size-4" /> Insights</button>
        <button type="button" onClick={() => setView('identities')} className={`inline-flex min-h-11 items-center gap-2 rounded-xl border px-4 text-sm font-medium transition ${activeView === 'identities' ? 'border-cyan-300/30 bg-cyan-300/10 text-cyan-100' : 'border-white/8 bg-slate-900/55 text-slate-400 hover:border-white/15 hover:text-slate-200'}`}><Fingerprint aria-hidden="true" className="size-4" /> Identity registry<span className="rounded-full bg-black/20 px-2 py-0.5 text-xs tabular-nums">{identities.length}</span></button>
        {!identitiesLoading && unknownIdentities > 0 && <button type="button" onClick={() => setView('identities')} className="ml-auto inline-flex min-h-11 items-center gap-2 rounded-xl border border-amber-300/20 bg-amber-300/[0.07] px-3.5 text-left text-sm font-medium text-amber-100 transition hover:border-amber-300/35 hover:bg-amber-300/10"><CircleQuestionMark aria-hidden="true" className="size-4 shrink-0 text-amber-300" /><span>{unknownIdentities} unknown {unknownIdentities === 1 ? 'actor' : 'actors'}<span className="ml-1 font-normal text-amber-100/60">· excluded from insights</span></span></button>}
      </nav>

      {activeView === 'identities' ? (
        <IdentityRegistryView identities={identities} loading={identitiesLoading} onChange={onIdentityChange} />
      ) : <>

      <section className="no-print mt-6 rounded-3xl border border-white/8 bg-slate-900/55 p-4 sm:p-5" aria-label="Analysis controls">
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[1fr_1fr_1fr_2fr]">
          <OwnerSelect owners={snapshot.owners} value={ownerId} onChange={(next) => { onOwnerChange(next); if (repositoryId && !snapshot.repositories.some((repo) => repo.id === repositoryId && (!next || repo.owner.id === next))) onRepositoryChange(undefined) }} />
          <label className="block text-xs font-medium text-slate-500">Repository<select value={repositoryId ?? ''} onChange={(event) => onRepositoryChange(event.target.value ? Number(event.target.value) : undefined)} className="mt-1.5 w-full rounded-lg border border-white/10 bg-slate-950 px-3 py-2 text-sm text-slate-200"><option value="">All repositories</option>{repositories.map((repo) => <option key={repo.id} value={repo.id}>{repo.name}</option>)}</select></label>
          <label className="block text-xs font-medium text-slate-500">Actor type<select value={actorKind ?? ''} onChange={(event) => onActorKindChange(event.target.value ? event.target.value as ActorKind : undefined)} className="mt-1.5 w-full rounded-lg border border-white/10 bg-slate-950 px-3 py-2 text-sm text-slate-200"><option value="">Humans + agents</option><option value="human">Human participation</option><option value="agent">Agent participation</option></select></label>
          <details className="rounded-xl border border-white/8 bg-slate-950/55 px-4 py-2.5"><summary className="cursor-pointer text-xs font-medium text-slate-400">Advanced windows · {sessionHours}h sessions · {adoptionDays}d adoption · {survivalDays}d quality</summary><div className="mt-4 grid gap-4 sm:grid-cols-3"><WindowInput label="Session hours" min={1} max={168} value={sessionHours} onChange={(value) => onWindowsChange({ sessionHours:value,adoptionDays,survivalDays })} /><WindowInput label="Adoption days" min={7} max={180} value={adoptionDays} onChange={(value) => onWindowsChange({ sessionHours,adoptionDays:value,survivalDays })} /><WindowInput label="Survival days" min={7} max={180} value={survivalDays} onChange={(value) => onWindowsChange({ sessionHours,adoptionDays,survivalDays:value })} /></div></details>
        </div>
        <div className="mt-3 flex items-center justify-between gap-3 rounded-xl border border-white/8 bg-slate-950/45 px-3 py-2 text-xs text-slate-500">
          <span>{excludeDead ? 'Dead projects are excluded from every graph and ranking.' : 'Dead projects are included in analysis.'}</span>
          <button type="button" onClick={onSettings} className="shrink-0 text-cyan-300 hover:text-cyan-100">Project settings</button>
        </div>
        {meta?.available_from && meta.available_to && dateFrom && dateTo && <DateRangeSlider availableFrom={meta.available_from} availableTo={meta.available_to} from={dateFrom} to={dateTo} granularity={meta.granularity} onChange={onDateRangeChange} />}
      </section>

      {meta?.unavailable?.includes('relationship_analysis_requires_commit_events') && <div className="mt-6 rounded-2xl border border-amber-300/15 bg-amber-300/[0.05] px-4 py-3 text-sm text-amber-100/80">Relationship analysis requires a refresh so the existing commit cache can retain event-level evidence.</div>}

      {excludeDead && repositories.length === 0 && <p className="mt-6 rounded-2xl border border-dashed border-white/10 py-10 text-center text-sm text-slate-500">All repositories are excluded by project settings.</p>}

      <section aria-label="Human and agent signals" aria-busy={insightsRefreshing.overview} className="report-summary mt-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">{cards.map(({label,value,detail,icon:Icon,tone}) => <article key={label} className="report-metric metric-card rounded-2xl border border-white/8 bg-slate-900/60 p-5"><div className="flex items-start justify-between gap-3"><div className="min-w-0"><p className="text-xs font-medium uppercase tracking-wider text-slate-500">{label}</p><p className="mt-2 truncate text-2xl font-semibold tracking-tight text-white">{insightsLoading.overview ? '…' : value}</p><p className="mt-1 text-xs text-slate-500">{detail}</p></div><span className={`rounded-xl bg-white/[0.04] p-2.5 ring-1 ring-white/8 ${tone}`}><Icon className="size-5" /></span></div></article>)}</section>

      <GraphSection eyebrow="Relationships" title="Team constellation" description="Who works with whom, based on co-authorship, reviews, and short same-repository commit handoffs." icon={<Network className="size-4" />} updating={!insightsLoading.network && insightsRefreshing.network}>
        <TeamNetwork network={insights.network} loading={insightsLoading.network} selectedKey={selectedIdentity} onSelect={setSelectedIdentity} onClassify={classifyIdentity} onRename={renameIdentity} onMerge={mergeIdentity} onUnmerge={unmergeIdentity} />
      </GraphSection>

      <GraphSection eyebrow="Frequency + quality" title="Momentum over time" description="Activity mix and quality proxies share the same time brush, without combining unlike measures on one axis." icon={<Activity className="size-4" />} updating={!insightsLoading.overview && insightsRefreshing.overview}>
        <MomentumChart overview={insights.overview} loading={insightsLoading.overview} />
      </GraphSection>

      <GraphSection eyebrow="Observed association" title="Human → agent ramp-up" description={`Compare ${sessionHours}-hour handoffs and ${adoptionDays}-day repository adoption windows.`} icon={<GitBranch className="size-4" />} updating={!insightsLoading.ramps && insightsRefreshing.ramps}>
        <RampChart ramps={insights.ramps} loading={insightsLoading.ramps} />
      </GraphSection>

      <GraphSection eyebrow="Transparent ranking" title="Who leads this period?" description="Rank one measured outcome at a time—never an opaque productivity score." icon={<TrendingUp className="size-4" />} actions={<RankControls cohort={rankCohort} metric={rankMetric} onChange={onRankChange} />} updating={!insightsLoading.rankings && insightsRefreshing.rankings}>
        <RankChart rankings={insights.rankings} loading={insightsLoading.rankings} />
      </GraphSection>

      <footer className="mt-8 flex flex-wrap items-center justify-between gap-3 border-t border-white/8 pt-5 text-xs text-slate-600"><p>{coverage?.unknown_commits ?? 0} commits excluded pending actor classification · classifications are evidence-based and editable.</p><p>Observed associations are not causal claims.</p></footer>
      </>}
    </div>
  )
}

function GraphSection({ eyebrow, title, description, icon, actions, updating = false, children }: { eyebrow:string;title:string;description:string;icon:React.ReactNode;actions?:React.ReactNode;updating?:boolean;children:React.ReactNode }) { return <section aria-busy={updating} className="report-activity relative mt-7 rounded-3xl border border-white/8 bg-slate-900/55 p-4 shadow-2xl shadow-black/10 sm:p-6"><div className="mb-6 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between"><div><div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-widest text-cyan-300">{icon}{eyebrow}</div><h2 className="mt-2 text-xl font-semibold text-white">{title}</h2><p className="mt-1 max-w-3xl text-sm text-slate-500">{description}</p></div><div className="flex flex-wrap items-center gap-3">{updating && <span role="status" className="inline-flex items-center gap-1.5 text-xs text-cyan-200/70"><RefreshCw aria-hidden="true" className="size-3.5 animate-spin" /> Updating</span>}{actions}</div></div>{children}</section> }

function RankControls({ cohort, metric, onChange }: { cohort:RankCohort;metric:RankMetric;onChange:(cohort:RankCohort,metric:RankMetric)=>void }) {
  const pair = cohort === 'human_agent' || cohort === 'human_human'
  const metrics: [RankMetric,string][] = pair ? [['interaction_days','Interaction days'],['handoffs','Handoffs'],['review_interactions','Reviews']] : [['commits','Commits'],['pull_requests','Pull requests'],['retained_line_rate','Retained lines'],['revert_rate','Revert rate']]
  return <div className="no-print flex flex-wrap gap-2"><select aria-label="Ranking cohort" value={cohort} onChange={(event) => { const next=event.target.value as RankCohort;onChange(next,next==='humans'||next==='agents'?'commits':'interaction_days') }} className="rounded-lg border border-white/10 bg-slate-950 px-3 py-2 text-xs text-slate-200"><option value="agents">Agents</option><option value="humans">Humans</option><option value="human_agent">Human + Agent</option><option value="human_human">Human + Human</option></select><select aria-label="Ranking metric" value={metric} onChange={(event) => onChange(cohort,event.target.value as RankMetric)} className="rounded-lg border border-white/10 bg-slate-950 px-3 py-2 text-xs text-slate-200">{metrics.map(([value,label]) => <option key={value} value={value}>{label}</option>)}</select></div>
}

function WindowInput({label,min,max,value,onChange}:{label:string;min:number;max:number;value:number;onChange:(value:number)=>void}) { return <label className="text-xs text-slate-500">{label}<input type="number" min={min} max={max} value={value} onChange={(event)=>onChange(Math.min(max,Math.max(min,Number(event.target.value))))} className="mt-1.5 w-full rounded-lg border border-white/10 bg-slate-900 px-3 py-2 text-sm text-slate-200" /></label> }
function HeaderButton({label,onClick,icon,disabled}:{label:string;onClick:()=>void;icon:React.ReactNode;disabled?:boolean}) { return <button type="button" onClick={onClick} disabled={disabled} aria-label={label} title={label} className="grid size-10 place-items-center rounded-xl border border-white/10 bg-white/5 text-slate-200 transition hover:border-cyan-300/30 hover:bg-cyan-300/5 disabled:opacity-50">{icon}</button> }

export function SyncNotification({ sync }: { sync: SyncStatus }) {
  const [dismissedFailure,setDismissedFailure]=useState<string>();const active=isSyncActive(sync);const failureKey=`${sync.id??'sync'}:${sync.state}`
  if(!active&&sync.state!=='failed')return null;if(sync.state==='failed'&&dismissedFailure===failureKey)return null
  const progress=sync.total_repositories>0?Math.round(sync.completed_repositories/sync.total_repositories*100):0;const workflow=sync.current_workflows?.[0]
  return <aside aria-live="polite" aria-atomic="true" className={`no-print fixed right-4 top-4 z-40 w-[min(26rem,calc(100vw-2rem))] overflow-hidden rounded-2xl border bg-slate-900/95 shadow-2xl backdrop-blur-xl ${sync.state==='failed'?'border-rose-400/25 text-rose-100':'border-cyan-300/25 text-cyan-100'}`}><div className="flex items-start gap-3 px-4 py-3.5"><RefreshCw className={`mt-0.5 size-4 shrink-0 ${active?'animate-spin':''}`} /><div className="min-w-0 flex-1"><div className="flex justify-between gap-2"><p className="text-sm font-medium">{sync.message??sync.state}</p>{active&&sync.total_repositories>0&&<span className="text-xs tabular-nums opacity-75">{sync.completed_repositories} / {sync.total_repositories} repositories</span>}</div>{workflow&&<p className="mt-1 truncate text-xs opacity-65">{workflow.message}</p>}</div>{sync.state==='failed'&&<button type="button" onClick={()=>setDismissedFailure(failureKey)} aria-label="Dismiss sync notification" className="rounded-lg p-1.5 opacity-60 hover:bg-white/10"><X className="size-4" /></button>}</div>{active&&sync.total_repositories>0&&<div className="h-0.5 bg-white/5"><div className="h-full bg-cyan-300" style={{width:`${progress}%`}} /></div>}</aside>
}

function formatDate(value:string){return new Intl.DateTimeFormat(undefined,{dateStyle:'medium',timeStyle:'short'}).format(new Date(value))}
